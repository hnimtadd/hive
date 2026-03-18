package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
)

// HiveAgent defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to hashicorp gRPC plugins later.
type HiveAgent interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// GetType returns the type/category of this agent (e.g., "code_editor", "test_runner", "deployer")
	GetType() string

	// CanHandle determines if this agent can process the given task
	CanHandle(task *types.HiveTask) bool

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *types.HiveTask) (*schema.Message, error)

	// Validate performs pre-execution validation of the task
	// Returns error if task cannot be executed due to invalid parameters
	Validate(task *types.HiveTask) error

	// Description return a short self-description about agent capabilities.
	Description() string
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

type AgentOutput struct {
	Status       types.Status      `json:"status"               jsonschema:"Updated job state, either: not_started, in_progress, completed, failed, paused"`
	Observations string            `json:"observations"         jsonschema:"What did you find? This will be added to history."`
	NewArtifacts map[string]string `json:"new_artifacts"        jsonschema:"Any data found (e.g., ticket_details, log_snippet)"`
	NextSteps    string            `json:"next_steps,omitempty" jsonschema:"Optional suggestion for the supervisor"`
}

func NewAgent(config *Config) (HiveAgent, error) {
	reactAgent, err := react.NewWithSystemPrompt(
		config.ID,
		config.LLM,
		config.Tools,
		config.Description,
		config.MaxSteps,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	schema, err := jsonschema.For[AgentOutput](nil)
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

// CanHandle implements [HiveAgent].
func (a *agent) CanHandle(task *types.HiveTask) bool {
	return true
}

// Description implements [HiveAgent].
func (a *agent) Description() string {
	return a.prompt
}

// Execute implements [HiveAgent].
func (a *agent) Execute(ctx context.Context, task *types.HiveTask) (*schema.Message, error) {
	retryConfig := errors.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription := task.JSONString()
	handler := errors.NewErrorHandler[*schema.Message]()
	msgs := []*schema.Message{schema.UserMessage(taskDescription)}
	return handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*schema.Message, error) {
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
		var output map[string]any
		if err := json.Unmarshal([]byte(content), &output); err != nil {
			msgs = append(msgs, schema.SystemMessage(fmt.Sprintf("failed to parse output ot agent ouptut schema: %s", err)))
			return nil, fmt.Errorf("failed to parse output ot agent ouptut schema: %w", err)
		}
		if err := a.outputValidator.Validate(output); err != nil {
			msgs = append(msgs, schema.SystemMessage(fmt.Sprintf("failed to validate output schema: %s", err)))
			return nil, fmt.Errorf("failed to validate output schema: %w", err)
		}
		agentOutput := AgentOutput{}
		if err := json.Unmarshal([]byte(content), &agentOutput); err != nil {
			msgs = append(msgs, schema.SystemMessage(fmt.Sprintf("failed to parse output ot agent output schema: %s", err)))
			return nil, fmt.Errorf("failed to parse output ot agent output schema: %w", err)
		}
		maps.Copy(task.Artifacts, agentOutput.NewArtifacts)
		task.InternalThoughts = ""
		if len(agentOutput.Observations) != 0 {
			task.InternalThoughts += fmt.Sprintf("AGENT OBSERVATION:\n%s\n====", agentOutput.Observations)
		}
		if result.ReasoningContent != "" {
			task.InternalThoughts += fmt.Sprintf("AGENT REASONING:\n%s\n=====", result.ReasoningContent)
		}
		if len(agentOutput.NextSteps) != 0 {
			task.NextAction = &agentOutput.NextSteps
		}
		task.Status = agentOutput.Status
		return result, nil
	})
}

// GetID implements [HiveAgent].
func (a *agent) GetID() string {
	return a.id
}

// GetType implements [HiveAgent].
func (a *agent) GetType() string {
	panic("unimplemented")
}

// Validate implements [HiveAgent].
func (a *agent) Validate(task *types.HiveTask) error {
	return nil
}
