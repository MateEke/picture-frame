package state

import (
	"cmp"
	"slices"
	"sync"
)

// Kind identifies the category of an event.
type Kind string

const (
	KindSensor       Kind = "sensor"
	KindScreen       Kind = "screen"
	KindWeather      Kind = "weather"
	KindImage        Kind = "image"
	KindKiosk        Kind = "kiosk"
	KindScreenAspect Kind = "screen_aspect"
)

// Event carries a single state change notification from a producer to subscribers.
type Event struct {
	ID      uint64 // monotonically increasing; used for SSE Last-Event-ID
	Kind    Kind
	Payload Payload
}

// subChannelBuffer caps each subscriber's queue; the publisher drops beyond it.
const subChannelBuffer = 16

// Bus is a thread-safe broadcast pub/sub hub. Each subscriber gets its own
// buffered channel (full buffers are skipped, never blocking the publisher),
// and the latest event per Kind is retained for snapshots to late joiners.
type Bus struct {
	mu   sync.Mutex
	seq  uint64
	subs map[chan Event]struct{}
	last map[Kind]Event
}

func NewBus() *Bus {
	return &Bus{
		subs: make(map[chan Event]struct{}),
		last: make(map[Kind]Event),
	}
}

// Snapshot returns the latest event per Kind in publish order (ascending ID),
// used to send initial state to new SSE clients.
func (b *Bus) Snapshot() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, 0, len(b.last))
	for _, e := range b.last {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b Event) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// Subscribe returns a channel of future events and an unsubscribe func that must
// be called to release it; unsubscribe closes the channel so ranges exit.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subChannelBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}
}

// Publish broadcasts e with a monotonic ID, skipping full subscribers.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	e.ID = b.seq
	b.last[e.Kind] = e
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
