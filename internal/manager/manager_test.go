package manager

import (
	"context"
	"os"
	"testing"

	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/types"
)

func setupTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()

	// Create temp directory for storage
	tmpDir, err := os.MkdirTemp("", "hive-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create storage
	store, err := storage.NewLocalStorage(storage.Options{
		Storage: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create queue
	q := queue.NewMemoryQueue()

	// Create manager
	mgr := NewManager(store, q)

	cleanup := func() {
		q.Close()
		os.RemoveAll(tmpDir)
	}

	return mgr, cleanup
}

func TestCreateTask(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	task, err := mgr.CreateTask(context.Background(), "Test goal", nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.ID == "" {
		t.Fatal("Task should have an ID")
	}

	if task.Goal != "Test goal" {
		t.Fatalf("Expected goal 'Test goal', got %s", task.Goal)
	}

	if task.Status != types.TaskStatusNotStarted {
		t.Fatalf("Expected status not_started, got %s", task.Status)
	}

}

func TestCreateTask_WithArtifacts(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	artifacts := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	task, err := mgr.CreateTask(context.Background(), "Test goal", artifacts)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if len(task.Artifacts) != len(artifacts) {
		t.Fatalf("Expected %d artifacts, got %d", len(artifacts), len(task.Artifacts))
	}

	for k, v := range artifacts {
		if task.Artifacts[k] != v {
			t.Fatalf("Expected artifact %s=%s, got %s=%s", k, v, k, task.Artifacts[k])
		}
	}
}

func TestCreateTask_PersistsToStorage(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	task, err := mgr.CreateTask(context.Background(), "Test goal", nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Verify task can be loaded
	loaded, err := mgr.LoadTask(task.ID)
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}

	if loaded.ID != task.ID {
		t.Fatalf("Expected task ID %s, got %s", task.ID, loaded.ID)
	}

	if loaded.Goal != task.Goal {
		t.Fatalf("Expected goal %s, got %s", task.Goal, loaded.Goal)
	}
}

func TestLoadTask_NotFound(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := mgr.LoadTask("nonexistent-id")
	if err == nil {
		t.Fatal("Expected error for non-existent task")
	}
}

func TestUpdateTask(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	task, err := mgr.CreateTask(context.Background(), "Test goal", nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Update task status
	task.Status = types.TaskStatusInProgress
	if err := mgr.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	// Verify update persisted
	loaded, err := mgr.LoadTask(task.ID)
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}

	if loaded.Status != types.TaskStatusInProgress {
		t.Fatalf("Expected status in_progress, got %s", loaded.Status)
	}
}

func TestIsTerminal(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	task, err := mgr.CreateTask(context.Background(), "Test goal", nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Not started is not terminal
	terminal, err := mgr.IsTerminal(task.ID)
	if err != nil {
		t.Fatalf("IsTerminal failed: %v", err)
	}
	if terminal {
		t.Fatal("Not started task should not be terminal")
	}

	// In progress is not terminal
	task.Status = types.TaskStatusInProgress
	mgr.UpdateTask(task) //nolint:errcheck
	terminal, _ = mgr.IsTerminal(task.ID)
	if terminal {
		t.Fatal("In progress task should not be terminal")
	}

	// Completed is terminal
	task.Status = types.TaskStatusCompleted
	mgr.UpdateTask(task) //nolint:errcheck
	terminal, _ = mgr.IsTerminal(task.ID)
	if !terminal {
		t.Fatal("Completed task should be terminal")
	}

	// Failed is terminal
	task.Status = types.TaskStatusFailed
	mgr.UpdateTask(task) //nolint:errcheck
	terminal, _ = mgr.IsTerminal(task.ID)
	if !terminal {
		t.Fatal("Failed task should be terminal")
	}
}
