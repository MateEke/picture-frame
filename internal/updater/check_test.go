package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeSource struct {
	releases []Release
	err      error
}

func (f *fakeSource) List(context.Context) ([]Release, error) { return f.releases, f.err }

func newTestUpdater(src ReleaseSource, auto bool) *Updater {
	return New(Options{
		Log: testutil.NopLogger(), Source: src,
		Current: "v1.2.0", Platform: "linux_armv6",
		AutoUpdate: auto, UpdateHour: 2,
	})
}

func TestCheckPopulatesAvailable(t *testing.T) {
	u := newTestUpdater(&fakeSource{releases: []Release{
		{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
	}}, false)

	u.check(context.Background())

	s := u.Status()
	if !s.Available || s.Latest != "v1.3.1" || !s.LastCheckOK {
		t.Fatalf("status after check: %+v", s)
	}
	if s.LastCheck.IsZero() || s.Phase != PhaseIdle {
		t.Errorf("expected LastCheck set + phase idle: %+v", s)
	}
}

func TestCheckOfflineIsGraceful(t *testing.T) {
	u := newTestUpdater(&fakeSource{err: errors.New("dial tcp: no route to host")}, false)

	u.check(context.Background()) // must not panic or propagate

	s := u.Status()
	if s.LastCheckOK {
		t.Error("offline check should set LastCheckOK=false")
	}
	if s.Available {
		t.Error("offline check should not report an update available")
	}
	if s.LastCheck.IsZero() {
		t.Error("offline check should still record LastCheck")
	}
}

func TestNextRunAfter(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		name     string
		now      time.Time
		hour     int
		wantHour int
		wantDay  int
	}{
		{"before hour today", time.Date(2026, 6, 11, 1, 0, 0, 0, loc), 2, 2, 11},
		{"after hour → tomorrow", time.Date(2026, 6, 11, 14, 0, 0, 0, loc), 2, 2, 12},
		{"exactly at hour → tomorrow", time.Date(2026, 6, 11, 2, 0, 0, 0, loc), 2, 2, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextRunAfter(tc.now, tc.hour)
			if got.Hour() != tc.wantHour || got.Day() != tc.wantDay {
				t.Errorf("got %v, want hour %d day %d", got, tc.wantHour, tc.wantDay)
			}
		})
	}
}

// settableSource lets a test swap the release list between synctest steps.
type settableSource struct {
	mu       sync.Mutex
	releases []Release
}

func (s *settableSource) List(context.Context) ([]Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.releases, nil
}

func (s *settableSource) set(r []Release) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases = r
}

func TestRunCheckNowRechecks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &settableSource{} // no releases yet
		u := newTestUpdater(src, false)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait() // initial check: nothing available

		if u.Status().Available {
			t.Fatal("no releases yet: should not be available")
		}
		src.set([]Release{{Version: "v1.3.1", Assets: assetsFor("1.3.1")}})
		u.Check()
		synctest.Wait()

		if s := u.Status(); !s.Available || s.Latest != "v1.3.1" {
			t.Errorf("Check should force a re-check: %+v", s)
		}
	})
}

func TestRunSeedsRollbackResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, rolledBackMarker), []byte("v1.3.1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		u := New(Options{
			Log: testutil.NopLogger(), Source: &fakeSource{},
			Current: "v1.2.0", Platform: "linux_armv6", InstallDir: dir,
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait()

		if got := u.Status().LastResult; got != "rolled back from v1.3.1" {
			t.Errorf("Run should surface the rollback marker once: %q", got)
		}
		if got := skipAutoVersions(dir); !slices.Equal(got, []string{"v1.3.1"}) {
			t.Errorf("rolled-back version should be recorded for auto-skip: %v", got)
		}
	})
}

func TestRunAutoSkipsRolledBackVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		// A prior rollback already recorded v1.3.1 as broken.
		if err := os.WriteFile(filepath.Join(dir, skipAutoFile), []byte("v1.3.1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		u := New(Options{
			Log: testutil.NopLogger(),
			Source: &fakeSource{releases: []Release{
				{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
			}},
			Current: "v1.2.0", Platform: "linux_armv6", InstallDir: dir,
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait()

		if s := u.Status(); !s.Available || s.Latest != "v1.3.1" {
			t.Errorf("rolled-back version should still be offered for manual retry: %+v", s)
		}
		u.mu.Lock()
		auto := u.autoTgt
		u.mu.Unlock()
		if auto != nil {
			t.Errorf("auto-apply must skip the rolled-back version, got %+v", auto)
		}
	})
}

// flakySource fails its first failsLeft List calls (network not ready after boot), then succeeds.
type flakySource struct {
	mu        sync.Mutex
	failsLeft int
	releases  []Release
}

func (s *flakySource) List(context.Context) ([]Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failsLeft > 0 {
		s.failsLeft--
		return nil, errors.New("net/http: TLS handshake timeout")
	}
	return s.releases, nil
}

func TestNextBackoff(t *testing.T) {
	cases := []struct{ prev, want time.Duration }{
		{0, 30 * time.Second},
		{30 * time.Second, time.Minute},
		{time.Minute, 2 * time.Minute},
		{20 * time.Minute, 30 * time.Minute}, // 40m capped to the 30m ceiling
		{30 * time.Minute, 30 * time.Minute}, // stays at the ceiling
	}
	for _, tc := range cases {
		if got := nextBackoff(tc.prev); got != tc.want {
			t.Errorf("nextBackoff(%v) = %v, want %v", tc.prev, got, tc.want)
		}
	}
}

func TestRunRetriesAfterFailedCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &flakySource{failsLeft: 1, releases: []Release{
			{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
		}}
		u := newTestUpdater(src, false)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait() // initial check fails (network not ready)

		if s := u.Status(); s.LastCheckOK || s.Available {
			t.Fatalf("initial check should have failed offline: %+v", s)
		}

		time.Sleep(30 * time.Second) // backoff retry fires, well before the daily hour
		synctest.Wait()

		if s := u.Status(); !s.LastCheckOK || !s.Available || s.Latest != "v1.3.1" {
			t.Errorf("backoff retry should recover after a transient failure: %+v", s)
		}
	})
}

func TestRunDoesInitialCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		u := newTestUpdater(&fakeSource{releases: []Release{
			{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
		}}, false)
		ctx, cancel := context.WithCancel(context.Background())
		go u.Run(ctx)
		synctest.Wait() // let the initial check complete

		if !u.Status().Available {
			t.Error("Run should perform an initial check")
		}
		cancel()
	})
}

// dropAfterFirstSource succeeds once (the boot check) then fails: the frame
// went offline after a healthy boot.
type dropAfterFirstSource struct {
	mu       sync.Mutex
	calls    int
	releases []Release
}

func (s *dropAfterFirstSource) List(context.Context) ([]Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls > 1 {
		return nil, errors.New("dial tcp: no route to host")
	}
	return s.releases, nil
}

// countingDownloader fails loudly: the tests using it assert no apply starts.
type countingDownloader struct{ calls int }

func (d *countingDownloader) Download(context.Context, string, string) error {
	d.calls++
	return errors.New("stub download failure")
}

func TestRunSkipsAutoApplyWhenNightlyCheckFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &dropAfterFirstSource{releases: []Release{
			{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
		}}
		dl := &countingDownloader{}
		u := New(Options{
			Log: testutil.NopLogger(), Source: src,
			Current: "v1.2.0", Platform: "linux_armv6",
			AutoUpdate: true, UpdateHour: 2,
			Downloader: dl, Preflight: fakePreflight{},
			InstallDir: t.TempDir(),
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait() // boot check succeeds; the auto target is now populated

		// Cross the nightly hour; that check fails, so the stale target from the
		// boot check must not be applied (and must not pollute LastResult).
		time.Sleep(3 * time.Hour)
		synctest.Wait()

		if dl.calls != 0 {
			t.Errorf("auto-apply ran despite a failed nightly check (downloads: %d)", dl.calls)
		}
		if s := u.Status(); s.LastResult != "" {
			t.Errorf("LastResult polluted by a doomed apply: %q", s.LastResult)
		}
	})
}

func TestRunAutoAppliesOnSchedule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dl := &countingDownloader{}
		u := New(Options{
			Log: testutil.NopLogger(),
			Source: &fakeSource{releases: []Release{
				{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
			}},
			Current: "v1.2.0", Platform: "linux_armv6",
			AutoUpdate: true, UpdateHour: 2,
			Downloader: dl, Preflight: fakePreflight{},
			InstallDir: t.TempDir(),
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait() // boot check succeeds

		time.Sleep(3 * time.Hour) // cross the nightly hour
		synctest.Wait()

		if dl.calls == 0 {
			t.Error("scheduled auto-apply never started")
		}
	})
}
