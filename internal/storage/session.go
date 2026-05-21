package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hnimtadd/hive/pkg/types"
)

const sessionFileExt = ".json"

type SessionMetadata struct {
	ID               string            `json:"id"`
	NextAction       *string           `json:"next_action"`
	Status           types.Status      `json:"status"`
	Summary          string            `json:"summary"`
	InternalThoughts string            `json:"internal_thoughts"`
	Artifacts        map[string]string `json:"artifacts"`
	Location         string            `json:"location"`
}

type SessionStorage interface {
	List() ([]*types.Conversation, error)
	Save(session *types.Conversation) error
	Load(sessionID string) (*types.Conversation, error)
}

type sessionStorage struct {
	root string
	mu   sync.Mutex
}

func NewSessionStorage(opts Options) (SessionStorage, error) {
	stat, err := os.Stat(opts.Storage)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("invalid session storage: %w", err)
		}
		if err = os.MkdirAll(opts.Storage, 0o700); err != nil {
			return nil, fmt.Errorf("failed to create session storage: %w", err)
		}
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("session storage is not a dir: %s", opts.Storage)
	}

	return &sessionStorage{
		root: opts.Storage,
	}, nil
}

// Save implements [SessionStorage].
func (s *sessionStorage) Save(session *types.Conversation) error {
	if session == nil {
		return errors.New("session is nil")
	}

	path, err := s.sessionFilePath(session.ID)
	if err != nil {
		return err
	}

	// Write the location to the session and create so that session is mutated and
	// Location is ready in sub-sequence session use.
	session.Location = path

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer file.Close()

	if err = writeJSON(file, session); err != nil {
		return fmt.Errorf("failed to write new session: %w", err)
	}

	return nil
}

// Load implements [SessionStorage].
func (s *sessionStorage) Load(sessionID string) (*types.Conversation, error) {
	path, err := s.sessionFilePath(sessionID)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	var conversation types.Conversation
	if err := dec.Decode(&conversation); err != nil {
		return nil, fmt.Errorf("failed to decode conversation: %w", err)
	}

	conversation.Location = path
	conversation.ID = sessionID
	return &conversation, nil
}

// List implements [SessionStorage].
func (s *sessionStorage) List() ([]*types.Conversation, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("failed to list session storage: %w", err)
	}

	sessions := make([]*types.Conversation, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != sessionFileExt {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), sessionFileExt)
		session, loadErr := s.Load(sessionID)
		if loadErr != nil {
			return nil, fmt.Errorf("failed to load session %s: %w", sessionID, loadErr)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (s *sessionStorage) sessionFilePath(sessionID string) (string, error) {
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, `\`) {
		return "", errors.New("session id cannot contain path separator")
	}
	return filepath.Join(s.root, sessionID+sessionFileExt), nil
}

func writeJSON(file *os.File, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal json value: %w", err)
	}
	if _, err = file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("failed to write json value: %w", err)
	}
	return nil
}
