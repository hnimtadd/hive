package worker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/storage"
)

func setupTestPool(t *testing.T) (*Pool, *channel.Manager, func()) {
	t.Helper()

	// Create temp directory for storage
	tmpDir, err := os.MkdirTemp("", "hive-worker-test-*")
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

	// Create channel manager
	channels := channel.NewManager()

	// Note: We can't create a full pool without LLM provider and registry.
	// This test setup is for structural validation only.
	// Real integration tests need the full server setup.

	cleanup := func() {
		q.Close()
		os.RemoveAll(tmpDir)
	}

	// Return a minimal pool for testing
	pool := &Pool{
		size:     2,
		queue:    q,
		storage:  store,
		channels: channels,
		done:     make(chan struct{}),
	}

	return pool, channels, cleanup
}

func TestPoolStartStop(t *testing.T) {
	pool, _, cleanup := setupTestPool(t)
	defer cleanup()

	ctx := context.Background()
	pool.Start(ctx)

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Pool.Stop() did not return in time")
	}
}

func TestPoolDoneChannel(t *testing.T) {
	pool, _, cleanup := setupTestPool(t)
	defer cleanup()

	ctx := context.Background()
	pool.Start(ctx)

	// Stop in background
	go pool.Stop()

	// Done channel should close
	select {
	case <-pool.Done():
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Done channel did not close")
	}
}

func TestPoolContextCancellation(t *testing.T) {
	pool, _, cleanup := setupTestPool(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)

	// Cancel context
	cancel()

	// Stop should complete quickly
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Pool.Stop() did not return after context cancellation")
	}
}
