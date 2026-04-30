package eventbus

import (
	"log/slog"
	"sync"
)

type EventBus[T any] struct {
	topics sync.Map
	logger *slog.Logger
}

func NewEventBus[T any]() *EventBus[T] {
	return &EventBus[T]{
		topics: sync.Map{},
		logger: slog.Default(),
	}
}

func (e *EventBus[T]) Subscribe(topic string) <-chan T {
	eventCh := make(chan T, channelBufferLength)
	brokerAny, ok := e.topics.Load(topic)
	if !ok {
		e.topics.Store(topic, []chan<- T{eventCh})
		return eventCh
	}
	// append the new event chan to the group
	broker := brokerAny.([]chan<- T) //nolint: errcheck // this is always event channel
	broker = append(broker, eventCh)
	e.topics.Store(topic, broker)

	return eventCh
}

func (e *EventBus[T]) Publish(topic string) chan<- T {
	eventCh := make(chan T)

	go func() {
		for event := range eventCh {
			groupAny, found := e.topics.Load(topic)
			if !found {
				continue
			}
			group := groupAny.([]chan<- T) //nolint: errcheck // this is always event channel
			for _, consumer := range group {
				select {
				case consumer <- event:
					continue
					// this is a non-blocking call
				default:
				}
			}
			return
		}
	}()

	return eventCh
}
