package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/require"
)

func newTestTask(id string) *types.HiveTask {
	return &types.HiveTask{
		ID:   id,
		Goal: "test task",
	}
}

func TestEnqueueDequeue(t *testing.T) {
	q := NewMemoryQueue()
	task := newTestTask("task-1")

	if err := q.Enqueue(context.Background(), task); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	got, err := q.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if got.ID != task.ID {
		t.Fatalf("Expected task ID %s, got %s", task.ID, got.ID)
	}
}

func TestDequeueBlocks(t *testing.T) {
	q := NewMemoryQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected deadline exceeded, got: %v", err)
	}
}

func TestDequeueWakesOnEnqueue(t *testing.T) {
	q := NewMemoryQueue()

	// Start dequeue in goroutine (will block)
	resultCh := make(chan *types.HiveTask, 1)
	go func() {
		task, _ := q.Dequeue(context.Background())
		resultCh <- task
	}()

	// Give goroutine time to block
	time.Sleep(50 * time.Millisecond)

	task := newTestTask("task-1")
	q.Enqueue(context.Background(), task) //nolint:errcheck

	select {
	case got := <-resultCh:
		if got.ID != task.ID {
			t.Fatalf("Expected task ID %s, got %s", task.ID, got.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Dequeue did not wake after enqueue")
	}
}

func TestDepth(t *testing.T) {
	q := NewMemoryQueue()

	if q.Length() != 0 {
		t.Fatal("Expected empty queue depth")
	}

	require.NoError(t, q.Enqueue(context.Background(), newTestTask("task-1")))
	require.NoError(t, q.Enqueue(context.Background(), newTestTask("task-2")))

	if q.Length() != 2 {
		t.Fatalf("Expected depth 2, got %d", q.Length())
	}

	q.Dequeue(context.Background()) //nolint:errcheck
	if q.Length() != 1 {
		t.Fatalf("Expected depth 1, got %d", q.Length())
	}
}

func TestClose(t *testing.T) {
	q := NewMemoryQueue()

	// Start blocking dequeue
	errCh := make(chan error, 1)
	go func() {
		_, err := q.Dequeue(context.Background())
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	q.Close()

	select {
	case err := <-errCh:
		if err != ErrQueueClosed {
			t.Fatalf("Expected ErrQueueClosed, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Close did not unblock waiting dequeue")
	}

	// Enqueue after close should fail
	if err := q.Enqueue(context.Background(), newTestTask("task-1")); err != ErrQueueClosed {
		t.Fatalf("Expected ErrQueueClosed after close, got: %v", err)
	}
}

func TestRequeue_ExceedsMaxRetries(t *testing.T) {
	q := NewMemoryQueue(WithMaxAttempts(3))
	task := newTestTask("task-1")

	q.Enqueue(context.TODO(), task) //nolint:errcheck
	q.Dequeue(context.Background()) //nolint:errcheck

	// First retry
	if err := q.Enqueue(context.TODO(), task); err != nil {
		t.Fatalf("First requeue failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	q.Dequeue(context.Background()) //nolint:errcheck

	// Second retry
	if err := q.Enqueue(context.TODO(), task); err != nil {
		t.Fatalf("Second requeue failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	q.Dequeue(context.Background()) //nolint: errcheck

	// Third retry should fail (max 2)
	if err := q.Enqueue(context.TODO(), task); err != ErrMaxRetries {
		t.Fatalf("Expected ErrMaxRetries, got: %v", err)
	}
}

func TestConcurrentEnqueueDequeue(t *testing.T) {
	q := NewMemoryQueue()
	var wg sync.WaitGroup
	count := 100

	// Concurrent enqueuers
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			q.Enqueue(context.TODO(), newTestTask(string(rune(id)))) //nolint:errcheck
		}(i)
	}

	wg.Wait()

	if q.Length() != count {
		t.Fatalf("Expected depth %d, got %d", count, q.Length())
	}

	// Concurrent dequeuers
	results := make(chan *types.HiveTask, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, _ := q.Dequeue(context.Background())
			results <- task
		}()
	}

	wg.Wait()
	close(results)

	seen := make(map[string]bool)
	for task := range results {
		if seen[task.ID] {
			t.Fatal("Same task dequeued twice")
		}
		seen[task.ID] = true
	}

	if len(seen) != count {
		t.Fatalf("Expected %d unique tasks, got %d", count, len(seen))
	}
}
