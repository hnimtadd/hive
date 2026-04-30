package eventbus

import (
	"sync"
	"testing"
	"time"
)

func TestPublishDeliversMultipleEvents(t *testing.T) {
	t.Parallel()

	bus := NewEventBus[int]()
	sub := bus.Subscribe("topic-a")
	pub := bus.Publish("topic-a")

	pub <- 1
	pub <- 2
	close(pub)

	if got := readWithTimeout(t, sub); got != 1 {
		t.Fatalf("expected first event 1, got %d", got)
	}
	if got := readWithTimeout(t, sub); got != 2 {
		t.Fatalf("expected second event 2, got %d", got)
	}
}

func TestSubscribeWithCancelStopsDelivery(t *testing.T) {
	t.Parallel()

	bus := NewEventBus[int]()
	sub, cancel := bus.SubscribeWithCancel("topic-b")
	pub := bus.Publish("topic-b")

	pub <- 10
	if got := readWithTimeout(t, sub); got != 10 {
		t.Fatalf("expected event 10, got %d", got)
	}

	cancel()

	select {
	case _, ok := <-sub:
		if ok {
			t.Fatal("expected subscriber channel to be closed after cancel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting subscriber close")
	}

	close(pub)
}

func TestConcurrentSubscribeGetsEvent(t *testing.T) {
	t.Parallel()

	bus := NewEventBus[int]()
	const subscribers = 32

	subs := make([]<-chan int, 0, subscribers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(subscribers)
	for i := 0; i < subscribers; i++ {
		go func() {
			defer wg.Done()
			sub := bus.Subscribe("topic-c")
			mu.Lock()
			subs = append(subs, sub)
			mu.Unlock()
		}()
	}
	wg.Wait()

	pub := bus.Publish("topic-c")
	pub <- 7
	close(pub)

	for _, sub := range subs {
		if got := readWithTimeout(t, sub); got != 7 {
			t.Fatalf("expected event 7, got %d", got)
		}
	}
}

func readWithTimeout(t *testing.T, ch <-chan int) int {
	t.Helper()
	select {
	case got, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before receiving event")
		}
		return got
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
		return 0
	}
}
