package middleware

import (
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/types"
)

// HiveMiddleware defines a standardized interface for agent/LLM instrumentation.
// Implementations can hook into request/response cycles, tool calls, and errors.
type HiveMiddleware interface {
	// OnRequest is called before sending messages to the LLM
	OnRequest(ctx context.Context, agentID string, messages []*schema.Message)

	// OnResponse is called after receiving LLM response
	OnResponse(ctx context.Context, agentID string, response *schema.Message)

	// OnToolCall is called during tool execution lifecycle
	OnToolCall(ctx context.Context, agentID string, toolName string, callID string, eventType types.ToolEventType, input string, output string, err error)
}
