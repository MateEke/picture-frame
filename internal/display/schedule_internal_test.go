package display

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type countingSwitcher struct{ calls int }

func (c *countingSwitcher) On(context.Context) error  { c.calls++; return nil }
func (c *countingSwitcher) Off(context.Context) error { c.calls++; return nil }

func TestLatestEdge(t *testing.T) {
	budapest, err := time.LoadLocation("Europe/Budapest")
	if err != nil {
		t.Fatal(err)
	}
	at := func(y int, m time.Month, d, h, mi int) time.Time { return time.Date(y, m, d, h, mi, 0, 0, budapest) }
	night := parseEdges(config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"})
	cases := []struct {
		name     string
		edges    []edge
		from, to time.Time
		wantOn   bool
		wantOK   bool
	}{
		{"none in range", night, at(2026, 10, 17, 8, 0), at(2026, 10, 17, 22, 59), false, false},
		{"lower bound exclusive", night, at(2026, 10, 17, 23, 0), at(2026, 10, 17, 23, 30), false, false},
		{"upper bound inclusive", night, at(2026, 10, 17, 22, 0), at(2026, 10, 17, 23, 0), false, true},
		{"across midnight", night, at(2026, 10, 17, 22, 0), at(2026, 10, 18, 0, 30), false, true},
		{"latest of several", night, at(2026, 10, 17, 22, 0), at(2026, 10, 18, 8, 0), true, true},
		{"empty schedule", nil, at(2026, 10, 17, 0, 0), at(2026, 10, 18, 0, 0), false, false},
		// 02:30 doesn't exist on the spring-forward day; Go normalizes it to 03:30.
		{
			"spring-forward gap", parseEdges(config.ScheduleConfig{OffAt: "02:30"}),
			at(2026, 3, 29, 1, 0), at(2026, 3, 29, 4, 0), false, true,
		},
		// 02:30 happens twice on the fall-back day.
		{
			"fall-back overlap", parseEdges(config.ScheduleConfig{OffAt: "02:30"}),
			at(2026, 10, 25, 1, 0), at(2026, 10, 25, 4, 0), false, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, ok := latestEdge(tc.edges, budapest, tc.from, tc.to)
			if ok != tc.wantOK || (ok && got.on != tc.wantOn) {
				t.Errorf("got (%+v, %v), want (on=%v, %v)", got, ok, tc.wantOn, tc.wantOK)
			}
		})
	}
}

func TestSchedulerClockGoesBack(t *testing.T) {
	sw := &countingSwitcher{}
	s := NewScheduler(SchedulerConfig{
		Log: testutil.NopLogger(), Screen: sw, Location: time.UTC,
		Schedule: config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"},
	})
	now := time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC)
	s.lastCheck = now

	earlier := now.Add(-12 * time.Hour)
	s.checkLocked(context.Background(), earlier)
	if sw.calls != 0 || !s.lastCheck.Equal(earlier) {
		t.Fatalf("calls=%d lastCheck=%v, want no switch and lastCheck reset", sw.calls, s.lastCheck)
	}
	s.checkLocked(context.Background(), now)
	if sw.calls != 1 {
		t.Fatalf("calls=%d, want 1", sw.calls)
	}
}

func TestSchedulerClockJumpsForwardOverLoneTime(t *testing.T) {
	sw := &countingSwitcher{}
	s := NewScheduler(SchedulerConfig{
		Log: testutil.NopLogger(), Screen: sw, Location: time.UTC,
		Schedule: config.ScheduleConfig{OffAt: "23:00"},
	})
	s.lastCheck = time.Date(2026, 10, 17, 22, 0, 0, 0, time.UTC)
	s.checkLocked(context.Background(), time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC))
	if sw.calls != 0 {
		t.Fatalf("calls=%d, want 0", sw.calls)
	}
}

func TestBoundaries(t *testing.T) {
	edges := parseEdges(config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"})
	from := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	var got []time.Time
	for at := range boundaries(edges, time.UTC, from, from.Add(24*time.Hour)) {
		got = append(got, at)
	}
	want := []time.Time{
		time.Date(2026, 10, 17, 23, 0, 0, 0, time.UTC), time.Date(2026, 10, 17, 7, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 18, 23, 0, 0, 0, time.UTC), time.Date(2026, 10, 18, 7, 0, 0, 0, time.UTC),
	}
	if !slices.EqualFunc(got, want, time.Time.Equal) {
		t.Fatalf("got %v, want %v", got, want)
	}

	n := 0
	for range boundaries(edges, time.UTC, from, from.Add(24*time.Hour)) {
		n++
		break
	}
	if n != 1 {
		t.Fatalf("yielded %d after break, want 1", n)
	}
}
