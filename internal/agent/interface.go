package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
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
	Execute(ctx context.Context, task *types.HiveTask) error

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

	agent *react.Agent
}

func NewAgent(config *Config) (HiveAgent, error) {
	reactAgent, err := react.NewWithSystemPrompt(
		config.ID, config.LLM, config.Tools, config.Description,
		config.MaxSteps,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init ReACT agent: %w", err)
	}
	return &agent{
		id:           config.ID,
		agent:        reactAgent,
		prompt:       config.Description,
		timeout:      config.Timeout,
		maxTasks:     config.MaxTasks,
		capabilities: config.Capabilities,
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
func (a *agent) Execute(ctx context.Context, task *types.HiveTask) error {
	retryConfig := errors.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	handler := errors.NewErrorHandler()
	_, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (any, error) {
		// Execute the task using the ReACT agent
		result, execErr := a.agent.Execute(ctx, task.Description)
		if execErr != nil {
			return nil, execErr
		}

		_ = result
		return "", nil
	})
	return err
}

// GetID implements [HiveAgent].
func (a *agent) GetID() string {
	panic("unimplemented")
}

// GetType implements [HiveAgent].
func (a *agent) GetType() string {
	panic("unimplemented")
}

// Validate implements [HiveAgent].
func (a *agent) Validate(task *types.HiveTask) error {
	panic("unimplemented")
}
