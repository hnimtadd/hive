package agent

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/pkg/types"
)

// CodeEditorAgent is an example implementation of the HiveAgent interface
// It handles tasks related to code editing and file modifications
type CodeEditorAgent struct {
	id           string
	agentType    string
	redisClient  *redis.Client
	maxTasks     int
	capabilities []string
}

// NewCodeEditorAgent creates a new code editor agent
func NewCodeEditorAgent(redisClient *redis.Client) (*CodeEditorAgent, error) {
	agent := &CodeEditorAgent{
		id:          "code-editor-" + uuid.New().String()[:8],
		agentType:   "code_editor",
		redisClient: redisClient,
		maxTasks:    5,
		capabilities: []string{
			"file_editing",
			"code_modification",
			"script_updating",
			"configuration_changes",
		},
	}

	return agent, nil
}

// GetID returns the agent's unique identifier
func (a *CodeEditorAgent) GetID() string {
	return a.id
}

// GetType returns the agent type
func (a *CodeEditorAgent) GetType() string {
	return a.agentType
}

// CanHandle determines if this agent can process the given task
func (a *CodeEditorAgent) CanHandle(task *types.HiveTask) bool {
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

// Execute performs the main work of the task
func (a *CodeEditorAgent) Execute(ctx context.Context, task *types.HiveTask) error {
	log.Printf("Agent %s executing task: %s", a.id, task.ID)

	// Mark task as started
	task.MarkStarted(a.id)
	if err := a.redisClient.UpdateTask(ctx, task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

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

		if err := a.redisClient.UpdateTask(ctx, task); err != nil {
			log.Printf("Failed to update task progress: %v", err)
		}

		// Simulate feedback request at 40% progress
		if step.progress == 40.0 {
			if _, err := a.RequestFeedback(ctx, task, "Should I proceed with modifying the main configuration file? This will affect the traffic routing."); err != nil {
				return fmt.Errorf("feedback request failed: %w", err)
			}
		}
	}

	// Simulate running tests
	if err := a.runValidationTests(ctx, task); err != nil {
		task.MarkFailed(fmt.Sprintf("Validation tests failed: %v", err))
		return a.redisClient.UpdateTask(ctx, task)
	}

	// Mark as completed
	summary := "Successfully updated traffic shift script. Modified 3 files, changed 15 lines of code. All tests passed."
	task.LinesChanged = 15
	task.FilesModified = []string{"config/traffic.go", "handlers/shift.go", "tests/traffic_test.go"}
	task.TestsPassed = true
	task.MarkCompleted(summary)

	return a.redisClient.UpdateTask(ctx, task)
}

// ReportStatus provides real-time status updates during execution
func (a *CodeEditorAgent) ReportStatus(ctx context.Context, task *types.HiveTask) error {
	// This would typically be called periodically during execution
	// For now, it's a no-op as status updates happen in Execute()
	return nil
}

// RequestFeedback pauses execution and requests human input
func (a *CodeEditorAgent) RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error) {
	log.Printf("Agent %s requesting feedback for task %s: %s", a.id, task.ID, message)

	// Mark task as paused and requiring feedback
	task.RequestFeedback(message)
	if err := a.redisClient.UpdateTask(ctx, task); err != nil {
		return "", fmt.Errorf("failed to update task for feedback: %w", err)
	}

	// Wait for feedback with timeout
	feedbackCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	response, err := a.redisClient.WaitForFeedback(feedbackCtx, task.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get feedback: %w", err)
	}

	// Process feedback and resume
	task.ProvideFeedback(response)
	if err := a.redisClient.UpdateTask(ctx, task); err != nil {
		return "", fmt.Errorf("failed to update task after feedback: %w", err)
	}

	log.Printf("Agent %s received feedback: %s", a.id, response)
	return response, nil
}

// Validate performs pre-execution validation of the task
func (a *CodeEditorAgent) Validate(task *types.HiveTask) error {
	if task.Goal == "" {
		return fmt.Errorf("task goal cannot be empty")
	}

	if task.WorkingDir == "" {
		return fmt.Errorf("working directory must be specified")
	}

	return nil
}

// Cleanup performs any necessary cleanup after task completion or failure
func (a *CodeEditorAgent) Cleanup(ctx context.Context, task *types.HiveTask) error {
	log.Printf("Agent %s cleaning up after task %s", a.id, task.ID)
	// Could include: closing file handles, cleaning temp files, etc.
	return nil
}

// GetCapabilities returns a list of capabilities this agent supports
func (a *CodeEditorAgent) GetCapabilities() []string {
	return a.capabilities
}

// Heartbeat indicates the agent is alive and ready to accept work
func (a *CodeEditorAgent) Heartbeat() error {
	ctx := context.Background()
	return a.redisClient.Heartbeat(ctx, a.id)
}

// runValidationTests simulates running tests to validate changes
func (a *CodeEditorAgent) runValidationTests(ctx context.Context, task *types.HiveTask) error {
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

// Start begins the agent's main execution loop
func (a *CodeEditorAgent) Start(ctx context.Context) error {
	log.Printf("Starting Code Editor Agent %s", a.id)

	// Register agent
	if err := a.redisClient.RegisterAgent(ctx, a.id, a.agentType); err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	// Start heartbeat goroutine
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.Heartbeat(); err != nil {
					log.Printf("Heartbeat failed: %v", err)
				}
			}
		}
	}()

	// Main task processing loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("Agent %s shutting down", a.id)
			return ctx.Err()
		default:
		}

		// Get next task from queue
		task, err := a.redisClient.GetNextTask(ctx)
		if err != nil {
			log.Printf("Failed to get next task: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if task == nil {
			// No tasks available, wait a bit
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if we can handle this task
		if !a.CanHandle(task) {
			// Put task back in queue for other agents
			if err := a.redisClient.SubmitTask(ctx, task); err != nil {
				log.Printf("Failed to requeue task: %v", err)
			}
			continue
		}

		// Validate and execute task
		if err := a.Validate(task); err != nil {
			task.MarkFailed(fmt.Sprintf("Validation failed: %v", err))
			a.redisClient.UpdateTask(ctx, task)
			continue
		}

		// Execute task
		if err := a.Execute(ctx, task); err != nil {
			log.Printf("Task execution failed: %v", err)
			task.MarkFailed(err.Error())
			a.redisClient.UpdateTask(ctx, task)
		}

		// Cleanup regardless of success or failure
		if err := a.Cleanup(ctx, task); err != nil {
			log.Printf("Cleanup failed: %v", err)
		}
	}
}

