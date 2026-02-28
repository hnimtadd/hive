package types //nolint:revive // this package name is acceptable

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusPaused     TaskStatus = "paused"
)

// TaskPriority represents the urgency of a task.
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// HiveTask represents a single task in the distributed system.
type HiveTask struct {
	// Core identifiers
	ID     string `json:"id"      db:"id"`
	JiraID string `json:"jira_id" db:"jira_id"`

	// Task metadata
	Title       string       `json:"title"       db:"title"`
	Description string       `json:"description" db:"description"`
	Goal        string       `json:"goal"        db:"goal"`
	Status      TaskStatus   `json:"status"      db:"status"`
	Priority    TaskPriority `json:"priority"    db:"priority"`

	// Agent assignment
	AssignedAgent string `json:"assigned_agent" db:"assigned_agent"`
	AgentType     string `json:"agent_type"     db:"agent_type"`

	// Execution context
	Command     string            `json:"command"     db:"command"`
	WorkingDir  string            `json:"working_dir" db:"working_dir"`
	Environment map[string]string `json:"environment" db:"environment"`

	// Progress tracking
	Progress      float64  `json:"progress"       db:"progress"`
	LinesChanged  int      `json:"lines_changed"  db:"lines_changed"`
	FilesModified []string `json:"files_modified" db:"files_modified"`
	TestsPassed   bool     `json:"tests_passed"   db:"tests_passed"`

	// Feedback and interaction
	RequiresFeedback bool   `json:"requires_feedback" db:"requires_feedback"`
	FeedbackMessage  string `json:"feedback_message"  db:"feedback_message"`
	FeedbackResponse string `json:"feedback_response" db:"feedback_response"`

	// Error handling
	ErrorMessage string `json:"error_message" db:"error_message"`
	RetryCount   int    `json:"retry_count"   db:"retry_count"`
	MaxRetries   int    `json:"max_retries"   db:"max_retries"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"   db:"created_at"`
	StartedAt   *time.Time `json:"started_at"   db:"started_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
	UpdatedAt   time.Time  `json:"updated_at"   db:"updated_at"`

	// Execution summary
	ExecutionSummary string        `json:"execution_summary" db:"execution_summary"`
	ExecutionTime    time.Duration `json:"execution_time"    db:"execution_time"`
}

// NewHiveTask creates a new task with default values.
func NewHiveTask(goal, jiraID string) *HiveTask {
	now := time.Now()
	return &HiveTask{
		ID:               uuid.New().String(),
		JiraID:           jiraID,
		Goal:             goal,
		Status:           TaskStatusPending,
		Priority:         TaskPriorityMedium,
		Progress:         0.0,
		MaxRetries:       3,
		Environment:      make(map[string]string),
		FilesModified:    make([]string, 0),
		RequiresFeedback: false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// IsActive returns true if the task is currently being processed.
func (t *HiveTask) IsActive() bool {
	return t.Status == TaskStatusInProgress || t.Status == TaskStatusPaused
}

// CanRetry returns true if the task can be retried.
func (t *HiveTask) CanRetry() bool {
	return t.Status == TaskStatusFailed && t.RetryCount < t.MaxRetries
}

// MarkStarted updates the task to indicate it has started.
func (t *HiveTask) MarkStarted(agentID string) {
	now := time.Now()
	t.Status = TaskStatusInProgress
	t.AssignedAgent = agentID
	t.StartedAt = &now
	t.UpdatedAt = now
}

// MarkCompleted updates the task to indicate successful completion.
func (t *HiveTask) MarkCompleted(summary string) {
	now := time.Now()
	t.Status = TaskStatusCompleted
	t.Progress = 100.0
	t.ExecutionSummary = summary
	t.CompletedAt = &now
	t.UpdatedAt = now

	if t.StartedAt != nil {
		t.ExecutionTime = now.Sub(*t.StartedAt)
	}
}

// MarkFailed updates the task to indicate failure.
func (t *HiveTask) MarkFailed(errorMsg string) {
	now := time.Now()
	t.Status = TaskStatusFailed
	t.ErrorMessage = errorMsg
	t.UpdatedAt = now
	t.RetryCount++
}

// RequestFeedback marks the task as requiring human interaction.
func (t *HiveTask) RequestFeedback(message string) {
	t.Status = TaskStatusPaused
	t.RequiresFeedback = true
	t.FeedbackMessage = message
	t.UpdatedAt = time.Now()
}

// ProvideFeedback provides the human response and resumes the task.
func (t *HiveTask) ProvideFeedback(response string) {
	t.Status = TaskStatusInProgress
	t.RequiresFeedback = false
	t.FeedbackResponse = response
	t.UpdatedAt = time.Now()
}

