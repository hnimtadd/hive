package react

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoreact "github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// Agent is a simplified ReACT agent wrapper.
type Agent struct {
	id    string
	agent *einoreact.Agent
}

// New creates a new ReACT agent with minimal configuration.
func New(id string, chatModel model.ToolCallingChatModel, tools []tool.InvokableTool, maxStep int) (*Agent, error) {
	config := &einoreact.AgentConfig{
		ToolCallingModel: chatModel,
		MaxStep:          maxStep,
	}

	// Configure tools if provided
	if len(tools) > 0 {
		baseTools := make([]tool.BaseTool, len(tools))
		for i, t := range tools {
			baseTools[i] = t
		}
		config.ToolsConfig = compose.ToolsNodeConfig{
			Tools: baseTools,
		}
	}

	// Create Eino's ReACT agent
	ctx := context.Background()
	agent, err := einoreact.NewAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	return &Agent{
		id:    id,
		agent: agent,
	}, nil
}

// NewWithSystemPrompt creates a new ReACT agent with a system prompt.
func NewWithSystemPrompt(id string, chatModel model.ToolCallingChatModel, tools []tool.InvokableTool, systemPrompt string, maxStep int) (*Agent, error) {
	config := &einoreact.AgentConfig{
		ToolCallingModel: chatModel,
		MaxStep:          maxStep, // Allow up to 30 reasoning/action steps
		MessageModifier: func(_ context.Context, input []*schema.Message) []*schema.Message {
			if len(input) > 0 && input[0].Role != schema.System {
				return append([]*schema.Message{schema.SystemMessage(systemPrompt)}, input...)
			}
			return input
		},
	}

	// Configure tools if provided
	if len(tools) > 0 {
		baseTools := make([]tool.BaseTool, len(tools))
		for i, t := range tools {
			baseTools[i] = t
		}
		config.ToolsConfig = compose.ToolsNodeConfig{
			Tools: baseTools,
		}
	}

	// Create Eino's ReACT agent
	ctx := context.Background()
	agent, err := einoreact.NewAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	return &Agent{
		id:    id,
		agent: agent,
	}, nil
}

// Execute runs the ReACT agent with the given input.
func (a *Agent) Execute(ctx context.Context, input string) (*schema.Message, error) {
	return a.ExecuteWithMessages(ctx, []*schema.Message{schema.UserMessage(input)})
}

// ExecuteWithMessages runs the ReACT agent with conversation history.
func (a *Agent) ExecuteWithMessages(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	return a.agent.Generate(ctx, messages)
}

// ID returns the agent ID.
func (a *Agent) ID() string {
	return a.id
}
