package react

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoreact "github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/types"
	"github.com/samber/lo"
)

// Agent is a simplified ReACT agent wrapper.
type Agent struct {
	id           string
	agent        *einoreact.Agent
	systemPrompt string
	history      []*schema.Message
}

type Config struct {
	ID           string
	ChatModel    model.ToolCallingChatModel
	Tools        []tool.InvokableTool
	SystemPrompt string
	MaxStep      int
}

// New creates a new ReACT agent with a system prompt.
func New(cfg Config) (*Agent, error) {
	// Create agent with default values
	a := &Agent{
		id:           cfg.ID,
		systemPrompt: cfg.SystemPrompt,
		history:      []*schema.Message{},
	}

	config := &einoreact.AgentConfig{
		ToolCallingModel: cfg.ChatModel,
		MaxStep:          cfg.MaxStep,
		MessageModifier: func(_ context.Context, input []*schema.Message) []*schema.Message {
			if len(input) > 0 && input[0].Role != schema.System {
				return append([]*schema.Message{schema.SystemMessage(cfg.SystemPrompt)}, input...)
			}
			return input
		},
	}

	// Configure tools if provided
	if len(cfg.Tools) > 0 {
		baseTools := make([]tool.BaseTool, len(cfg.Tools))
		for i, t := range cfg.Tools {
			baseTools[i] = t
		}
		config.ToolsConfig = compose.ToolsNodeConfig{
			Tools: baseTools,
		}
	}

	// Add tool execution middleware that executes custom middlewares
	config.ToolsConfig.ToolCallMiddlewares = append(config.ToolsConfig.ToolCallMiddlewares, a.hiveMiddleware())

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
	ctx, _ = observability.ContextWithChildSpan(ctx)
	observability.Logger(ctx).DebugContext(ctx, "ReACT agent generating",
		slog.String("agent_id", a.id),
		slog.Int("message_count", len(messages)),
	)
	middleware := middleware.MiddlewareFromContext(ctx)
	middleware.OnRequest(ctx, a.id, types.LLMRequest{
		Input: messages[len(messages)-1].Content,
	})
	start := time.Now()

	result, err := a.agent.Generate(ctx, messages)
	if err != nil && !errors.Is(err, context.Canceled) {
		observability.Logger(ctx).ErrorContext(ctx, "ReACT generation failed",
			slog.String("agent_id", a.id),
			slog.Any("error", err),
		)
	}
	if err == nil {
		observability.Logger(ctx).DebugContext(ctx, "ReACT generation completed",
			slog.String("agent_id", a.id),
			slog.Int("response_length", len(result.Content)),
		)
	}
	if result != nil {
		toolCalls := lo.Reduce(
			result.ToolCalls,
			func(tools []string, toolCall schema.ToolCall, _ int) []string {
				if toolCall.Type == "function" {
					return append(tools, toolCall.Function.Name)
				}
				return tools
			}, []string{})

		middleware.OnResponse(ctx, a.id, types.LLMResponse{
			Output:       result.Content,
			ToolCalls:    toolCalls,
			FinishReason: result.ResponseMeta.FinishReason,
			TokenUsed: types.TokenUsage{
				PromptTokens:     result.ResponseMeta.Usage.PromptTokens,
				TotalTokens:      result.ResponseMeta.Usage.TotalTokens,
				CompletionTokens: result.ResponseMeta.Usage.CompletionTokens,
			},
			ExecutionTimeMs: int(time.Since(start).Milliseconds()),
		})
	}

	return result, err
}

// ID returns the agent ID.
func (a *Agent) ID() string {
	return a.id
}

func (a *Agent) hiveMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				mw := middleware.MiddlewareFromContext(ctx)
				ctx, _ = observability.ContextWithChildSpan(ctx)
				toolCall := types.ToolCallRequest{
					ToolName:  input.Name,
					Arguments: input.Arguments,
					CallID:    input.CallID,
				}
				mw.OnToolCall(ctx, a.id, toolCall)
				start := time.Now()

				// Execute the tool
				output, err := next(ctx, input)
				tr := types.ToolCallResponse{
					CallID:          input.CallID,
					ExecutionTimeMs: int(time.Since(start).Milliseconds()),
				}
				if err != nil {
					// Fire STARTED event to custom middlewares
					tr.Succeed = false
					tr.Error = err
				} else {
					tr.Succeed = true
					tr.Output = output.Result
				}
				// Fire FINISH event to custom middlewares
				mw.OnToolCallResponse(ctx, a.id, tr)
				return output, nil
			}
		},
		EnhancedInvokable: func(next compose.EnhancedInvokableToolEndpoint) compose.EnhancedInvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.EnhancedInvokableToolOutput, error) {
				mw := middleware.MiddlewareFromContext(ctx)
				ctx, _ = observability.ContextWithChildSpan(ctx)
				toolCall := types.ToolCallRequest{
					ToolName:  input.Name,
					Arguments: input.Arguments,
					CallID:    input.CallID,
				}
				mw.OnToolCall(ctx, a.id, toolCall)
				start := time.Now()

				// Execute the tool
				output, err := next(ctx, input)
				tr := types.ToolCallResponse{
					CallID:          input.CallID,
					ExecutionTimeMs: int(time.Since(start).Milliseconds()),
				}
				if err != nil {
					// Fire STARTED event to custom middlewares
					tr.Succeed = false
					tr.Error = err
				} else {
					tr.Succeed = true
					tr.Output = output.Result.Parts[len(output.Result.Parts)-1].Text
				}

				// Fire event to custom middlewares
				mw.OnToolCallResponse(ctx, a.id, tr)
				return output, err
			}
		},
		EnhancedStreamable: func(next compose.EnhancedStreamableToolEndpoint) compose.EnhancedStreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.EnhancedStreamableToolOutput, error) {
				mw := middleware.MiddlewareFromContext(ctx)
				ctx, _ = observability.ContextWithChildSpan(ctx)
				toolCall := types.ToolCallRequest{
					ToolName:  input.Name,
					Arguments: input.Arguments,
					CallID:    input.CallID,
				}
				mw.OnToolCall(ctx, a.id, toolCall)
				start := time.Now()

				// Execute the tool
				output, err := next(ctx, input)
				go func(output compose.EnhancedStreamableToolOutput, err error) {
					tr := types.ToolCallResponse{
						CallID:          input.CallID,
						ExecutionTimeMs: int(time.Since(start).Milliseconds()),
					}
					if err != nil {
						// Fire FINISH event to custom middlewares
						tr.Succeed = false
						tr.Error = err
						// Fire STARTED event to custom middlewares
						mw.OnToolCallResponse(ctx, a.id, tr)
						return
					}
					tr.Succeed = true
					defer output.Result.Close()
					for {
						var chunk *schema.ToolResult
						chunk, err = output.Result.Recv()
						if errors.Is(err, io.EOF) {
							break
						}
						tr.Output += chunk.Parts[len(chunk.Parts)-1].Text
					}

					// Fire event to custom middlewares
					mw.OnToolCallResponse(ctx, a.id, tr)
				}(*output, err)
				return output, err
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
				mw := middleware.MiddlewareFromContext(ctx)
				ctx, _ = observability.ContextWithChildSpan(ctx)
				toolCall := types.ToolCallRequest{
					ToolName:  input.Name,
					Arguments: input.Arguments,
					CallID:    input.CallID,
				}
				// Fire event to custom middlewares
				mw.OnToolCall(ctx, a.id, toolCall)
				start := time.Now()

				// Execute the tool
				output, err := next(ctx, input)
				go func(output compose.StreamToolOutput, err error) {
					tr := types.ToolCallResponse{
						CallID:          input.CallID,
						ExecutionTimeMs: int(time.Since(start).Milliseconds()),
					}
					if err != nil {
						// Fire STARTED event to custom middlewares
						tr.Succeed = false
						tr.Error = err
						// Fire STARTED event to custom middlewares
						mw.OnToolCallResponse(ctx, a.id, tr)
						return
					}
					tr.Succeed = true
					defer output.Result.Close()
					var chunk string
					for {
						chunk, err = output.Result.Recv()
						if errors.Is(err, io.EOF) {
							break
						}
						tr.Output += chunk
					}
					// Fire event to custom middlewares
					mw.OnToolCallResponse(ctx, a.id, tr)
				}(*output, err)
				return output, err
			}
		},
	}
}
