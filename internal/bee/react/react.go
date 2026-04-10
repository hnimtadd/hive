package react

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoreact "github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/internal/types"
)

// Agent is a simplified ReACT agent wrapper.
type Agent struct {
	id           string
	agent        *einoreact.Agent
	systemPrompt string
	history      []*schema.Message
}

// AgentOption configures the agent during creation.
type AgentOption func(*Agent)

// NewWithSystemPrompt creates a new ReACT agent with a system prompt.
func NewWithSystemPrompt(id string, chatModel model.ToolCallingChatModel, tools []tool.InvokableTool, systemPrompt string, maxStep int, opts ...AgentOption) (*Agent, error) {
	// Create agent with default values
	a := &Agent{
		id:           id,
		systemPrompt: systemPrompt,
		history:      []*schema.Message{},
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}
	config := &einoreact.AgentConfig{
		ToolCallingModel: chatModel,
		MaxStep:          maxStep,
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

	// Add tool execution middleware that executes custom middlewares
	config.ToolsConfig.ToolCallMiddlewares = append(
		config.ToolsConfig.ToolCallMiddlewares,
		compose.ToolMiddleware{Invokable: a.invokableToolMiddleware()},
	)

	// Create Eino's ReACT agent
	ctx := context.Background()
	reactAgent, err := einoreact.NewAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	a.agent = reactAgent
	return a, nil
}

// ExecuteWithMessages runs the ReACT agent with conversation history.
func (a *Agent) ExecuteWithMessages(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	trace.Logger(ctx).DebugContext(ctx, "ReACT agent generating",
		slog.String("agent_id", a.id),
		slog.Int("message_count", len(messages)),
	)

	result, err := a.agent.Generate(ctx, messages)

	if err != nil {
		trace.Logger(ctx).ErrorContext(ctx, "ReACT generation failed",
			slog.String("agent_id", a.id),
			slog.Any("error", err),
		)
	} else {
		trace.Logger(ctx).DebugContext(ctx, "ReACT generation completed",
			slog.String("agent_id", a.id),
			slog.Int("response_length", len(result.Content)),
		)
	}

	return result, err
}

// Execute runs the ReACT agent with stateful message.
func (a *Agent) Execute(ctx context.Context, message string) (*schema.Message, error) {
	a.history = append(a.history, schema.UserMessage(message))
	return a.agent.Generate(ctx, a.history)
}

// ID returns the agent ID.
func (a *Agent) ID() string {
	return a.id
}

func (a *Agent) invokableToolMiddleware() compose.InvokableToolMiddleware {
	return func(handler compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
		return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
			mw, isInjected := middleware.GetMiddleware(ctx)
			if !isInjected {
				return handler(ctx, input)
			}
			// Fire STARTED event to custom middlewares
			mw.OnToolCall(ctx, a.id, input.Name, input.CallID, types.ToolEventStarted, input.Arguments, "", nil)

			// Execute the tool
			output, err := handler(ctx, input)
			if err != nil {
				// Fire STARTED event to custom middlewares
				mw.OnToolCall(ctx, a.id, input.Name, input.CallID, types.ToolEventFailed, input.Arguments, "", err)
				return output, err
			}

			mw.OnToolCall(ctx, a.id, input.Name, input.CallID, types.ToolEventCompleted, input.Arguments, output.Result, err)
			return output, nil
		}
	}
}
