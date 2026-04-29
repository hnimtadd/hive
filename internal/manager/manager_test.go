package manager_test

import (
	"context"
	"os"
	"testing"

	"github.com/hnimtadd/hive/internal/manager"
	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (*manager.Manager, func()) {
	t.Helper()

	// Create temp directory for storage
	tmpDir := t.TempDir()

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
	mgr := manager.NewManager(store, q)

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
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Test goal", task.Goal)
	assert.Equal(t, types.TaskStatusNotStarted, task.Status)
}

func TestCreateTask_WithArtifacts(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	artifacts := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	task, err := mgr.CreateTask(context.Background(), "Test goal", artifacts)
	require.NoError(t, err)
	require.Len(t, task.Artifacts, len(artifacts))
	for k, v := range artifacts {
		require.Contains(t, task.Artifacts, k)
		require.Equal(t, v, task.Artifacts[k])
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
	require.NoError(t, err)

	// Update task status
	task.Status = types.TaskStatusInProgress
	require.NoError(t, mgr.UpdateTask(task))

	// Verify update persisted
	loaded, err := mgr.LoadTask(task.ID)
	require.NoError(t, err)
	require.Equal(t, types.TaskStatusInProgress, loaded.Status)
}
