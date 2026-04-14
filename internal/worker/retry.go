package worker

import (
	"context"
	"time"

	"github.com/hnimtadd/hive/internal/queue"
	"github.com/hnimtadd/hive/pkg/types"
)

const (
	DefaultBaseDelay  = 1 * time.Second
	DefaultMaxDelay   = 30 * time.Second
	DefaultMaxRetries = 3
)

type RetryConfig struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	MaxRetries int
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		BaseDelay:  DefaultBaseDelay,
		MaxDelay:   DefaultMaxDelay,
		MaxRetries: DefaultMaxRetries,
	}
}

func CalculateBackoff(attempt uint, config *RetryConfig) time.Duration {
	if config == nil {
		config = DefaultRetryConfig()
	}

	delay := config.BaseDelay << attempt
	if delay > config.MaxDelay {
		return config.MaxDelay
	}
	return delay
}

func ScheduleRetry(ctx context.Context, q queue.Queue, task *types.HiveTask, attempt uint, config *RetryConfig) error {
	_ = CalculateBackoff(attempt, config)
	return q.ScheduleRetry(ctx, task, attempt)
}
