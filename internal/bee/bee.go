package bee

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
)

type baseBee interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// Description return a short self-description about agent capabilities.
	Description() string
}

// Config holds configuration for agent initialization.
type Config struct {
	ID           string   `json:"id"              yaml:"id"`
	TimeoutInSec int      `json:"timeout_seconds" yaml:"timeout_seconds"`
	Capabilities []string `json:"capabilities"    yaml:"capabilities"`
	MaxSteps     int      `json:"max_steps"       yaml:"max_steps"`

	RequiredTools []string `json:"tools"       yaml:"tools"`
	ModelName     string   `json:"model_name"  yaml:"model_name"`
	Description   string   `json:"description" yaml:"description"`
	Persona       string   `json:"persona"     yaml:"-"`

	LLM   model.ToolCallingChatModel `json:"-" yaml:"-"`
	Tools []tool.InvokableTool       `json:"-" yaml:"-"`
}
