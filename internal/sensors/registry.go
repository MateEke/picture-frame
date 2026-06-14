package sensors

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// initialBackoff is the starting delay before restarting a source that errored;
// it doubles on each consecutive failure up to maxBackoff.
const initialBackoff = time.Second

// Registry supervises a set of Sources, restarting them on error with
// exponential backoff, and fans all readings into a single emit callback.
type Registry struct {
	log     *slog.Logger
	sources []Source
	emit    func(Reading)
}

func NewRegistry(log *slog.Logger, sources []Source, emit func(Reading)) *Registry {
	return &Registry{
		log:     log,
		sources: sources,
		emit:    emit,
	}
}

// Run starts all sources and blocks until ctx is cancelled.
func (r *Registry) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, src := range r.sources {
		wg.Go(func() {
			r.runSource(ctx, src)
		})
	}
	wg.Wait()
}

func (r *Registry) runSource(ctx context.Context, s Source) {
	out := make(chan Reading, 8)
	var fanout sync.WaitGroup
	fanout.Go(func() {
		for reading := range out {
			r.emit(reading)
		}
	})
	// Close out and let the drain finish before returning, so no readings are
	// still in flight when the source's WaitGroup entry completes.
	defer func() {
		close(out)
		fanout.Wait()
	}()

	backoff := initialBackoff
	const maxBackoff = 60 * time.Second

	for {
		r.log.Info("starting sensor source", "id", s.ID())
		start := time.Now()
		err := s.Start(ctx, out)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			r.log.Info("sensor source stopped", "id", s.ID())
			return
		}
		// A source that ran long enough was healthy; retry quickly rather than
		// carrying over backoff from earlier failures.
		if time.Since(start) >= maxBackoff {
			backoff = initialBackoff
		}
		r.log.Error("sensor source error, retrying", "id", s.ID(), "err", err, "backoff", backoff)
		select {
		case <-time.After(backoff):
			if backoff < maxBackoff {
				backoff *= 2
			}
		case <-ctx.Done():
			return
		}
	}
}
