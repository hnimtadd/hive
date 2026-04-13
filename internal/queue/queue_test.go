package queue

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hnimtadd/hive/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	type TestCase struct {
		name       string
		maxAttemps int
		tasks      []Priority
		validation func(t *testing.T, queue Queue)
	}
	tcs := []TestCase{
		{
			name:  "2 task with same priority, use sequence as order",
			tasks: []Priority{PriorityNormal, PriorityNormal},
			validation: func(t *testing.T, queue Queue) {
				task, err := queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "0:0", task.ID)

				task, err = queue.Dequeue(context.Background())
				require.NoError(t, err)
				assert.Equal(t, "1:0", task.ID)
			},
		},
		{
			name:  "0 task, get with timeout",
			tasks: []Priority{},
			validation: func(t *testing.T, queue Queue) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
				defer cancel()

				task, err := queue.Dequeue(ctx)
				assert.Nil(t, task)
				require.ErrorIs(t, context.DeadlineExceeded, err)
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			queue := NewMemoryQueue(tc.maxAttemps)
			for i, priority := range tc.tasks {
				err := queue.Enqueue(&types.Task{ID: fmt.Sprintf("%d:%d", i, priority)}, priority)
				require.NoError(t, err)
			}
			tc.validation(t, queue)
		})
	}
}
