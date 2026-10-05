package library_test

import (
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func loadManifestFile(t *testing.T, root *os.Root) map[string]map[string]any {
	t.Helper()
	f, err := root.Open(".manifest.json")
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return out
}

func TestManifestRoundTrip(t *testing.T) {
	root, _ := setup(t)
	m := library.LoadManifest(testutil.NopLogger(), root)
	m.Set("a.jpg", library.ManifestEntry{ID: "id-a", Version: "v1", Bytes: 100})
	m.Set("b.jpg", library.ManifestEntry{ID: "id-b", Version: "v2", Bytes: 200})
	if err := m.Flush(); err != nil {
		t.Fatal(err)
	}

	reloaded := library.LoadManifest(testutil.NopLogger(), root)
	if !reloaded.Has("a.jpg") || !reloaded.Has("b.jpg") {
		t.Error("reloaded manifest lost entries")
	}
	if got := reloaded.TotalBytes(); got != 300 {
		t.Errorf("TotalBytes = %d, want 300", got)
	}
	if miss := reloaded.Missing([]library.Asset{{ID: "id-a", Version: "v1"}, {ID: "id-b", Version: "v2"}}); len(miss) != 0 {
		t.Errorf("Missing = %v, want none", miss)
	}
}

func TestManifestLoadCorruptStartsEmpty(t *testing.T) {
	root, _ := setup(t)
	f, err := root.OpenFile(".manifest.json", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{not json")
	_ = f.Close()

	m := library.LoadManifest(testutil.NopLogger(), root)
	if got := m.TotalBytes(); got != 0 {
		t.Errorf("corrupt manifest TotalBytes = %d, want 0", got)
	}
}

func TestManifestMissingDetectsVersionChange(t *testing.T) {
	m := library.NewMemoryManifest(testutil.NopLogger())
	m.Set("old.jpg", library.ManifestEntry{ID: "id-a", Version: "v1", Bytes: 10})

	miss := m.Missing([]library.Asset{{ID: "id-a", Version: "v2"}, {ID: "id-b", Version: "v1"}})
	if len(miss) != 2 {
		t.Fatalf("Missing = %v, want both assets (stale version + new id)", miss)
	}
}

func TestManifestDropMissing(t *testing.T) {
	m := library.NewMemoryManifest(testutil.NopLogger())
	m.Set("gone.jpg", library.ManifestEntry{ID: "id-a", Version: "v1", Bytes: 10})
	m.Set("here.jpg", library.ManifestEntry{ID: "id-b", Version: "v1", Bytes: 10})

	m.DropMissing(map[string]bool{"here.jpg": true})
	if m.Has("gone.jpg") {
		t.Error("entry for vanished file not dropped")
	}
	if !m.Has("here.jpg") {
		t.Error("entry for present file dropped")
	}
}

// Files predating the manifest (or written around a crash) are adopted, not
// re-downloaded: content stays byte-identical.
func TestSyncAdoptsPreManifestFiles(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	a := asset(idA, 1)
	r.set(a)

	name := library.SyncedFilename(a)
	f, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	custom := []byte("pre-existing-bytes")
	if _, err := f.Write(custom); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	m := library.LoadManifest(testutil.NopLogger(), root)
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{}, library.WithManifest(m))
	runOnce(t, s)

	rf, err := root.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	got, _ := io.ReadAll(rf)
	if string(got) != string(custom) {
		t.Errorf("adopted file rewritten: %q", got)
	}
	if !m.Has(name) {
		t.Error("adopted file not tracked in manifest")
	}
}

func TestSyncWritesManifest(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1), asset(idB, 2))
	m := library.LoadManifest(testutil.NopLogger(), root)
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{}, library.WithManifest(m))
	runOnce(t, s)

	entries := loadManifestFile(t, root)
	if len(entries) != 2 {
		t.Fatalf("manifest entries = %d, want 2", len(entries))
	}
	for name, e := range entries {
		if e["bytes"].(float64) <= 0 {
			t.Errorf("%s: bytes = %v, want > 0", name, e["bytes"])
		}
		if e["id"] == "" || e["version"] == "" {
			t.Errorf("%s: missing id/version: %v", name, e)
		}
	}
}

// A file deleted from disk behind the syncer's back re-downloads: the stale
// manifest entry is dropped, not trusted.
func TestSyncRecoversDeletedFile(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1))
	m := library.LoadManifest(testutil.NopLogger(), root)
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{}, library.WithManifest(m))
	runOnce(t, s)

	name := library.SyncedFilename(asset(idA, 1))
	if err := root.Remove(name); err != nil {
		t.Fatal(err)
	}
	runOnce(t, s)

	if _, err := root.Stat(name); err != nil {
		t.Errorf("deleted file not re-downloaded: %v", err)
	}
}

// Removing an asset upstream deletes the file and its manifest entry: adding
// the same version back re-downloads instead of trusting a stale entry.
func TestSyncStaleRemovalDropsManifestEntry(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	a := asset(idA, 1)
	r.set(a)
	m := library.LoadManifest(testutil.NopLogger(), root)
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{}, library.WithManifest(m))
	runOnce(t, s)

	r.set() // remote drops the asset
	runOnce(t, s)

	if entries := loadManifestFile(t, root); len(entries) != 0 {
		t.Fatalf("manifest entries after removal = %v, want empty", entries)
	}

	r.set(a) // same version returns
	runOnce(t, s)
	if _, err := root.Stat(library.SyncedFilename(a)); err != nil {
		t.Errorf("re-added asset not re-downloaded: %v", err)
	}
}

// Over budget, only the first downloads happen; the rest are skipped without
// error and the slideshow keeps what is cached.
func TestSyncBudgetSkipsDownloads(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1), asset(idB, 2))
	adv := &fakeAdvancer{}
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, adv, library.WithMaxBytes(1))
	runOnce(t, s)

	files := names(t, root)
	if len(files) != 1 {
		t.Fatalf("files = %v, want exactly 1 under a 1-byte budget", files)
	}
	if lib.Len() != 1 {
		t.Errorf("library len = %d, want 1", lib.Len())
	}
	if st := s.Status(); st.AssetCount != 2 || st.LastError != "" {
		t.Errorf("status = %+v, want count 2 and no error (skips are not failures)", st)
	}

	// A second cycle must not churn: the cached file stays, the other stays skipped.
	runOnce(t, s)
	if files2 := names(t, root); len(files2) != 1 || files2[0] != files[0] {
		t.Errorf("second cycle files = %v, want stable %v", files2, files)
	}
}

// Unlimited budget (default) downloads everything.
func TestSyncNoBudgetDownloadsAll(t *testing.T) {
	root, lib := setup(t)
	r := &fakeRemote{}
	r.set(asset(idA, 1), asset(idB, 2))
	s := library.NewSyncer(testutil.NopLogger(), r, lib, root, time.Hour, &fakeAdvancer{})
	runOnce(t, s)

	if lib.Len() != 2 {
		t.Errorf("library len = %d, want 2", lib.Len())
	}
}
