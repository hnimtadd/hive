package timeout

import (
	"context"
	"fmt"
	"time"
)

// Budget manages a timeout budget across multiple operations
type Budget struct {
	total     time.Duration
	remaining time.Duration
	start     time.Time
}

// NewBudget creates a new timeout budget
func NewBudget(total time.Duration) *Budget {
	return &Budget{
		total:     total,
		remaining: total,
		start:     time.Now(),
	}
}

// Allocate reserves a portion of the budget for an operation
// Returns the allocated duration and true if budget is available
func (b *Budget) Allocate(requested time.Duration) (time.Duration, bool) {
	// Update remaining based on elapsed time
	elapsed := time.Since(b.start)
	b.remaining = b.total - elapsed

	if b.remaining <= 0 {
		return 0, false
	}

	if requested > b.remaining {
		return b.remaining, true
	}

	return requested, true
}

// AllocatePercent reserves a percentage of remaining budget
func (b *Budget) AllocatePercent(percent float64) (time.Duration, bool) {
	if percent <= 0 || percent > 100 {
		percent = 100
	}

	elapsed := time.Since(b.start)
	b.remaining = b.total - elapsed

	if b.remaining <= 0 {
		return 0, false
	}

	allocated := time.Duration(float64(b.remaining) * percent / 100)
	return allocated, true
}

// Remaining returns the remaining budget
func (b *Budget) Remaining() time.Duration {
	elapsed := time.Since(b.start)
	remaining := b.total - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExhausted returns true if budget is exhausted
func (b *Budget) IsExhausted() bool {
	return b.Remaining() <= 0
}

// Context creates a context with the remaining budget as timeout
func (b *Budget) Context(parent context.Context) (context.Context, context.CancelFunc) {
	remaining := b.Remaining()
	if remaining <= 0 {
		// Return already cancelled context
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}
	return context.WithTimeout(parent, remaining)
}

// String returns budget status for debugging
func (b *Budget) String() string {
	return fmt.Sprintf("Budget{total: %s, remaining: %s, elapsed: %s}",
		b.total, b.Remaining(), time.Since(b.start))
}

// Operation represents an operation that uses timeout budget
type Operation struct {
	budget   *Budget
	allocated time.Duration
}

// StartOperation begins a new operation with allocated budget
func (b *Budget) StartOperation(requested time.Duration) (*Operation, bool) {
	allocated, ok := b.Allocate(requested)
	if !ok {
		return nil, false
	}
	return &Operation{
		budget:    b,
		allocated: allocated,
	}, true
}

// Context creates a context for the operation
func (o *Operation) Context(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, o.allocated)
}

// Release returns unused budget (call with remaining time)
func (o *Operation) Release(used time.Duration) {
	unused := o.allocated - used
	if unused > 0 {
		o.budget.remaining += unused
	}
}
