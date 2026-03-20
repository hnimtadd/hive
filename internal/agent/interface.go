package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
)

type BaseAgent interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// GetType returns the type/category of this agent (e.g., "code_editor", "test_runner", "deployer")
	GetType() string

	// Description return a short self-description about agent capabilities.
	Description() string
}

// WorkerAgent defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to hashicorp gRPC plugins later.
type WorkerAgent interface {
	BaseAgent

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

// Config holds configuration for agent initialization.
type Config struct {
	ID           string   `json:"id"`
	MaxTasks     int      `json:"max_tasks"`
	Timeout      int      `json:"timeout_seconds"`
	Capabilities []string `json:"capabilities"`
	Description  string   `json:"description"`
	MaxSteps     int      `json:"max_steps"`

	Persona string `json:"persona"`

	LLM   model.ToolCallingChatModel `json:"-"`
	Tools []tool.InvokableTool       `json:"-"`
}

type agent struct {
	id           string
	prompt       string
	capabilities []string
	timeout      int
	maxTasks     int

	outputValidator *jsonschema.Resolved

	agent *react.Agent
}

type Output struct {
	Status       types.Status      `json:"status"               jsonschema:"Updated job state, either: not_started, in_progress, completed, failed, paused"`
	Observations string            `json:"observations"         jsonschema:"What did you find? This will be added to history."`
	NewArtifacts map[string]string `json:"new_artifacts"        jsonschema:"Any data found (e.g., ticket_details, log_snippet)"`
	NextSteps    string            `json:"next_steps,omitempty" jsonschema:"Optional suggestion for the supervisor"`
}
type Input struct {
	Context   string            `json:"status"    jsonschema:"High-level goal for the entire run"`
	Task      string            `json:"task"      jsonschema:"The exact instruction from the supervisor"`
	Artifacts map[string]string `json:"artifacts" jsonschema:"specfic data relevant to your task"`
}

func NewWorkerAgent(config *Config) (WorkerAgent, error) {
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
		prompt:          config.Description,
		timeout:         config.Timeout,
		maxTasks:        config.MaxTasks,
		capabilities:    config.Capabilities,
		outputValidator: resolved,
	}, nil
}

// CanHandle implements [WorkerAgent].
func (a *agent) CanHandle(_ *Input) bool {
	return true
}

// Description implements [WorkerAgent].
func (a *agent) Description() string {
	return a.prompt
}

// Execute implements [WorkerAgent].
func (a *agent) Execute(ctx context.Context, input *Input) (*Output, error) {
	retryConfig := errors.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	handler := errors.NewErrorHandler[*Output]()
	msgs := []*schema.Message{schema.UserMessage(string(taskDescription))}
	return handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*Output, error) {
		log.Println("agent receive:", string(taskDescription))
		fmt.Println(msgs)
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

func getSystemPrompt() (string, error) {
	inputDescription, err := utils.DescribeJSONSchema[Input]()
	if err != nil {
		return "", fmt.Errorf("Failed to self describe the input: %w", err)
	}
	outputDescription, err := utils.DescribeJSONSchema[Output]()
	if err != nil {
		return "", fmt.Errorf("Failed to self describe the output: %w", err)
	}
	return fmt.Sprintf(`\nAs a worker agent type, you suppose to handle input and output with these specific formats
		========= INPUT ========
		YOU ONLY RECEIVE THIS JSON ONLY AS INPUT
		%s

		========= OUTPUT ======
		YOU HAVE TO RESPONSE A RAW JSON THAT FOLLOWS THIS SCHEMA
		%s

		`, inputDescription, outputDescription), nil
}

// GetID implements [WorkerAgent].
func (a *agent) GetID() string {
	return a.id
}

// GetType implements [WorkerAgent].
func (a *agent) GetType() string {
	panic("unimplemented")
}

// Validate implements [WorkerAgent].
func (a *agent) Validate(input Input) error {
	return nil
}
