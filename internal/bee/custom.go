package bee

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/bee/react"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/utils"
)

// CustomBee defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to hashicorp gRPC plugins later.
type CustomBee[I, O any] interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// Description return a short self-description about agent capabilities.
	Description() string

	// CanHandle determines if this agent can process the given task
	CanHandle(task *I) bool

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *I) (*O, error)
}

type customBee[I, O any] struct {
	id           string
	persona      string
	description  string
	capabilities []string

	outputValidator *jsonschema.Resolved

	agent *react.Agent

	config *Config
}

func NewCustomBee[I, O any](config *Config, agentOpts ...react.AgentOption) (CustomBee[I, O], error) {
	systemPrompt, err := GetSystemPrompt[I, O](config.Persona)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}
	reactAgent, err := react.New(
		react.Config{
			ID:           config.ID,
			ChatModel:    config.LLM,
			Tools:        config.Tools,
			SystemPrompt: systemPrompt,
			MaxStep:      config.MaxSteps,
		},
		agentOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	schema, err := jsonschema.For[O](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent output schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent schema: %w", err)
	}

	return &customBee[I, O]{
		id:              config.ID,
		agent:           reactAgent,
		persona:         config.Persona,
		description:     config.Description,
		capabilities:    config.Capabilities,
		outputValidator: resolved,
		config:          config,
	}, nil
}

// CanHandle implements [WorkerBee].
func (a *customBee[I, O]) CanHandle(_ *I) bool {
	return true
}

// Description implements [WorkerBee].
func (a *customBee[I, O]) Description() string {
	return a.description
}

func (a *customBee[I, O]) Capabilities() []string {
	return a.capabilities
}

// Execute implements [WorkerBee].
func (a *customBee[I, O]) Execute(ctx context.Context, input *I) (*O, error) {
	logger := trace.Logger(ctx)
	logger.InfoContext(ctx,
		"worker execution started",
		slog.String("agent_id", a.id),
		slog.Any("task", input),
	)

	retryConfig := errors.RetryConfig{
		MaxAttempts:   a.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := json.Marshal(input)
	if err != nil {
		logger.ErrorContext(ctx, "failed to marshal task input", slog.Any("error", err))
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(a.config.TimeoutInSec)*time.Second)
	defer cancel()
	handler := errors.NewErrorHandler[*O]()
	msgs := []*schema.Message{schema.UserMessage(string(taskDescription))}
	output, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*O, error) {
		logger.DebugContext(ctx, "worker executing", slog.String("agent_id", a.id))
		// Execute the task using the ReACT agent
		result, execErr := a.agent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
			logger.ErrorContext(ctx, "worker ReACT execution failed", slog.Any("error", execErr))
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

		trace.Logger(ctx).Debug("worker output received", slog.Int("content_length", len(content)))
		msgs = append(msgs, result)

		content, err = utils.HeristicallyExtractJSONString(content)
		if err != nil {
			logger.ErrorContext(ctx, "validation failed - raw output preserved for debugging",
				slog.String("agent_id", a.id),
				slog.String("error_type", "JSON_EXTRACTION"),
				slog.String("error", err.Error()),
				slog.String("raw_output", result.Content), // Log the actual output
				slog.Int("raw_output_length", len(result.Content)),
			)
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("output is not a valid JSON: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output to agent output schema", err).WithContext("raw_output", result.Content)
		}

		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			logger.ErrorContext(ctx, "validation failed - extracted content preserved",
				slog.String("agent_id", a.id),
				slog.String("error_type", "JSON_PARSE"),
				slog.String("error", err.Error()),
				slog.String("extracted_content", content),
				slog.String("raw_output", result.Content),
			)
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output to agent output schema", err).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		if err = a.outputValidator.Validate(output); err != nil {
			// Dump the full output structure for debugging
			outputJSON, _ := json.Marshal(output)
			logger.ErrorContext(ctx, "validation failed - schema mismatch",
				slog.String("agent_id", a.id),
				slog.String("error_type", "SCHEMA_VALIDATION"),
				slog.String("error", err.Error()),
				slog.String("parsed_output", string(outputJSON)),
				slog.String("extracted_content", content),
				slog.String("raw_output", result.Content),
			)
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("output is not followed JSON schema: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to validate output schema", err).
				WithContext("parsed_output", output).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		agentOutput := new(O)
		if err = json.Unmarshal([]byte(content), agentOutput); err != nil {
			logger.ErrorContext(ctx, "validation failed - type unmarshaling error",
				slog.String("agent_id", a.id),
				slog.String("error_type", "TYPE_UNMARSHAL"),
				slog.String("error", err.Error()),
				slog.String("content", content),
			)
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output to agent output schema", err).
				WithContext("content", content)
		}

		return agentOutput, nil
	})

	if err != nil {
		logger.ErrorContext(ctx, "worker execution failed",
			slog.String("agent_id", a.id),
			slog.Any("error", err),
			slog.Any("msgs", msgs),
		)
	} else {
		logger.InfoContext(ctx, "worker execution completed", slog.String("agent_id", a.id))
	}

	return output, err
}

// GetID implements [WorkerBee].
func (a *customBee[I, O]) GetID() string {
	return a.id
}
