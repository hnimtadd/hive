package trace

import (
	"context"
	"log/slog"

	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/types"
	"google.golang.org/grpc"
)

const defaultTraceID = "unavailable"

// traceMiddleware wraps SessionLogger to implement HiveMiddleware.
type traceMiddleware struct {
	logger *SessionLogger
}

func (t *traceMiddleware) IsEnabled() bool {
	return t.logger != nil && t.logger.IsEnabled()
}

// OnRequest implements [middleware.HiveMiddleware].
func (t *traceMiddleware) OnRequest(ctx context.Context, agentID string, req types.LLMRequest) {
	if !t.IsEnabled() {
		return
	}
	traceCtx, found := TraceContextFromContext(ctx)
	traceID := defaultTraceID
	if found {
		traceID = traceCtx.TraceID
	}
	t.logger.LogLLMRequest(ctx, &LLMRequestLog{
		AgentID: agentID,
		CallID:  req.CallID,
		TraceID: traceID,
		Input:   req.Input,
	})
}

// OnResponse implements [middleware.HiveMiddleware].
func (t *traceMiddleware) OnResponse(ctx context.Context, agentID string, resp types.LLMResponse) {
	if !t.IsEnabled() {
		return
	}
	traceCtx, found := TraceContextFromContext(ctx)
	traceID := defaultTraceID
	if found {
		traceID = traceCtx.TraceID
	}

	t.logger.LogLLMResponse(ctx, &LLMResponseLog{
		AgentID:      agentID,
		CallID:       resp.CallID,
		TraceID:      traceID,
		FinishReason: resp.FinishReason,
		Content:      resp.Output,
		ToolsCalls:   resp.ToolCalls,
		Usage: &UsageLog{
			PromptTokens:     resp.TokenUsed.PromptToken,
			CompletionTokens: resp.TokenUsed.CompletionToken,
			TotalTokens:      resp.TokenUsed.TotalToken,
		},
	})
}

// OnToolCall implements [middleware.HiveMiddleware].
func (t *traceMiddleware) OnToolCall(ctx context.Context, agentID string, toolEvent types.ToolCall) {
	if !t.IsEnabled() {
		return
	}
	traceCtx, found := TraceContextFromContext(ctx)
	traceID := defaultTraceID
	if found {
		traceID = traceCtx.TraceID
	}
	t.logger.LogToolCall(ctx, &ToolCallLog{
		TraceID:  traceID,
		AgentID:  agentID,
		Output:   toolEvent.Output,
		CallID:   toolEvent.CallID,
		ToolName: toolEvent.ToolName,
		Input:    toolEvent.Arguments,
		Error:    toolEvent.Error.Error(),
	})
	panic("unimplemented")
}

func NewTraceMiddleware(sessionLogger *SessionLogger) middleware.HiveMiddleware {
	return &traceMiddleware{logger: sessionLogger}
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
		ctx = ContextWithTraceContext(ctx, NewRootTraceContext())

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
		ctx = ContextWithTraceContext(ctx, NewRootTraceContext())

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
