package channel

import (
	"log/slog"
	"sync"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
)

// TaskChannels holds the communication channels for a single task.
// InputCh:  Server -> WorkerPool (client feedback, cancel)
// OutputCh: WorkerPool -> Server (progress, completion, errors)
// DoneCh:   WorkerPool -> Server (task reached terminal state).
type TaskChannels struct {
	InputCh  chan *agentv1.ExecuteTaskRequest
	OutputCh chan *agentv1.ExecuteTaskResponse
	DoneCh   chan struct{}
}

// Manager manages per-task communication channels.
// It is the shared coordination layer between Server and WorkerPool.
type Manager struct {
	channels sync.Map // taskID -> *TaskChannels
}

// NewManager creates a new channel manager.
func NewManager() *Manager {
	return &Manager{}
}

// ForTask returns the channels for a given task ID.
// Creates and caches new channels on first call.
func (m *Manager) ForTask(taskID string) *TaskChannels {
	slog.Default().Debug("channel manager open for task", slog.String("task_id", taskID))
	if ch, ok := m.channels.Load(taskID); ok {
		return ch.(*TaskChannels) //nolint:errcheck // this is always true.
	}

	ch := &TaskChannels{
		InputCh:  make(chan *agentv1.ExecuteTaskRequest, 10),
		OutputCh: make(chan *agentv1.ExecuteTaskResponse, 100),
		DoneCh:   make(chan struct{}),
	}
	m.channels.Store(taskID, ch)
	return ch
}

// Cleanup closes and removes channels for a task.
// Safe to call multiple times.
func (m *Manager) Cleanup(taskID string) {
	if ch, ok := m.channels.LoadAndDelete(taskID); ok {
		tasksCh := ch.(*TaskChannels) //nolint:errcheck // this is always true.
		close(tasksCh.OutputCh)
		close(tasksCh.InputCh)
		// DoneCh is closed by the worker when task completes
	}
}
