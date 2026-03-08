package coder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
)

// Agent is an enhanced AI-powered code editor that uses ReACT pattern
type Agent struct {
	id           string
	reactAgent   *react.Agent
	tools        []tool.InvokableTool
	errorHandler *errors.ErrorHandler
	capabilities []string
	workspaceDir string
}

// NewAgent creates a new enhanced coder agent with ReACT capabilities.
func NewAgent(chatModel model.ToolCallingChatModel, appConfig *config.Config) (*Agent, error) {
	if chatModel == nil {
		return nil, errors.ErrValidation("chat model is required")
	}

	// Create agentTools for the agent
	agentTools := []tool.InvokableTool{
		tools.NewExecuteShellTool(""),
		tools.NewListFilesTool(""),
		tools.NewLocalFileReadTool(""),
		tools.NewLocalFileWriteTool(""),
	}
	if appConfig != nil && appConfig.Gitlab.Enabled {
		token := os.Getenv(appConfig.Gitlab.TokenEnv)
		if token == "" {
			return nil, fmt.Errorf("GitLab token not found in environment variable %s", appConfig.Gitlab.TokenEnv)
		}
		tool, err := tools.NewGitlabAPITool(appConfig.Gitlab.URL, token)
		if err != nil {
			return nil, fmt.Errorf("faield to create gitlab client: %w", err)
		}
		agentTools = append(agentTools, tool)
	}

	// Create ReACT agent with Eino
	agentID := "coder-" + uuid.New().String()[:8]
	reactAgent, err := react.NewWithSystemPrompt(
		agentID,
		chatModel,
		agentTools,
		getCoderSystemPrompt(appConfig.WorkspaceDir),
		appConfig.AI.MaxStep,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	return &Agent{
		id:           agentID,
		reactAgent:   reactAgent,
		tools:        agentTools,
		errorHandler: errors.NewErrorHandler(),
		workspaceDir: appConfig.WorkspaceDir,
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
func getCoderSystemPrompt(workspaceDir string) string {
	return fmt.Sprintf(`You are an advanced AI coding assistant that uses a structured reasoning approach.
## Your workspace: %s
## Your Workflow:

1. **Understand the Task**: Read the task details carefully
2. **Plan**: Think about what needs to be done and the order of operations
3. **Execute**: Use tools to accomplish the task step by step
4. **Verify**: Check that your work is complete
5. **Finish**: Once the task is done, stop and return success

## Available Tools:

- **execute_shell**: Run git commands, shell operations (git clone, git checkout, git add, git commit, git push, etc.)
- **read_local_file**: Read files from the local filesystem
- **write_local_file**: Write or update files
- **list_files**: List directory contents
- **gitlab_api**: GitLab API operations (create_merge_request, get_project, etc.)

## Important Guidelines:

- **Be efficient**: Complete tasks in the minimum number of steps
- **Execute commands sequentially**: Clone repo → checkout branch → make changes → commit → push → create MR
- **Don't repeat actions**: If you already cloned the repo, don't clone it again
- **Use working_dir**: Specify the working directory for git related resource
- **Commit before pushing**: Always commit your changes before pushing
- **Stop when done**: Once you've completed all steps (including creating the MR if requested), STOP

## Common Workflow for GitLab Tasks:

1. Use gitlab_api to get project info (get repo URL)
2. execute_shell: git clone <url>
3. execute_shell: cd <repo> && git checkout -b <branch>
4. write_local_file: Create/update files as needed
5. execute_shell: cd <repo> && git add .
6. execute_shell: cd <repo> && git commit -m "message"
7. execute_shell: cd <repo> && git push -u origin <branch>
8. Use gitlab_api: create_merge_request

Once all steps are complete, you're done. Don't continue executing actions unnecessarily.`, workspaceDir)
}

// GetID returns the agent's unique identifier
func (a *Agent) GetID() string {
	return a.id
}

// GetType returns the agent type
func (a *Agent) GetType() string {
	return "enhanced_coder"
}

// GetCapabilities returns the agent's capabilities
func (a *Agent) GetCapabilities() []string {
	return a.capabilities
}

// CanHandle determines if this agent can process the given task
func (a *Agent) CanHandle(task *types.HiveTask) bool {
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
		"programming", "program", "variable", "think", "design",
		"software", "development", "coding", "script", "library",
	}

	for _, keyword := range codingKeywords {
		if strings.Contains(goal, keyword) {
			return true
		}
	}

	return false
}

// TaskDetail represents the essential information that the Grab Agent needs
// to analyze and prepare a task for execution
type TaskDetail struct {
	// Core task information
	Goal        string `json:"goal"`
	Description string `json:"description"`
	JiraID      string `json:"jira_id,omitempty"`

	// Existing context
	Context          string `json:"context,omitempty"`
	TechnicalContext string `json:"technical_context,omitempty"`
	FeatureSpec      string `json:"feature_spec,omitempty"`

	// Work scope
	FilesToModify []string `json:"files_to_modify,omitempty"`
	FilesToCreate []string `json:"files_to_create,omitempty"`
	WorkingDir    string   `json:"working_dir,omitempty"`
	Repo          string   `json:"repo,omitempty"`
	SourceBranch  string   `json:"source_branch,omitempty"`
	TargetBranch  string   `json:"target_branch,omitempty"`

	// Environment (for context about runtime)
	Environment map[string]string `json:"environment,omitempty"`
}

// Detail returns a focused JSON object containing only the information
// that agents (particularly the Coder Agent) need for task execution.
// This excludes internal tracking fields, progress metrics, and timestamps.
func (a *Agent) detail(t *types.HiveTask) (string, error) {
	detail := &TaskDetail{
		Goal:             t.Goal,
		Description:      t.Description,
		JiraID:           t.JiraID,
		Context:          t.Context,
		TechnicalContext: t.TechnicalContext,
		FeatureSpec:      t.FeatureSpec,
		FilesToModify:    t.FilesToModify,
		FilesToCreate:    t.FilesToCreate,
		Repo:             t.GitlabProjectPath,
		SourceBranch:     t.SourceBranch,
		TargetBranch:     t.TargetBranch,
		WorkingDir:       t.WorkingDir,
		Environment:      t.Environment,
	}

	// Convert to map[string]interface{} for flexible JSON representation

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *Agent) buildExectionPrompt(task *types.HiveTask) string {
	// Get the focused task detail that agents care about
	taskDetailJSON, _ := a.detail(task)

	prompt := fmt.Sprintf(`
Execute this task comprehensively.

TASK DETAILS:
%s`, taskDetailJSON)

	// Add additional guidance if Jira ticket is present
	if task.GitlabProjectPath != "" {
		prompt += fmt.Sprintf(`
IMPORTANT: This task is associated with this gitlab project: %s
Please init the project if it is not ready and working on the local gitlab project.
`, task.GitlabProjectPath)
	}
	return prompt

}

// Execute performs the coding task using ReACT pattern with error handling.
func (a *Agent) Execute(ctx context.Context, task *types.HiveTask) error {
	if task == nil {
		return errors.ErrValidation("task cannot be nil")
	}
	if task.GitlabProjectPath == "" && task.WorkingDir == "" {
		return errors.ErrValidation("coder task must specified the gitlab project or the current working dir, otherwise agent will not know where to do the task")
	}

	// Use error handler with retry for resilient execution
	retryConfig := errors.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500, // 500ms
		BackoffFactor: 2.0,
		MaxDelay:      5000, // 5s max delay
	}
	prompt := a.buildExectionPrompt(task)

	_, err := a.errorHandler.WithRetry(ctx, retryConfig, func(ctx context.Context) (any, error) {
		// Execute the task using the ReACT agent
		result, execErr := a.reactAgent.Execute(ctx, prompt)
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
func (a *Agent) Setup(ctx context.Context, feedbackCh agent.FeedbackChannel) error {
	// Enhanced agent is self-contained and doesn't need external setup
	return nil
}

// ReportStatus provides real-time status updates
func (a *Agent) ReportStatus(ctx context.Context, task *types.HiveTask) error {
	// Status is handled by the ReACT agent internally
	return nil
}

// RequestFeedback requests human feedback for complex decisions
func (a *Agent) RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error) {
	// For now, return a default response - this could be enhanced with actual human-in-the-loop
	return "Please proceed with your best judgment.", nil
}

// Validate performs pre-execution validation
func (a *Agent) Validate(task *types.HiveTask) error {
	if task == nil {
		return errors.ErrValidation("task is required")
	}

	if strings.TrimSpace(task.Goal) == "" {
		return errors.ErrValidation("task goal cannot be empty")
	}

	return nil
}

// Cleanup performs post-execution cleanup
func (a *Agent) Cleanup(ctx context.Context, task *types.HiveTask) error {
	// Enhanced agent is stateless and doesn't require cleanup
	return nil
}

// Heartbeat indicates the agent is alive and ready
func (a *Agent) Heartbeat() error {
	// Enhanced agent is always ready
	return nil
}

// AddTool adds a new tool to the agent (for backwards compatibility)
func (a *Agent) AddTool(newTool tool.InvokableTool) error {
	// Add to our tools slice
	a.tools = append(a.tools, newTool)

	// For now, we'd need to recreate the agent to add tools
	// This could be enhanced in the future if Eino supports dynamic tool addition
	return nil
}

// ListTools returns all available tools
func (a *Agent) ListTools() []tool.InvokableTool {
	return a.tools
}

// GetAgent returns the underlying Eino ReACT agent for advanced usage
func (a *Agent) GetAgent() *react.Agent {
	return a.reactAgent
}
