package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/bee/registry"
)

type delegateTaskInput struct {
	ID        string            `json:"agent_id"  jsonschema:"Agent ID"`
	Context   string            `json:"status"    jsonschema:"The global context taht the agent need to be aware about"`
	Task      string            `json:"task"      jsonschema:"The detail task that agent have to complete in this run"`
	Artifacts map[string]string `json:"artifacts" jsonschema:"artifacts that the agent need to execute"`
}

func DelegateTool() (tool.InvokableTool, error) {
	// Create enhanced tool
	return utils.InferTool(
		"delegate_agent",
		"Look up if an agent with ID is exists and delegate task to that agent",
		func(ctx context.Context, input *delegateTaskInput) (*schema.ToolResult, error) {
			reg, exists := registry.RegistryFromContext(ctx)
			if !exists {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Registry %s not found", input.ID)},
					},
				}, nil
			}
			a, exists := reg.GetByID(input.ID)
			if !exists {
				log.Println("agent with ID not found", input.ID)
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s not found", input.ID)},
					},
				}, nil
			}
			i := &bee.WorkerInput{
				Context:   input.Context,
				Artifacts: input.Artifacts,
				Task:      input.Task,
			}

			if !a.CanHandle(i) {
				log.Println("agent can't handle")
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s could not handle the task", input.ID)},
					},
				}, nil
			}
			output, err := a.Execute(ctx, i)
			if err != nil {
				log.Println("failed to execute", err)
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s failed to not handle the task: %s", input.ID, err)},
					},
				}, nil
			}
			outputJSON, err := json.Marshal(output)
			if err != nil {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to convert output to a json: %v", err)},
					},
				}, nil
			}

			return &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: string(outputJSON)}}}, nil
		},
	)
}
