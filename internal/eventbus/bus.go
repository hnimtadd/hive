package eventbus

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

type EventBus[T any] struct {
	mu     sync.RWMutex
	topics map[string]*topic[T]
	logger *slog.Logger
	nextID atomic.Uint64
}

type topic[T any] struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan T
}

func NewEventBus[T any]() *EventBus[T] {
	return &EventBus[T]{
		topics: map[string]*topic[T]{},
		logger: slog.Default(),
	}
}

func (e *EventBus[T]) Subscribe(topic string) <-chan T {
	eventCh, _ := e.SubscribeWithCancel(topic)
	return eventCh
}

func (e *EventBus[T]) Publish(topic string) chan<- T {
	return e.PublishWithContext(context.Background(), topic)
}

func (e *EventBus[T]) PublishWithContext(ctx context.Context, topic string) chan<- T {
	eventCh := make(chan T, publisherBufferLength)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				e.dispatch(topic, event)
			}
		}
	}()

	return eventCh
}

func (e *EventBus[T]) SubscribeWithCancel(topicName string) (<-chan T, func()) {
	eventCh := make(chan T, channelBufferLength)
	subID := e.nextID.Add(1)
	topic := e.getOrCreateTopic(topicName)

	topic.mu.Lock()
	topic.subscribers[subID] = eventCh
	topic.mu.Unlock()

	cancel := func() {
		e.unsubscribe(topicName, subID)
	}

	return eventCh, cancel
}

func (e *EventBus[T]) dispatch(topicName string, event T) {
	topic := e.getTopic(topicName)
	if topic == nil {
		return
	}

	topic.mu.RLock()
	dropped := 0
	subscriberCount := len(topic.subscribers)
	for _, consumer := range topic.subscribers {
		select {
		case consumer <- event:
		default:
			dropped++
		}
	}
	topic.mu.RUnlock()

	if dropped > 0 {
		e.logger.Warn("eventbus: dropped events for slow subscribers",
			slog.String("topic", topicName),
			slog.Int("dropped_subscribers", dropped),
			slog.Int("subscriber_count", subscriberCount),
		)
	}
}

func (e *EventBus[T]) unsubscribe(topicName string, subID uint64) {
	topic := e.getTopic(topicName)
	if topic == nil {
		return
	}

	var shouldDeleteTopic bool
	topic.mu.Lock()
	consumer, ok := topic.subscribers[subID]
	if ok {
		delete(topic.subscribers, subID)
		close(consumer)
	}
	shouldDeleteTopic = len(topic.subscribers) == 0
	topic.mu.Unlock()

	if shouldDeleteTopic {
		e.mu.Lock()
		defer e.mu.Unlock()
		// Re-check under bus lock to avoid deleting a recreated/populated topic.
		current := e.topics[topicName]
		if current == topic {
			current.mu.RLock()
			empty := len(current.subscribers) == 0
			current.mu.RUnlock()
			if empty {
				delete(e.topics, topicName)
			}
		}
	}
}

func (e *EventBus[T]) getTopic(topicName string) *topic[T] {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.topics[topicName]
}

func (e *EventBus[T]) getOrCreateTopic(topicName string) *topic[T] {
	if topic := e.getTopic(topicName); topic != nil {
		return topic
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if topic, ok := e.topics[topicName]; ok {
		return topic
	}

	topic := &topic[T]{
		subscribers: make(map[uint64]chan T),
	}
	e.topics[topicName] = topic
	return topic
}
