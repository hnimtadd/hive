package bee

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"slices"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/bee/react"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
	hiveutils "github.com/hnimtadd/hive/pkg/utils"
)

type QueenBee interface {
	baseBee

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *types.HiveTask) (*QueenOutput, error)
}

type QueenOutput struct {
	Status     types.Status `json:"status"                jsonschema:"Task status - 'in_progress': completed this cycle, need another cycle to continue; 'paused': need user input before next cycle; 'completed': task finished successfully; 'failed': task cannot be completed"`
	Content    string       `json:"content"               jsonschema:"For in_progress: what you accomplished this cycle. For paused: question for user. For completed: final summary of results. For failed: reason for failure"`
	NextAction string       `json:"next_action,omitempty" jsonschema:"What you plan to do in the next execution cycle (primarily for in_progress status)"`
	Thought    string       `json:"-"`
}

type queen struct {
	id           string
	persona      string
	capabilities []string

	outputValidator *jsonschema.Resolved

	reactAgent *react.Agent
	config     *Config
}

func NewQueenBee(registry Registry, config *Config, agentOpts ...react.AgentOption) (QueenBee, error) {
	exploreTool := exploreTool(config)
	config.Tools = append(config.Tools,
		delegateTool(registry),
		exploreTool,
	)
	reactAgent, err := react.NewWithSystemPrompt(
		config.ID,
		config.LLM,
		config.Tools,
		config.Persona,
		config.MaxSteps,
		agentOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	schema, err := jsonschema.For[QueenOutput](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent output schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent schema: %w", err)
	}
	return &queen{
		id:              config.ID,
		reactAgent:      reactAgent,
		persona:         config.Persona,
		capabilities:    config.Capabilities,
		outputValidator: resolved,
		config:          config,
	}, nil
}

// Execute implements [QueenBee].
func (s *queen) Execute(ctx context.Context, task *types.HiveTask) (*QueenOutput, error) {
	logger := trace.Logger(ctx)
	logger.InfoContext(ctx, "queen execution started",
		slog.String("queen_id", s.id),
		slog.String("task_id", task.ID),
		slog.String("task_status", string(task.Status)),
	)

	retryConfig := errors.RetryConfig{
		MaxAttempts:   s.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := task.JSONString()
	if err != nil {
		logger.ErrorContext(ctx, "failed to serialize task to JSON", slog.Any("error", err))
		return nil, fmt.Errorf("task could be tranlsated to JSON: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.config.TimeoutInSec)*time.Second)
	defer cancel()

	handler := errors.NewErrorHandler[*QueenOutput]()
	msgs := []*schema.Message{schema.UserMessage(taskDescription)}
	msg, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*QueenOutput, error) {
		trace.Logger(ctx).Debug("queen executing", slog.Int("message_count", len(msgs)))
		// Execute the task using the ReACT agent
		result, execErr := s.reactAgent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
			trace.Logger(ctx).Error("queen ReACT execution failed", slog.Any("error", execErr))
			return nil, execErr
		}

		content := func() string {
			if len(result.MultiContent) != 0 {
				for content := range slices.Values(result.MultiContent) {
					switch content.Type {
					case schema.ChatMessagePartTypeText:
						return content.Text
						// process agent output update
					default:
						continue
					}
				}
			}
			return result.Content
		}()
		msgs = append(msgs, result)

		content, err = hiveutils.HeristicallyExtractJSONString(content)
		if err != nil {
			trace.Logger(ctx).Debug("failed to extract JSON string", slog.Any("error", err))
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not a valid JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output ot agent ouptut schema", err)
		}
		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			trace.Logger(ctx).Debug("failed to unmarshal JSON string", slog.Any("error", err))
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to map output to an object"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output ot agent ouptut schema", err)
		}

		if err = s.outputValidator.Validate(output); err != nil {
			trace.Logger(ctx).Debug("failed to validate JSON against output schema", slog.Any("error", err))
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not follow schema"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err)
		}
		agentOutput := QueenOutput{}
		if err = json.Unmarshal([]byte(content), &agentOutput); err != nil {
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to parse output JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err)
		}
		if len(result.ReasoningContent) > 0 {
			agentOutput.Thought += fmt.Sprintf("system thought: %s", result.ReasoningContent)
		}

		switch agentOutput.Status {
		case types.TaskStatusCompleted,
			types.TaskStatusFailed,
			types.TaskStatusInProgress,
			types.TaskStatusPaused:
			task.Status = agentOutput.Status
		default:
			msgs = append(
				msgs,
				schema.UserMessage(
					fmt.Sprintf(
						"invalid task output status: %s. Valid statuses are: 'in_progress' (need another execution cycle), 'paused' (need user input), 'completed' (task done), 'failed' (cannot complete)",
						agentOutput.Status,
					),
				),
			)
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid task status", nil)
		}
		return &agentOutput, nil
	})

	if err != nil {
		logger.ErrorContext(ctx, "queen execution failed",
			slog.String("task_id", task.ID),
			slog.Any("error", err),
		)
	} else {
		logger.InfoContext(ctx, "queen execution completed",
			slog.String("task_id", task.ID),
			slog.String("status", string(msg.Status)),
		)
	}

	return msg, err
}

// Description implements [QueenBee].
func (s *queen) Description() string {
	return s.config.Description
}

// GetID implements [QueenBee].
func (s *queen) GetID() string {
	return s.id
}

type delegateTaskInput struct {
	WorkerInput

	ID string `json:"agent_id"`
}

func delegateTool(registry Registry) tool.InvokableTool {
	// Manually define ToolInfo
	toolInfo := &schema.ToolInfo{
		Name: "delegate_agent",
		Desc: "Look up if an agent with ID is exists and delegate task to that agent",
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
			i := &WorkerInput{
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
			outputJSON, err := json.Marshal(output)
			if err != nil {
				return &schema.ToolResult{
					Parts: []schema.ToolOutputPart{
						{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to convert output to a json: %v", err)},
					},
				}, nil
			}

			return &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: string(outputJSON)}}}, nil
		},
	)
}
