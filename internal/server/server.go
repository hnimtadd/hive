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
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/mapper"
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
	supervisor bee.SupervisorBee
	config     *config.Config

	grpcServer *grpc.Server
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(cfg *config.Config, llm model.ToolCallingChatModel, registry bee.Registry) (*HiveServer, error) {
	persona, err := getSupervisorPersona(registry)
	if err != nil {
		return nil, err
	}

	// Use configured default timeout, capped at max timeout
	timeout := cfg.Server.MaxTimeout

	supervisor, err := bee.NewSupervisorAgent(&bee.Config{
		ID:           uuid.New().String(),
		Persona:      persona,
		MaxSteps:     3,
		TimeoutInSec: int(timeout.Seconds()),
		LLM:          llm,
		Tools:        []tool.InvokableTool{bee.DelegateTool(registry)},
	})
	if err != nil {
		return nil, err
	}
	return &HiveServer{
		registry:   registry,
		supervisor: supervisor,
		config:     cfg,
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

// timeoutUnaryInterceptor adds timeout to unary RPCs
func timeoutUnaryInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

// timeoutStreamInterceptor adds timeout to streaming RPCs
func timeoutStreamInterceptor(timeout time.Duration) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
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

	logger.Info("task created", slog.String("task_id", task.ID), slog.String("goal", task.Goal), slog.Int("artifact_count", len(task.Artifacts)))
loop:
	for {
		var output *bee.SupervisorOutput

		// Create a timeout context for each supervisor execution iteration
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		output, err = s.supervisor.Execute(execCtx, task)
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
			update := mapper.ToTaskUpdateFailed(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to send failure update", slog.Any("error", err))
				return err
			}
			logger.Error("task failed", slog.String("content", output.Content))
			break loop

		case types.TaskStatusPaused:
			trace.Logger(ctx).Error("task paused",
				slog.String("task_id", task.ID),
				slog.String("reason", output.Content),
			)
			update := mapper.ToTaskUpdateRequireFeedback(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to require feedback from user", slog.Any("error", err))
				return err
			}
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
				// TODO: feed feedback to model
				logger.Info("user feedback received", slog.String("feedback", feedback.String()))
				break
			}

		// TODO: mock the event stream to the internal model so client could have a visibility on tool calling and thoughts here
		case types.TaskStatusInProgress:
			update := mapper.ToTaskUpdateInProgress(output)
			if err = srv.Send(update); err != nil {
				logger.Error("failed to send in progress update", slog.Any("error", err))
				return err
			}

			logger.Info("task in progress", slog.String("status", string(output.Status)), slog.String("content", output.Content))
			break loop
		}
	}
	return nil
}

func getSupervisorPersona(registry bee.Registry) (string, error) {
	agents := registry.ListAgents()
	persona := `
Role: You are the Central Orchestrator for a multi-agent swarm. Your goal is to navigate a complex task to completion by delegating to specialized workers.
Core Responsibilities:
	- Analyze State: Review the conversation history. Identify what has been achieved and what is still missing.
    - Prevent Redundancy: If a supervisee has already failed at a specific approach, do not assign them the same task again without new instructions.
    - Evaluate Capabilities: Match the requirements of the next step against the specific tools and expertise of the available agents.
	- Terminate with Purpose:
		* Output FINISH if the user's goal is met along with the information
        * Output FAILED if the available agents lack the capabilities to proceed or if a logical dead-end is reached.
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
