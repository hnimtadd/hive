package trace

// LLMRequestLog represents a logged LLM request.
type LLMRequestLog struct {
	TraceID string `json:"trace_id"`
	AgentID string `json:"agent_id"`
	Input   string `json:"messages"` // truncated content of each message
}

// LLMResponseLog represents a logged LLM response.
type LLMResponseLog struct {
	TraceID      string    `json:"trace_id"`
	AgentID      string    `json:"agent_id"`
	Content      string    `json:"content"`
	FinishReason string    `json:"finish_reason,omitempty"`
	Usage        *UsageLog `json:"usage,omitempty"`
	ToolsCalls   []string  `json:"tool_calls,omitempty"`
}

// UsageLog represents token usage.
type UsageLog struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolCallLog represents a logged tool call.
type ToolCallLog struct {
	TraceID  string `json:"trace_id"`
	AgentID  string `json:"agent_id"`
	CallID   string `json:"call_id"`
	ToolName string `json:"tool_name"`
	Input    string `json:"input"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ErrorLog represents a logged error.
type ErrorLog struct {
	TraceID string `json:"trace_id"`
	AgentID string `json:"agent_id"`
	CallID  string `json:"call_id"`
	Error   string `json:"error"`
	Context string `json:"context,omitempty"`
}
