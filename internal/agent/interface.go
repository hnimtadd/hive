package agent

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/pkg/types"
)

type BaseAgent interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// GetType returns the type/category of this agent (e.g., "code_editor", "test_runner", "deployer")
	GetType() string

	// Description return a short self-description about agent capabilities.
	Description() string
}

// Config holds configuration for agent initialization.
type Config struct {
	ID           string   `json:"id"`
	MaxTasks     int      `json:"max_tasks"`
	Timeout      int      `json:"timeout_seconds"`
	Capabilities []string `json:"capabilities"`
	Description  string   `json:"description"`
	MaxSteps     int      `json:"max_steps"`

	Persona string `json:"persona"`

	LLM   model.ToolCallingChatModel `json:"-"`
	Tools []tool.InvokableTool       `json:"-"`
}

type Output struct {
	Status       types.Status      `json:"status"               jsonschema:"Updated job state, either: not_started, in_progress, completed, failed, paused"`
	Observations string            `json:"observations"         jsonschema:"What did you find? This will be added to history."`
	NewArtifacts map[string]string `json:"new_artifacts"        jsonschema:"Any data found (e.g., ticket_details, log_snippet)"`
	NextSteps    string            `json:"next_steps,omitempty" jsonschema:"Optional suggestion for the supervisor"`
}
type Input struct {
	Context   string            `json:"status"    jsonschema:"High-level goal for the entire run"`
	Task      string            `json:"task"      jsonschema:"The exact instruction from the supervisor"`
	Artifacts map[string]string `json:"artifacts" jsonschema:"specfic data relevant to your task"`
}
