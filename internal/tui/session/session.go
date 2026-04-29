package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Message represents a saved chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	TaskID  string `json:"task_id"`
}

// Session represents a saved chat session.
type Session struct {
	Messages []Message `json:"messages"`
}

// GetDefaultSessionFile returns the default session file path.
func GetDefaultSessionFile() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home cannot be determined
		return ".hive_session.json"
	}
	return filepath.Join(homeDir, ".hive_session.json")
}

// SaveSession saves the session to a JSON file.
func SaveSession(filepath string, session *Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from a JSON file.
func LoadSession(filepath string) (*Session, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty session if file doesn't exist
			return &Session{Messages: []Message{}}, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	unmarshalErr := json.Unmarshal(data, &session)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal session (corrupted file): %w", unmarshalErr)
	}

	return &session, nil
}
