package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hnimtadd/hive/internal/internaltypes"
	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/observability"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
)

type eventStreamMiddleware struct {
	eventCh chan<- *agentv1.SessionEvent
}

// OnRequest implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnRequest(ctx context.Context, agentID string, req internaltypes.LLMRequest) {
	event := agentv1.NewSessionEventNotification("", &agentv1.Notification{
		Payload: &agentv1.Notification_Info{
			Info: fmt.Sprintf("llm request: agent=%s input_len=%d", agentID, len(req.Input)),
		},
	})

	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
	}
}

// OnResponse implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnResponse(ctx context.Context, agentID string, resp internaltypes.LLMResponse) {
	event := agentv1.NewSessionEventNotification("", &agentv1.Notification{
		Payload: &agentv1.Notification_Info{
			Info: fmt.Sprintf("llm response: agent=%s finish_reason=%s", agentID, resp.FinishReason),
		},
	})
	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
	}
}

// OnToolCall implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnToolCall(ctx context.Context, agentID string, toolEvent internaltypes.ToolCallRequest) {
	event := agentv1.NewSessionEventNotification("", &agentv1.Notification{
		Payload: &agentv1.Notification_Info{
			Info: fmt.Sprintf("tool call: agent=%s tool=%s call_id=%s", agentID, toolEvent.ToolName, toolEvent.CallID),
		},
	})
	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
			slog.String("agent_id", agentID),
			slog.String("call_id", toolEvent.CallID),
			slog.String("error", err.Error()),
		)
	}
}

// OnToolCall implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnToolCallResponse(ctx context.Context, agentID string, toolEvent internaltypes.ToolCallResponse) {
	status := "failed"
	if toolEvent.Succeed {
		status = "succeeded"
	}
	event := agentv1.NewSessionEventNotification("", &agentv1.Notification{
		Payload: &agentv1.Notification_Info{
			Info: fmt.Sprintf("tool response: agent=%s call_id=%s status=%s", agentID, toolEvent.CallID, status),
		},
	})
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
