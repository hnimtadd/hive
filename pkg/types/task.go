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
	ID               string            `json:"id"                    db:"id"          jsonschema:"ID of the task"`
	Status           Status            `json:"status"                db:"status"      jsonschema:"Current task status"`
	NextAction       *string           `json:"next_action,omitempty" db:"next_action" jsonschema:"Previous agent suggested next action to complete"`
	Plan             []TaskPlan        `json:"plan"                  db:"plan"        jsonschema:"Our mastery plan"`
	Artifacts        map[string]string `json:"artifacts"             db:"artifacts"   jsonschema:"Shared artifacts extracted by other agents that uses for this task life-cycle"`
	InternalThoughts string            `json:"internal_thoughts"                      jsonschema:"Thoughts of previous agent"`
	Messages         []Message         `json:"message"               db:"message"     jsonschema:"global conversation"`
	Context          string            `json:"context"               db:"context"     jsonschema:"Task core context and description"`
}

// NewHiveTask creates a new task with default values.
func NewHiveTask(goal string) *HiveTask {
	return &HiveTask{
		ID:      uuid.New().String(),
		Status:  TaskStatusNotStarted,
		Context: goal,
	}
}

func (t *HiveTask) JSONString() string {
	jsonBytes, err := json.Marshal(t)
	if err != nil {
		return "unknow"
	}
	return string(jsonBytes)
}
