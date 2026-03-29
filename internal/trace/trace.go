package trace

import "github.com/google/uuid"

type ID string

type Context struct {
	TraceID ID // Root trace identifier
}

func NewID() ID {
	return ID(uuid.New().String())
}
