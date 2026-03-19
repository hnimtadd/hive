package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
)

type SupervisorAgent interface {
	BaseAgent

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *types.HiveTask) (*SupervisorOutput, error)
}

type SupervisorOutput struct {
	Status     string `json:"status"                jsonschema:"Current state: RUNNING, FINISHED, INTERRUPT, FAILED"`
	Content    string `json:"content"               jsonschema:"The final answer or message for the user"`
	NextAction string `json:"next_action,omitempty" jsonschema:"next action you want to do"`
	Thought    string `json:"-"`
}

type supervisor struct {
	id           string
	prompt       string
	capabilities []string
	timeout      int
	maxTasks     int

	outputValidator *jsonschema.Resolved

	agent *react.Agent
}

func NewSupervisorAgent(config *Config) (SupervisorAgent, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}
	reactAgent, err := react.NewWithSystemPrompt(
		config.ID,
		config.LLM,
		config.Tools,
		config.Description+systemPrompt,
		config.MaxSteps,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	schema, err := jsonschema.For[SupervisorOutput](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent output schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent schema: %w", err)
	}
	return &supervisor{
		id:              config.ID,
		agent:           reactAgent,
		prompt:          config.Description,
		timeout:         config.Timeout,
		maxTasks:        config.MaxTasks,
		capabilities:    config.Capabilities,
		outputValidator: resolved,
	}, nil
}

// Execute implements [SupervisorAgent].
func (s *supervisor) Execute(ctx context.Context, task *types.HiveTask) (*SupervisorOutput, error) {
	retryConfig := errors.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription := task.JSONString()
	handler := errors.NewErrorHandler[*SupervisorOutput]()
	msgs := []*schema.Message{schema.UserMessage(string(taskDescription))}
	msg, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*SupervisorOutput, error) {
		// Execute the task using the ReACT agent
		result, execErr := s.agent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
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
		var output map[string]any
		if err := json.Unmarshal([]byte(content), &output); err != nil {
			msgs = append(msgs, schema.SystemMessage(fmt.Sprintf("failed to parse output ot agent ouptut schema: %s", err)))
			return nil, fmt.Errorf("failed to parse output ot agent ouptut schema: %w", err)
		}
		if err := s.outputValidator.Validate(output); err != nil {
			msgs = append(msgs, schema.SystemMessage(fmt.Sprintf("failed to validate output schema: %s", err)))
			return nil, fmt.Errorf("failed to validate output schema: %w", err)
		}
		agentOutput := SupervisorOutput{}
		if err := json.Unmarshal([]byte(content), &agentOutput); err != nil {
			msgs = append(msgs, schema.SystemMessage(fmt.Sprintf("failed to parse output ot agent output schema: %s", err)))
			return nil, fmt.Errorf("failed to parse output ot agent output schema: %w", err)
		}
		agentOutput.Thought = result.ReasoningContent
		return &agentOutput, nil
	})
	return msg, err
}

// Description implements [SupervisorAgent].
func (s *supervisor) Description() string {
	return s.Description()
}

// GetID implements [SupervisorAgent].
func (s *supervisor) GetID() string {
	return s.id
	panic("unimplemented")
}

// GetType implements [SupervisorAgent].
func (s *supervisor) GetType() string {
	panic("unimplemented")
}
