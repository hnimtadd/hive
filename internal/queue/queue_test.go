package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTask(id string) *types.HiveTask {
	return &types.HiveTask{
		ID:   id,
		Goal: "test task",
	}
}

func TestEnqueueDequeue(t *testing.T) {
	q := queue.NewMemoryQueue()
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
	q := queue.NewMemoryQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDequeueWakesOnEnqueue(t *testing.T) {
	q := queue.NewMemoryQueue()

	// Start dequeue in goroutine (will block)
	resultCh := make(chan *types.HiveTask, 1)
	go func() {
		task, _ := q.Dequeue(context.Background())
		resultCh <- task
	}()

	// Give goroutine time to block
	time.Sleep(50 * time.Millisecond)

	task := newTestTask("task-1")
	require.NoError(t, q.Enqueue(context.Background(), task))

	select {
	case got := <-resultCh:
		assert.Equal(t, task.ID, got.ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Dequeue did not wake after enqueue")
	}
}

func TestDepth(t *testing.T) {
	q := queue.NewMemoryQueue()

	if q.Length() != 0 {
		t.Fatal("Expected empty queue depth")
	}

	require.NoError(t, q.Enqueue(context.Background(), newTestTask("task-1")))
	require.NoError(t, q.Enqueue(context.Background(), newTestTask("task-2")))

	if q.Length() != 2 {
		t.Fatalf("Expected depth 2, got %d", q.Length())
	}

	_, err := q.Dequeue(context.Background())
	require.NoError(t, err)

	if q.Length() != 1 {
		t.Fatalf("Expected depth 1, got %d", q.Length())
	}
}

func TestClose(t *testing.T) {
	q := queue.NewMemoryQueue()

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
		require.ErrorIs(t, err, queue.ErrQueueClosed)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Close did not unblock waiting dequeue")
	}

	// Enqueue after close should fail
	require.ErrorIs(t, q.Enqueue(context.Background(), newTestTask("task-1")), queue.ErrQueueClosed)
}

func TestRequeue_ExceedsMaxRetries(t *testing.T) {
	q := queue.NewMemoryQueue(queue.WithMaxRetries(2))
	task := newTestTask("task-1")

	require.NoError(t, q.Enqueue(context.TODO(), task))
	_, err := q.Dequeue(context.Background())
	require.NoError(t, err)

	// First retry
	require.NoError(t, q.ScheduleRetry(context.TODO(), task))

	time.Sleep(50 * time.Millisecond)
	_, err = q.Dequeue(context.Background())
	require.NoError(t, err)

	// Second retry
	require.NoError(t, q.ScheduleRetry(context.TODO(), task))

	time.Sleep(50 * time.Millisecond)
	_, err = q.Dequeue(context.Background())
	require.NoError(t, err)

	// Third retry should fail (max 2)
	require.ErrorIs(t, q.ScheduleRetry(context.TODO(), task), queue.ErrMaxRetries)
}

func TestConcurrentEnqueueDequeue(t *testing.T) {
	q := queue.NewMemoryQueue()
	var wg sync.WaitGroup
	count := 100

	// Concurrent enqueuers
	for i := range count {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			q.Enqueue(context.TODO(), newTestTask(string(rune(id))))
		}(i)
	}

	wg.Wait()

	assert.Equal(t, count, q.Length())

	// Concurrent dequeuers
	results := make(chan *types.HiveTask, count)
	for range count {
		wg.Go(func() {
			task, _ := q.Dequeue(context.Background())
			results <- task
		})
	}

	wg.Wait()
	close(results)

	seen := make(map[string]bool)
	for task := range results {
		require.NotContains(t, seen, task.ID, "Same task dequeued twice")
		seen[task.ID] = true
	}

	require.Len(t, seen, count)
}
