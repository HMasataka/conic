package eventbus

import (
	"context"
	"sync"
)

// Handler represents an event handler function
type Handler func(event *Event)

// Bus represents an event bus
type Bus interface {
	// Publish publishes an event to all subscribers
	Publish(event *Event)

	// PublishAsync publishes an event asynchronously
	PublishAsync(event *Event)

	// Subscribe subscribes to events of a specific type
	Subscribe(eventType EventType, handler Handler) string

	// SubscribeAll subscribes to all events
	SubscribeAll(handler Handler) string

	// Unsubscribe removes a subscription
	Unsubscribe(id string)

	// Start starts the event bus
	Start(ctx context.Context)

	// Stop stops the event bus
	Stop()
}

// subscription represents a single subscription
type subscription struct {
	id        string
	eventType EventType
	handler   Handler
}

// InMemoryBus is an in-memory implementation of the event bus
type InMemoryBus struct {
	subscribers map[EventType][]*subscription
	allHandlers []*subscription
	mu          sync.RWMutex
	eventChan   chan *Event
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewInMemoryBus creates a new in-memory event bus
func NewInMemoryBus(bufferSize int) *InMemoryBus {
	return &InMemoryBus{
		subscribers: make(map[EventType][]*subscription),
		allHandlers: make([]*subscription, 0),
		eventChan:   make(chan *Event, bufferSize),
	}
}

// Publish publishes an event synchronously
func (b *InMemoryBus) Publish(event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Notify specific event type subscribers
	if subs, ok := b.subscribers[event.Type]; ok {
		for _, sub := range subs {
			sub.handler(event)
		}
	}

	// Notify all-event subscribers
	for _, sub := range b.allHandlers {
		sub.handler(event)
	}
}

// PublishAsync publishes an event asynchronously
func (b *InMemoryBus) PublishAsync(event *Event) {
	select {
	case b.eventChan <- event:
	default:
		// Channel is full, handle overflow
		// In production, consider logging or metrics
	}
}

// Subscribe subscribes to events of a specific type
func (b *InMemoryBus) Subscribe(eventType EventType, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &subscription{
		id:        generateID(),
		eventType: eventType,
		handler:   handler,
	}

	b.subscribers[eventType] = append(b.subscribers[eventType], sub)
	return sub.id
}

// SubscribeAll subscribes to all events
func (b *InMemoryBus) SubscribeAll(handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &subscription{
		id:      generateID(),
		handler: handler,
	}

	b.allHandlers = append(b.allHandlers, sub)
	return sub.id
}

// Unsubscribe removes a subscription
func (b *InMemoryBus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check type-specific subscriptions
	for eventType, subs := range b.subscribers {
		for i, sub := range subs {
			if sub.id == id {
				b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}

	// Check all-event subscriptions
	for i, sub := range b.allHandlers {
		if sub.id == id {
			b.allHandlers = append(b.allHandlers[:i], b.allHandlers[i+1:]...)
			return
		}
	}
}

// Start starts the event bus
func (b *InMemoryBus) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go b.processEvents()
}

// Stop stops the event bus
func (b *InMemoryBus) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	close(b.eventChan)
}

// processEvents processes events from the channel
func (b *InMemoryBus) processEvents() {
	defer b.wg.Done()

	for {
		select {
		case <-b.ctx.Done():
			return
		case event := <-b.eventChan:
			if event != nil {
				b.Publish(event)
			}
		}
	}
}
