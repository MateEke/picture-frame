package state_test

import (
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
)

func recv(t *testing.T, ch <-chan state.Event) state.Event {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
		return state.Event{}
	}
}

func TestPublishSubscribe(t *testing.T) {
	b := state.NewBus()
	ch, unsub := b.Subscribe()
	defer unsub()

	b.Publish(state.Event{Kind: state.KindScreen})

	e := recv(t, ch)
	if e.Kind != state.KindScreen {
		t.Fatalf("got kind %q, want %q", e.Kind, state.KindScreen)
	}
	if e.ID != 1 {
		t.Fatalf("got ID %d, want 1", e.ID)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := state.NewBus()
	ch1, unsub1 := b.Subscribe()
	defer unsub1()
	ch2, unsub2 := b.Subscribe()
	defer unsub2()

	b.Publish(state.Event{Kind: state.KindImage})

	for _, ch := range []<-chan state.Event{ch1, ch2} {
		e := recv(t, ch)
		if e.Kind != state.KindImage {
			t.Fatalf("got %q, want %q", e.Kind, state.KindImage)
		}
	}
}

func TestMonotonicIDs(t *testing.T) {
	b := state.NewBus()
	ch, unsub := b.Subscribe()
	defer unsub()

	b.Publish(state.Event{Kind: state.KindSensor})
	b.Publish(state.Event{Kind: state.KindSensor})

	e1 := recv(t, ch)
	e2 := recv(t, ch)
	if e2.ID <= e1.ID {
		t.Fatalf("IDs not monotonic: %d then %d", e1.ID, e2.ID)
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := state.NewBus()
	ch, unsub := b.Subscribe()
	unsub()

	_, ok := <-ch
	if ok {
		t.Fatal("expected channel closed after unsub")
	}

	// Publish after unsub must not panic.
	b.Publish(state.Event{Kind: state.KindScreen})
}

func TestSnapshotEmpty(t *testing.T) {
	b := state.NewBus()
	if snap := b.Snapshot(); len(snap) != 0 {
		t.Fatalf("expected empty snapshot before any publish, got %d events", len(snap))
	}
}

func TestSnapshotRetainsLastPerKind(t *testing.T) {
	b := state.NewBus()
	b.Publish(state.Event{Kind: state.KindImage, Payload: state.ImagePayload{Names: []string{"a.jpg"}}})
	b.Publish(state.Event{Kind: state.KindImage, Payload: state.ImagePayload{Names: []string{"b.jpg"}}})
	b.Publish(state.Event{Kind: state.KindWeather, Payload: state.WeatherPayload{Temp: 20}})

	snap := b.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 kinds in snapshot, got %d", len(snap))
	}
	byKind := make(map[state.Kind]state.Event, len(snap))
	for _, e := range snap {
		byKind[e.Kind] = e
	}
	img, ok := byKind[state.KindImage]
	if !ok {
		t.Fatal("expected image event in snapshot")
	}
	if img.Payload.(state.ImagePayload).Names[0] != "b.jpg" {
		t.Errorf("expected last image b.jpg, got %v", img.Payload)
	}
}

func TestSnapshotOrderedByPublishID(t *testing.T) {
	b := state.NewBus()
	b.Publish(state.Event{Kind: state.KindWeather, Payload: state.WeatherPayload{Temp: 20}})
	b.Publish(state.Event{Kind: state.KindImage, Payload: state.ImagePayload{Names: []string{"a.jpg"}}})
	b.Publish(state.Event{Kind: state.KindScreen, Payload: state.ScreenPayload{On: true}})

	snap := b.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 events, got %d", len(snap))
	}
	for i := 1; i < len(snap); i++ {
		if snap[i-1].ID >= snap[i].ID {
			t.Errorf("snapshot not in ascending ID order: %v", snap)
		}
	}
}

func TestSnapshotDoesNotBlockNewSubscribers(t *testing.T) {
	b := state.NewBus()
	b.Publish(state.Event{Kind: state.KindImage, Payload: state.ImagePayload{Names: []string{"x.jpg"}}})

	// Subscriber created after publish should not receive past events via channel,
	// but Snapshot should return the last image.
	ch, unsub := b.Subscribe()
	defer unsub()

	select {
	case <-ch:
		t.Fatal("channel should not have buffered past events")
	default:
	}

	snap := b.Snapshot()
	if len(snap) != 1 || snap[0].Kind != state.KindImage {
		t.Fatalf("unexpected snapshot: %v", snap)
	}
}

func TestSlowSubscriberDropped(t *testing.T) {
	b := state.NewBus()
	ch, unsub := b.Subscribe() // buffer = 16
	defer unsub()

	payload := state.SensorPayload{DeviceID: "dev", Kind: "temperature", Value: 21.0}
	for range 20 {
		b.Publish(state.Event{Kind: state.KindSensor, Payload: payload})
	}

	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			if count == 0 {
				t.Fatal("expected at least one event")
			}
			if count > 16 {
				t.Fatalf("received %d events, buffer is 16", count)
			}
			return
		}
	}
}
