package trace

import (
	"context"
	"log/slog"

	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/pkg/config"
	"google.golang.org/grpc"
)

// middlewareAdapter wraps SessionLogger to implement HiveMiddleware
type middlewareAdapter struct {
	logger *SessionLogger
}

func (m *middlewareAdapter) OnRequest(ctx context.Context, agentID string, messages []*schema.Message) {
	if m.logger != nil && m.logger.IsEnabled() {
		m.logger.LogLLMRequest(ctx, NewLLMRequestLog(
			agentID,
			"",
			"", // Model not available at this level
			messages,
			nil,
			m.logger,
		))
	}
}

func (m *middlewareAdapter) OnResponse(ctx context.Context, agentID string, response *schema.Message) {
	if m.logger != nil && m.logger.IsEnabled() {
		m.logger.LogLLMResponse(ctx, NewLLMResponseLog(
			agentID,
			"",
			response,
			m.logger,
		))
	}
}

func (m *middlewareAdapter) OnToolCall(ctx context.Context, agentID, toolName, input, output string, err error, stage string) {
	if m.logger != nil && m.logger.IsEnabled() {
		m.logger.LogToolCall(ctx, NewToolCallLog(
			agentID,
			"",
			toolName,
			input,
			output,
			err,
			m.logger,
		))
	}
}

// NewMiddlewareFromSessionLogger creates a HiveMiddleware from SessionLogger
func NewMiddlewareFromSessionLogger(logger *SessionLogger) *middlewareAdapter {
	return &middlewareAdapter{logger: logger}
}

// UnaryServerInterceptor adds tracing to unary gRPC calls.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// Create trace context
		traceID := NewID()
		ctx = ContextWithTraceContext(ctx, traceID)

		Logger(ctx).Info("grpc request received",
			slog.String("method", info.FullMethod),
		)

		resp, err := handler(ctx, req)

		if err != nil {
			Logger(ctx).Error("grpc request failed",
				slog.String("method", info.FullMethod),
				slog.Any("error", err),
			)
		} else {
			Logger(ctx).Info("grpc request completed",
				slog.String("method", info.FullMethod),
			)
		}

		return resp, err
	}
}

// StreamServerInterceptor adds tracing to streaming gRPC calls.
func StreamServerInterceptor(sessionCfg *config.SessionLogConfig) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := stream.Context()

		// Create trace context
		traceID := NewID()
		ctx = ContextWithTraceContext(ctx, traceID)

		Logger(ctx).Info("grpc stream started",
			slog.String("method", info.FullMethod),
		)

		// Wrap stream to inject traced context
		wrapped := &tracedServerStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		err := handler(srv, wrapped)

		if err != nil {
			Logger(ctx).Error("grpc stream failed",
				slog.String("method", info.FullMethod),
				slog.Any("error", err),
			)
		} else {
			Logger(ctx).Info("grpc stream completed",
				slog.String("method", info.FullMethod),
			)
		}

		return err
	}
}

// tracedServerStream wraps grpc.ServerStream to return traced context.
type tracedServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s *tracedServerStream) Context() context.Context {
	return s.ctx
}
