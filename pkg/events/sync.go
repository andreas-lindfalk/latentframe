package events

import (
	"context"
	"fmt"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Handler reacts to a CloudEvent.
type Handler func(ctx context.Context, e cloudevents.Event) error

// SyncBus is an in-process Publisher: Publish dispatches synchronously to every
// handler subscribed to the event's type, in registration order. If any handler
// errors, Publish returns that error (fail-fast) — enough for the monolith; the
// Pub/Sub implementation will add real delivery semantics (retries, acks) later.
type SyncBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewSyncBus returns an empty in-process bus.
func NewSyncBus() *SyncBus {
	return &SyncBus{handlers: map[string][]Handler{}}
}

// Subscribe registers h for events of the given type.
func (b *SyncBus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// Publish dispatches e to all handlers registered for its type.
func (b *SyncBus) Publish(ctx context.Context, e cloudevents.Event) error {
	b.mu.RLock()
	hs := append([]Handler(nil), b.handlers[e.Type()]...)
	b.mu.RUnlock()
	for _, h := range hs {
		if err := h(ctx, e); err != nil {
			return fmt.Errorf("handling %s: %w", e.Type(), err)
		}
	}
	return nil
}

var _ Publisher = (*SyncBus)(nil)
