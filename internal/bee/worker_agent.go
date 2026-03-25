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
	"github.com/hnimtadd/hive/pkg/utils"
)

// WorkerBee defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to hashicorp gRPC plugins later.
type WorkerBee interface {
	BaseBee

	// CanHandle determines if this agent can process the given task
	CanHandle(task *Input) bool

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *Input) (*Output, error)

	// Validate performs pre-execution validation of the task
	// Returns error if task cannot be executed due to invalid parameters
	Validate(task Input) error
}

type agent struct {
	id           string
	persona      string
	description  string
	capabilities []string
	timeout      int
	maxTasks     int

	outputValidator *jsonschema.Resolved

	agent *react.Agent

	config *Config
}

func NewWorkerBee(config *Config) (WorkerBee, error) {
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
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	schema, err := jsonschema.For[Output](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent output schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent schema: %w", err)
	}

	return &agent{
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
func (a *agent) CanHandle(_ *Input) bool {
	return true
}

// Description implements [WorkerBee].
func (a *agent) Description() string {
	return a.description
}

func (a *agent) Capabilities() []string {
	return a.capabilities
}

// Execute implements [WorkerBee].
func (a *agent) Execute(ctx context.Context, input *Input) (*Output, error) {
	retryConfig := errors.RetryConfig{
		MaxAttempts:   a.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(a.config.TimeoutInSec)*time.Second)
	defer cancel()
	handler := errors.NewErrorHandler[*Output]()
	msgs := []*schema.Message{schema.UserMessage(string(taskDescription))}
	return handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*Output, error) {
		log.Println("agent receive:", string(taskDescription))
		// Execute the task using the ReACT agent
		result, execErr := a.agent.ExecuteWithMessages(ctx, msgs)
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

		log.Println("agent output:", content)
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
		agentOutput := Output{}
		if err = json.Unmarshal([]byte(content), &agentOutput); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "failed to parse output ot agent output schema", err)
		}
		return &agentOutput, nil
	})
}

func getSystemPrompt(persona string) (string, error) {
	inputDescription, err := utils.DescribeJSONSchema[Input]()
	if err != nil {
		return "", fmt.Errorf("failed to self describe the input: %w", err)
	}
	outputDescription, err := utils.DescribeJSONSchema[Output]()
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
func (a *agent) GetID() string {
	return a.id
}

// GetType implements [WorkerBee].
func (a *agent) GetType() string {
	panic("unimplemented")
}

// Validate implements [WorkerBee].
func (a *agent) Validate(input Input) error {
	return nil
}
