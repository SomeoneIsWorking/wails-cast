package events

import "sync"

// Event is a simple topic + payload envelope
type Event struct {
	Topic   string
	Payload any
}

// EventBus provides a simple pub/sub for backend components
type EventBus struct {
	mu     sync.RWMutex
	subs   map[int]chan Event
	nextID int
}

// NewEventBus creates a new EventBus
func NewEventBus() *EventBus {
	return &EventBus{
		subs: make(map[int]chan Event),
	}
}

// Subscribe returns a channel receiving all events and an unsubscribe function
func (eb *EventBus) Subscribe(callback func(string, any)) func() {
	eb.mu.Lock()
	id := eb.nextID
	eb.nextID++
	// Buffered so bursts of events (e.g. token-by-token translation streaming)
	// don't block the publisher; combined with a blocking send in Emit this
	// guarantees in-order, lossless delivery.
	ch := make(chan Event, 1024)
	eb.subs[id] = ch
	eb.mu.Unlock()

	unsubscribe := func() {
		eb.mu.Lock()
		close(ch)
		delete(eb.subs, id)
		eb.mu.Unlock()
	}

	go func() {
		for value := range ch {
			callback(value.Topic, value.Payload)
		}
	}()

	return unsubscribe
}

// Emit publishes an event to all subscribers. The send is blocking (not a
// best-effort select/default) so events are never dropped and stay ordered —
// dropping was producing garbled live translation streams. Holding RLock for
// the send is safe because Unsubscribe (which closes the channel) takes the
// write lock and therefore cannot run concurrently with a send.
func (eb *EventBus) Emit(topic string, payload any) {
	e := Event{Topic: topic, Payload: payload}
	eb.mu.RLock()
	for _, ch := range eb.subs {
		ch <- e
	}
	eb.mu.RUnlock()
}

// global event bus instance
var eventBus = NewEventBus()

func Emit(topic string, payload any) {
	eventBus.Emit(topic, payload)
}

func Subscribe(callback func(string, any)) func() {
	return eventBus.Subscribe(callback)
}
