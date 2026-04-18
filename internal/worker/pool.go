package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/hnimtadd/hive/internal/bee/queen"
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/middleware/system"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/config"
	context_pkg "github.com/hnimtadd/hive/pkg/context"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
)

// Pool manages a set of worker goroutines that execute tasks from the queue.
type Pool struct {
	size          int
	queue         queue.Queue
	storage       storage.Storage
	channels      *channel.Manager
	registry      registry.Registry
	provider      llm.Provider
	sessionLogger *observability.SessionLogger
	cfg           *config.Config

	workers sync.WaitGroup
	done    chan struct{}
	cancel  context.CancelFunc
	ctx     context.Context
	stopped atomic.Bool
}

// NewPool creates a new worker pool.
func NewPool(
	size int,
	q queue.Queue,
	store storage.Storage,
	channels *channel.Manager,
	reg registry.Registry,
	provider llm.Provider,
	sessionLogger *observability.SessionLogger,
	cfg *config.Config,
) *Pool {
	return &Pool{
		size:          size,
		queue:         q,
		storage:       store,
		channels:      channels,
		registry:      reg,
		provider:      provider,
		sessionLogger: sessionLogger,
		cfg:           cfg,
		done:          make(chan struct{}),
	}
}

// Start launches worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.ctx = ctx
	p.cancel = cancel

	for i := range p.size {
		p.workers.Add(1)
		go p.worker(i)
	}
}

// Stop gracefully shuts down all workers.
func (p *Pool) Stop() {
	if !p.stopped.CompareAndSwap(false, true) {
		return // Already stopped
	}

	if p.cancel != nil {
		p.cancel()
	}
	p.workers.Wait()
	close(p.done)
}

// Done returns a channel that is closed when all workers have stopped.
func (p *Pool) Done() <-chan struct{} {
	return p.done
}

// worker is the main worker loop that pulls tasks from the queue and executes them.
func (p *Pool) worker(id int) {
	defer p.workers.Done()

	log := slog.Default().With(slog.Int("worker_id", id))
	log.Info("worker started")

	for {
		select {
		case <-p.ctx.Done():
			log.Info("worker stopping")
			return
		default:
		}

		// Pull task from queue
		task, err := p.queue.Dequeue(p.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, queue.ErrQueueClosed) {
				return // Context cancelled or queue closed, shut down
			}
			log.Error("failed to dequeue task", slog.Any("error", err))
			continue
		}

		log.Info("processing task", slog.String("task_id", task.ID))
		// Execute task with retry logic
		p.executeWithRetry(task)
	}
}

// processTask executes a single task using the supervisor (Queen Bee).
func (p *Pool) processTask(task *types.HiveTask) {
	log := slog.Default().With(slog.String("task_id", task.ID))
	ch := p.channels.ForTask(task.ID)

	eventMW, eventCh := system.EventStreamMiddleware()
	traceMW := observability.NewTraceMiddleware(p.sessionLogger)
	ctx := middleware.ContextWithMiddleware(task.Context, middleware.JointMiddleware(eventMW, traceMW))

	// Inject context budget for context management
	budget := context_pkg.NewContextBudget(p.cfg.AI.Context)
	ctx = context_pkg.ContextWithBudget(ctx, budget)

	// Create supervisor
	supervisor, err := p.createSupervisor(task.ID)
	if err != nil {
		log.Error("worker: failed to create supervisor", slog.Any("error", err))
		task.Status = types.TaskStatusFailed
		_ = p.storage.Update(task)
		ch.OutputCh <- agentv1.NewExecuteTaskResponseErr(fmt.Sprintf("Failed to create supervisor: %v", err))
		return
	}

	go forwardEvent(ctx, ch, eventCh)

	// Execute supervisor loop
	for {
		select {
		case <-ctx.Done():
			log.Info("context cancelled during execution")
			task.Status = types.TaskStatusFailed
			_ = p.storage.Update(task)
			return

		case req := <-ch.InputCh:
			// Handle feedback from client
			if fb := req.GetFeedback(); fb != nil {
				task.Messages = append(task.Messages, types.NewMessage(types.RoleUser, fb.GetFeedback()))
				log.Info("received feedback", slog.String("feedback", fb.GetFeedback()))
				continue // Wait for next iteration to execute supervisor with new context
			}
			if req.GetCancel() != nil {
				log.Info("task cancelled by user")
				task.Status = types.TaskStatusFailed
				task.Messages = append(task.Messages, types.NewMessage(types.RoleAssistant, "Task cancelled by user"))
				_ = p.storage.Update(task)
				ch.OutputCh <- agentv1.NewExecuteTaskResponseErr("Task cancelled by user")
				return
			}
			continue // Unknown request type, wait for next input

		default:
			// Execute supervisor iteration
			var output *queen.QueenOutput
			output, err = supervisor.Execute(ctx, task)
			if err != nil {
				log.Error("supervisor execution failed", slog.Any("error", err))
				task.Status = types.TaskStatusFailed
				task.Messages = append(task.Messages, types.NewMessage(types.RoleAssistant, fmt.Sprintf("Supervisor error: %v", err)))
				_ = p.storage.Update(task)
				ch.OutputCh <- agentv1.NewExecuteTaskResponseErr(fmt.Sprintf("Supervisor error: %v", err))
				return
			}

			// Handle output based on status
			task.Status = output.Status
			task.Messages = append(task.Messages, types.NewMessage(types.RoleAssistant, output.Content))

			// Persist state
			_ = p.storage.Update(task)

			// Send update to client
			switch output.Status {
			case types.TaskStatusCompleted:
				ch.OutputCh <- agentv1.NewExecuteTaskResponseSuccess(utils.SanitizeUTF8(output.Content))
				log.Info("task completed")
				return

			case types.TaskStatusFailed:
				ch.OutputCh <- agentv1.NewExecuteTaskResponseErr(utils.SanitizeUTF8(output.Content))
				log.Info("task failed")
				return

			case types.TaskStatusPaused:
				ch.OutputCh <- agentv1.NewExecuteTaskResponseFeedback(utils.SanitizeUTF8(output.Content))
				log.Info("task paused, waiting for feedback")
				// Wait for feedback before continuing to execute supervisor
				select {
				case <-ctx.Done():
					log.Info("context cancelled while waiting for feedback")
					continue
				case req := <-ch.InputCh:
					if fb := req.GetFeedback(); fb != nil {
						task.Messages = append(task.Messages, types.NewMessage(types.RoleUser, fb.GetFeedback()))
						log.Info("received feedback", slog.String("feedback", fb.GetFeedback()))
					}
					if req.GetCancel() != nil {
						log.Info("task cancelled by user")
						task.Status = types.TaskStatusFailed
						task.Messages = append(task.Messages, types.NewMessage(types.RoleAssistant, "Task cancelled by user"))
						_ = p.storage.Update(task)
						ch.OutputCh <- agentv1.NewExecuteTaskResponseErr("Task cancelled by user")
						return
					}
				}

			case types.TaskStatusInProgress:
				ch.OutputCh <- agentv1.NewExecuteTaskResponseUpdate(
					utils.SanitizeUTF8(string(output.Status)),
					utils.SanitizeUTF8(fmt.Sprintf("%s-next: %s", output.Content, output.NextAction)),
				)
				log.Info("task in progress")
				// Continue to next iteration
			}
		}
	}
}

// createSupervisor creates a new Queen Bee supervisor for a task.
func (p *Pool) createSupervisor(taskID string) (queen.QueenBee, error) {
	return queen.NewQueenBee(taskID, 10, p.registry, p.cfg.Server.MaxTimeout, p.provider)
}

// executeWithRetry executes a task with retry support.
// It runs the task and schedules retry via queue if task is not terminal.
func (p *Pool) executeWithRetry(task *types.HiveTask) {
	// Run task with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("worker panic recovered",
					slog.String("task_id", task.ID),
					slog.Any("panic", r),
				)
				task.Status = types.TaskStatusFailed
				task.Messages = append(task.Messages, types.NewMessage(types.RoleAssistant, fmt.Sprintf("Worker panic: %v", r)))
				_ = p.storage.Update(task)
				ch := p.channels.ForTask(task.ID)
				ch.OutputCh <- agentv1.NewExecuteTaskResponseErr(fmt.Sprintf("Worker panic: %v", r))
				p.channels.Cleanup(task.ID)
			}
		}()

		p.processTask(task)
	}()

	// If task is not terminal, schedule retry via queue
	if !task.Status.IsTerminal() {
		_ = p.queue.ScheduleRetry(p.ctx, task)
	} else {
		p.channels.Cleanup(task.ID)
	}
}

func forwardEvent(ctx context.Context, ch *channel.TaskChannels, eventCh <-chan system.ExecutionEvent) {
	logger := slog.Default()
	for {
		select {
		case <-ctx.Done():
		case event := <-eventCh:
			switch event.Type {
			case system.EventTypeToolCallStart:
				logger.DebugContext(ctx, "receive tool call event")
				status := fmt.Sprintf("call_id: %s, agent_id: %s, tool_name: %s, input: %s", event.ToolReq.CallID, event.ToolReq.AgentID, event.ToolReq.ToolName, event.ToolReq.Arguments)
				ch.OutputCh <- agentv1.NewExecuteTaskResponseUpdate("tool_start", utils.SanitizeUTF8(status))

			case system.EventTypeToolCallFinish:
				logger.DebugContext(ctx, "receive tool finish event")
				toolResp := event.ToolResp
				var status string
				if toolResp.Succeed {
					status = fmt.Sprintf("call_id: %s, output: %s", toolResp.CallID, toolResp.Output)
				} else {
					status = fmt.Sprintf("call_id: %s, error: %s", toolResp.CallID, toolResp.Error)
				}
				ch.OutputCh <- agentv1.NewExecuteTaskResponseUpdate("tool_response", utils.SanitizeUTF8(status))

			case system.EventTypeLLMRequestStart:
				logger.DebugContext(ctx, "receive llm start event")
				llmReq := event.Req
				status := fmt.Sprintf("agent_id: %s, input: %s", llmReq.AgentID, llmReq.Input)
				ch.OutputCh <- agentv1.NewExecuteTaskResponseUpdate("llm_start", utils.SanitizeUTF8(status))

			case system.EventTypeLLMRequestFinish:
				logger.DebugContext(ctx, "receive llm finish event")
				llmResp := event.Resp
				status := fmt.Sprintf("agent_id: %s, finish_reason: %s, token_used: %d", llmResp.AgentID, llmResp.FinishReason, llmResp.TokenUsed.TotalTokens)
				ch.OutputCh <- agentv1.NewExecuteTaskResponseUpdate("llm_response", utils.SanitizeUTF8(status))
			}
		}
	}
}
