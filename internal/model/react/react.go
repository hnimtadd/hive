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

// Model is a simplified ReACT agent wrapper.
type Model struct {
	id           string
	agent        *einoreact.Agent
	systemPrompt string
	history      []*schema.Message
}

// ToolExecutionEvent represents a tool execution lifecycle event.
type ToolExecutionEvent struct {
	AgentID   string
	ToolName  string
	CallID    string
	EventType ToolEventType
	Input     string // JSON-encoded arguments
	Output    string // JSON-encoded result
	Error     error
}

// ToolEventType represents the stage of tool execution.
type ToolEventType string

const (
	ToolEventStarted   ToolEventType = "started"
	ToolEventCompleted ToolEventType = "completed"
	ToolEventFailed    ToolEventType = "failed"
)

// AgentOption configures the agent during creation.
type AgentOption func(*Model)

// NewWithSystemPrompt creates a new ReACT agent with a system prompt.
func NewWithSystemPrompt(id string, chatModel model.ToolCallingChatModel, tools []tool.InvokableTool, systemPrompt string, maxStep int, opts ...AgentOption) (*Model, error) {
	// Create agent with default values
	a := &Model{
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
func (a *Model) ExecuteWithMessages(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
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
func (a *Model) Execute(ctx context.Context, message string) (*schema.Message, error) {
	a.history = append(a.history, schema.UserMessage(message))
	return a.agent.Generate(ctx, a.history)
}

// ID returns the agent ID.
func (a *Model) ID() string {
	return a.id
}

func (a *Model) invokableToolMiddleware() compose.InvokableToolMiddleware {
	return func(handler compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
		return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
			mw, isInjected := MiddlewareFromContext(ctx)
			if !isInjected {
				return handler(ctx, input)
			}

			// Fire STARTED event to custom middlewares
			startEvent := &ToolExecutionEvent{
				AgentID:   a.id,
				ToolName:  input.Name,
				CallID:    input.CallID,
				EventType: ToolEventStarted,
				Input:     input.Arguments,
			}

			if err := mw(ctx, startEvent); err != nil {
				trace.Logger(ctx).ErrorContext(ctx, "middleware error on tool start",
					slog.String("tool", input.Name),
					slog.Any("error", err))
			}

			// Execute the tool
			output, err := handler(ctx, input)
			if err != nil {
				failedEvent := &ToolExecutionEvent{
					AgentID:   a.id,
					ToolName:  input.Name,
					CallID:    input.CallID,
					EventType: ToolEventFailed,
					Input:     input.Arguments,
					Error:     err,
				}

				if mwErr := mw(ctx, failedEvent); mwErr != nil {
					trace.Logger(ctx).ErrorContext(ctx, "middleware error on tool failure",
						slog.String("tool", input.Name),
						slog.Any("error", mwErr))
				}

				return output, err
			}

			return output, nil
		}
	}
}
