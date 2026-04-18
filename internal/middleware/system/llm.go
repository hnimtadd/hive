package system

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/types"
)

type eventStreamMiddleware struct {
	eventCh chan<- ExecutionEvent
}

type EventType string

const (
	EventTypeLLMRequestStart  EventType = "llm_request_start"
	EventTypeLLMRequestFinish EventType = "llm_request_finish"
	EventTypeToolCallStart    EventType = "tool_call_start"
	EventTypeToolCallFinish   EventType = "tool_call_finish"
)

type ExecutionEvent struct {
	Type     EventType
	Req      types.LLMRequest
	Resp     types.LLMResponse
	ToolReq  types.ToolCallRequest
	ToolResp types.ToolCallResponse
}

// OnRequest implements [middleware.LLMMiddleware].
func (e *eventStreamMiddleware) OnRequest(ctx context.Context, agentID string, req types.LLMRequest) {
	event := ExecutionEvent{
		Type: EventTypeLLMRequestStart,
		Req:  req,
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
	event := ExecutionEvent{
		Type: EventTypeLLMRequestFinish,
		Resp: resp,
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
	event := ExecutionEvent{
		Type:    EventTypeToolCallStart,
		ToolReq: toolEvent,
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
	event := ExecutionEvent{
		Type:     EventTypeToolCallFinish,
		ToolResp: toolEvent,
	}
	if err := e.pushEvent(ctx, event); err != nil {
		observability.Logger(ctx).WarnContext(ctx, "failed to push event",
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
		return errors.New("middleware: execution channel full")
	}
}

var _ middleware.LLMMiddleware = &eventStreamMiddleware{}

func EventStreamMiddleware() (middleware.LLMMiddleware, <-chan ExecutionEvent) {
	eventCh := make(chan ExecutionEvent, 100)
	return &eventStreamMiddleware{
		eventCh: eventCh,
	}, eventCh
}
