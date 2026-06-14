package mqtt

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

// connectRetry paces initial attempts; paho auto-reconnects after.
const connectRetry = 15 * time.Second

// Hub multiplexes one MQTT connection; subscriptions and OnConnect handlers
// persist across reconnects.
type Hub struct {
	log    *slog.Logger
	client Client

	// connected lets Subscribe issue the broker call immediately on a hot Hub.
	connected atomic.Bool

	mu       sync.Mutex
	handlers []func()
	subs     map[string]subscription
}

type subscription struct {
	qos     byte
	handler func([]byte)
}

// NewHub wires Hub into client callbacks; consumers go through Hub from here on.
func NewHub(log *slog.Logger, client Client) *Hub {
	h := &Hub{
		log:    log,
		client: client,
		subs:   make(map[string]subscription),
	}
	client.OnConnect(h.onConnect)
	client.OnConnectionLost(h.onConnectionLost)
	return h
}

// AddOnConnect registers f for every (re)connect. f runs on paho's onConnect
// goroutine, hop off it if you own mutable state.
func (h *Hub) AddOnConnect(f func()) {
	h.mu.Lock()
	h.handlers = append(h.handlers, f)
	h.mu.Unlock()
}

// Subscribe records a persistent subscription, replayed on every (re)connect;
// also issued synchronously when called on an already-connected Hub.
func (h *Hub) Subscribe(topic string, qos byte, handler func([]byte)) {
	h.mu.Lock()
	h.subs[topic] = subscription{qos: qos, handler: handler}
	h.mu.Unlock()
	if h.connected.Load() {
		if err := h.client.Subscribe(topic, qos, handler); err != nil {
			h.log.Warn("mqtt: subscribe failed (will retry on reconnect)", "topic", topic, "err", err)
		}
	}
}

// Publish forwards to the underlying client.
func (h *Hub) Publish(topic string, qos byte, retain bool, payload []byte) error {
	return h.client.Publish(topic, qos, retain, payload)
}

// Connect retries until success or ctx is cancelled; paho keeps the connection
// alive after.
func (h *Hub) Connect(ctx context.Context) bool {
	for {
		err := h.client.Connect()
		if err == nil {
			return true
		}
		h.log.Warn("mqtt: connect failed, retrying", "err", err, "in", connectRetry)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(connectRetry):
		}
	}
}

// Disconnect closes the underlying connection. Safe to call without Connect.
func (h *Hub) Disconnect() {
	h.connected.Store(false)
	h.client.Disconnect()
}

func (h *Hub) onConnectionLost(err error) {
	h.connected.Store(false)
	h.log.Warn("mqtt: connection lost, paho will auto-reconnect", "err", err)
}

// onConnect replays subs then fans out to handlers. Sets `connected` first so
// a racing Subscribe is safely idempotent.
func (h *Hub) onConnect() {
	h.connected.Store(true)
	h.mu.Lock()
	subs := maps.Clone(h.subs)
	handlers := append([]func(){}, h.handlers...)
	h.mu.Unlock()

	for topic, s := range subs {
		if err := h.client.Subscribe(topic, s.qos, s.handler); err != nil {
			h.log.Warn("mqtt: replay subscription failed", "topic", topic, "err", err)
		}
	}
	for _, f := range handlers {
		f()
	}
}
