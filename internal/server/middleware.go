package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/internal/types"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor adds tracing to unary gRPC calls.
func (s *HiveServer) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// Create trace context
		ctx = trace.ContextWithTraceContext(ctx, trace.NewRootTraceContext())
		ctx = middleware.ContextWithMiddleware(ctx, trace.NewTraceMiddleware(s.sessionLogger))

		trace.Logger(ctx).Info("grpc request received",
			slog.String("method", info.FullMethod),
		)

		resp, err := handler(ctx, req)

		if err != nil {
			trace.Logger(ctx).Error("grpc request failed",
				slog.String("method", info.FullMethod),
				slog.Any("error", err),
			)
		} else {
			trace.Logger(ctx).Info("grpc request completed",
				slog.String("method", info.FullMethod),
			)
		}

		return resp, err
	}
}

// StreamServerInterceptor adds tracing to streaming gRPC calls.
func (s *HiveServer) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := stream.Context()

		// Create trace context
		ctx = trace.ContextWithTraceContext(ctx, trace.NewRootTraceContext())
		ctx = middleware.ContextWithMiddleware(ctx, trace.NewTraceMiddleware(s.sessionLogger))

		trace.Logger(ctx).Info("grpc stream started",
			slog.String("method", info.FullMethod),
		)

		// Wrap stream to inject traced context
		wrapped := &tracedServerStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		err := handler(srv, wrapped)

		if err != nil {
			trace.Logger(ctx).Error("grpc stream failed",
				slog.String("method", info.FullMethod),
				slog.Any("error", err),
			)
		} else {
			trace.Logger(ctx).Info("grpc stream completed",
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

// timeoutUnaryInterceptor adds timeout to unary RPCs.
func (s *HiveServer) timeoutUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, s.config.Server.MaxTimeout)
		defer cancel()
		return handler(ctx, req)
	}
}

// timeoutStreamInterceptor adds timeout to streaming RPCs.
func (s *HiveServer) timeoutStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, cancel := context.WithTimeout(stream.Context(), s.config.Server.MaxTimeout)
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

type eventStreamMiddleware struct {
	eventCh chan<- any
}

type EventType string

const (
	EventTypeLLMRequest  EventType = "llm_req"
	EventTypeLLMResponse EventType = "llm_resp"
	EventTypeToolCall    EventType = "tool_call"
)

type ExecutionEvent struct {
	typ  EventType
	req  types.LLMRequest
	resp types.LLMResponse
	tool types.ToolCall
}

// OnRequest implements [middleware.HiveMiddleware].
func (e *eventStreamMiddleware) OnRequest(ctx context.Context, agentID string, req types.LLMRequest) {
	event := ExecutionEvent{
		typ: EventTypeLLMRequest,
		req: req,
	}

	if err := e.pushEvent(ctx, event); err != nil {
		trace.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
	}
}

// OnResponse implements [middleware.HiveMiddleware].
func (e *eventStreamMiddleware) OnResponse(ctx context.Context, agentID string, resp types.LLMResponse) {
	event := ExecutionEvent{
		typ:  EventTypeLLMRequest,
		resp: resp,
	}
	if err := e.pushEvent(ctx, event); err != nil {
		trace.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
	}
}

// OnToolCall implements [middleware.HiveMiddleware].
func (e *eventStreamMiddleware) OnToolCall(ctx context.Context, agentID string, toolEvent types.ToolCall) {
	event := ExecutionEvent{
		typ:  EventTypeToolCall,
		tool: toolEvent,
	}
	if err := e.pushEvent(ctx, event); err != nil {
		trace.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("call_id", toolEvent.CallID),
			slog.String("error", err.Error()),
		)
	}
}

func (e *eventStreamMiddleware) pushEvent(ctx context.Context, event ExecutionEvent) error {
	select {
	case e.eventCh <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("execution channel full")
	}
}

var _ middleware.HiveMiddleware = &eventStreamMiddleware{}

func (s *HiveServer) EventStreamMiddleware() (middleware.HiveMiddleware, <-chan any) {
	eventCh := make(chan any)
	return &eventStreamMiddleware{
		eventCh: eventCh,
	}, eventCh
}
