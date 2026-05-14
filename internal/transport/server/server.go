package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/eventbus"
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

	grpcServer *grpc.Server
	config     *config.Config
	pipeline   *pipeline.Pipeline
	eventbus   *eventbus.EventBus[*agentv1.SessionEvent]
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(cfg *config.Config, provider llm.Provider, reg registry.Registry, sessionStorage storage.SessionStorage) (*HiveServer, error) {
	// Create task manager (storage + queue)
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
		config:   cfg,
		pipeline: pipeline,
		eventbus: eventbus,
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

// HiveSession implements [agentv1.AgentServiceServer].
func (s *HiveServer) HiveSession(
	_ grpc.BidiStreamingServer[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse],
) error {
	panic("Not implemented")
}
