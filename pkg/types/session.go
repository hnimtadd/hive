package types

import "github.com/google/uuid"

type HiveSession struct {
	ID       string    `json:"_id"`
	Messages []Message `json:"messages" jsonschema:"session messages"`
}

func NewHiveSession() *HiveSession {
	return &HiveSession{
		ID:       uuid.New().String(),
		Messages: []Message{},
	}
}

type EventType string

type HiveEvent struct {
	Type    EventType
	Payload any
}
