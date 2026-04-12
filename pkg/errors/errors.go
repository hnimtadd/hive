package errors

import (
	"context"
	"fmt"
	"maps"
	"time"
)

// ErrorType represents different categories of errors for better handling
type ErrorType string

const (
	ErrTypeValidation     ErrorType = "VALIDATION"
	ErrTypeNotFound       ErrorType = "NOT_FOUND"
	ErrTypeAuthorization  ErrorType = "AUTHORIZATION"
	ErrTypeNetwork        ErrorType = "NETWORK"
	ErrTypeTimeout        ErrorType = "TIMEOUT"
	ErrTypeRateLimit      ErrorType = "RATE_LIMIT"
	ErrTypeInternal       ErrorType = "INTERNAL"
	ErrTypeRetryExhausted ErrorType = "RETRY_EXHAUSTED"
)

// HiveError represents a structured error with additional context
type HiveError struct {
	Type      ErrorType
	Message   string
	Cause     error
	Context   map[string]interface{}
	Timestamp time.Time
}

// NewHiveError creates a new structured error
func NewHiveError(errType ErrorType, message string, cause error) *HiveError {
	return &HiveError{
		Type:      errType,
		Message:   message,
		Cause:     cause,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// Error implements the error interface
func (e *HiveError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %s)", e.Type, e.Message, e.Cause.Error())
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause for error unwrapping
func (e *HiveError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error.
func (e *HiveError) WithContext(key string, value interface{}) *HiveError {
	// Create a copy to avoid mutating the original
	newErr := *e
	newErr.Context = make(map[string]interface{})
	maps.Copy(newErr.Context, e.Context)
	newErr.Context[key] = value
	return &newErr
}

// RetryConfig defines configuration for retry behavior.
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	BackoffFactor float64
	MaxDelay      time.Duration
}

// GetDelay calculates the delay for a given attempt using exponential backoff
func (c *RetryConfig) GetDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return c.InitialDelay
	}

	// Calculate exponential backoff: InitialDelay * BackoffFactor^(attempt-1)
	multiplier := 1.0
	for i := 1; i < attempt; i++ {
		multiplier *= c.BackoffFactor
	}

	delay := time.Duration(float64(c.InitialDelay) * multiplier)
	if c.MaxDelay > 0 && delay > c.MaxDelay {
		return c.MaxDelay
	}
	return delay
}

// ErrorHandler provides utilities for error handling with retry and graceful degradation.
type ErrorHandler[T any] struct{}

// NewErrorHandler creates a new error handler.
func NewErrorHandler[T any]() *ErrorHandler[T] {
	return &ErrorHandler[T]{}
}

// WithRetry executes a function with retry logic for recoverable errors.
func (h *ErrorHandler[T]) WithRetry(
	ctx context.Context,
	config RetryConfig,
	fn func(context.Context) (T, error),
) (t T, err error) {
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation before each attempt
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return t, err
		default:
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't sleep on the last attempt
		if attempt < config.MaxAttempts {
			delay := config.GetDelay(attempt)

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return t, ctx.Err()
			case <-timer.C:
				// Continue to next attempt
			}
		}
	}

	// All attempts exhausted
	return t, NewHiveError(
		ErrTypeRetryExhausted,
		fmt.Sprintf("all %d retry attempts exhausted", config.MaxAttempts),
		lastErr,
	)
}

// WithGracefulDegradation executes a primary function with fallback on error.
func (h *ErrorHandler[T]) WithGracefulDegradation(
	ctx context.Context,
	primary func(context.Context) (T, error),
	fallback func(context.Context, error) (T, error),
) (T, error) {
	result, err := primary(ctx)
	if err == nil {
		return result, nil
	}

	// Primary failed, try fallback
	return fallback(ctx, err)
}

// AsHiveError checks if an error is a HiveError and extracts it
func AsHiveError(err error, target **HiveError) bool {
	if hiveErr, ok := err.(*HiveError); ok {
		*target = hiveErr
		return true
	}
	return false
}

// Common error constructors for convenience

// ErrValidation creates a validation error
func ErrValidation(message string) *HiveError {
	return NewHiveError(ErrTypeValidation, message, nil)
}

// ErrNotFound creates a not found error
func ErrNotFound(resource, id string) *HiveError {
	return NewHiveError(ErrTypeNotFound, fmt.Sprintf("%s '%s' not found", resource, id), nil)
}

// ErrAuthorization creates an authorization error
func ErrAuthorization(message string) *HiveError {
	return NewHiveError(ErrTypeAuthorization, message, nil)
}

// ErrNetwork creates a network error
func ErrNetwork(message string, cause error) *HiveError {
	return NewHiveError(ErrTypeNetwork, message, cause)
}

// ErrTimeout creates a timeout error
func ErrTimeout(duration time.Duration) *HiveError {
	return NewHiveError(ErrTypeTimeout, fmt.Sprintf("operation timed out after %s", duration), nil)
}

// ErrRateLimit creates a rate limit error
func ErrRateLimit(message string) *HiveError {
	return NewHiveError(ErrTypeRateLimit, message, nil)
}

// ErrInternal creates an internal error
func ErrInternal(message string, cause error) *HiveError {
	return NewHiveError(ErrTypeInternal, message, cause)
}
