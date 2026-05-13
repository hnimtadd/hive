package system

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/types"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
)

type eventStreamMiddleware struct {
	eventCh chan<- *agentv1.SessionEvent
}

// OnRequest implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnRequest(ctx context.Context, agentID string, req types.LLMRequest) {
	// TODO: continue on this
	event := &agentv1.SessionEvent{
		Type:    agentv1.SessionEventType("on-request"),
		Payload: nil,
	}

	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
	}
}

// OnResponse implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnResponse(ctx context.Context, agentID string, resp types.LLMResponse) {
	// TOOD: define event for this
	event := &agentv1.SessionEvent{
		Type:    agentv1.SessionEventType("on-request"),
		Payload: nil,
	}
	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
	}
}

// OnToolCall implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnToolCall(ctx context.Context, agentID string, toolEvent types.ToolCallRequest) {
	// define event for this
	event := &agentv1.SessionEvent{
		Type:    agentv1.SessionEventType("on-request"),
		Payload: nil,
	}
	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("call_id", toolEvent.CallID),
			slog.String("error", err.Error()),
		)
	}
}

// OnToolCall implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnToolCallResponse(ctx context.Context, agentID string, toolEvent types.ToolCallResponse) {
	// define event for this
	event := &agentv1.SessionEvent{
		Type:    agentv1.SessionEventType("on-request"),
		Payload: nil,
	}
	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("call_id", toolEvent.CallID),
			slog.String("error", err.Error()),
		)
	}
}

func (e *eventStreamMiddleware) pushEvent(ctx context.Context, event *agentv1.SessionEvent) error {
	if event == nil || e.eventCh == nil {
		return nil
	}

	select {
	case e.eventCh <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("middleware: execution channel full")
	}
}

var _ middleware.LLMMiddleware = &eventStreamMiddleware{}

func EventStreamMiddleware(eventCh chan<- *agentv1.SessionEvent) middleware.LLMMiddleware {
	return &eventStreamMiddleware{
		eventCh: eventCh,
	}
}
