package library_test

import (
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func writeTestJPEG(t *testing.T, dir, name string, w, h int) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
}

func newAspectStore(t *testing.T, dir string) *library.AspectStore {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	t.Cleanup(func() { root.Close() })
	s, err := library.LoadAspectStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatalf("load aspect store: %v", err)
	}
	return s
}

func TestAspectStoreEmpty(t *testing.T) {
	s := newAspectStore(t, t.TempDir())
	if _, ok := s.Ratio("a.jpg"); ok {
		t.Error("unknown name should report not-ok")
	}
}

func TestAspectStoreSetAndRatio(t *testing.T) {
	s := newAspectStore(t, t.TempDir())
	s.Set("a.jpg", 800, 600)
	got, ok := s.Ratio("a.jpg")
	if !ok {
		t.Fatal("Set name should be known")
	}
	if want := 800.0 / 600.0; got != want {
		t.Errorf("Ratio = %v, want %v", got, want)
	}
}

func TestAspectStoreRatioUnknownOrZeroHeight(t *testing.T) {
	s := newAspectStore(t, t.TempDir())
	if _, ok := s.Ratio("missing.jpg"); ok {
		t.Error("missing name should be not-ok")
	}
	s.Set("bad.jpg", 5, 0)
	if _, ok := s.Ratio("bad.jpg"); ok {
		t.Error("zero height should be not-ok")
	}
}

func TestAspectStoreDelete(t *testing.T) {
	s := newAspectStore(t, t.TempDir())
	s.Set("a.jpg", 4, 3)
	s.Delete("a.jpg")
	if _, ok := s.Ratio("a.jpg"); ok {
		t.Error("deleted name should be not-ok")
	}
}

func TestAspectStoreMissing(t *testing.T) {
	s := newAspectStore(t, t.TempDir())
	s.Set("a.jpg", 4, 3)
	got := s.Missing([]string{"a.jpg", "b.jpg", "c.jpg"})
	if len(got) != 2 || got[0] != "b.jpg" || got[1] != "c.jpg" {
		t.Errorf("Missing = %v, want [b.jpg c.jpg]", got)
	}
}

func TestLoadAspectStoreCorruptIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".aspect-index.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("write corrupt index: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer root.Close()
	if _, err := library.LoadAspectStore(testutil.NopLogger(), root); err == nil {
		t.Error("expected error loading a corrupt index")
	}
}

func TestAspectStoreBackfillMissing(t *testing.T) {
	dir := t.TempDir()
	writeTestJPEG(t, dir, "wide.jpg", 4, 2)
	writeTestJPEG(t, dir, "known.jpg", 9, 9)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer root.Close()
	store, err := library.LoadAspectStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	store.Set("known.jpg", 1, 1) // pre-known: must not be re-decoded/overwritten

	lib := library.New([]library.Image{{Name: "wide.jpg"}, {Name: "known.jpg"}}, false)
	if !store.BackfillMissing(context.Background(), lib) {
		t.Error("BackfillMissing should report true after decoding a missing image")
	}

	if ratio, ok := store.Ratio("wide.jpg"); !ok || ratio != 2.0 {
		t.Errorf("backfilled wide.jpg ratio = %v ok=%v, want 2.0", ratio, ok)
	}
	if ratio, _ := store.Ratio("known.jpg"); ratio != 1.0 {
		t.Errorf("known.jpg should keep its cached 1.0, got %v", ratio)
	}

	reloaded, err := library.LoadAspectStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := reloaded.Ratio("wide.jpg"); !ok {
		t.Error("backfill did not persist wide.jpg")
	}
}

func TestAspectStoreBackfillSkipsUndecodable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.jpg"), []byte("not an image"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	root, _ := os.OpenRoot(dir)
	defer root.Close()
	store, _ := library.LoadAspectStore(testutil.NopLogger(), root)
	lib := library.New([]library.Image{{Name: "broken.jpg"}}, false)
	if store.BackfillMissing(context.Background(), lib) {
		t.Error("BackfillMissing should report false when nothing decodes")
	}
	if _, ok := store.Ratio("broken.jpg"); ok {
		t.Error("undecodable file should remain unknown")
	}
}

func TestAspectStoreConcurrentFlush(t *testing.T) {
	s := newAspectStore(t, t.TempDir())
	s.Set("a.jpg", 16, 9)

	errs := make([]error, 24)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { errs[i] = s.Flush() })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent Flush %d errored: %v", i, err)
		}
	}
}

func TestAspectStoreFlushRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := newAspectStore(t, dir)
	s.Set("a.jpg", 1920, 1080)
	if err := s.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	reloaded := newAspectStore(t, dir)
	got, ok := reloaded.Ratio("a.jpg")
	if !ok || got != 1920.0/1080.0 {
		t.Errorf("reloaded Ratio = %v, ok=%v; want persisted", got, ok)
	}
}
