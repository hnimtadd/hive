package types

// ToolCallRequest represent our tool call request/response.
type ToolCallRequest struct {
	AgentID   string
	CallID    string
	ToolName  string
	Arguments string
}

// ToolCallResponse represent our tool call response.
type ToolCallResponse struct {
	CallID          string
	ExecutionTimeMs int
	Succeed         bool
	Output          string
	Error           error
}

// LLMRequest represents our llm request.
type LLMRequest struct {
	AgentID string
	Input   string
}

// LLMResponse represents our llm response.
type LLMResponse struct {
	AgentID          string
	Output           string
	ToolCalls        []string
	ReasoningContent string
	ExecutionTimeMs  int
	FinishReason     string
	TokenUsed        TokenUsage
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
