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
	Input  string
	CallID string
}

// LLMResponse represents our llm response.
type LLMResponse struct {
	CallID           string
	Output           string
	ToolCalls        []string
	ReasoningContent string
	ExecutionTimeMs  int
	FinishReason     string
	TokenUsed        TokenUsage
}

type TokenUsage struct {
	PromptToken     int
	CompletionToken int
	TotalToken      int
}
