package bee

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
)

type SupervisorBee interface {
	BaseBee

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *types.HiveTask) (*SupervisorOutput, error)
}

type SupervisorOutput struct {
	Status     types.Status `json:"status"                jsonschema:"Current state: not_started, in_progress, completed, failed, pause"`
	Content    string       `json:"content"               jsonschema:"The final answer or message for the user"`
	NextAction string       `json:"next_action,omitempty" jsonschema:"next action you want to do"`
	Thought    string       `json:"-"`
}

type supervisor struct {
	id           string
	persona      string
	capabilities []string

	outputValidator *jsonschema.Resolved

	reactAgent *react.Agent
	config     *Config
}

// Capabilities implements [SupervisorBee].
func (s *supervisor) Capabilities() []string {
	panic("unimplemented")
}

func NewSupervisorAgent(config *Config) (SupervisorBee, error) {
	reactAgent, err := react.NewWithSystemPrompt(
		config.ID,
		config.LLM,
		config.Tools,
		config.Persona,
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
		reactAgent:      reactAgent,
		persona:         config.Persona,
		capabilities:    config.Capabilities,
		outputValidator: resolved,
		config:          config,
	}, nil
}

// Execute implements [SupervisorBee].
func (s *supervisor) Execute(ctx context.Context, task *types.HiveTask) (*SupervisorOutput, error) {
	retryConfig := errors.RetryConfig{
		MaxAttempts:   s.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := task.JSONString()
	if err != nil {
		return nil, fmt.Errorf("task could be tranlsated to JSON: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.config.TimeoutInSec)*time.Second)
	defer cancel()

	handler := errors.NewErrorHandler[*SupervisorOutput]()
	msgs := []*schema.Message{schema.UserMessage(taskDescription)}
	msg, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*SupervisorOutput, error) {
		log.Println("supervisor: executing", msgs[len(msgs)-1].String())
		// Execute the task using the ReACT agent
		result, execErr := s.reactAgent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
			log.Println("execError", execErr)
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

		content, err = utils.HeristicallyExtractJSONString(content)
		if err != nil {
			log.Printf("failed to extract JSON string: %s\n", err)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not a valid JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output ot agent ouptut schema", err)
		}
		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			log.Printf("failed to unmarshal JSON string: %s\n", err)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to map output to an object"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output ot agent ouptut schema", err)
		}

		if err = s.outputValidator.Validate(output); err != nil {
			log.Printf("failed to validate JSON agains output schema: %s\n", err)
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not follow schema"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err)
		}
		agentOutput := SupervisorOutput{}
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
			types.TaskStatusNotStarted,
			types.TaskStatusPaused:
			task.Status = agentOutput.Status
		default:
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid task output status: %s", agentOutput.Status)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid task status", nil)
		}
		return &agentOutput, nil
	})
	return msg, err
}

// Description implements [SupervisorBee].
func (s *supervisor) Description() string {
	return s.config.Description
}

// GetID implements [SupervisorBee].
func (s *supervisor) GetID() string {
	return s.id
}

// GetType implements [SupervisorBee].
func (s *supervisor) GetType() string {
	panic("unimplemented")
}
