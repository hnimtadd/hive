package react

import "context"

type contextKey string

const contextKeyMiddleware contextKey = "middleware"

func ContextWithToolMiddleware(ctx context.Context, mw ToolExecutionMiddleware) context.Context {
	return context.WithValue(ctx, contextKeyMiddleware, mw)
}

func MiddlewareFromContext(ctx context.Context) (ToolExecutionMiddleware, bool) {
	mwAny := ctx.Value(contextKeyMiddleware)
	mw, isMw := mwAny.(ToolExecutionMiddleware)
	return mw, isMw
}
