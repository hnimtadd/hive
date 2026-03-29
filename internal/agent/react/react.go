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
	"github.com/hnimtadd/hive/internal/trace"
)

// Agent is a simplified ReACT agent wrapper.
type Agent struct {
	id           string
	agent        *einoreact.Agent
	systemPrompt string
	history      []*schema.Message
}

// NewWithSystemPrompt creates a new ReACT agent with a system prompt.
func NewWithSystemPrompt(id string, chatModel model.ToolCallingChatModel, tools []tool.InvokableTool, systemPrompt string, maxStep int) (*Agent, error) {
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
	config.ToolsConfig.ToolCallMiddlewares = append(config.ToolsConfig.ToolCallMiddlewares, compose.ToolMiddleware{
		Invokable: func(ite compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				trace.Logger(ctx).Info("tool invocation started",
					slog.String("tool", input.Name),
					slog.String("call_id", input.CallID),
					slog.Int("args_length", len(input.Arguments)),
				)

				output, err := ite(ctx, input)

				if err != nil {
					trace.Logger(ctx).Error("tool invocation failed",
						slog.String("tool", input.Name),
						slog.String("call_id", input.CallID),
						slog.Any("error", err),
					)
					return output, err
				}

				trace.Logger(ctx).Info("tool invocation completed",
					slog.String("tool", input.Name),
					slog.String("call_id", input.CallID),
					slog.Int("output_length", len(output.Result)),
				)
				return output, nil
			}
		},
	})

	// Create Eino's ReACT agent
	ctx := context.Background()
	agent, err := einoreact.NewAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	return &Agent{
		id:           id,
		systemPrompt: systemPrompt,
		history:      []*schema.Message{},
		agent:        agent,
	}, nil
}

// ExecuteWithMessages runs the ReACT agent with conversation history.
func (a *Agent) ExecuteWithMessages(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	trace.Logger(ctx).Debug("ReACT agent generating",
		slog.String("agent_id", a.id),
		slog.Int("message_count", len(messages)),
	)

	result, err := a.agent.Generate(ctx, messages)

	if err != nil {
		trace.Logger(ctx).Error("ReACT generation failed",
			slog.String("agent_id", a.id),
			slog.Any("error", err),
		)
	} else {
		trace.Logger(ctx).Debug("ReACT generation completed",
			slog.String("agent_id", a.id),
			slog.Int("response_length", len(result.Content)),
		)
	}

	return result, err
}

// Execute runs the ReACT agent with stateful message
func (a *Agent) Execute(ctx context.Context, message string) (*schema.Message, error) {
	a.history = append(a.history, schema.UserMessage(message))
	return a.agent.Generate(ctx, a.history)
}

// ID returns the agent ID.
func (a *Agent) ID() string {
	return a.id
}
