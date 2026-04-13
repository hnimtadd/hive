package manager

import (
	"context"
	"fmt"

	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/types"
)

// Manager handles task lifecycle: creation, persistence, and queuing.
// It knows nothing about channels, events, or transport layers.
type Manager struct {
	storage storage.Storage
	queue   queue.Queue
}

// NewManager creates a new task manager.
func NewManager(storage storage.Storage, queue queue.Queue) *Manager {
	return &Manager{
		storage: storage,
		queue:   queue,
	}
}

// CreateTask creates a new task, persists it, and enqueues it for execution.
func (m *Manager) CreateTask(ctx context.Context, goal string, artifacts map[string]string) (*types.HiveTask, error) {
	task := types.NewHiveTask(goal, artifacts)

	// Persist to storage
	if err := m.storage.Add(task); err != nil {
		return nil, fmt.Errorf("failed to persist task: %w", err)
	}

	// Enqueue for execution
	if err := m.queue.Enqueue(task); err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	return task, nil
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

// IsTerminal returns true if the task has reached a terminal state (completed or failed).
func (m *Manager) IsTerminal(id string) (bool, error) {
	task, err := m.LoadTask(id)
	if err != nil {
		return false, err
	}
	return task.Status == types.TaskStatusCompleted || task.Status == types.TaskStatusFailed, nil
}
