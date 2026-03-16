package server

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/pkg/types"
)

type delegateTaskInput struct {
	ID string `json:"agent_id"`
}

func agentLookupTool(registry agent.Registry) tool.InvokableTool {
	// Manually define ToolInfo
	toolInfo := &schema.ToolInfo{
		Name: "agent_lookup",
		Desc: "Look up if an agent with ID is exists",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_name": {Type: "string", Desc: "Agent ID"},
		}),
	}

	// Create enhanced tool
	return utils.NewTool(
		toolInfo,
		func(ctx context.Context, input *delegateTaskInput) (*schema.ToolResult, error) {
			agent, exists := registry.GetByID(input.ID)
			if !exists {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s not found", input.ID)},
					},
				}, nil
			}
			task, found := types.TaskFromContext(ctx)
			if !found {
				return nil, fmt.Errorf("task not found from context")
			}
			if !agent.CanHandle(task) {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s could not handle the task", input.ID)},
					},
				}, nil
			}
			result, err := agent.Execute(ctx, task)
			if err != nil {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s failed to not handle the task: %s", input.ID, err)},
					},
				}, nil
			}

			return &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: result.String()}}}, nil
		},
	)

}
