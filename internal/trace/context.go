package trace

import (
	"context"
)

type contextKey string

const (
	traceContextKey contextKey = "trace"
)

func ContextWithTrace(ctx context.Context, id ID) context.Context {
	tc := &Context{
		TraceID: id,
	}
	return context.WithValue(ctx, traceContextKey, tc)
}

func TraceFromContext(ctx context.Context) (*Context, bool) {
	tc, ok := ctx.Value(traceContextKey).(*Context)
	return tc, ok
}
