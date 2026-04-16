package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/hnimtadd/hive/pkg/types"
)

var (
	ErrQueueClosed  = errors.New("queue is closed")
	ErrMaxRetries   = errors.New("max retries exceeded")
	ErrTaskNotFound = errors.New("task not found")
)

// Queue defines the interface for task scheduling.
type Queue interface {
	// Enqueue adds a task to the queue.
	Enqueue(task *types.HiveTask) error
	// Dequeue removes and returns the next task. Blocks until a task is available or context is cancelled.
	Dequeue(ctx context.Context) (*types.HiveTask, uint, error)
	// Length returns the current number of tasks waiting in the queue.
	Length() int
	// Close shuts down the queue, unblocking all waiting Dequeue calls.
	Close()
	// MaxRetries returns the maximum number of retries for a task.
	MaxRetries() int
	// ScheduleRetry schedules a task to be re-enqueued after a backoff delay.
	// The ctx is used to cancel the retry if the queue is closed.
	ScheduleRetry(ctx context.Context, task *types.HiveTask, attempt uint) error
}

// MemoryQueue is an in-memory implementation of Queue.
type MemoryQueue struct {
	mu       sync.Mutex
	items    []*types.HiveTask
	closed   bool
	attempts map[string]uint // taskID → retry count
	maxRetry uint
}

// MemoryQueueOption configures the MemoryQueue.
type MemoryQueueOption func(*MemoryQueue)

// WithMaxAttempts sets the maximum number of retries before a task is considered failed.
func WithMaxAttempts(n uint) MemoryQueueOption {
	return func(q *MemoryQueue) {
		q.maxRetry = n
	}
}

// NewMemoryQueue creates a new in-memory task queue.
func NewMemoryQueue(opts ...MemoryQueueOption) *MemoryQueue {
	q := &MemoryQueue{
		items:    make([]*types.HiveTask, 0),
		attempts: make(map[string]uint),
		maxRetry: 3, // Default: 3 retries
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}

// Enqueue implements [Queue].
func (q *MemoryQueue) Enqueue(task *types.HiveTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}
	attempts, isAttempts := q.attempts[task.ID]
	switch {
	case isAttempts && attempts >= q.maxRetry:
		return ErrMaxRetries
	case !isAttempts:
		q.attempts[task.ID] = 1
	default:
		q.attempts[task.ID]++
	}

	q.items = append(q.items, task)
	return nil
}

// Dequeue implements [Queue].
func (q *MemoryQueue) Dequeue(ctx context.Context) (*types.HiveTask, uint, error) {
	tickCh := time.Tick(time.Millisecond * 10)
	for {
		q.mu.Lock()

		// Check if there's a task available
		if len(q.items) > 0 {
			task := q.items[0]
			q.items = q.items[1:]
			q.mu.Unlock()

			return task, uint(q.attempts[task.ID]), nil
		}

		// Check if queue is closed
		if q.closed {
			q.mu.Unlock()
			return nil, 0, ErrQueueClosed
		}

		q.mu.Unlock()

		// Wait with context awareness (polling with signal)
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-tickCh:
			// Poll again
		}
	}
}

// Length implements [Queue].
func (q *MemoryQueue) Length() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Close implements [Queue].
func (q *MemoryQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
}

// MaxRetries implements [Queue].
func (q *MemoryQueue) MaxRetries() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int(q.maxRetry)
}

// ScheduleRetry schedules a task to be re-enqueued after a backoff delay.
// It spawns a goroutine to handle the delay and re-enqueue.
func (q *MemoryQueue) ScheduleRetry(ctx context.Context, task *types.HiveTask, attempt uint) error {
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	delay := min(baseDelay<<attempt, maxDelay)

	go func() {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}

		slog.Info("re-enqueueing task after backoff",
			slog.String("task_id", task.ID),
			slog.Uint64("attempt", uint64(attempt)),
			slog.Duration("delay", delay),
		)
		if err := q.Enqueue(task); err != nil {
			slog.Info("failed to re-enqueueing task", slog.String("err", err.Error()))
			return
		}
	}()

	return nil
}
