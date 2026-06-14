package slideshow_test

import (
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/slideshow"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func newSlideshow(lib *library.Library) (*slideshow.Slideshow, *state.Bus) {
	bus := state.NewBus()
	ss := slideshow.New(testutil.NopLogger(), lib, bus, 20*time.Millisecond)
	return ss, bus
}

func receiveImage(t *testing.T, ch <-chan state.Event, timeout time.Duration) string {
	t.Helper()
	select {
	case e := <-ch:
		return e.Payload.(state.ImagePayload).Name
	case <-time.After(timeout):
		t.Fatal("timeout waiting for image event")
		return ""
	}
}

func TestRunPublishesCurrentImmediately(t *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}}, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	name := receiveImage(t, ch, time.Second)
	if name != "a.jpg" {
		t.Errorf("expected a.jpg on startup, got %s", name)
	}
}

func TestRunDoesNotPublishWhenEmpty(t *testing.T) {
	lib := library.New(nil, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	select {
	case e := <-ch:
		t.Fatalf("expected no event for empty library, got %v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRunAdvancesOnTick(t *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}}, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	receiveImage(t, ch, time.Second)         // initial "a.jpg"
	name := receiveImage(t, ch, time.Second) // first tick → "b.jpg"
	if name != "b.jpg" {
		t.Errorf("expected b.jpg after tick, got %s", name)
	}
}

func TestNextAdvancesImmediately(t *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}}, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	receiveImage(t, ch, time.Second) // initial "a.jpg"
	ss.Next()
	name := receiveImage(t, ch, time.Second)
	if name != "b.jpg" {
		t.Errorf("expected b.jpg after Next(), got %s", name)
	}
}

func TestNextDoesNotPublishWhenEmpty(t *testing.T) {
	lib := library.New(nil, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	ss.Next()
	select {
	case e := <-ch:
		t.Fatalf("expected no event for empty library, got %v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSetIntervalTriggersImmediateAdvance(t *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}, {Name: "c.jpg"}}, false)
	// Start with a very long interval so no tick fires naturally during the test.
	bus := state.NewBus()
	ss := slideshow.New(testutil.NopLogger(), lib, bus, 10*time.Second)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	receiveImage(t, ch, time.Second) // initial "a.jpg"

	// SetInterval sends on the advance channel, causing immediate advance + timer reset.
	ss.SetInterval(20 * time.Millisecond)

	receiveImage(t, ch, time.Second) // advance fires → "b.jpg"
	// Ticker is now 20ms; next tick should arrive quickly.
	receiveImage(t, ch, time.Second) // tick → "c.jpg"
}

func TestSetRandomizeDoesNotPanic(_ *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}}, false)
	ss, _ := newSlideshow(lib)
	ss.SetRandomize(true)
	ss.SetRandomize(false)
}

// Library starts empty; an image is added later. The slideshow should resume
// ticking only after Next() is signaled, and publish the new image immediately.
func TestRunResumesAfterAddToEmptyLibrary(t *testing.T) {
	lib := library.New(nil, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctx := t.Context()
	go ss.Run(ctx)

	// Verify no events while empty even past the interval.
	select {
	case e := <-ch:
		t.Fatalf("did not expect events on empty library, got %v", e)
	case <-time.After(50 * time.Millisecond):
	}

	lib.Add("a.jpg")
	ss.Next()
	name := receiveImage(t, ch, time.Second)
	if name != "a.jpg" {
		t.Errorf("expected a.jpg after add+Next, got %s", name)
	}
}
