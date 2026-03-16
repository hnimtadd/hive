package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/agent"
)

type AgentLookupInput struct {
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
	return utils.NewTool[*AgentLookupInput, *schema.ToolResult](
		toolInfo,
		func(ctx context.Context, input *AgentLookupInput) (*schema.ToolResult, error) {
			agent, exists := registry.GetByID(input.ID)
			if !exists {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Agent %s not found", input.ID)},
					},
				}, nil
			}
			agentInfo := map[string]string{
				"agentID":     input.ID,
				"description": agent.Description(),
			}
			infoJSON, err := json.Marshal(agentInfo)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal agent information")
			}

			return &schema.ToolResult{Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: string(infoJSON)},
			},
			}, nil
		},
	)

}
