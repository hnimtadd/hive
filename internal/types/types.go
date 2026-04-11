package types

// ToolCall represent our tool call request/response.
type ToolCall struct {
	ToolName        string
	CallID          string
	Arguments       string
	ExecutionTimeMs int
	Succeed         bool
	Output          string
	Error           error
}

// LLMRequest represents our llm request.
type LLMRequest struct {
	Input string
}

// LLMResponse represents our llm response.
type LLMResponse struct {
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
