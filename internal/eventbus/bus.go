package eventbus

import "time"

type EventBus interface {
	Publish(event Event)
	Subscribe(filter EventFilter, handler Handler) Subscription
}

type Event interface {
	ID() string
	Type() string
	Payload() any
	Timestamp() time.Time
}

type EventFilter func(event Event) bool
type Handler func(event Event)
type Subscription interface {
	Unsubscribe()
}
