package queue

import (
	"context"
	"errors"
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
	Dequeue(ctx context.Context) (*types.HiveTask, error)
	// Length returns the current number of tasks waiting in the queue.
	Length() int
	// Close shuts down the queue, unblocking all waiting Dequeue calls.
	Close()
}

// MemoryQueue is an in-memory implementation of Queue.
// Suitable for personal use
type MemoryQueue struct {
	mu       sync.Mutex
	items    []*types.HiveTask
	closed   bool
	attempts map[string]int // taskID → retry count
	maxRetry int
}

// MemoryQueueOption configures the MemoryQueue.
type MemoryQueueOption func(*MemoryQueue)

// WithMaxAttempts sets the maximum number of retries before a task is considered failed.
func WithMaxAttempts(n int) MemoryQueueOption {
	return func(q *MemoryQueue) {
		q.maxRetry = n
	}
}

// NewMemoryQueue creates a new in-memory task queue.
func NewMemoryQueue(opts ...MemoryQueueOption) *MemoryQueue {
	q := &MemoryQueue{
		items:    make([]*types.HiveTask, 0),
		attempts: make(map[string]int),
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
	if isAttempts && attempts >= q.maxRetry {
		return ErrMaxRetries
	} else if !isAttempts {
		q.attempts[task.ID] = 1
	} else {
		q.attempts[task.ID] += 1
	}

	q.items = append(q.items, task)
	return nil
}

// Dequeue implements [Queue].
func (q *MemoryQueue) Dequeue(ctx context.Context) (*types.HiveTask, error) {
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
		case <-time.After(10 * time.Millisecond):
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
