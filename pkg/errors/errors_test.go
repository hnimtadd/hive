package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHiveError_Creation(t *testing.T) {
	// Test basic error creation
	err := NewHiveError(ErrTypeValidation, "test validation error", nil)
	assert.Equal(t, ErrTypeValidation, err.Type)
	assert.Equal(t, "test validation error", err.Message)
	assert.Nil(t, err.Cause)
	assert.NotZero(t, err.Timestamp)

	// Test error with cause
	cause := errors.New("underlying error")
	err = NewHiveError(ErrTypeInternal, "wrapper error", cause)
	assert.Equal(t, ErrTypeInternal, err.Type)
	assert.Equal(t, "wrapper error", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestHiveError_Error(t *testing.T) {
	// Test error without cause
	err := NewHiveError(ErrTypeValidation, "validation failed", nil)
	expected := "VALIDATION: validation failed"
	assert.Equal(t, expected, err.Error())

	// Test error with cause
	cause := errors.New("underlying issue")
	err = NewHiveError(ErrTypeInternal, "internal error", cause)
	expected = "INTERNAL: internal error (caused by: underlying issue)"
	assert.Equal(t, expected, err.Error())
}

func TestHiveError_Unwrap(t *testing.T) {
	// Test unwrapping without cause
	err := NewHiveError(ErrTypeValidation, "validation failed", nil)
	assert.Nil(t, err.Unwrap())

	// Test unwrapping with cause
	cause := errors.New("underlying issue")
	err = NewHiveError(ErrTypeInternal, "internal error", cause)
	assert.Equal(t, cause, err.Unwrap())
}

func TestHiveError_WithContext(t *testing.T) {
	err := NewHiveError(ErrTypeValidation, "validation failed", nil)

	enrichedErr := err.WithContext("field", "username").WithContext("value", "invalid@")

	assert.Len(t, enrichedErr.Context, 2)
	assert.Equal(t, "username", enrichedErr.Context["field"])
	assert.Equal(t, "invalid@", enrichedErr.Context["value"])

	// Original error should be unchanged
	assert.Len(t, err.Context, 0)
}

func TestRetryConfig(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		BackoffFactor: 2.0,
		MaxDelay:      1 * time.Second,
	}

	// Test delay calculation
	delays := []time.Duration{
		config.GetDelay(1), // First attempt (no delay needed but returns initial)
		config.GetDelay(2), // First retry (InitialDelay)
		config.GetDelay(3), // Second retry (InitialDelay * BackoffFactor)
		config.GetDelay(4), // Third retry (should cap at MaxDelay)
	}

	assert.Equal(t, 100*time.Millisecond, delays[0]) // Initial delay
	assert.Equal(t, 200*time.Millisecond, delays[1]) // 100ms * 2
	assert.Equal(t, 400*time.Millisecond, delays[2]) // 100ms * 2^2
	assert.Equal(t, 800*time.Millisecond, delays[3]) // 100ms * 2^3, but not capped yet since 800ms < 1s

	// Test capping at MaxDelay
	cappedDelay := config.GetDelay(5) // Should be capped at 1s
	assert.Equal(t, 1*time.Second, cappedDelay)
}

func TestErrorHandler_WithRetry(t *testing.T) {
	handler := NewErrorHandler[any]()

	t.Run("successful execution on first attempt", func(t *testing.T) {
		callCount := 0
		config := RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  10 * time.Millisecond,
			BackoffFactor: 2.0,
		}

		result, err := handler.WithRetry(context.Background(), config, func(ctx context.Context) (interface{}, error) {
			callCount++
			return "success", nil
		})

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.Equal(t, 1, callCount)
	})

	t.Run("success after retries", func(t *testing.T) {
		callCount := 0
		config := RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  10 * time.Millisecond,
			BackoffFactor: 2.0,
		}

		result, err := handler.WithRetry(context.Background(), config, func(ctx context.Context) (interface{}, error) {
			callCount++
			if callCount < 3 {
				return nil, NewHiveError(ErrTypeNetwork, "network error", nil)
			}
			return "success", nil
		})

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.Equal(t, 3, callCount)
	})

	t.Run("non-recoverable error stops retry", func(t *testing.T) {
		callCount := 0
		config := RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  10 * time.Millisecond,
			BackoffFactor: 2.0,
		}

		result, err := handler.WithRetry(context.Background(), config, func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, NewHiveError(ErrTypeValidation, "validation error", nil)
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, 1, callCount) // Should not retry
	})

	t.Run("max attempts exhausted", func(t *testing.T) {
		callCount := 0
		config := RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  10 * time.Millisecond,
			BackoffFactor: 2.0,
		}

		result, err := handler.WithRetry(context.Background(), config, func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, NewHiveError(ErrTypeNetwork, "persistent network error", nil)
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, 3, callCount) // All attempts exhausted

		// Should be wrapped in a retry exhausted error
		var hiveErr *HiveError
		assert.True(t, errors.As(err, &hiveErr))
		assert.Equal(t, ErrTypeRetryExhausted, hiveErr.Type)
	})

	t.Run("context cancellation", func(t *testing.T) {
		callCount := 0
		config := RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  100 * time.Millisecond, // Longer delay to test cancellation
			BackoffFactor: 2.0,
		}

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		result, err := handler.WithRetry(ctx, config, func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, NewHiveError(ErrTypeNetwork, "network error", nil)
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.Equal(t, 1, callCount) // Should stop on context cancellation
	})
}

func TestErrorHandler_WithGracefulDegradation(t *testing.T) {
	handler := NewErrorHandler[any]()

	t.Run("primary function succeeds", func(t *testing.T) {
		result, err := handler.WithGracefulDegradation(
			context.Background(),
			func(ctx context.Context) (interface{}, error) {
				return "primary success", nil
			},
			func(ctx context.Context, err error) (interface{}, error) {
				t.Errorf("Fallback should not be called")
				return nil, fmt.Errorf("fallback called unexpectedly")
			},
		)

		assert.NoError(t, err)
		assert.Equal(t, "primary success", result)
	})

	t.Run("primary function fails, fallback succeeds", func(t *testing.T) {
		primaryErr := NewHiveError(ErrTypeNetwork, "network error", nil)

		result, err := handler.WithGracefulDegradation(
			context.Background(),
			func(ctx context.Context) (interface{}, error) {
				return nil, primaryErr
			},
			func(ctx context.Context, err error) (interface{}, error) {
				assert.Equal(t, primaryErr, err)
				return "fallback success", nil
			},
		)

		assert.NoError(t, err)
		assert.Equal(t, "fallback success", result)
	})

	t.Run("both primary and fallback fail", func(t *testing.T) {
		primaryErr := NewHiveError(ErrTypeNetwork, "network error", nil)
		fallbackErr := NewHiveError(ErrTypeInternal, "fallback error", nil)

		result, err := handler.WithGracefulDegradation(
			context.Background(),
			func(ctx context.Context) (interface{}, error) {
				return nil, primaryErr
			},
			func(ctx context.Context, err error) (interface{}, error) {
				return nil, fallbackErr
			},
		)

		assert.Error(t, err)
		assert.Nil(t, result)

		// Should return the fallback error
		var hiveErr *HiveError
		assert.True(t, errors.As(err, &hiveErr))
		assert.Equal(t, ErrTypeInternal, hiveErr.Type)
	})
}

func TestCommonErrors(t *testing.T) {
	// Test pre-defined error creators
	validationErr := ErrValidation("invalid input")
	assert.Equal(t, ErrTypeValidation, validationErr.Type)
	assert.Contains(t, validationErr.Message, "invalid input")

	notFoundErr := ErrNotFound("resource", "123")
	assert.Equal(t, ErrTypeNotFound, notFoundErr.Type)
	assert.Contains(t, notFoundErr.Message, "resource")
	assert.Contains(t, notFoundErr.Message, "123")

	timeoutErr := ErrTimeout(5 * time.Second)
	assert.Equal(t, ErrTypeTimeout, timeoutErr.Type)
	assert.Contains(t, timeoutErr.Message, "5s")
}

