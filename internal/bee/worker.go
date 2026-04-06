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
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
)

// WorkerBee defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to hashicorp gRPC plugins later.
type WorkerBee interface {
	baseBee

	// CanHandle determines if this agent can process the given task
	CanHandle(task *WorkerInput) bool

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *WorkerInput) (*WorkerOutput, error)
}

type WorkerOutput struct {
	Status       types.Status      `json:"status"               jsonschema:"Updated job state, either: not_started, in_progress, completed, failed, paused"`
	Observations string            `json:"observations"         jsonschema:"What did you find? This will be added to history."`
	NewArtifacts map[string]string `json:"new_artifacts"        jsonschema:"Any data found (e.g., ticket_details, log_snippet)"`
	NextSteps    string            `json:"next_steps,omitempty" jsonschema:"Optional suggestion for the supervisor"`
}
type WorkerInput struct {
	Context   string            `json:"status"    jsonschema:"High-level goal for the entire run"`
	Task      string            `json:"task"      jsonschema:"The exact instruction from the supervisor"`
	Artifacts map[string]string `json:"artifacts" jsonschema:"specfic data relevant to your task"`
}

type bee struct {
	id           string
	persona      string
	description  string
	capabilities []string

	outputValidator *jsonschema.Resolved

	agent *react.Agent

	config *Config
}

func NewCustomBee(config *Config, agentOpts ...react.AgentOption) (WorkerBee, error) {
	systemPrompt, err := getSystemPrompt(config.Persona)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}
	reactAgent, err := react.NewWithSystemPrompt(
		config.ID,
		config.LLM,
		config.Tools,
		systemPrompt,
		config.MaxSteps,
		agentOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	schema, err := jsonschema.For[WorkerOutput](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent output schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent schema: %w", err)
	}

	return &bee{
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
func (a *bee) CanHandle(_ *WorkerInput) bool {
	return true
}

// Description implements [WorkerBee].
func (a *bee) Description() string {
	return a.description
}

func (a *bee) Capabilities() []string {
	return a.capabilities
}

// Execute implements [WorkerBee].
func (a *bee) Execute(ctx context.Context, input *WorkerInput) (*WorkerOutput, error) {
	logger := trace.Logger(ctx)
	logger.InfoContext(ctx,
		"worker execution started",
		slog.String("agent_id", a.id),
		slog.String("task", input.Task),
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
	handler := errors.NewErrorHandler[*WorkerOutput]()
	msgs := []*schema.Message{schema.UserMessage(string(taskDescription))}
	output, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*WorkerOutput, error) {
		trace.Logger(ctx).Debug("worker executing", slog.String("agent_id", a.id))
		// Execute the task using the ReACT agent
		result, execErr := a.agent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
			trace.Logger(ctx).Error("worker ReACT execution failed", slog.Any("error", execErr))
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
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("output is not a valid JSON: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output ot agent ouptut schema", err)
		}
		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output ot agent ouptut schema", err)
		}
		if err = a.outputValidator.Validate(output); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("output is not followed JSON schema: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to validate output schema", err)
		}
		agentOutput := WorkerOutput{}
		if err = json.Unmarshal([]byte(content), &agentOutput); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output ot agent output schema", err)
		}
		return &agentOutput, nil
	})

	if err != nil {
		logger.ErrorContext(ctx, "worker execution failed",
			slog.String("agent_id", a.id),
			slog.Any("error", err),
		)
	} else {
		logger.InfoContext(ctx, "worker execution completed",
			slog.String("agent_id", a.id),
			slog.String("status", string(output.Status)),
		)
	}

	return output, err
}

func getSystemPrompt(persona string) (string, error) {
	inputDescription, err := utils.DescribeJSONSchema[WorkerInput]()
	if err != nil {
		return "", fmt.Errorf("failed to self describe the input: %w", err)
	}
	outputDescription, err := utils.DescribeJSONSchema[WorkerOutput]()
	if err != nil {
		return "", fmt.Errorf("failed to self describe the output: %w", err)
	}
	return fmt.Sprintf(`%s
		You suppose to handle input and output with these specific formats
		========= INPUT ========
		YOU ONLY RECEIVE THIS JSON ONLY AS INPUT
		%s

		========= OUTPUT ======
		YOU HAVE TO RESPONSE A RAW JSON THAT FOLLOWS THIS SCHEMA
		%s

		`, persona, inputDescription, outputDescription), nil
}

// GetID implements [WorkerBee].
func (a *bee) GetID() string {
	return a.id
}
