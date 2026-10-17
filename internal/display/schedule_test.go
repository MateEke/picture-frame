package display_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeSwitcher struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeSwitcher) On(context.Context) error  { return f.record("on") }
func (f *fakeSwitcher) Off(context.Context) error { return f.record("off") }

func (f *fakeSwitcher) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, call)
	return nil
}

func (f *fakeSwitcher) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeSwitcher) got() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

type fakeScheduleStore struct {
	mu      sync.Mutex
	last    time.Time
	loadErr error
	saveErr error
	saved   []time.Time
}

func (f *fakeScheduleStore) LoadLastCheck() (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last, f.loadErr
}

func (f *fakeScheduleStore) SaveLastCheck(t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.last = t
	f.saved = append(f.saved, t)
	return nil
}

func (f *fakeScheduleStore) savedTimes() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.saved)
}

// The synctest bubble's clock starts at 2000-01-01 00:00 UTC.
func newScheduler(t *testing.T, sw display.Switcher, store *fakeScheduleStore, sched config.ScheduleConfig) *display.Scheduler {
	t.Helper()
	cfg := display.SchedulerConfig{Log: testutil.NopLogger(), Screen: sw, Schedule: sched, Location: time.UTC}
	if store != nil {
		cfg.Store = store
	}
	return display.NewScheduler(cfg)
}

func runScheduler(t *testing.T, s *display.Scheduler) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s.Start(ctx)
	go s.Run(ctx)
}

func advance(d time.Duration) {
	time.Sleep(d)
	synctest.Wait()
}

func wantCalls(t *testing.T, sw *fakeSwitcher, want ...string) {
	t.Helper()
	if got := sw.got(); !slices.Equal(got, want) {
		t.Fatalf("switches = %v, want %v", got, want)
	}
}

func TestSchedulerSwitchesOffThenOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		runScheduler(t, newScheduler(t, sw, nil, config.ScheduleConfig{OffAt: "00:01", OnAt: "00:03"}))

		advance(50 * time.Second)
		wantCalls(t, sw)
		advance(20 * time.Second)
		wantCalls(t, sw, "off")
		advance(2 * time.Minute)
		wantCalls(t, sw, "off", "on")
		advance(time.Hour)
		wantCalls(t, sw, "off", "on")
	})
}

func TestSchedulerRepeatsDaily(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		runScheduler(t, newScheduler(t, sw, nil, config.ScheduleConfig{OffAt: "12:00"}))

		advance(48 * time.Hour)
		wantCalls(t, sw, "off", "off")
	})
}

func TestSchedulerOnOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		runScheduler(t, newScheduler(t, sw, nil, config.ScheduleConfig{OnAt: "07:00"}))

		advance(8 * time.Hour)
		wantCalls(t, sw, "on")
	})
}

func TestSchedulerEmptyDoesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		runScheduler(t, newScheduler(t, sw, nil, config.ScheduleConfig{}))

		advance(48 * time.Hour)
		wantCalls(t, sw)
	})
}

func TestSchedulerRetriesFailedSwitch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{err: errors.New("compositor gone")}
		store := &fakeScheduleStore{}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "00:01", OnAt: "08:00"}))

		advance(2 * time.Minute)
		wantCalls(t, sw)
		if len(store.savedTimes()) != 0 {
			t.Fatalf("a failed switch must not be recorded")
		}

		sw.setErr(nil)
		advance(10 * time.Second)
		wantCalls(t, sw, "off")
		if len(store.savedTimes()) != 1 {
			t.Fatalf("saved = %v, want one save", store.savedTimes())
		}
	})
}

func TestSchedulerLoneTimeRetryGivesUpAfterGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{err: errors.New("compositor gone")}
		runScheduler(t, newScheduler(t, sw, nil, config.ScheduleConfig{OffAt: "00:01"}))

		advance(10 * time.Minute)
		sw.setErr(nil)
		advance(time.Minute)
		wantCalls(t, sw)
	})
}

func TestSchedulerFullWindowRetryGivesUpAfterGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{err: errors.New("compositor gone")}
		store := &fakeScheduleStore{}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "00:01", OnAt: "00:30"}))

		advance(10 * time.Minute)
		sw.setErr(nil)
		advance(time.Minute)
		wantCalls(t, sw)
		if len(store.savedTimes()) != 1 {
			t.Fatalf("saved = %v, want the give-up recorded", store.savedTimes())
		}

		advance(20 * time.Minute)
		wantCalls(t, sw, "on")
	})
}

func TestSchedulerStartCatchesUpFullWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		// Down since 14:00 yesterday: the 23:00 off was missed.
		store := &fakeScheduleStore{last: time.Now().Add(-10 * time.Hour)}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"}))

		synctest.Wait()
		wantCalls(t, sw, "off")
		if saved := store.savedTimes(); len(saved) != 1 || !saved[0].Equal(time.Now()) {
			t.Fatalf("saved = %v, want [now]", saved)
		}
	})
}

func TestSchedulerStartCatchesUpLatestBoundaryOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		store := &fakeScheduleStore{last: time.Now().Add(-7 * 24 * time.Hour)}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"}))

		synctest.Wait()
		wantCalls(t, sw, "off")
	})
}

func TestSchedulerStartSkipsMissedLoneTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		store := &fakeScheduleStore{last: time.Now().Add(-10 * time.Hour)}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "23:00"}))

		synctest.Wait()
		wantCalls(t, sw)
	})
}

func TestSchedulerStartKeepsAppliedState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		// The 23:00 off already ran.
		store := &fakeScheduleStore{last: time.Now().Add(-30 * time.Minute)}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"}))

		synctest.Wait()
		wantCalls(t, sw)
	})
}

func TestSchedulerStartWithoutSavedState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		runScheduler(t, newScheduler(t, sw, &fakeScheduleStore{}, config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"}))

		synctest.Wait()
		wantCalls(t, sw)
	})
}

func TestSchedulerStartLoadError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		store := &fakeScheduleStore{last: time.Now().Add(-10 * time.Hour), loadErr: errors.New("io")}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"}))

		synctest.Wait()
		wantCalls(t, sw)
		advance(time.Minute)
		wantCalls(t, sw)
	})
}

func TestSchedulerSaveErrorStillSwitches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		store := &fakeScheduleStore{saveErr: errors.New("read-only")}
		runScheduler(t, newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "00:01"}))

		advance(2 * time.Minute)
		wantCalls(t, sw, "off")
	})
}

func TestSchedulerSetWaitsForNextBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		store := &fakeScheduleStore{}
		s := newScheduler(t, sw, store, config.ScheduleConfig{OffAt: "00:05"})
		runScheduler(t, s)

		advance(3 * time.Minute)
		if !s.Set(config.ScheduleConfig{OffAt: "00:02", OnAt: "00:04"}, time.UTC) {
			t.Fatal("Set reported no change")
		}
		if saved := store.savedTimes(); len(saved) != 1 || !saved[0].Equal(time.Now()) {
			t.Fatalf("saved = %v, want [now]", saved)
		}
		advance(30 * time.Second)
		wantCalls(t, sw)
		advance(time.Minute)
		wantCalls(t, sw, "on")
		advance(2 * time.Minute)
		wantCalls(t, sw, "on")
	})
}

func TestSchedulerSetUnchangedIsNoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		store := &fakeScheduleStore{}
		sched := config.ScheduleConfig{OffAt: "00:01"}
		s := newScheduler(t, sw, store, sched)
		// Offset the checks so the save lands after the boundary but before a check.
		time.Sleep(5 * time.Second)
		runScheduler(t, s)

		advance(57 * time.Second)
		if s.Set(sched, time.UTC) {
			t.Fatal("Set reported a change for identical settings")
		}
		advance(5 * time.Second)
		wantCalls(t, sw, "off")
		if len(store.savedTimes()) != 1 {
			t.Fatalf("saved = %v, want only the switch", store.savedTimes())
		}
	})
}

func TestSchedulerSetLocation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sw := &fakeSwitcher{}
		s := newScheduler(t, sw, nil, config.ScheduleConfig{})
		runScheduler(t, s)

		// 01:00 in UTC+1 is 00:00 UTC, already past; 02:00 there is 01:00 UTC.
		plusOne := time.FixedZone("UTC+1", 3600)
		s.Set(config.ScheduleConfig{OffAt: "02:00"}, plusOne)
		advance(59 * time.Minute)
		wantCalls(t, sw)
		advance(time.Minute)
		wantCalls(t, sw, "off")
	})
}

func TestSchedulerRunStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newScheduler(t, &fakeSwitcher{}, nil, config.ScheduleConfig{OffAt: "00:01"})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			s.Run(ctx)
			close(done)
		}()
		cancel()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("Run did not return after cancel")
		}
	})
}

func TestSchedulerNext(t *testing.T) {
	today := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		sched  config.ScheduleConfig
		want   display.NextChange
		wantOK bool
	}{
		{"none", config.ScheduleConfig{}, display.NextChange{}, false},
		{
			"on comes first", config.ScheduleConfig{OffAt: "23:00", OnAt: "07:00"},
			display.NextChange{At: today.Add(7 * time.Hour), On: true, Clock: config.Clock{Hour: 7}}, true,
		},
		{
			"now is not next", config.ScheduleConfig{OffAt: "00:00"},
			display.NextChange{At: today.AddDate(0, 0, 1)}, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				got, ok := newScheduler(t, &fakeSwitcher{}, nil, tc.sched).Next()
				if ok != tc.wantOK || !got.At.Equal(tc.want.At) || got.On != tc.want.On || got.Clock != tc.want.Clock {
					t.Errorf("Next() = (%+v, %v), want (%+v, %v)", got, ok, tc.want, tc.wantOK)
				}
			})
		})
	}
}
