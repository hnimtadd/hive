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

type QueueTask[t any] struct {
	Task t
	Ctx  context.Context

	attempts int
}

// Queue defines the interface for task scheduling.
type Queue[t any] interface {
	// Enqueue adds a task to the queue.
	Enqueue(ctx context.Context, task t) error
	// Dequeue removes and returns the next task. Blocks until a task is available or context is cancelled.
	Dequeue(ctx context.Context) (*QueueTask[t], error)
	// Length returns the current number of tasks waiting in the queue.
	Length() int
	// Close shuts down the queue, unblocking all waiting Dequeue calls.
	Close()
	// MaxRetries returns the maximum number of retries for a task.
	MaxRetries() int
	// ScheduleRetry schedules a task to be re-enqueued after a backoff delay
	// The ctx is used to cancel the retry if the queue is closed.
	ScheduleRetry(ctx context.Context, task *QueueTask[t]) error
}

// MemoryQueue is an in-memory implementation of Queue.
type MemoryQueue struct {
	mu          sync.Mutex
	items       []*QueueTask[*types.HiveTask]
	closed      bool
	maxAttempts uint
}

// MemoryQueueOption configures the MemoryQueue.
type MemoryQueueOption func(*MemoryQueue)

// WithMaxAttempts sets the maximum number of retries before a task is considered failed.
func WithMaxAttempts(n uint) MemoryQueueOption {
	return func(q *MemoryQueue) {
		q.maxAttempts = n
	}
}

// NewMemoryQueue creates a new in-memory task queue.
func NewMemoryQueue(opts ...MemoryQueueOption) Queue[*types.HiveTask] {
	q := &MemoryQueue{
		items:       make([]*QueueTask[*types.HiveTask], 0),
		maxAttempts: 3, // Default: 3 retries
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}

// Enqueue implements [Queue].
func (q *MemoryQueue) Enqueue(ctx context.Context, task *types.HiveTask) error {
	queueTask := &QueueTask[*types.HiveTask]{
		Task:     task,
		Ctx:      ctx,
		attempts: 0,
	}

	return q.enqueue(ctx, queueTask)
}

func (q *MemoryQueue) enqueue(ctx context.Context, qt *QueueTask[*types.HiveTask]) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	q.items = append(q.items, qt)
	return nil
}

// Dequeue implements [Queue].
func (q *MemoryQueue) Dequeue(ctx context.Context) (*QueueTask[*types.HiveTask], error) {
	tickCh := time.Tick(time.Millisecond * 10)
	for {
		q.mu.Lock()

		// Check if there's a task available
		if len(q.items) > 0 {
			task := q.items[0]
			q.items = q.items[1:]
			q.mu.Unlock()

			return task, nil
		}

		// Check if queue is closed
		if q.closed {
			q.mu.Unlock()
			return nil, ErrQueueClosed
		}

		q.mu.Unlock()

		// Wait with context awareness (polling with signal)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
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
	return int(q.maxAttempts)
}

// ScheduleRetry schedules a task to be re-enqueued after a backoff delay.
// It spawns a goroutine to handle the delay and re-enqueue.
func (q *MemoryQueue) ScheduleRetry(ctx context.Context, task *QueueTask[*types.HiveTask]) error {
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second
	task.attempts++

	attempt := task.attempts
	if attempt > int(q.maxAttempts) {
		return ErrMaxRetries
	}

	delay := min(baseDelay<<attempt, maxDelay)

	go func() {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}

		slog.Info("re-enqueueing task after backoff",
			slog.String("task_id", task.Task.ID),
			slog.Uint64("attempt", uint64(attempt)),
			slog.Duration("delay", delay),
		)
		if err := q.enqueue(ctx, task); err != nil {
			slog.Info("failed to re-enqueueing task", slog.String("err", err.Error()))
			return
		}
	}()

	return nil
}
