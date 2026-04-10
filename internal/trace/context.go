package trace

import (
	"context"
)

type contextKey string

const (
	traceContextKey  contextKey = "trace"
	sessionLoggerKey contextKey = "hive_session_logger"
)

// ContextWithTraceContext adds trace ID to context
func ContextWithTraceContext(ctx context.Context, id ID) context.Context {
	tc := &TraceContext{
		TraceID: id,
	}
	return context.WithValue(ctx, traceContextKey, tc)
}

// TraceContextFromContext retrieves trace context
func TraceContextFromContext(ctx context.Context) (*TraceContext, bool) {
	tc, ok := ctx.Value(traceContextKey).(*TraceContext)
	return tc, ok
}

// ContextWithSessionLogger adds session logger to context
func ContextWithSessionLogger(ctx context.Context, logger *SessionLogger) context.Context {
	return context.WithValue(ctx, sessionLoggerKey, logger)
}

// SessionLoggerFromContext retrieves session logger from context
func SessionLoggerFromContext(ctx context.Context) (*SessionLogger, bool) {
	sl, ok := ctx.Value(sessionLoggerKey).(*SessionLogger)
	return sl, ok
}

// GetSessionLogger returns session logger from context or no-op if not present
func GetSessionLogger(ctx context.Context) *SessionLogger {
	if logger, ok := SessionLoggerFromContext(ctx); ok {
		return logger
	}
	return &SessionLogger{config: nil}
}

// ContextWithTraceAndLogger creates context with both trace ID and session logger
func ContextWithTraceAndLogger(ctx context.Context, traceID ID, logger *SessionLogger) context.Context {
	ctx = ContextWithTraceContext(ctx, traceID)
	if logger != nil {
		ctx = ContextWithSessionLogger(ctx, logger)
	}
	return ctx
}

// TraceContext holds trace information
type TraceContext struct {
	TraceID ID
}
