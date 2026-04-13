package queue

import (
	"context"
	"slices"
	"sync"

	"github.com/hnimtadd/hive/internal/types"
)

type Queue interface {
	Enqueue(task *types.Task, priority Priority) error
	Dequeue(ctx context.Context) (*types.Task, error)
}

type QueuedTask struct {
	Task     *types.Task
	Priority Priority
	Attempts int
}

type MemoryQueue struct {
	mu       sync.Mutex
	queue    []*QueuedTask
	cond     *sync.Cond
	maxRetry int
	attemps  map[string]int
}

func NewMemoryQueue(maxRetry int) Queue {
	return &MemoryQueue{
		queue:    make([]*QueuedTask, 0, 1000),
		maxRetry: maxRetry,
		attemps:  make(map[string]int),
		mu:       sync.Mutex{},
		cond:     sync.NewCond(&sync.Mutex{}),
	}
}

// Enqueue implements [Queue].
func (m *MemoryQueue) Enqueue(task *types.Task, priority Priority) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	attemps, isRetried := m.attemps[task.ID]
	if isRetried && attemps >= m.maxRetry {
		return ErrMaxRetriesExceed
	} else if !isRetried {
		m.attemps[task.ID] = 0
	}

	m.queue = append(m.queue, &QueuedTask{
		Task:     task,
		Priority: priority,
		Attempts: 0,
	})
	slices.SortStableFunc(m.queue, func(a, b *QueuedTask) int {
		if b == nil {
			return -1
		}
		if a == nil {
			return 1
		}
		return int(b.Priority) - int(a.Priority)
	})
	m.cond.Signal()
	return nil
}

// Dequeue implements [Queue].
func (m *MemoryQueue) Dequeue(ctx context.Context) (*types.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(m.queue) == 0 {
		m.cond.L.Lock()
		waitCh := make(chan struct{})
		go func() {
			// This will be return when the new queuedTask is ready, which is trigger by m.cond.Signal
			m.cond.Wait()
			close(waitCh)
		}()
		select {
		case <-waitCh:
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	// at this point, it's guarantee that q.queue will has at least 1 elemenet.
	item := m.queue[0]
	m.queue = m.queue[1:]
	m.attemps[item.Task.ID]++
	return item.Task, nil
}

var _ Queue = &MemoryQueue{}
