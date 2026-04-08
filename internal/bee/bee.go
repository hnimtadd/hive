package bee

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
)

// Config holds configuration for agent initialization.
type Config struct {
	ID           string   `json:"id"              yaml:"id"`
	TimeoutInSec int      `json:"timeout_seconds" yaml:"timeout_seconds"`
	Capabilities []string `json:"capabilities"    yaml:"capabilities"`
	MaxSteps     int      `json:"max_steps"       yaml:"max_steps"`

	RequiredTools []string `json:"tools"       yaml:"tools"`
	Description   string   `json:"description" yaml:"description"`
	Persona       string   `json:"persona"     yaml:"-"`

	LLM   model.ToolCallingChatModel `json:"-" yaml:"-"`
	Tools []tool.InvokableTool       `json:"-" yaml:"-"`
}
