package events

import "sync"

// Event is a simple topic + payload envelope
type Event struct {
	Topic   string
	Payload interface{}
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
func (eb *EventBus) Subscribe() (<-chan Event, func()) {
	eb.mu.Lock()
	id := eb.nextID
	eb.nextID++
	ch := make(chan Event, 32)
	eb.subs[id] = ch
	eb.mu.Unlock()

	unsubscribe := func() {
		eb.mu.Lock()
		c, ok := eb.subs[id]
		if ok {
			close(c)
			delete(eb.subs, id)
		}
		eb.mu.Unlock()
	}
	return ch, unsubscribe
}

// Emit publishes an event to all subscribers
func (eb *EventBus) Emit(topic string, payload any) {
	e := Event{Topic: topic, Payload: payload}
	eb.mu.RLock()
	for _, ch := range eb.subs {
		select {
		case ch <- e:
		default:
			// If buffer is full, drop to avoid blocking
		}
	}
	eb.mu.RUnlock()
}

// global event bus instance
var eventBus = NewEventBus()

func Emit(topic string, payload any) {
	eventBus.Emit(topic, payload)
}

func Subscribe() (<-chan Event, func()) {
	return eventBus.Subscribe()
}
