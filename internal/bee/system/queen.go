package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/bee/react"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
	hiveutils "github.com/hnimtadd/hive/pkg/utils"
)

type QueenBee interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// Description return a short self-description about agent capabilities.
	Description() string

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
	config     *bee.Config
}

func NewQueenBee(config *bee.Config, agentOpts ...react.AgentOption) (QueenBee, error) {
	reactAgent, err := react.New(react.Config{
		ID:           config.ID,
		ChatModel:    config.LLM,
		Tools:        config.Tools,
		SystemPrompt: config.Persona,
		MaxStep:      config.MaxSteps,
	}, agentOpts...)
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

		// Track validation failures for observability
		validationErrors := make([]string, 0)

		content, err = hiveutils.HeristicallyExtractJSONString(content)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("JSON_EXTRACTION: %s", err))
			logger.ErrorContext(ctx, "validation failed - raw output preserved for debugging",
				slog.String("queen_id", s.id),
				slog.String("error_type", "JSON_EXTRACTION"),
				slog.String("error", err.Error()),
				slog.String("raw_output", result.Content),
				slog.Int("raw_output_length", len(result.Content)),
			)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not a valid JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output to agent output schema", err).
				WithContext("raw_output", result.Content)
		}

		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("JSON_PARSE: %s", err))
			logger.ErrorContext(ctx, "validation failed - extracted content preserved",
				slog.String("queen_id", s.id),
				slog.String("error_type", "JSON_PARSE"),
				slog.String("error", err.Error()),
				slog.String("extracted_content", content),
				slog.String("raw_output", result.Content),
			)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to map output to an object"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output to agent output schema", err).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		if err = s.outputValidator.Validate(output); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("SCHEMA_VALIDATION: %s", err))
			// Dump the full output structure for debugging
			outputJSON, _ := json.Marshal(output)
			logger.ErrorContext(ctx, "validation failed - schema mismatch",
				slog.String("queen_id", s.id),
				slog.String("error_type", "SCHEMA_VALIDATION"),
				slog.String("error", err.Error()),
				slog.String("parsed_output", string(outputJSON)),
				slog.String("extracted_content", content),
				slog.String("raw_output", result.Content),
			)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not follow schema"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err).
				WithContext("parsed_output", output).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		agentOutput := QueenOutput{}
		if err = json.Unmarshal([]byte(content), &agentOutput); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("TYPE_UNMARSHAL: %s", err))
			logger.ErrorContext(ctx, "validation failed - type unmarshaling error",
				slog.String("queen_id", s.id),
				slog.String("error_type", "TYPE_UNMARSHAL"),
				slog.String("error", err.Error()),
				slog.String("content", content),
			)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to parse output JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err).
				WithContext("content", content)
		}

		// Log successful validation for debugging
		if len(validationErrors) > 0 {
			logger.DebugContext(ctx, "output recovered after validation attempts",
				slog.String("queen_id", s.id),
				slog.Int("attempts", len(validationErrors)),
				slog.Any("errors_encountered", validationErrors),
			)
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
