package types

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Status represents the current state of a session.
type Status string

const (
	SessionStatusNotStarted Status = "not_started"
	SessionStatusInProgress Status = "in_progress"
	SessionStatusCompleted  Status = "completed"
	SessionStatusFailed     Status = "failed"
	SessionStatusPaused     Status = "paused"
)

type sessionPlan struct {
	Target string `json:"target" db:"target"`
	Status Status `json:"status" db:"status"`
}

// Conversation represents a single session in the distributed system.
type Conversation struct {
	// Core identifiers
	ID               string            `json:"_id"                   jsonschema:"ID of the session"`
	Status           Status            `json:"status"                jsonschema:"Current session status"`
	NextAction       *string           `json:"next_action,omitempty" jsonschema:"Previous agent suggested next action to complete"`
	Plan             []sessionPlan     `json:"plan"                  jsonschema:"Our mastery plan"`
	Artifacts        map[string]string `json:"artifacts"             jsonschema:"Shared artifacts extracted by other agents that uses for this session life-cycle"`
	InternalThoughts string            `json:"internal_thoughts"     jsonschema:"Thoughts of previous agent"`
	Summary          string            `json:"summary,omitempty"     jsonschema:"Compressed history of previous execution cycles"`
	Messages         []Message         `json:"messages"              jsonschema:"global conversation"`

	Location string          `json:"-"`
	Context  context.Context `json:"-"`
	Retries  uint            `json:"retries"`
}

// NewConversation creates a new session with default values.
func NewConversation() *Conversation {
	return &Conversation{
		ID:        uuid.New().String(),
		Status:    SessionStatusNotStarted,
		Artifacts: map[string]string{},
		Messages:  []Message{},
	}
}

func (t *Conversation) JSONString() (string, error) {
	jsonBytes, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// CompactJSONString returns a JSON representation of the session with reduced message history.
// Only includes the most recent messages instead of the full conversation history.
// This helps prevent context window overflow by keeping session descriptions compact.
func (t *Conversation) CompactJSONString() (string, error) {
	type compactView struct {
		ID               string            `json:"_id"`
		Status           Status            `json:"status"`
		NextAction       *string           `json:"next_action,omitempty"`
		Plan             []sessionPlan     `json:"plan"`
		Artifacts        map[string]string `json:"artifacts"`
		InternalThoughts string            `json:"internal_thoughts"`
		Summary          string            `json:"summary,omitempty"`
		RecentMessages   []Message         `json:"recent_messages"`
		Goal             string            `json:"context"`
	}

	// Keep only the last 3 messages
	recentCount := min(3, len(t.Messages))

	compact := compactView{
		ID:               t.ID,
		Status:           t.Status,
		NextAction:       t.NextAction,
		Plan:             t.Plan,
		Artifacts:        t.Artifacts,
		InternalThoughts: t.InternalThoughts,
		Summary:          t.Summary,
		RecentMessages:   t.Messages[len(t.Messages)-recentCount:],
	}

	jsonBytes, err := json.Marshal(compact)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// IsTerminal returns true if the session has reached a terminal state.
// Terminal states are: completed, failed.
// Non-terminal states are: not_started, in_progress, paused.
func (s Status) IsTerminal() bool {
	return s == SessionStatusCompleted || s == SessionStatusFailed
}

type Role string

const RoleUser Role = "user"
const RoleAssistant Role = "assistant"

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

func NewMessage(role Role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}
