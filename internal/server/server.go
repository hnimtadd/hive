package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/internal/manager"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/config"
	"google.golang.org/grpc"
)

type HiveServer struct {
	agentv1.AgentServiceServer

	grpcServer     *grpc.Server
	config         *config.Config
	taskManager    *manager.Manager
	channelManager *channel.Manager
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(cfg *config.Config, provider llm.Provider, reg registry.Registry, storage storage.Storage) (*HiveServer, error) {
	panic("not implemented")
}

func (s *HiveServer) Serve(addr string) error {
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

func (s *HiveServer) ExecuteTask(srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse]) error {
	ctx := srv.Context()
	ctx, cancel := context.WithCancelCause(ctx)

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
		err = fmt.Errorf("server: failed to create task: %w", err)
		cancel(err)
		return err
	}
	ch := s.channelManager.ForTask(task.ID)
	defer s.channelManager.Cleanup(task.ID)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err = s.forwardInput(ctx, srv, ch.InputCh); err != nil {
			cancel(err)
		}
	}()

	go func() {
		defer wg.Done()
		if err = s.forwardOutput(ctx, srv, ch.OutputCh); err != nil {
			cancel(err) // Signal failure to everyone
		}
	}()

	wg.Wait()
	return context.Cause(ctx)
}

func (s *HiveServer) forwardInput(
	ctx context.Context,
	srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse],
	ch chan *agentv1.ExecuteTaskRequest) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := srv.Recv()
			if err != nil {
				return fmt.Errorf("server: failed to receive user message: %w", err)
			}

			// Use select here to avoid blocking forever on a full channel if the context was
			// cancelled
			select {
			case ch <- msg:
			case <-ctx.Done():
				return ctx.Err()
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

			if err := srv.Send(msg); err != nil {
				return fmt.Errorf("server: output stream error: %w", err)
			}
		}
	}
}
