// Package kioskwatch monitors heartbeats from the kiosk frontend.
// If no heartbeat is received for a configured timeout, it invokes a callback.
package kioskwatch

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Watch struct {
	log       *slog.Logger
	timeout   time.Duration
	onTimeout func()
	lastBeat  atomic.Int64
	// onBeat observes each beat's version, cleared once it returns true. The mutex serialises
	// handle-and-clear so concurrent beats can't double-fire it.
	mu     sync.Mutex
	onBeat func(version string) bool
}

func New(log *slog.Logger, timeout time.Duration, onTimeout func()) *Watch {
	w := &Watch{
		log:       log,
		timeout:   timeout,
		onTimeout: onTimeout,
	}

	w.lastBeat.Store(time.Now().UnixNano())
	return w
}

// Beat records a heartbeat carrying the frontend's build version. Safe from any goroutine.
func (w *Watch) Beat(version string) {
	w.lastBeat.Store(time.Now().UnixNano())
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.onBeat != nil && w.onBeat(version) {
		w.onBeat = nil // handled; stop observing
	}
}

// OnBeat registers a callback run with each beat's version until it returns true (then
// cleared), the updater uses it to commit only once the reloaded new build beats.
func (w *Watch) OnBeat(fn func(version string) bool) {
	w.mu.Lock()
	w.onBeat = fn
	w.mu.Unlock()
}

func (w *Watch) tick() bool {
	elapsed := time.Since(time.Unix(0, w.lastBeat.Load()))
	if elapsed <= w.timeout {
		return false
	}
	w.log.Error("kiosk heartbeat lost",
		"elapsed", elapsed.Round(time.Second), "timeout", w.timeout)
	w.onTimeout()
	return true
}

func (w *Watch) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.tick() {
				return
			}
		}
	}
}
