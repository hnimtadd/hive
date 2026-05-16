package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hnimtadd/hive/pkg/types"
)

const sessionFileExt = ".jsonl"

type SessionStorage interface {
	Create(session *types.Session) error
	List() ([]*types.Session, error)
	Save(session *types.Session) error
	Load(sessionID string) (*types.Session, error)
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

// Create implements [SessionStorage].
func (s *sessionStorage) Create(session *types.Session) error {
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

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer file.Close()

	if err = writeJSONL(file, session); err != nil {
		return fmt.Errorf("failed to write new session: %w", err)
	}
	return nil
}

// Save implements [SessionStorage].
func (s *sessionStorage) Save(session *types.Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	path, err := s.sessionFilePath(session.ID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	if err = writeJSONL(file, session); err != nil {
		return fmt.Errorf("failed to append session state: %w", err)
	}
	return nil
}

// Load implements [SessionStorage].
func (s *sessionStorage) Load(sessionID string) (*types.Session, error) {
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

	session, err := readLastSessionRecord(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read session %s: %w", sessionID, err)
	}
	session.Location = path
	session.ID = sessionID
	return session, nil
}

// List implements [SessionStorage].
func (s *sessionStorage) List() ([]*types.Session, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("failed to list session storage: %w", err)
	}

	sessions := make([]*types.Session, 0, len(entries))
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
	if sessionID == "" {
		return "", errors.New("session id is required")
	}
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, `\`) {
		return "", errors.New("session id cannot contain path separator")
	}
	return filepath.Join(s.root, sessionID+sessionFileExt), nil
}

func writeJSONL(file *os.File, session *types.Session) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if _, err = file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("failed to write session line: %w", err)
	}
	return nil
}

func readLastSessionRecord(file *os.File) (*types.Session, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024*10)
	var (
		lastLine string
		lineNo   int
	)
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lastLine = line
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan session file: %w", err)
	}
	if lastLine == "" {
		return nil, errors.New("session file is empty")
	}

	var session types.Session
	if err := json.Unmarshal([]byte(lastLine), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session snapshot at line %d: %w", lineNo, err)
	}
	return &session, nil
}
