// Package status carries server-side events to connected extensions and
// probes the health of the installed runtimes.
package status

import (
	"sync"
	"time"
)

// Event is one server-sent event. Type is the SSE event name the extension
// listens for: runtime.status, index.progress, folder.changed, server.notice.
type Event struct {
	Type string `json:"type"`
	At   int64  `json:"at"`
	Data any    `json:"data,omitempty"`
}

// Bus is a fan-out of events to every open /v1/events stream.
//
// Publishing never blocks: a subscriber that cannot keep up loses events
// rather than stalling an indexing pass or a chat turn.
type Bus struct {
	mu   sync.RWMutex
	next int
	subs map[int]chan Event
}

// NewBus creates an empty bus.
func NewBus() *Bus { return &Bus{subs: map[int]chan Event{}} }

// Subscribe returns a channel of events and a function that closes it.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, 64)
	b.subs[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if c, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(c)
		}
	}
}

// Publish delivers an event to every subscriber, dropping it for any
// subscriber whose buffer is full.
func (b *Bus) Publish(ev Event) {
	if ev.At == 0 {
		ev.At = time.Now().UnixMilli()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Emit is a shorthand for Publish with a type and payload.
func (b *Bus) Emit(typ string, data any) { b.Publish(Event{Type: typ, Data: data}) }

// Subscribers reports how many streams are open.
func (b *Bus) Subscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
