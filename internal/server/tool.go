package server

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/pkg/types"
)

// TODO: handle the supervisor to pass context-specific task instead of passing
// the whole state here, so move the state update logic out of agent and put at server layer
type delegateTaskInput struct {
	ID string `json:"agent_id"`
}

func agentLookupTool(registry agent.Registry) tool.InvokableTool {
	// Manually define ToolInfo
	toolInfo := &schema.ToolInfo{
		Name: "agent_lookup",
		Desc: "Look up if an agent with ID is exists",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"agent_id": {Type: "string", Desc: "Agent ID"},
		}),
	}

	// Create enhanced tool
	return utils.NewTool(
		toolInfo,
		func(ctx context.Context, input *delegateTaskInput) (*schema.ToolResult, error) {
			agent, exists := registry.GetByID(input.ID)
			if !exists {
				log.Println("agent with ID not found", input.ID)
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s not found", input.ID)},
					},
				}, nil
			}
			task, found := types.TaskFromContext(ctx)
			if !found {
				log.Println("task not found")
				return nil, fmt.Errorf("task not found from context")
			}
			if !agent.CanHandle(task) {
				log.Println("agent can't handle")
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s could not handle the task", input.ID)},
					},
				}, nil
			}
			result, err := agent.Execute(ctx, task)
			if err != nil {
				log.Println("failed to execute", err)
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
