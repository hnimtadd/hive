package types

import "time"

// AgentCycleLog represents a complete agent execution cycle
type AgentCycleLog struct {
	AgentID    string    `json:"agent_id"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"`
	LLMCalls   int       `json:"llm_calls"`
	ToolCalls  int       `json:"tool_calls"`
	Input      string    `json:"input"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
}

// ToolExecutionEvent represents a tool execution lifecycle event.
type ToolExecutionEvent struct {
	AgentID   string
	ToolName  string
	CallID    string
	EventType ToolEventType
	Input     string // JSON-encoded arguments
	Output    string // JSON-encoded result
	Error     error
}

// ToolEventType represents the stage of tool execution.
type ToolEventType string

const (
	ToolEventStarted   ToolEventType = "started"
	ToolEventCompleted ToolEventType = "completed"
	ToolEventFailed    ToolEventType = "failed"
)
