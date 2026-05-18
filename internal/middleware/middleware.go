package middleware

import (
	"context"
	"fmt"
	"slices"

	"github.com/hnimtadd/hive/internal/internaltypes"
)

// LLMMiddleware defines a standardized interface for agent/LLM instrumentation.
// Implementations can hook into request/response cycles, tool calls, and errors.
type LLMMiddleware interface {
	// OnRequest is called before sending messages to the LLM
	OnRequest(ctx context.Context, agentID string, req internaltypes.LLMRequest)

	// OnResponse is called after receiving LLM response
	OnResponse(ctx context.Context, agentID string, response internaltypes.LLMResponse)

	// OnToolCall is called during tool execution lifecycle
	OnToolCall(ctx context.Context, agentID string, toolEvent internaltypes.ToolCallRequest)

	// OnToolCallResponse is called during tool execution lifecycle
	OnToolCallResponse(ctx context.Context, agentID string, toolEvent internaltypes.ToolCallResponse)
}

type noopMiddleware struct {
}

// OnRequest implements [LLMMiddleware].
func (n noopMiddleware) OnRequest(_ context.Context, _ string, _ internaltypes.LLMRequest) {
	fmt.Println("<====== NOOP.OnRequest")
}

// OnResponse implements [LLMMiddleware].
func (n noopMiddleware) OnResponse(_ context.Context, _ string, _ internaltypes.LLMResponse) {
	fmt.Println("<====== NOOP.OnResponse")
}

// OnToolCall implements [LLMMiddleware].
func (n noopMiddleware) OnToolCall(_ context.Context, _ string, _ internaltypes.ToolCallRequest) {
	fmt.Println("<====== NOOP.OnToolCall")
}

// OnToolCall implements [LLMMiddleware].
func (n noopMiddleware) OnToolCallResponse(_ context.Context, _ string, _ internaltypes.ToolCallResponse) {
	fmt.Println("<====== NOOP.OnToolResponse")
}

func NoopMiddleware() LLMMiddleware {
	return noopMiddleware{}
}

type joinMiddleware struct {
	mws []LLMMiddleware
}

// OnRequest implements [LLMMiddleware].
func (j *joinMiddleware) OnRequest(ctx context.Context, agentID string, req internaltypes.LLMRequest) {
	for mw := range slices.Values(j.mws) {
		mw.OnRequest(ctx, agentID, req)
	}
}

// OnResponse implements [LLMMiddleware].
func (j *joinMiddleware) OnResponse(ctx context.Context, agentID string, response internaltypes.LLMResponse) {
	for mw := range slices.Values(j.mws) {
		mw.OnResponse(ctx, agentID, response)
	}
}

// OnToolCall implements [LLMMiddleware].
func (j *joinMiddleware) OnToolCall(ctx context.Context, agentID string, toolRequest internaltypes.ToolCallRequest) {
	for mw := range slices.Values(j.mws) {
		mw.OnToolCall(ctx, agentID, toolRequest)
	}
}

// OnToolCallResponse implements [LLMMiddleware].
func (j *joinMiddleware) OnToolCallResponse(ctx context.Context, agentID string, toolResponse internaltypes.ToolCallResponse) {
	for mw := range slices.Values(j.mws) {
		mw.OnToolCallResponse(ctx, agentID, toolResponse)
	}
}

func JointMiddleware(mws ...LLMMiddleware) LLMMiddleware {
	return &joinMiddleware{
		mws: mws,
	}
}
