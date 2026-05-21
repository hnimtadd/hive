package types //nolint:revive // this package name is acceptable

import "context"

func Ptr[T any](val T) *T {
	return &val
}

type contextKey string

var taskContextKey contextKey = "task"

func ContextWithTask(ctx context.Context, task *Conversation) context.Context {
	return context.WithValue(ctx, taskContextKey, task)
}

func TaskFromContext(ctx context.Context) (*Conversation, bool) {
	taskAny := ctx.Value(taskContextKey)
	if taskAny == nil {
		return nil, false
	}
	task, isTask := taskAny.(*Conversation)
	if !isTask {
		return nil, false
	}
	return task, true
}
