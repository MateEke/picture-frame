package adapter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MateEke/picture-frame/internal/library/adapter"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	lib, err := adapter.Load(testutil.NopLogger(), dir, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lib.Len() != 0 {
		t.Fatalf("expected 0 images, got %d", lib.Len())
	}
}

func TestLoadCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "images")
	if _, err := adapter.Load(testutil.NopLogger(), dir, false); err != nil {
		t.Fatalf("Load should create dir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dir not created: %v", err)
	}
}

func TestLoadSortsByName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"c.jpg", "a.jpg", "b.jpeg", "d.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := adapter.Load(testutil.NopLogger(), dir, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	list := lib.List()
	if len(list) != 4 {
		t.Fatalf("expected 4, got %d", len(list))
	}
	want := []string{"a.jpg", "b.jpeg", "c.jpg", "d.png"}
	for i, img := range list {
		if img.Name != want[i] {
			t.Errorf("[%d] got %q, want %q", i, img.Name, want[i])
		}
	}
}

func TestLoadIncludesPNG(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"photo.jpg", "snap.png", "IMG_20221223~2.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := adapter.Load(testutil.NopLogger(), dir, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lib.Len() != 3 {
		t.Errorf("expected 3 images (jpg+png), got %d: %v", lib.Len(), lib.List())
	}
}

func TestLoadIgnoresUnsupportedExtensions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"photo.jpg", "readme.txt", "video.mp4", "icon.gif"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := adapter.Load(testutil.NopLogger(), dir, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lib.Len() != 1 || lib.List()[0].Name != "photo.jpg" {
		t.Errorf("expected only photo.jpg, got %v", lib.List())
	}
}

// Names the /img route would reject (spaces, unicode, uppercase extension) must
// not enter the library; they would freeze the kiosk on an unfetchable image.
func TestLoadSkipsUnservableNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"family photo.jpg", "nyaralás.jpg", "IMG_1234.JPG", "ok.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := adapter.Load(testutil.NopLogger(), dir, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lib.Len() != 1 || lib.List()[0].Name != "ok.jpg" {
		t.Errorf("expected only ok.jpg, got %v", lib.List())
	}
}

func TestLoadIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir.jpg"), 0o750); err != nil {
		t.Fatal(err)
	}
	lib, err := adapter.Load(testutil.NopLogger(), dir, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lib.Len() != 0 {
		t.Fatalf("directories should not be treated as images")
	}
}
