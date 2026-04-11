package trace

import (
	"context"
	"crypto/rand"
	"fmt"
)

type contextKey string

const (
	traceContextKey contextKey = "trace"
)

// Context holds trace information.
type Context struct {
	TraceID  string
	ParentID string
	SpanID   string
}

func NewRootTraceContext() *Context {
	traceID := NewTraceID()
	spanID := NewSpanID()
	return &Context{
		TraceID:  traceID,
		ParentID: traceID,
		SpanID:   spanID,
	}
}

// ContextWithChildSpan adds trace ID to context..
func ContextWithChildSpan(ctx context.Context) (context.Context, *Context) {
	tc, found := TraceContextFromContext(ctx)
	if !found {
		tc = NewRootTraceContext()
	}
	childCtx := tc.ChildContext()
	return ContextWithTraceContext(ctx, childCtx), childCtx
}

// ContextWithTraceContext adds trace ID to context..
func ContextWithTraceContext(ctx context.Context, tc *Context) context.Context {
	return context.WithValue(ctx, traceContextKey, tc)
}

// TraceContextFromContext retrieves trace context.
func TraceContextFromContext(ctx context.Context) (*Context, bool) { //nolint: revive // this name is acceptable
	tc, ok := ctx.Value(traceContextKey).(*Context)
	return tc, ok
}

func (ctx *Context) ChildContext() *Context {
	return &Context{
		TraceID:  ctx.TraceID,
		ParentID: ctx.SpanID,
		SpanID:   NewSpanID(),
	}
}

func NewTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%032x", b)
}

func NewSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%016x", b)
}
