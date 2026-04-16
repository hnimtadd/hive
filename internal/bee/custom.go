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
	systemPrompt    string

	// agent *react.Agent

	config *Config
}

func NewCustomBee[I, O any](config *Config) (CustomBee[I, O], error) {
	systemPrompt, err := GetSystemPrompt[I, O](config.Persona)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
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
		systemPrompt:    systemPrompt,
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

		reactAgent, err := react.New(
			react.Config{
				ID:           a.config.ID,
				ChatModel:    a.config.ModelPool(),
				Tools:        a.config.Tools,
				SystemPrompt: a.systemPrompt,
				MaxStep:      a.config.MaxSteps,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
		}
		// Execute the task using the ReACT agent
		result, execErr := reactAgent.ExecuteWithMessages(ctx, msgs)
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
			errorMsg := "ERROR: Your output is not valid JSON. You must respond with ONLY a JSON object.\n\n" +
				"Your output was:\n" + result.Content + "\n\n" +
				"Please respond with ONLY the JSON object, no markdown code blocks (no ``` json), no explanations, no extra text.\n\n" +
				"Example valid response:\n" +
				`{"summary": "Found 5 files", "relevant_files": [...]}`
			msgs = append(msgs, schema.SystemMessage(errorMsg))
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
			errorMsg := "ERROR: Your JSON is malformed and cannot be parsed.\n\n" +
				"Your JSON:\n" + content + "\n\n" +
				"Parse error: " + err.Error() + "\n\n" +
				"Please ensure:\n" +
				"- All strings are in double quotes\n" +
				"- No trailing commas\n" +
				"- Proper JSON syntax\n" +
				"- Arrays use [], objects use {}"
			msgs = append(msgs, schema.SystemMessage(errorMsg))
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
			errorMsg := "ERROR: Your JSON doesn't match the required schema.\n\n" +
				"Your output:\n" + content + "\n\n" +
				"Schema validation error: " + err.Error() + "\n\n" +
				"Please review the CRITICAL output format section in your system prompt and ensure your JSON output matches the exact schema required.\n\n" +
				"Tips:\n" +
				"- Check that all required fields are present\n" +
				"- Verify field names match exactly (case-sensitive)\n" +
				"- Ensure field types are correct (string, array, object)\n" +
				"- Remove any extra fields not in the schema"
			msgs = append(msgs, schema.SystemMessage(errorMsg))
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
			errorMsg := "ERROR: JSON structure is valid but field types are incorrect.\n\n" +
				"Your JSON:\n" + content + "\n\n" +
				"Type error: " + err.Error() + "\n\n" +
				"Common issues:\n" +
				"- Using numbers instead of strings (use \"123\" not 123)\n" +
				"- Using booleans instead of strings (use \"true\" not true)\n" +
				"- Missing quotes around string values\n" +
				"- Incorrect array/object structure\n\n" +
				"Ensure all field types match the schema exactly."
			msgs = append(msgs, schema.SystemMessage(errorMsg))
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
