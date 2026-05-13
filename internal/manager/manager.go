package manager

import (
	"context"
	"fmt"

	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/types"
)

// Manager handles task lifecycle: creation, persistence, and queuing.
// It knows nothing about channels, events, or transport layers.
type Manager struct {
	storage        storage.Storage
	sessionStorage storage.SessionStorage
}

// NewManager creates a new task manager.
func NewManager(sessionStorage storage.SessionStorage, storage storage.Storage) *Manager {
	return &Manager{
		sessionStorage: sessionStorage,
		storage:        storage,
	}
}

func (m *Manager) createTask(goal string, artifacts map[string]string) (*types.HiveTask, error) {
	task := types.NewHiveTask(goal, artifacts)

	// Persist to storage
	if err := m.storage.Add(task); err != nil {
		return nil, fmt.Errorf("failed to persist task: %w", err)
	}

	return task, nil
}

// CreateTask creates a new task, persists it, and enqueues it for execution.
func (m *Manager) CreateTask(goal string, artifacts map[string]string) (*types.HiveTask, error) {
	return m.createTask(goal, artifacts)
}

// LoadTask retrieves a task from storage.
func (m *Manager) LoadTask(id string) (*types.HiveTask, error) {
	task, err := m.storage.Load(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load task: %w", err)
	}
	return task, nil
}

// UpdateTask updates a task's state in storage.
func (m *Manager) UpdateTask(task *types.HiveTask) error {
	if err := m.storage.Update(task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

// CreateTask creates a new task, persists it, and enqueues it for execution.
func (m *Manager) CreateSession(ctx context.Context) (*types.HiveSession, error) {
	session := types.NewHiveSession()
	if err := m.sessionStorage.Create(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	return session, nil
}

// CreateTask creates a new task, persists it, and enqueues it for execution.
func (m *Manager) LoadSession(ctx context.Context, sessionID string) (*types.HiveSession, error) {
	session, err := m.sessionStorage.Load(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	return session, nil
}
