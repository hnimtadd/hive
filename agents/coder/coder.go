package coder

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
)

// CoderAgent is an enhanced AI-powered code editor that uses ReACT pattern
type CoderAgent struct {
	id           string
	reactAgent   *react.ReACTAgent
	tools        []tool.InvokableTool
	errorHandler *errors.ErrorHandler
	capabilities []string
}

// NewCoderAgent creates a new enhanced coder agent with ReACT capabilities
func NewCoderAgent(chatModel model.ToolCallingChatModel) (*CoderAgent, error) {
	if chatModel == nil {
		return nil, errors.ErrValidation("chat model is required")
	}

	// Create tools for the agent
	tools := []tool.InvokableTool{
		tools.NewThinkTool(),
		tools.NewFileReadTool(),
		tools.NewFileWriteTool(),
	}

	// Create ReACT agent with Eino
	agentID := "enhanced-coder-" + uuid.New().String()[:8]
	reactAgent, err := react.NewReACTAgent(
		agentID,
		chatModel,
		tools,
		react.WithSystemPrompt(getCoderSystemPrompt()),
		react.WithGraphName(fmt.Sprintf("coder-agent-%s", agentID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	return &CoderAgent{
		id:           agentID,
		reactAgent:   reactAgent,
		tools:        tools,
		errorHandler: errors.NewErrorHandler(),
		capabilities: []string{
			"react_reasoning",
			"file_operations",
			"code_analysis",
			"step_by_step_thinking",
			"error_recovery",
		},
	}, nil
}

// getCoderSystemPrompt returns a specialized system prompt for coding tasks
func getCoderSystemPrompt() string {
	return `You are an advanced AI coding assistant that uses a structured reasoning approach.

When given a coding task:

1. **Think** first: Analyze the requirements, plan your approach, and consider edge cases
2. **Act**: Use the available tools to read files, write code, and implement solutions
3. **Observe**: Review your work and iterate if needed

Available tools:
- think: Record your reasoning and analysis
- read_file: Read existing files to understand the codebase
- write_file: Create or modify files with your implementation

Guidelines:
- Always think through problems step-by-step
- Read existing code before making changes to understand context and patterns
- Write clean, well-documented, and tested code
- Follow existing code style and conventions
- Consider error handling and edge cases
- Test your implementations when possible

Be thorough but efficient. Ask for clarification if requirements are unclear.`
}

// GetID returns the agent's unique identifier
func (a *CoderAgent) GetID() string {
	return a.id
}

// GetType returns the agent type
func (a *CoderAgent) GetType() string {
	return "enhanced_coder"
}

// GetCapabilities returns the agent's capabilities
func (a *CoderAgent) GetCapabilities() []string {
	return a.capabilities
}

// CanHandle determines if this agent can process the given task
func (a *CoderAgent) CanHandle(task *types.HiveTask) bool {
	if task == nil {
		return false
	}

	goal := strings.ToLower(task.Goal)

	// Enhanced keyword detection for coding tasks
	codingKeywords := []string{
		"code", "implement", "write", "create", "build", "develop",
		"function", "method", "class", "struct", "interface",
		"fix", "debug", "refactor", "optimize", "test",
		"file", "read", "write", "analyze", "review",
		"algorithm", "logic", "feature", "module",
	}

	for _, keyword := range codingKeywords {
		if strings.Contains(goal, keyword) {
			return true
		}
	}

	return false
}

// Execute performs the coding task using ReACT pattern with error handling
func (a *CoderAgent) Execute(ctx context.Context, task *types.HiveTask) error {
	if task == nil {
		return errors.ErrValidation("task cannot be nil")
	}

	// Use error handler with retry for resilient execution
	retryConfig := errors.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500, // 500ms
		BackoffFactor: 2.0,
		MaxDelay:      5000, // 5s max delay
	}

	_, err := a.errorHandler.WithRetry(ctx, retryConfig, func(ctx context.Context) (any, error) {
		// Execute the task using the ReACT agent
		result, execErr := a.reactAgent.Run(ctx, task.Goal)
		if execErr != nil {
			return nil, execErr
		}

		// Store result in task (assuming task has a result field)
		// This would need to be adapted based on actual HiveTask structure
		_ = result

		return nil, nil
	})

	return err
}

// Setup initializes the agent (no-op for enhanced agent)
func (a *CoderAgent) Setup(ctx context.Context, feedbackCh agent.FeedbackChannel) error {
	// Enhanced agent is self-contained and doesn't need external setup
	return nil
}

// ReportStatus provides real-time status updates
func (a *CoderAgent) ReportStatus(ctx context.Context, task *types.HiveTask) error {
	// Status is handled by the ReACT agent internally
	return nil
}

// RequestFeedback requests human feedback for complex decisions
func (a *CoderAgent) RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error) {
	// For now, return a default response - this could be enhanced with actual human-in-the-loop
	return "Please proceed with your best judgment.", nil
}

// Validate performs pre-execution validation
func (a *CoderAgent) Validate(task *types.HiveTask) error {
	if task == nil {
		return errors.ErrValidation("task is required")
	}

	if strings.TrimSpace(task.Goal) == "" {
		return errors.ErrValidation("task goal cannot be empty")
	}

	return nil
}

// Cleanup performs post-execution cleanup
func (a *CoderAgent) Cleanup(ctx context.Context, task *types.HiveTask) error {
	// Enhanced agent is stateless and doesn't require cleanup
	return nil
}

// Heartbeat indicates the agent is alive and ready
func (a *CoderAgent) Heartbeat() error {
	// Enhanced agent is always ready
	return nil
}

// AddTool adds a new tool to the agent (for backwards compatibility)
func (a *CoderAgent) AddTool(newTool tool.InvokableTool) error {
	// Add to our tools slice
	a.tools = append(a.tools, newTool)

	// For now, we'd need to recreate the agent to add tools
	// This could be enhanced in the future if Eino supports dynamic tool addition
	return nil
}

// ListTools returns all available tools
func (a *CoderAgent) ListTools() []tool.InvokableTool {
	return a.tools
}

// GetAgent returns the underlying Eino ReACT agent for advanced usage
func (a *CoderAgent) GetAgent() *react.ReACTAgent {
	return a.reactAgent
}
