package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type delegateTaskInput struct {
	Input

	ID string `json:"agent_id"`
}

func DelegateTool(registry Registry) tool.InvokableTool {
	// Manually define ToolInfo
	toolInfo := &schema.ToolInfo{
		Name: "delegate_agent",
		Desc: "Look up if an agent with ID is exists and delegate task to that agent",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"agent_id": {Type: "string", Desc: "Agent ID"},
			"context":  {Type: "string", Desc: "The global context that the agent need to be aware about"},
			"task":     {Type: "string", Desc: "The detail task that agent have to complete in this run"},
			"artifact": {Type: schema.Object, Desc: "artifacts that the agent need to execute"},
		}),
	}

	// Create enhanced tool
	return utils.NewTool(
		toolInfo,
		func(ctx context.Context, input *delegateTaskInput) (*schema.ToolResult, error) {
			a, exists := registry.GetByID(input.ID)
			if !exists {
				log.Println("agent with ID not found", input.ID)
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s not found", input.ID)},
					},
				}, nil
			}
			i := &Input{
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
