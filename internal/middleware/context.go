package middleware

import "context"

// MiddlewareKey is the context key for HiveMiddleware
type MiddlewareKey string

const middlewareKey MiddlewareKey = "hive_middleware"

// ContextWithMiddleware adds a HiveMiddleware to context
func ContextWithMiddleware(ctx context.Context, mw HiveMiddleware) context.Context {
	return context.WithValue(ctx, middlewareKey, mw)
}

// GetMiddleware retrieves HiveMiddleware from context
func GetMiddleware(ctx context.Context) (HiveMiddleware, bool) {
	mw, ok := ctx.Value(middlewareKey).(HiveMiddleware)
	return mw, ok
}
