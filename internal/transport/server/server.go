package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/internal/manager"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/internal/worker"
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
	workerPool     *worker.Pool
	poolCtx        context.Context
	poolCancel     context.CancelFunc
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(cfg *config.Config, provider llm.Provider, reg registry.Registry, storage storage.Storage) (*HiveServer, error) {
	// Create task queue
	tq := queue.NewMemoryQueue()

	// Create channel manager for per-task communication
	cm := channel.NewManager()

	// Create task manager (storage + queue)
	tm := manager.NewManager(storage, tq)

	// Create worker pool
	poolSize := cfg.Bees.PoolSize
	if poolSize <= 0 {
		poolSize = 3 // Default: 3 concurrent workers
	}
	sessionLogger, err := observability.NewSessionLogger(&cfg.Tracing.SessionLog)
	if err != nil {
		return nil, err
	}

	pool := worker.NewPool(poolSize, tq, storage, cm, reg, provider, sessionLogger, cfg)

	return &HiveServer{
		config:         cfg,
		taskManager:    tm,
		channelManager: cm,
		workerPool:     pool,
	}, nil
}

func (s *HiveServer) Serve(addr string) error {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	s.workerPool.Start(ctx)
	s.poolCancel = cancel
	s.poolCtx = ctx

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

	// 2. Stop worker pool (cancels context, waits for in-flight tasks)
	if s.poolCancel != nil {
		s.poolCancel()
	}
	if s.workerPool != nil {
		s.workerPool.Stop()
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
	task, err := s.taskManager.CreateTask(ctx, req.GetGlobalGoal(), req.GetInitialArtifacts())
	if err != nil {
		logger.Error("server: failed to create task", slog.Any("error", err))
		err = fmt.Errorf("server: failed to create task: %w", err)
		cancel(err)
		return err
	}
	ch := s.channelManager.ForTask(task.ID)
	defer ch.CloseInput()

	ch.OutputCh <- agentv1.NewExecuteTaskResponseACK(task.ID)

	var wg sync.WaitGroup

	wg.Go(func() {
		err = s.forwardInput(ctx, srv, ch.InputCh)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("server: failed to forward output", slog.Any("error", err))
			cancel(err)
			return
		}
		logger.Error("forward input return")
		cancel(nil)
	})

	wg.Go(func() {
		err = s.forwardOutput(ctx, srv, ch.OutputCh)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("server: failed to forward output", slog.Any("error", err))
			cancel(err)
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
	ch chan *agentv1.ExecuteTaskRequest) error {
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

			// Use select here to avoid blocking forever on a full channel if the context was
			// cancelled
			select {
			case ch <- msg:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (s *HiveServer) forwardOutput(
	ctx context.Context,
	srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse],
	ch chan *agentv1.ExecuteTaskResponse,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			logger.Info("forward output", slog.Any("msg", msg.String()))

			if err := srv.Send(msg); err != nil {
				return fmt.Errorf("server: output stream error: %w", err)
			}
		}
	}
}
