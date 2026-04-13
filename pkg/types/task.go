package types

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Status represents the current state of a task.
type Status string

const (
	TaskStatusNotStarted Status = "not_started"
	TaskStatusInProgress Status = "in_progress"
	TaskStatusCompleted  Status = "completed"
	TaskStatusFailed     Status = "failed"
	TaskStatusPaused     Status = "paused"
)

type TaskPlan struct {
	Target string `json:"target" db:"target"`
	Status Status `json:"status" db:"status"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// HiveTask represents a single task in the distributed system.
type HiveTask struct {
	// Core identifiers
	ID               string            `json:"_id"                   jsonschema:"ID of the task"`
	Status           Status            `json:"status"                jsonschema:"Current task status"`
	NextAction       *string           `json:"next_action,omitempty" jsonschema:"Previous agent suggested next action to complete"`
	Plan             []TaskPlan        `json:"plan"                  jsonschema:"Our mastery plan"`
	Artifacts        map[string]string `json:"artifacts"             jsonschema:"Shared artifacts extracted by other agents that uses for this task life-cycle"`
	InternalThoughts string            `json:"internal_thoughts"     jsonschema:"Thoughts of previous agent"`
	Messages         []Message         `json:"message"               jsonschema:"global conversation"`
	Goal             string            `json:"context"               jsonschema:"Task core context and description"`
}

// NewHiveTask creates a new task with default values.
func NewHiveTask(goal string, artifacts map[string]string) *HiveTask {
	return &HiveTask{
		ID:        uuid.New().String(),
		Status:    TaskStatusNotStarted,
		Goal:      goal,
		Artifacts: artifacts,
		Messages: []Message{
			{
				Role:    "user",
				Content: goal,
			},
		},
	}
}

func (t *HiveTask) JSONString() (string, error) {
	jsonBytes, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// IsTerminal returns true if the task has reached a terminal state.
// Terminal states are: completed, failed.
// Non-terminal states are: not_started, in_progress, paused.
func (s Status) IsTerminal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed
}
