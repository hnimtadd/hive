package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hnimtadd/hive/pkg/types"
)

// codeEditorAgent is an example implementation of the HiveAgent interface
// It handles tasks related to code editing and file modifications.
type codeEditorAgent struct {
	id           string
	agentType    string
	maxTasks     int
	capabilities []string
	feedbackCh   FeedbackChannel
}

// NewCodeEditorAgent creates a new code editor agent.
func NewCodeEditorAgent() (HiveAgent, error) {
	agent := &codeEditorAgent{
		id:        "code-editor-" + uuid.New().String()[:8],
		agentType: "code_editor",
		maxTasks:  5,
		capabilities: []string{
			"file_editing",
			"code_modification",
			"script_updating",
			"configuration_changes",
		},
	}

	return agent, nil
}

// GetID returns the agent's unique identifier.
func (a *codeEditorAgent) GetID() string {
	return a.id
}

// GetType returns the agent type.
func (a *codeEditorAgent) GetType() string {
	return a.agentType
}

// CanHandle determines if this agent can process the given task.
func (a *codeEditorAgent) CanHandle(task *types.HiveTask) bool {
	// Check if task involves code editing keywords
	goal := strings.ToLower(task.Goal)

	codeKeywords := []string{
		"update", "modify", "change", "fix", "script", "file",
		"function", "method", "class", "component", "code",
	}

	for _, keyword := range codeKeywords {
		if strings.Contains(goal, keyword) {
			return true
		}
	}

	return false
}

// Execute performs the main work of the task.
func (a *codeEditorAgent) Execute(ctx context.Context, task *types.HiveTask) error {
	log.Printf("Agent %s executing task: %s", a.id, task.ID)

	// Mark task as started
	task.MarkStarted(ctx, a.id)

	// Simulate work progress
	steps := []struct {
		description string
		progress    float64
		duration    time.Duration
	}{
		{"Analyzing code structure", 20.0, 2 * time.Second},
		{"Identifying target files", 40.0, 1 * time.Second},
		{"Making code modifications", 70.0, 3 * time.Second},
		{"Running validation tests", 90.0, 2 * time.Second},
		{"Finalizing changes", 100.0, 1 * time.Second},
	}

	for _, step := range steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Simulate work
		time.Sleep(step.duration)

		// Update progress
		task.Progress = step.progress
		task.ExecutionSummary = step.description

		// Simulate feedback request at 40% progress
		if step.progress == 40.0 {
			if _, err := a.RequestFeedback(ctx, task, "Should I proceed with modifying the main configuration file? This will affect the traffic routing."); err != nil {
				return fmt.Errorf("feedback request failed: %w", err)
			}
		}
	}

	// Simulate running tests
	if err := a.runValidationTests(ctx, task); err != nil {
		task.MarkFailed(ctx, fmt.Sprintf("Validation tests failed: %v", err))
		return fmt.Errorf("run validtion failed: %w", err)
	}

	// Mark as completed
	summary := "Successfully updated traffic shift script. Modified 3 files, changed 15 lines of code. All tests passed."
	task.LinesChanged = 15
	task.FilesModified = []string{"config/traffic.go", "handlers/shift.go", "tests/traffic_test.go"}
	task.TestsPassed = true
	task.MarkCompleted(ctx, summary)

	return nil
}

// ReportStatus provides real-time status updates during execution..
func (a *codeEditorAgent) ReportStatus(_ context.Context, _ *types.HiveTask) error {
	// This would typically be called periodically during execution
	// For now, it's a no-op as status updates happen in Execute()
	return nil
}

// RequestFeedback pauses execution and requests human input.
func (a *codeEditorAgent) RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error) {
	log.Printf("Agent %s requesting feedback for task %s: %s", a.id, task.ID, message)

	// Mark task as paused and requiring feedback
	task.RequestFeedback(ctx, message)
	if err := a.feedbackCh.SendRequest(ctx, task.ID, message); err != nil {
		return "", fmt.Errorf("failed to send feedback request: %w", err)
	}

	// Wait for feedback with timeout
	feedbackCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	response, err := a.feedbackCh.WaitForResponse(feedbackCtx, task.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get feedback: %w", err)
	}

	// Process feedback and resume
	task.ProvideFeedback(ctx, response)
	return response, nil
}

// Validate performs pre-execution validation of the task.
func (a *codeEditorAgent) Validate(task *types.HiveTask) error {
	if task.Goal == "" {
		return errors.New("task goal cannot be empty")
	}

	if task.WorkingDir == "" {
		return errors.New("working directory must be specified")
	}

	return nil
}

// Cleanup performs any necessary cleanup after task completion or failure.
func (a *codeEditorAgent) Cleanup(_ context.Context, task *types.HiveTask) error {
	log.Printf("Agent %s cleaning up after task %s", a.id, task.ID)
	// Could include: closing file handles, cleaning temp files, etc.
	return nil
}

// GetCapabilities returns a list of capabilities this agent supports.
func (a *codeEditorAgent) GetCapabilities() []string {
	return a.capabilities
}

// Heartbeat indicates the agent is alive and ready to accept work.
func (a *codeEditorAgent) Heartbeat() error {
	return nil
}

// runValidationTests simulates running tests to validate changes.
func (a *codeEditorAgent) runValidationTests(ctx context.Context, task *types.HiveTask) error {
	log.Printf("Running validation tests for task %s", task.ID)

	// Simulate running tests - in a real implementation, this would:
	// 1. Run actual unit tests
	// 2. Check code compilation
	// 3. Verify syntax
	// 4. Run linters

	// For demo purposes, we'll simulate a test command
	if task.WorkingDir != "" {
		cmd := exec.CommandContext(ctx, "echo", "Running tests...")
		cmd.Dir = task.WorkingDir

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("test execution failed: %s", output)
		}
	}

	// Simulate 90% success rate
	// In real implementation, this would be based on actual test results
	return nil
}

func (a *codeEditorAgent) Setup(_ context.Context, feedbackCh FeedbackChannel) error {
	a.feedbackCh = feedbackCh
	return nil
}
