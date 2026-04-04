package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"net"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/mapper"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type HiveServer struct {
	agentv1.AgentServiceServer

	registry   bee.Registry
	config     *config.Config
	storage    storage.TaskStorage
	grpcServer *grpc.Server

	// Supervisor configuration for creating per-request supervisors
	supervisorConfig  *bee.Config
	supervisorPersona string
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(cfg *config.Config, llm model.ToolCallingChatModel, registry bee.Registry) (*HiveServer, error) {
	persona, err := getSupervisorPersona(registry)
	if err != nil {
		return nil, err
	}
	taskStorage, err := storage.NewLocalStorage(storage.Options{
		Storage: cfg.Tasks.Storage,
	})
	if err != nil {
		return nil, err
	}

	// Use configured default timeout, capped at max timeout
	timeout := cfg.Server.MaxTimeout

	// Store supervisor configuration for creating per-request supervisors
	supervisorConfig := &bee.Config{
		Persona:      persona,
		MaxSteps:     3,
		TimeoutInSec: int(timeout.Seconds()),
		LLM:          llm,
		Tools:        []tool.InvokableTool{bee.DelegateTool(registry)},
	}

	return &HiveServer{
		registry:          registry,
		config:            cfg,
		storage:           taskStorage,
		supervisorConfig:  supervisorConfig,
		supervisorPersona: persona,
	}, nil
}

func (s *HiveServer) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create gRPC server with timeout and tracing interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			timeoutUnaryInterceptor(s.config.Server.MaxTimeout),
			trace.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			timeoutStreamInterceptor(s.config.Server.MaxTimeout),
			trace.StreamServerInterceptor(),
		),
	)
	agentv1.RegisterAgentServiceServer(grpcServer, s)
	s.grpcServer = grpcServer

	log.Printf("HiveServer starting on %s with max request timeout %s", addr, s.config.Server.MaxTimeout)

	if err = grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (s *HiveServer) Stop() {
	if s.grpcServer == nil {
		return
	}

	// Use configured graceful shutdown timeout
	timeout := s.config.Server.GracefulShutdownTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Attempt graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Graceful shutdown completed")
	case <-ctx.Done():
		log.Println("Graceful shutdown timeout exceeded, forcing stop")
		s.grpcServer.Stop()
	}
}

// timeoutUnaryInterceptor adds timeout to unary RPCs.
func timeoutUnaryInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

// timeoutStreamInterceptor adds timeout to streaming RPCs.
func timeoutStreamInterceptor(timeout time.Duration) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, cancel := context.WithTimeout(stream.Context(), timeout)
		defer cancel()

		// Wrap the stream with the new context
		wrapped := &timeoutServerStream{
			ServerStream: stream,
			ctx:          ctx,
		}
		return handler(srv, wrapped)
	}
}

type timeoutServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s *timeoutServerStream) Context() context.Context {
	return s.ctx
}

func (s *HiveServer) ExecuteTask(srv grpc.BidiStreamingServer[agentv1.ClientMessage, agentv1.ServerMessage]) error {
	// Use request context which includes the timeout from interceptor
	ctx := srv.Context()
	logger := trace.Logger(ctx)
	msg, err := srv.Recv()
	if err != nil {
		logger.Error("failed to recevie initial message", slog.Any("error", err))
		return err
	}
	req := msg.GetRequest()
	if req == nil {
		logger.Error("first message must be a task request")
		return errors.New("first message must be a task request")
	}

	// Use configured timeout (could be extended to extract from request metadata)
	timeout := s.config.Tasks.Timeout

	task := types.NewHiveTask(req.GetGlobalGoal())
	maps.Copy(task.Artifacts, req.GetInitialArtifacts())

	// Create initial task record
	if err = s.storage.Add(task); err != nil {
		logger.ErrorContext(ctx, "failed to create initial task record", slog.String("reason", err.Error()))
		return fmt.Errorf("failed to store task: %w", err)
	}

	// Ensure final state is persisted
	defer func() {
		if err = s.storage.Update(task); err != nil {
			logger.ErrorContext(ctx, "failed to update final task state", slog.String("reason", err.Error()))
		}
	}()

	logger.Info("task created", slog.String("task_id", task.ID), slog.String("goal", task.Goal), slog.Int("artifact_count", len(task.Artifacts)))

	// Create tool event channel and streaming middleware
	toolEventCh := make(chan *react.ToolExecutionEvent, 100)
	streamingMW := react.EventStreamingMiddleware(toolEventCh)
	ctx = react.ContextWithToolMiddleware(ctx, streamingMW)

	// Create supervisor with streaming middleware for this request
	supervisorConfig := *s.supervisorConfig // Copy config
	supervisorConfig.ID = uuid.New().String()
	supervisor, err := bee.NewSupervisorBee(&supervisorConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create supervisor", slog.Any("error", err))
		return fmt.Errorf("failed to create supervisor: %w", err)
	}

	// Start goroutine to forward tool events to client
	eventsDone := make(chan struct{})
	go s.forwardToolEvents(ctx, srv, toolEventCh, eventsDone)

	// Ensure proper cleanup
	defer func() {
		close(toolEventCh) // Close channel to stop forwarder
		<-eventsDone       // Wait for forwarder to finish
	}()

loop:
	for {
		var output *bee.SupervisorOutput

		// Create a timeout context for each supervisor execution iteration
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		output, err = supervisor.Execute(execCtx, task)
		cancel() // Release context resources immediately
		if err != nil {
			// Check for timeout errors
			if ctx.Err() == context.DeadlineExceeded || execCtx.Err() == context.DeadlineExceeded {
				logger.Error("task execution timed out", slog.String("task_id", task.ID), slog.Duration("timeout", timeout))
				timeoutUpdate := mapper.ToTaskUpdateFailed(&bee.SupervisorOutput{
					Status:  types.TaskStatusFailed,
					Content: fmt.Sprintf("Task execution timed out after %s", timeout),
				})
				if sendErr := srv.Send(timeoutUpdate); sendErr != nil {
					logger.Error("failed to send timeout update", slog.Any("error", sendErr))
				}
			}
			return err
		}

		logger.Debug("supervisor iteration completed", slog.String("task_id", task.ID), slog.String("status", string(output.Status)))
		switch output.Status {
		case types.TaskStatusCompleted:
			trace.Logger(ctx).Info("task completed successfully",
				slog.String("task_id", task.ID),
			)

			// Persist completed state
			if err = s.storage.Update(task); err != nil {
				logger.ErrorContext(ctx, "failed to persist completed task state", slog.Any("error", err))
			}

			update := mapper.ToTaskUpdateSuccess(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to send success update", slog.Any("error", err))
				return err
			}
			logger.Info("task finished", slog.String("content", output.Content))
			break loop

		case types.TaskStatusFailed:
			trace.Logger(ctx).Error("task failed",
				slog.String("task_id", task.ID),
				slog.String("reason", output.Content),
			)

			// Persist failed state
			if err = s.storage.Update(task); err != nil {
				logger.ErrorContext(ctx, "failed to persist failed task state", slog.Any("error", err))
			}

			update := mapper.ToTaskUpdateFailed(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to send failure update", slog.Any("error", err))
				return err
			}
			logger.Error("task failed", slog.String("content", output.Content))
			break loop

		case types.TaskStatusPaused:
			trace.Logger(ctx).Info("task paused, requesting user feedback",
				slog.String("task_id", task.ID),
				slog.String("question", output.Content),
			)

			// Store the supervisor's question in the conversation history
			task.Messages = append(task.Messages, types.Message{
				Role:    "assistant",
				Content: output.Content,
			})

			// Persist paused state with question
			if err = s.storage.Update(task); err != nil {
				logger.ErrorContext(ctx, "failed to persist paused task state", slog.Any("error", err))
			}

			update := mapper.ToTaskUpdateRequireFeedback(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to require feedback from user", slog.Any("error", err))
				return err
			}

			// Wait for user feedback
			for {
				msg, err = srv.Recv()
				if err != nil {
					logger.Error("failed to receive feedback from user", slog.Any("error", err))
					return err
				}
				feedback := msg.GetFeedback()
				if feedback == nil {
					logger.Warn("feedback from user is required")
					continue
				}

				// Store the user's feedback in the conversation history
				userFeedback := feedback.GetFeedback()
				task.Messages = append(task.Messages, types.Message{
					Role:    "user",
					Content: userFeedback,
				})

				// Persist user feedback
				if err = s.storage.Update(task); err != nil {
					logger.ErrorContext(ctx, "failed to persist task after user feedback", slog.Any("error", err))
				}

				logger.Info("user feedback received and stored in task history",
					slog.String("task_id", task.ID),
					slog.String("feedback", userFeedback),
				)
				break
			}

		// TODO: mock the event stream to the internal model so client could have a visibility on tool calling and thoughts here
		case types.TaskStatusInProgress:
			// Store the progress update in the conversation history
			task.Messages = append(task.Messages, types.Message{
				Role:    "assistant",
				Content: output.Content,
			})

			// Persist in-progress state after each cycle
			if err = s.storage.Update(task); err != nil {
				logger.ErrorContext(ctx, "failed to persist in-progress task state", slog.Any("error", err))
			}

			update := mapper.ToTaskUpdateInProgress(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to send in progress update", slog.Any("error", err))
				return err
			}

			logger.Info("task in progress",
				slog.String("task_id", task.ID),
				slog.String("status", string(output.Status)),
				slog.String("content", output.Content),
			)
		}
	}
	return nil
}

func getSupervisorPersona(registry bee.Registry) (string, error) {
	agents := registry.ListAgents()
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

This is the task state, which is your input that you and your team are working with:
%s

This is the output that your response must follow this schema only, don't return anything except the raw JSON without any formatting.
%s

Available Agents:
%s
`
	agentsDescription := map[string]string{}
	for _, agent := range agents {
		agentsDescription[agent.GetID()] = agent.Description()
	}
	log.Println(agentsDescription)
	yamlBytes, err := yaml.Marshal(agentsDescription)
	if err != nil {
		return "", fmt.Errorf("failed to build system prompt: %w", err)
	}
	taskDescription, err := utils.DescribeJSONSchema[types.HiveTask]()
	if err != nil {
		return "", fmt.Errorf("failed to describe JSON schema: %w", err)
	}
	outputDescription, err := utils.DescribeJSONSchema[bee.SupervisorOutput]()
	if err != nil {
		return "", fmt.Errorf("failed to describe JSON schema: %w", err)
	}
	return fmt.Sprintf(persona, taskDescription, outputDescription, string(yamlBytes)), nil
}

// forwardToolEvents forwards tool execution events from the channel to the gRPC stream.
func (s *HiveServer) forwardToolEvents(
	ctx context.Context,
	srv grpc.BidiStreamingServer[agentv1.ClientMessage, agentv1.ServerMessage],
	eventCh <-chan *react.ToolExecutionEvent,
	done chan<- struct{},
) {
	defer close(done)
	logger := trace.Logger(ctx)

	for event := range eventCh {
		content := ""
		switch event.EventType {
		case react.ToolEventStarted:
			content = fmt.Sprintf("%s is calling tool: %s, callID: %s", event.AgentID, event.ToolName, event.CallID)
		case react.ToolEventCompleted:
			content = fmt.Sprintf("tool call: %s completed", event.CallID)
		case react.ToolEventFailed:
			content = fmt.Sprintf("tool call: %s failed, error: %s", event.CallID, event.Error)
		}
		// Convert to protobuf message
		updateMsg := &agentv1.InProgressUpdate{
			Content: content,
			Status:  string(event.EventType),
		}

		// Send to client via gRPC stream
		if err := srv.Send(&agentv1.ServerMessage{
			Payload: &agentv1.ServerMessage_Update{
				Update: updateMsg,
			},
		}); err != nil {
			logger.ErrorContext(ctx, "failed to send tool execution event",
				slog.String("agent_id", event.AgentID),
				slog.String("tool", event.ToolName),
				slog.String("event_type", string(event.EventType)),
				slog.Any("error", err),
			)
			return
		}

		logger.DebugContext(ctx, "tool execution event sent",
			slog.String("agent_id", event.AgentID),
			slog.String("tool", event.ToolName),
			slog.String("event_type", string(event.EventType)),
		)
	}
}
