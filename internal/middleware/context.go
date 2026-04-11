package middleware

import "context"

// MiddlewareKey is the context key for HiveMiddleware.
type contextKey string

const middlewareKey contextKey = "hive_middleware"

// ContextWithMiddleware adds a HiveMiddleware to context.
func ContextWithMiddleware(ctx context.Context, mw HiveMiddleware) context.Context {
	return context.WithValue(ctx, middlewareKey, mw)
}

// MiddlewareFromContext retrieves HiveMiddleware from context.
func MiddlewareFromContext(ctx context.Context) HiveMiddleware {
	mw, ok := ctx.Value(middlewareKey).(HiveMiddleware)
	if mw == nil || !ok {
		return NoopMiddleware()
	}
	return mw
}
