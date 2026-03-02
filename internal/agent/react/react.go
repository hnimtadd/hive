package react

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	einoreact "github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// ReACTAgent wraps Eino's built-in ReACT agent for easy use
type ReACTAgent struct {
	id     string
	agent  *einoreact.Agent
	tools  []tool.InvokableTool
	config *einoreact.AgentConfig
}

// NewReACTAgent creates a new ReACT agent using Eino's built-in implementation
func NewReACTAgent(id string, chatModel model.ToolCallingChatModel, tools []tool.InvokableTool, options ...Option) (*ReACTAgent, error) {
	reactAgent := &ReACTAgent{
		id:    id,
		tools: tools,
		config: &einoreact.AgentConfig{
			ToolCallingModel: chatModel,
		},
	}

	// Apply configuration options
	for _, opt := range options {
		opt(reactAgent)
	}

	// Configure tools if provided
	if len(tools) > 0 {
		// Convert tools to tool.BaseTool interface
		baseTools := make([]tool.BaseTool, len(tools))
		for i, t := range tools {
			baseTools[i] = t
		}

		// Set up tools config
		reactAgent.config.ToolsConfig = compose.ToolsNodeConfig{
			Tools: baseTools,
		}
	}

	// Create Eino's ReACT agent
	ctx := context.Background()
	einoAgent, err := einoreact.NewAgent(ctx, reactAgent.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Eino ReACT agent: %w", err)
	}

	reactAgent.agent = einoAgent
	return reactAgent, nil
}

// Run executes the ReACT agent with the given input
func (a *ReACTAgent) Run(ctx context.Context, input string) (*schema.Message, error) {
	messages := []*schema.Message{
		schema.UserMessage(input),
	}

	result, err := a.agent.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("ReACT agent execution failed: %w", err)
	}

	return result, nil
}

// Stream executes the ReACT agent with streaming response
func (a *ReACTAgent) Stream(ctx context.Context, input string) (*schema.StreamReader[*schema.Message], error) {
	messages := []*schema.Message{
		schema.UserMessage(input),
	}

	stream, err := a.agent.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("ReACT agent streaming failed: %w", err)
	}

	return stream, nil
}

// RunWithMessages executes the ReACT agent with a conversation history
func (a *ReACTAgent) RunWithMessages(ctx context.Context, messages []*schema.Message, opts ...agent.AgentOption) (*schema.Message, error) {
	result, err := a.agent.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("ReACT agent execution failed: %w", err)
	}

	return result, nil
}

// StreamWithMessages executes the ReACT agent with streaming response and conversation history
func (a *ReACTAgent) StreamWithMessages(ctx context.Context, messages []*schema.Message, opts ...agent.AgentOption) (*schema.StreamReader[*schema.Message], error) {
	stream, err := a.agent.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("ReACT agent streaming failed: %w", err)
	}

	return stream, nil
}

// ID returns the agent ID
func (a *ReACTAgent) ID() string {
	return a.id
}

// GetTools returns the tools available to the agent
func (a *ReACTAgent) GetTools() []tool.InvokableTool {
	return a.tools
}

// GetAgent returns the underlying Eino ReACT agent for advanced usage
func (a *ReACTAgent) GetAgent() *einoreact.Agent {
	return a.agent
}

// Option defines configuration options for the agent
type Option func(*ReACTAgent)

// WithMaxIterations sets the maximum number of iterations (placeholder for future Eino support)
func WithMaxIterations(max int) Option {
	return func(a *ReACTAgent) {
		// Eino's ReACT agent doesn't directly expose max iterations in config
		// This could be implemented via graph compile options in the future
	}
}

// WithSystemPrompt sets a custom system prompt via message modifier
func WithSystemPrompt(prompt string) Option {
	return func(a *ReACTAgent) {
		a.config.MessageModifier = func(ctx context.Context, input []*schema.Message) []*schema.Message {
			// Add system prompt if not already present
			if len(input) > 0 && input[0].Role != schema.System {
				return append([]*schema.Message{schema.SystemMessage(prompt)}, input...)
			}
			return input
		}
	}
}

// WithMessageModifier sets a custom message modifier
func WithMessageModifier(modifier einoreact.MessageModifier) Option {
	return func(a *ReACTAgent) {
		a.config.MessageModifier = modifier
	}
}

// WithToolCallingModel upgrades to use a tool calling model for better performance
func WithToolCallingModel(model model.ToolCallingChatModel) Option {
	return func(a *ReACTAgent) {
		a.config.ToolCallingModel = model
	}
}

// WithGraphName sets the graph name for debugging and logging
func WithGraphName(name string) Option {
	return func(a *ReACTAgent) {
		a.config.GraphName = name
	}
}
