package trace

import (
	"context"
	"sync"

	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/types"
)

const defaultTraceID = "unavailable"

// traceMiddleware wraps SessionLogger to implement HiveMiddleware.
type traceMiddleware struct {
	logger    *SessionLogger
	toolCalls map[string]*ToolCallLog
	mu        sync.Mutex // Protects toolCalls map from concurrent access
}

func (t *traceMiddleware) IsEnabled() bool {
	return t.logger != nil && t.logger.IsEnabled()
}

// OnRequest implements [middleware.LLMMiddleware].
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
		TraceID: traceID,
		Input:   req.Input,
	})
}

// OnResponse implements [middleware.LLMMiddleware].
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
		TraceID:      traceID,
		FinishReason: resp.FinishReason,
		Content:      resp.Output,
		ToolsCalls:   resp.ToolCalls,
		Usage: &UsageLog{
			PromptTokens:     resp.TokenUsed.PromptTokens,
			CompletionTokens: resp.TokenUsed.CompletionTokens,
			TotalTokens:      resp.TokenUsed.TotalTokens,
		},
	})
}

// OnToolCall implements [middleware.LLMMiddleware].
func (t *traceMiddleware) OnToolCall(ctx context.Context, agentID string, toolEvent types.ToolCallRequest) {
	if !t.IsEnabled() {
		return
	}
	traceCtx, found := TraceContextFromContext(ctx)
	traceID := defaultTraceID
	if found {
		traceID = traceCtx.TraceID
	}

	toolCall := &ToolCallLog{
		TraceID:  traceID,
		AgentID:  agentID,
		CallID:   toolEvent.CallID,
		ToolName: toolEvent.ToolName,
		Input:    toolEvent.Arguments,
	}
	t.mu.Lock()
	t.toolCalls[toolEvent.CallID] = toolCall
	t.mu.Unlock()
}

// OnToolCall implements [middleware.LLMMiddleware].
func (t *traceMiddleware) OnToolCallResponse(ctx context.Context, _ string, toolEvent types.ToolCallResponse) {
	if !t.IsEnabled() {
		return
	}
	t.mu.Lock()
	toolCall, ok := t.toolCalls[toolEvent.CallID]
	if !ok {
		t.mu.Unlock()
		return
	}

	if toolEvent.Succeed {
		toolCall.Output = toolEvent.Output
	}
	if toolEvent.Error != nil {
		toolCall.Error = toolEvent.Error.Error()
	}
	// Remove from map to prevent memory leak
	delete(t.toolCalls, toolEvent.CallID)
	t.mu.Unlock()

	// Log outside of lock
	t.logger.LogToolCall(ctx, toolCall)
}

func NewTraceMiddleware(sessionLogger *SessionLogger) middleware.LLMMiddleware {
	return &traceMiddleware{
		logger:    sessionLogger,
		toolCalls: make(map[string]*ToolCallLog),
	}
}
