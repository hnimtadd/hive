package middleware

import (
	"context"

	"github.com/hnimtadd/hive/internal/types"
)

// HiveMiddleware defines a standardized interface for agent/LLM instrumentation.
// Implementations can hook into request/response cycles, tool calls, and errors.
type HiveMiddleware interface {
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

// OnRequest implements [HiveMiddleware].
func (n noopMiddleware) OnRequest(_ context.Context, _ string, _ types.LLMRequest) {
}

// OnResponse implements [HiveMiddleware].
func (n noopMiddleware) OnResponse(_ context.Context, _ string, _ types.LLMResponse) {
}

// OnToolCall implements [HiveMiddleware].
func (n noopMiddleware) OnToolCall(_ context.Context, _ string, _ types.ToolCallRequest) {
}

// OnToolCall implements [HiveMiddleware].
func (n noopMiddleware) OnToolCallResponse(_ context.Context, _ string, _ types.ToolCallResponse) {
}

func NoopMiddleware() HiveMiddleware {
	return noopMiddleware{}
}
