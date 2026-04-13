package queue_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	type TestCase struct {
		name       string
		maxAttemps int
		tasks      []queue.Priority
		validation func(t *testing.T, queue queue.Queue)
	}
	tcs := []TestCase{
		{
			name:  "2 task with same priority, use sequence as order",
			tasks: []queue.Priority{queue.PriorityNormal, queue.PriorityNormal},
			validation: func(t *testing.T, queue queue.Queue) {
				task, err := queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "0:1", task.ID)

				task, err = queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "1:1", task.ID)
			},
		},
		{
			name:  "0 task, get with timeout",
			tasks: []queue.Priority{},
			validation: func(t *testing.T, queue queue.Queue) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
				defer cancel()

				task, err := queue.Dequeue(ctx)
				assert.Nil(t, task)
				require.ErrorIs(t, context.DeadlineExceeded, err)
			},
		},
		{
			name:  "different task with priorities",
			tasks: []queue.Priority{queue.PriorityNormal, queue.PriorityHigh, queue.PriorityNormal, queue.PriorityLow},
			validation: func(t *testing.T, queue queue.Queue) {
				// high first
				task, err := queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "1:2", task.ID)

				// normal later
				task, err = queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "0:1", task.ID)

				// normal next
				task, err = queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "2:1", task.ID)

				// low last
				task, err = queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "3:0", task.ID)

				ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
				defer cancel()

				task, err = queue.Dequeue(ctx)
				assert.Nil(t, task)
				require.ErrorIs(t, context.DeadlineExceeded, err)
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			queue := queue.NewMemoryQueue(tc.maxAttemps)
			for i, priority := range tc.tasks {
				err := queue.Enqueue(&types.Task{ID: fmt.Sprintf("%d:%d", i, priority)}, priority)
				require.NoError(t, err)
			}
			tc.validation(t, queue)
		})
	}
}
