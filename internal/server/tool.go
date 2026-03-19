package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/agent"
)

// TODO: handle the supervisor to pass context-specific task instead of passing
// the whole state here, so move the state update logic out of agent and put at server layer
type delegateTaskInput struct {
	ID string `json:"agent_id"`
	agent.Input
}

func agentLookupTool(registry agent.Registry) tool.InvokableTool {
	// Manually define ToolInfo
	toolInfo := &schema.ToolInfo{
		Name: "agent_lookup",
		Desc: "Look up if an agent with ID is exists",
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
			// task, found := types.TaskFromContext(ctx)
			// if !found {
			// 	log.Println("task not found")
			// 	return nil, fmt.Errorf("task not found from context")
			// }
			i := &agent.Input{
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
			fmt.Println("output", output)
			outputJSON, err := json.Marshal(output)
			if err != nil {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to convert output to a json", input.ID)},
					},
				}, nil
			}

			return &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: string(outputJSON)}}}, nil
		},
	)

}
