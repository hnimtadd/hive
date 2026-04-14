package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/bee/system"
	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
	toolSystem "github.com/hnimtadd/hive/internal/tools/system"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
)

// Pool manages a set of worker goroutines that execute tasks from the queue.
type Pool struct {
	size     int
	queue    queue.Queue
	storage  storage.Storage
	channels *channel.Manager
	registry registry.Registry
	provider llm.Provider
	cfg      *config.Config

	workers sync.WaitGroup
	done    chan struct{}
	cancel  context.CancelFunc
	ctx     context.Context
}

// NewPool creates a new worker pool.
func NewPool(
	size int,
	q queue.Queue,
	store storage.Storage,
	channels *channel.Manager,
	reg registry.Registry,
	provider llm.Provider,
	cfg *config.Config,
) *Pool {
	return &Pool{
		size:     size,
		queue:    q,
		storage:  store,
		channels: channels,
		registry: reg,
		provider: provider,
		cfg:      cfg,
		done:     make(chan struct{}),
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

	log := slog.With(slog.Int("worker_id", id))
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
			if p.ctx.Err() != nil {
				return // Context cancelled, shut down
			}
			log.Error("failed to dequeue task", slog.Any("error", err))
			continue
		}

		log.Info("processing task", slog.String("task_id", task.ID))

		// Execute task with recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("worker panic recovered",
						slog.String("task_id", task.ID),
						slog.Any("panic", r),
					)
					// Mark task as failed
					task.Status = types.TaskStatusFailed
					task.Messages = append(task.Messages, types.NewMessage(types.RoleAssistant, fmt.Sprintf("Worker panic: %v", r)))
					_ = p.storage.Update(task)
					ch := p.channels.ForTask(task.ID)
					ch.OutputCh <- agentv1.NewExecuteTaskResponseErr(fmt.Sprintf("Worker panic: %v", r))
					close(ch.DoneCh)
					p.channels.Cleanup(task.ID)
				}
			}()

			p.processTask(task)
		}()
	}
}

// processTask executes a single task using the supervisor (Queen Bee).
func (p *Pool) processTask(task *types.HiveTask) {
	log := slog.With(slog.String("task_id", task.ID))
	ch := p.channels.ForTask(task.ID)
	defer p.channels.Cleanup(task.ID)
	defer close(ch.DoneCh)

	// Create supervisor
	supervisor, err := p.createSupervisor(task.ID)
	if err != nil {
		log.Error("failed to create supervisor", slog.Any("error", err))
		task.Status = types.TaskStatusFailed
		_ = p.storage.Update(task)
		ch.OutputCh <- agentv1.NewExecuteTaskResponseErr(fmt.Sprintf("Failed to create supervisor: %v", err))
		return
	}

	// Execute supervisor loop
	for {
		select {
		case <-p.ctx.Done():
			log.Info("context cancelled during execution")
			task.Status = types.TaskStatusFailed
			_ = p.storage.Update(task)
			return

		case req := <-ch.InputCh:
			// Handle feedback from client
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

		default:
			// Execute supervisor iteration
			output, err := supervisor.Execute(p.ctx, task)
			if err != nil {
				log.Error("supervisor execution failed", slog.Any("error", err))
				task.Status = types.TaskStatusFailed
				task.Messages = append(task.Messages, types.Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Supervisor error: %v", err),
				})
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
				// Don't return, wait for feedback via InputCh
				continue

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
func (p *Pool) createSupervisor(taskID string) (system.QueenBee, error) {
	// Get supervisor persona from registry
	persona := p.buildSupervisorPersona()

	// Create delegate and explore tools
	delegateTool, err := toolSystem.DelegateTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create delegate tool: %w", err)
	}
	exploreTool, err := toolSystem.ExploreTool(p.provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create explore tool: %w", err)
	}

	// Use configured default timeout, capped at max timeout
	timeout := p.cfg.Server.MaxTimeout

	config := &bee.Config{
		ID:           taskID,
		Persona:      persona,
		MaxSteps:     10,
		TimeoutInSec: int(timeout.Seconds()),
		ModelPool:    p.provider.ModelPool(llm.TierSmart),
		Tools:        []tool.InvokableTool{delegateTool, exploreTool},
	}

	return system.NewQueenBee(config)
}

// buildSupervisorPersona builds the system prompt for the supervisor.
// This is copied from server.go - should be extracted to a shared function.
func (p *Pool) buildSupervisorPersona() string {
	agents := p.registry.ListAgents()
	persona := `
Role: You are the Central Orchestrator for a multi-agent swarm. Your goal is to navigate a complex task to completion by delegating to specialized workers.

Core Responsibilities:
	- Analyze State: Review the task's "message" field which contains the full conversation history, including your previous progress updates and any user feedback. Identify what has been achieved and what is still missing.
    - Prevent Redundancy: If a supervisee has already failed at a specific approach, do not assign them the same task again without new instructions.
    - Evaluate Capabilities: Match the requirements of the next step against the specific tools and expertise of the available agents.
	- Delegate and coordinate: Use available tools to delegate work to specialized agents.
	- Context Awareness: Always check the "message" field in the task to see what was previously accomplished and what the user has said. This helps you avoid repeating work or asking the same questions.

Status Selection Guidelines - Choose the appropriate status for each response:

	1. "in_progress": Use this when you completed one execution cycle but need to continue in the next cycle.
	   - You delegated to an agent and received results, but need to delegate to another agent or do more work
	   - You gathered some information but need additional steps to complete the task
	   - You made progress toward the goal but it's not yet complete
	   - Set "content" to describe what you just accomplished (e.g., "Received search results from agent X, now analyzing...")
	   - The system will immediately call you again to continue - your next invocation will have access to the tool results from this cycle
	   - DO NOT use this when you need user input - use "paused" instead

	2. "paused": Use this ONLY when you need information or clarification from the user before you can proceed.
	   - The task requirements are ambiguous and you cannot proceed without clarification
	   - You need the user to make a decision between multiple valid approaches
	   - You require additional context that only the user can provide (not available through any agent)
	   - Set "content" to your question for the user
	   - The system will WAIT for user feedback, then call you again with their response

	3. "completed": Use this when the user's goal is fully achieved.
	   - All task requirements have been met and no further work is needed
	   - Set "content" to a summary of what was accomplished and the final results

	4. "failed": Use this when the task cannot be completed.
	   - Available agents lack the necessary capabilities to fulfill the request
	   - A logical dead-end is reached and there's no path forward
	   - Set "content" to explain why the task cannot be completed

Constraint: Do not perform the task yourself. Your only tools are delegation and synthesis.
`
	_ = agents // TODO: Include agent descriptions in persona
	return persona
}
