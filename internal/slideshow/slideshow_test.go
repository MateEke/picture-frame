package slideshow_test

import (
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/slideplan"
	"github.com/MateEke/picture-frame/internal/slideshow"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// unknownRatios keeps every image solo (the single-name tests below).
func unknownRatios(string) (float64, bool) { return 0, false }

func newPlannerFor(lib *library.Library, ratioOf func(string) (float64, bool)) *slideplan.Planner {
	src := slideshow.NewLibrarySource(lib)
	return slideplan.NewPlanner(src, ratioOf, slideplan.Threshold{Factor: 1.5}, true)
}

func newSlideshow(lib *library.Library) (*slideshow.Slideshow, *state.Bus) {
	bus := state.NewBus()
	ss := slideshow.New(testutil.NopLogger(), lib, newPlannerFor(lib, unknownRatios), bus, 20*time.Millisecond)
	return ss, bus
}

func receiveNames(t *testing.T, ch <-chan state.Event, timeout time.Duration) []string {
	t.Helper()
	select {
	case e := <-ch:
		return e.Payload.(state.ImagePayload).Names
	case <-time.After(timeout):
		t.Fatal("timeout waiting for image event")
		return nil
	}
}

func receiveImage(t *testing.T, ch <-chan state.Event, timeout time.Duration) string {
	t.Helper()
	names := receiveNames(t, ch, timeout)
	if len(names) == 0 {
		return ""
	}
	return names[0]
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
	ss := slideshow.New(testutil.NopLogger(), lib, newPlannerFor(lib, unknownRatios), bus, 10*time.Second)

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

func TestRunSkipsDeletedImage(t *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}, {Name: "c.jpg"}}, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	go ss.Run(t.Context())

	receiveImage(t, ch, time.Second) // initial "a.jpg"
	lib.Remove("b.jpg")              // delete the next image out from under the cached plan
	ss.Next()
	name := receiveImage(t, ch, time.Second)
	if name != "c.jpg" {
		t.Errorf("expected b.jpg to be skipped server-side, got %s", name)
	}
}

func TestRunSkipsManyDeletedImages(t *testing.T) {
	lib := library.New([]library.Image{
		{Name: "a.jpg"}, {Name: "b.jpg"}, {Name: "c.jpg"}, {Name: "d.jpg"}, {Name: "e.jpg"},
	}, false)
	ss, bus := newSlideshow(lib)

	ch, unsub := bus.Subscribe()
	defer unsub()

	go ss.Run(t.Context())

	receiveImage(t, ch, time.Second) // initial "a.jpg"
	for _, name := range []string{"b.jpg", "c.jpg", "d.jpg", "e.jpg"} {
		lib.Remove(name)
	}
	ss.Next()
	name := receiveImage(t, ch, time.Second)
	if name != "a.jpg" {
		t.Errorf("expected only the surviving a.jpg, got a deleted image: %s", name)
	}
}

func TestPublishesPairedSlide(t *testing.T) {
	lib := library.New([]library.Image{{Name: "p1.jpg"}, {Name: "p2.jpg"}}, false)
	bus := state.NewBus()
	portrait := func(string) (float64, bool) { return 0.66, true }
	planner := newPlannerFor(lib, portrait)
	planner.SetScreenAspect(16.0 / 9.0)
	ss := slideshow.New(testutil.NopLogger(), lib, planner, bus, 20*time.Millisecond)

	ch, unsub := bus.Subscribe()
	defer unsub()

	go ss.Run(t.Context())

	names := receiveNames(t, ch, time.Second)
	if len(names) != 2 || names[0] != "p1.jpg" || names[1] != "p2.jpg" {
		t.Errorf("expected paired slide [p1.jpg p2.jpg], got %v", names)
	}
}

func TestSetSplitConfigDoesNotPanic(_ *testing.T) {
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}}, false)
	ss, _ := newSlideshow(lib)
	ss.SetSplitConfig(false, slideplan.Threshold{Factor: 2.0})
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
