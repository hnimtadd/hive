package trace

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
)

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
		ctx = ContextWithTrace(ctx, traceID)

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
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := stream.Context()

		// Create trace context
		traceID := NewID()
		ctx = ContextWithTrace(ctx, traceID)

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
