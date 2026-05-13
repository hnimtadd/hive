package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/internal/eventbus"
	"github.com/hnimtadd/hive/internal/manager"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/pipeline"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/config"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
	"google.golang.org/grpc"
)

type HiveServer struct {
	agentv1.AgentServiceServer

	grpcServer     *grpc.Server
	config         *config.Config
	taskManager    *manager.Manager
	channelManager *channel.Manager
	pipeline       *pipeline.Pipeline
	eventbus       *eventbus.EventBus[*agentv1.SessionEvent]
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(cfg *config.Config, provider llm.Provider, reg registry.Registry, sessionStorage storage.SessionStorage, storage storage.Storage) (*HiveServer, error) {
	// Create channel manager for per-task communication
	cm := channel.NewManager()

	// Create task manager (storage + queue)
	tm := manager.NewManager(sessionStorage, storage)

	// Create worker pool
	poolSize := cfg.Bees.PoolSize
	if poolSize <= 0 {
		poolSize = 3 // Default: 3 concurrent workers
	}
	sessionLogger, err := observability.NewSessionLogger(&cfg.Session)
	if err != nil {
		return nil, err
	}

	eventbus := eventbus.NewEventBus[*agentv1.SessionEvent]()
	pipeline := pipeline.NewPipeline(pipeline.PipelineDependencies{
		EventBus:      eventbus,
		SessionLogger: sessionLogger,
		Config:        *cfg,
		Registry:      reg,
		Provider:      provider,
	})

	return &HiveServer{
		config:         cfg,
		taskManager:    tm,
		channelManager: cm,
		pipeline:       pipeline,
		eventbus:       eventbus,
	}, nil
}

func (s *HiveServer) Serve(addr string) error {
	logger := slog.Default()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create gRPC server with timeout and tracing interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			s.timeoutUnaryInterceptor(),
			s.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			s.timeoutStreamInterceptor(),
			s.StreamServerInterceptor(),
		),
	)
	agentv1.RegisterAgentServiceServer(grpcServer, s)
	s.grpcServer = grpcServer

	logger.Info("server.starting", "addr", addr, "max_timeout", s.config.Server.MaxTimeout)

	if err = grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (s *HiveServer) Stop() {
	logger := slog.Default()
	if s.grpcServer != nil {
		// Use configured graceful shutdown timeout
		timeout := s.config.Server.GracefulShutdownTimeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// 1. Stop accepting new requests (gRPC graceful shutdown)
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("server: gRPC graceful shutdown completed")
		case <-ctx.Done():
			logger.Info("server: gRPC shutdown timeout exceeded, forcing stop")
			s.grpcServer.Stop()
		}
	}
}

func (s *HiveServer) ExecuteTask(srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse]) error {
	ctx := srv.Context()
	ctx, cancel := context.WithCancelCause(ctx)
	logger := observability.Logger(ctx)

	msg, err := srv.Recv()
	if err != nil {
		err = fmt.Errorf("server: failed to receive user request: %w", err)
		cancel(err)
		return err
	}
	req := msg.GetRequest()
	if req == nil {
		err = fmt.Errorf("server: first request is not a user request: %s", msg.String())
		cancel(err)
		return err
	}
	task, err := s.taskManager.CreateTask(req.GetGlobalGoal(), req.GetInitialArtifacts())
	if err != nil {
		logger.Error("server: failed to create task", slog.Any("error", err))
		err = fmt.Errorf("server: failed to create task: %w", err)
		cancel(err)
		return err
	}
	if err = srv.Send(agentv1.NewExecuteTaskResponseACK(task.ID)); err != nil {
		err = fmt.Errorf("server: failed to send task ack: %w", err)
		cancel(err)
		return err
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		_, runErr := s.pipeline.Execute(ctx, pipeline.NewPipelineState(ctx, task))
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			logger.Error("server: failed to execute pipeline", slog.Any("error", runErr))
			cancel(fmt.Errorf("server: failed to execute pipeline: %w", runErr))
			return
		}
		cancel(nil)
	})

	wg.Go(func() {
		inputErr := s.forwardInput(ctx, srv, task.ID)
		if inputErr != nil && !errors.Is(inputErr, context.Canceled) {
			logger.Error("server: failed to forward input", slog.Any("error", inputErr))
			cancel(inputErr)
			return
		}
		logger.Error("forward input return")
		cancel(nil)
	})

	wg.Go(func() {
		outputErr := s.forwardOutput(ctx, srv, task.ID)
		if outputErr != nil && !errors.Is(outputErr, context.Canceled) {
			logger.Error("server: failed to forward output", slog.Any("error", outputErr))
			cancel(outputErr)
			return
		}
		logger.Error("forward output return")
		cancel(nil)
	})

	wg.Wait()
	if err = context.Cause(ctx); errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (s *HiveServer) forwardInput(
	ctx context.Context,
	srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse],
	taskID string) error {
	inputCh := make(chan *agentv1.ExecuteTaskRequest, 10)
	go func() {
		defer close(inputCh)
		for {
			msg, err := srv.Recv()
			if err != nil {
				return
			}
			inputCh <- msg
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok := <-inputCh:
			if !ok {
				return nil
			}
			if err := s.handleSessionInputEvent(ctx, taskID, msg); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				return fmt.Errorf("server: failed to handle input event: %w", err)
			}
		}
	}
}

func (s *HiveServer) forwardOutput(
	ctx context.Context,
	srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse],
	taskID string,
) error {
	logger := observability.Logger(ctx)
	ch, unsubscribe := s.eventbus.SubscribeWithCancel(taskID)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			resp, err := sessionEventToExecuteTaskResponse(msg)
			if err != nil {
				return fmt.Errorf("server: failed to convert session event to execute response: %w", err)
			}
			if resp == nil {
				continue
			}

			logger.Info("forward output", slog.String("event_type", string(msg.Type)))

			if err := srv.Send(resp); err != nil {
				return fmt.Errorf("server: output stream error: %w", err)
			}
		}
	}
}

func (s *HiveServer) handleSessionInputEvent(ctx context.Context, taskID string, msg *agentv1.ExecuteTaskRequest) error {
	if msg == nil {
		return fmt.Errorf("empty execute task input message")
	}

	switch payload := msg.Payload.(type) {
	case *agentv1.ExecuteTaskRequest_Feedback:
		return s.pipeline.Handle(ctx, pipeline.PipelineCommand{
			Key: pipeline.PipelineSubmitInputKey,
			Payload: pipeline.PipelineSubmitInputPayload{
				CorrelationID: taskID,
				Input:         payload.Feedback.GetFeedback(),
			},
		})
	case *agentv1.ExecuteTaskRequest_Cancel:
		return context.Canceled
	case *agentv1.ExecuteTaskRequest_Request:
		// The first request payload already bootstraps the task before this loop.
		return nil
	default:
		return fmt.Errorf("unsupported execute task request payload type %T", payload)
	}
}

func sessionEventToExecuteTaskResponse(event *agentv1.SessionEvent) (*agentv1.ExecuteTaskResponse, error) {
	if event == nil || event.Payload == nil {
		return nil, nil
	}

	switch payload := event.Payload.(type) {
	case *agentv1.SessionEventTurnResponsePayload:
		resp := payload.Response
		if resp == nil || resp.Payload == nil {
			return nil, nil
		}

		switch turnPayload := resp.Payload.(type) {
		case *agentv1.TurnResponse_Update:
			return agentv1.NewExecuteTaskResponseUpdate("in_progress", turnPayload.Update.GetContent()), nil
		case *agentv1.TurnResponse_Completed:
			if success := turnPayload.Completed.GetSuccess(); success != nil {
				return agentv1.NewExecuteTaskResponseSuccess(success.GetContent()), nil
			}
			if failed := turnPayload.Completed.GetFailed(); failed != nil {
				return agentv1.NewExecuteTaskResponseErr(failed.GetMessage()), nil
			}
			return nil, nil
		case *agentv1.TurnResponse_Ack:
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported turn response payload type %T", turnPayload)
		}

	case *agentv1.SessionEventInputRequiredPayload:
		if payload.Input == nil {
			return nil, nil
		}
		return agentv1.NewExecuteTaskResponseFeedback(payload.Input.GetQuestion()), nil

	case *agentv1.SessionEventNotificationPayload:
		if payload.Notification == nil {
			return nil, nil
		}
		if errMsg := payload.Notification.GetError(); errMsg != "" {
			return agentv1.NewExecuteTaskResponseErr(errMsg), nil
		}
		if info := payload.Notification.GetInfo(); info != "" {
			return agentv1.NewExecuteTaskResponseUpdate("info", info), nil
		}
		return nil, nil

	case *agentv1.SessionEventCreateConversationPayload, *agentv1.SessionEventPongPayload:
		// ExecuteTask stream does not expose these HiveSession-specific payloads.
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported session event payload type %T", payload)
	}
}

// HiveSession implements [agentv1.AgentServiceServer].
func (s *HiveServer) HiveSession(srv grpc.BidiStreamingServer[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse]) error {
	// TODO: implement this follow our diagram
	// ctx := srv.Context()
	// msg, err := srv.Recv()
	// switch payload := msg.Payload.(type) {
	// case *agentv1.HiveSessionRequest_CreateConversation:
	// default:
	// 	srv.Send(&agentv1.HiveSessionResponse{
	// 		Payload: &agentv1.HiveSessionResponse_Notification{
	// 			Notification: &agentv1.Notification{
	// 				Payload: &agentv1.Notification_Error{
	// 					Error: "The first message should be the createconversation",
	// 				},
	// 			},
	// 		},
	// 	})
	// }
	//
	// var (
	// 	session *types.HiveSession
	// 	err     error
	// )
	// convMsg := msg.GetCreateConversation()
	// switch mode := convMsg.GetMode().(type) {
	// case *agentv1.CreateConversationRequest_CreateNew:
	// 	session, err = s.taskManager.CreateSession(ctx)
	// case *agentv1.CreateConversationRequest_ResumeId:
	// 	session, err = s.taskManager.LoadSession(ctx, mode.ResumeId)
	// }
	// s.taskManager
	panic("Not implemented")
}
