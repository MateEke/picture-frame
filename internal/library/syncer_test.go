package library_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeRemote struct {
	mu       sync.Mutex
	assets   []library.Asset
	bodies   map[string][]byte
	listErr  error
	fetchErr error
}

func (f *fakeRemote) set(assets ...library.Asset) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assets = append([]library.Asset(nil), assets...)
	if f.bodies == nil {
		f.bodies = map[string][]byte{}
	}
	for _, a := range assets {
		if _, ok := f.bodies[a.ID]; !ok {
			f.bodies[a.ID] = []byte("img:" + a.ID)
		}
	}
}

func (f *fakeRemote) List(context.Context) ([]library.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]library.Asset, len(f.assets))
	copy(out, f.assets)
	return out, nil
}

func (f *fakeRemote) Fetch(_ context.Context, id string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	b, ok := f.bodies[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

type fakeAdvancer struct{ n int }

func (a *fakeAdvancer) Next() { a.n++ }

func setup(t *testing.T) (*os.Root, *library.Library) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, library.New(nil, false)
}

func asset(id string, ts int64) library.Asset {
	return library.Asset{ID: id, UpdatedAt: time.Unix(ts, 0)}
}

func names(t *testing.T, root *os.Root) []string {
	t.Helper()
	d, err := root.OpenFile(".", os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	entries, err := d.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func runOnce(t *testing.T, s *library.Syncer) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		// Cancel immediately after the first sync to make Run return.
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	s.Run(ctx)
}

const (
	idA = "00000000-0000-0000-0000-00000000000a"
	idB = "00000000-0000-0000-0000-00000000000b"
)

func TestSyncedFilename(t *testing.T) {
	a := asset(idA, 1700000000)
	got := library.SyncedFilename(a)
	want := idA + "-1700000000.jpg"
	if got != want {
		t.Errorf("SyncedFilename = %q, want %q", got, want)
	}
}

func TestIsSyncedName(t *testing.T) {
	cases := map[string]bool{
		idA + "-1700000000.jpg": true,
		idA + ".jpg":            false,
		"local-upload.jpg":      false,
		"garbage":               false,
	}
	for in, want := range cases {
		if got := library.IsSyncedName(in); got != want {
			t.Errorf("IsSyncedName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSyncDownloadsNewAssets(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1), asset(idB, 2))
	adv := &fakeAdvancer{}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, adv)
	runOnce(t, s)

	got := names(t, root)
	want := []string{idA + "-1.jpg", idB + "-2.jpg"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("files: got %v, want %v", got, want)
	}
	if lib.Len() != 2 {
		t.Errorf("library len = %d, want 2", lib.Len())
	}
	if adv.n != 1 {
		t.Errorf("advancer called %d times, want 1 (empty → non-empty)", adv.n)
	}
}

func TestSyncTriggerRunsOutOfBandSync(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root, lib := setup(t)
		r := &fakeRemote{}
		s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx)

		// Initial sync runs against an empty remote, then Run blocks on the select.
		synctest.Wait()
		if lib.Len() != 0 {
			t.Fatalf("library len = %d, want 0 before trigger", lib.Len())
		}

		// A new asset appears upstream; Trigger forces a sync long before the 1h tick.
		r.set(asset(idA, 1))
		s.Trigger()
		synctest.Wait()

		got := names(t, root)
		if len(got) != 1 || got[0] != idA+"-1.jpg" {
			t.Errorf("after trigger files = %v, want [%s]", got, idA+"-1.jpg")
		}

		cancel()
		synctest.Wait()
	})
}

func TestSyncRunsOnInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root, lib := setup(t)
		r := &fakeRemote{}
		s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Minute, &fakeAdvancer{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx)
		synctest.Wait() // initial (empty) sync

		// A new asset appears; advancing the virtual clock fires the interval tick.
		r.set(asset(idA, 1))
		time.Sleep(time.Minute)
		synctest.Wait()

		if got := names(t, root); len(got) != 1 || got[0] != idA+"-1.jpg" {
			t.Errorf("after interval files = %v, want [%s]", got, idA+"-1.jpg")
		}

		cancel()
		synctest.Wait()
	})
}

func TestSyncTriggerCoalesces(t *testing.T) {
	root, lib := setup(t)
	s := library.NewSyncer(testutil.NopLogger(), &fakeRemote{}, lib, root, time.Hour, &fakeAdvancer{})
	// No Run draining the channel: the second Trigger must hit the non-blocking path.
	s.Trigger()
	s.Trigger()
}

func TestSyncDeletesRemovedAssets(t *testing.T) {
	root, lib := setup(t)
	// Seed local with two files; remote has only one.
	for _, n := range []string{idA + "-1.jpg", idB + "-2.jpg"} {
		if err := os.WriteFile(filepath.Join(root.Name(), n), []byte("seed"), 0o600); err != nil {
			t.Fatal(err)
		}
		lib.Add(n)
	}
	r := &fakeRemote{}
	r.set(asset(idA, 1))
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	got := names(t, root)
	if len(got) != 1 || got[0] != idA+"-1.jpg" {
		t.Errorf("files: got %v, want [%s-1.jpg]", got, idA)
	}
	if lib.Len() != 1 {
		t.Errorf("library len = %d, want 1", lib.Len())
	}
}

func TestSyncReplacesEditedAsset(t *testing.T) {
	root, lib := setup(t)
	old := idA + "-1.jpg"
	if err := os.WriteFile(filepath.Join(root.Name(), old), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	lib.Add(old)
	r := &fakeRemote{}
	r.set(asset(idA, 2))
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	got := names(t, root)
	if len(got) != 1 || got[0] != idA+"-2.jpg" {
		t.Errorf("files: got %v, want [%s-2.jpg]", got, idA)
	}
	if !lib.Has(idA + "-2.jpg") {
		t.Error("library should contain new version")
	}
	if lib.Has(old) {
		t.Error("library should no longer contain old version")
	}
}

func TestSyncKeepsCacheOnListError(t *testing.T) {
	root, lib := setup(t)
	name := idA + "-1.jpg"
	if err := os.WriteFile(filepath.Join(root.Name(), name), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	lib.Add(name)
	r := &fakeRemote{listErr: errors.New("network down")}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	if got := names(t, root); len(got) != 1 {
		t.Errorf("files should be untouched, got %v", got)
	}
	if lib.Len() != 1 {
		t.Errorf("library len = %d, want 1", lib.Len())
	}
}

func TestSyncSkipsFetchErrorButCleansTmp(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1))
	r.fetchErr = errors.New("403")
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	if got := names(t, root); len(got) != 0 {
		t.Errorf("no files should be written on fetch error, got %v", got)
	}
	if lib.Len() != 0 {
		t.Errorf("library len = %d, want 0", lib.Len())
	}
	if st := s.Status(); !strings.Contains(st.LastError, "downloads failed") {
		t.Errorf("partial-failure status: got %q, want substring 'downloads failed'", st.LastError)
	}
}

func TestSyncStatusOKAfterCleanCycle(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1), asset(idB, 2))
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)
	st := s.Status()
	if st.LastError != "" {
		t.Errorf("LastError = %q, want empty", st.LastError)
	}
	if st.AssetCount != 2 {
		t.Errorf("AssetCount = %d, want 2", st.AssetCount)
	}
	if st.LastSync.IsZero() {
		t.Error("LastSync should be set")
	}
}

func TestSyncStatusRedactsLocalPaths(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{listErr: errors.New("read /var/lib/picture-frame/secret: permission denied")}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)
	st := s.Status()
	if strings.Contains(st.LastError, "/var/lib") {
		t.Errorf("LastError leaks filesystem path: %q", st.LastError)
	}
	if !strings.Contains(st.LastError, "<path>") {
		t.Errorf("LastError should mark redaction: %q", st.LastError)
	}
}

func TestSyncCleansLeftoverTmp(t *testing.T) {
	root, lib := setup(t)
	stale := idA + "-1.jpg.tmp"
	if err := os.WriteFile(filepath.Join(root.Name(), stale), []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &fakeRemote{}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	for _, n := range names(t, root) {
		if n == stale {
			t.Errorf("leftover %s should have been cleaned", stale)
		}
	}
}

func TestSyncIgnoresUnrelatedFilenames(t *testing.T) {
	root, lib := setup(t)
	for _, n := range []string{"local-upload.jpg", "random.txt"} {
		if err := os.WriteFile(filepath.Join(root.Name(), n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := &fakeRemote{}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	got := names(t, root)
	if len(got) != 2 {
		t.Errorf("non-synced files should be left alone, got %v", got)
	}
}

func TestSyncDoesNotAdvanceWhenLibraryWasNonEmpty(t *testing.T) {
	root, lib := setup(t)
	seed := idA + "-1.jpg"
	if err := os.WriteFile(filepath.Join(root.Name(), seed), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	lib.Add(seed)
	r := &fakeRemote{}
	r.set(asset(idA, 1), asset(idB, 2))
	adv := &fakeAdvancer{}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, adv)
	runOnce(t, s)

	if adv.n != 0 {
		t.Errorf("advancer should NOT fire when library was already non-empty, got %d calls", adv.n)
	}
}

func TestSyncRejectsOversizedAsset(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1))
	r.bodies[idA] = make([]byte, 50<<20+1)
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)
	if lib.Len() != 0 {
		t.Error("oversized asset must not enter the library")
	}
	if st := s.Status(); st.LastError != "downloads failed: 1" {
		t.Errorf("LastError: got %q, want downloads failed: 1", st.LastError)
	}
}

// Exactly at the cap the asset must download whole: a truncated copy would
// corrupt the image.
func TestSyncAcceptsAssetAtSizeCap(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1))
	r.bodies[idA] = make([]byte, 50<<20)
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)
	if lib.Len() != 1 {
		t.Fatal("exact-cap asset must sync")
	}
	fi, err := root.Stat(library.SyncedFilename(asset(idA, 1)))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Size() != 50<<20 {
		t.Errorf("size: got %d, want %d (truncated?)", fi.Size(), 50<<20)
	}
}

func TestSyncNoAdvanceWhenNothingDownloaded(t *testing.T) {
	root, lib := setup(t)
	adv := &fakeAdvancer{}
	s := library.NewSyncer(testutil.NopLogger(), &fakeRemote{}, lib, root, time.Hour, adv)
	runOnce(t, s)
	if adv.n != 0 {
		t.Errorf("advancer called %d times with nothing downloaded, want 0", adv.n)
	}
}

func TestSyncCleansAllLeftoverTmpFiles(t *testing.T) {
	root, lib := setup(t)
	for _, n := range []string{"a.tmp", "b.tmp"} {
		f, err := root.OpenFile(n, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	s := library.NewSyncer(testutil.NopLogger(), &fakeRemote{}, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)
	if got := names(t, root); len(got) != 0 {
		t.Errorf("leftover tmp files survived the sweep: %v", got)
	}
}
