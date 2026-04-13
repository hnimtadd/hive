package middleware

import (
	"context"

	"github.com/hnimtadd/hive/internal/types"
)

// LLMMiddleware defines a standardized interface for agent/LLM instrumentation.
// Implementations can hook into request/response cycles, tool calls, and errors.
type LLMMiddleware interface {
	// OnRequest is called before sending messages to the LLM
	OnRequest(ctx context.Context, agentID string, req types.LLMRequest)

	// OnResponse is called after receiving LLM response
	OnResponse(ctx context.Context, agentID string, response types.LLMResponse)

	// OnToolCall is called during tool execution lifecycle
	OnToolCall(ctx context.Context, agentID string, toolEvent types.ToolCallRequest)

	// OnToolCallResponse is called during tool execution lifecycle
	OnToolCallResponse(ctx context.Context, agentID string, toolEvent types.ToolCallResponse)
}

type noopMiddleware struct {
}

// OnRequest implements [LLMMiddleware].
func (n noopMiddleware) OnRequest(_ context.Context, _ string, _ types.LLMRequest) {
}

// OnResponse implements [LLMMiddleware].
func (n noopMiddleware) OnResponse(_ context.Context, _ string, _ types.LLMResponse) {
}

// OnToolCall implements [LLMMiddleware].
func (n noopMiddleware) OnToolCall(_ context.Context, _ string, _ types.ToolCallRequest) {
}

// OnToolCall implements [LLMMiddleware].
func (n noopMiddleware) OnToolCallResponse(_ context.Context, _ string, _ types.ToolCallResponse) {
}

func NoopMiddleware() LLMMiddleware {
	return noopMiddleware{}
}
