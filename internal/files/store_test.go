package files_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/files"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func open(t *testing.T, dir string) *files.Store {
	t.Helper()
	s, err := files.Open(testutil.NopLogger(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"Kesä 2024.MP4":         "Kesa_2024.mp4",
		"../../etc/passwd":      "passwd",
		`C:\Users\x\doc.pdf`:    "doc.pdf",
		".hidden":               "hidden",
		"ääkköset ja 😀.txt":     "aakkoset_ja_.txt",
		"":                      "tiedosto",
		"😀😀":                    "tiedosto",
		"a.verylongextensionxx": "a.verylongextensionxx",
	}
	for in, want := range cases {
		got := files.Sanitize(in)
		if got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
		if !files.ValidName(got) {
			t.Errorf("Sanitize(%q) = %q is not a valid name", in, got)
		}
	}
	long := strings.Repeat("x", 300) + ".pdf"
	if got := files.Sanitize(long); len(got) > 120 || !strings.HasSuffix(got, ".pdf") {
		t.Errorf("long name = %q", got)
	}
}

func TestSaveListDelete(t *testing.T) {
	s := open(t, t.TempDir())
	m, err := s.Save("Loma video.mp4", strings.NewReader("data"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Loma_video.mp4" || m.Original != "Loma video.mp4" || m.Size != 4 || m.MIME != "video/mp4" {
		t.Fatalf("meta = %+v", m)
	}
	m2, err := s.Save("Loma video.mp4", strings.NewReader("more"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if m2.Name != "Loma_video-1.mp4" {
		t.Fatalf("collision name = %q", m2.Name)
	}
	if l := s.List(); len(l) != 2 || l[0].Name != m2.Name {
		t.Fatalf("list = %+v", l)
	}
	f, got, err := s.OpenFile(m.Name)
	if err != nil || got.Name != m.Name {
		t.Fatalf("open: %v", err)
	}
	f.Close()
	if err := s.Delete(m.Name); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(m.Name); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
	if _, _, err := s.OpenFile(m.Name); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("open deleted = %v", err)
	}
}

func TestSaveLimit(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	if _, err := s.Save("big.bin", strings.NewReader("12345"), 4); !errors.Is(err, files.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Fatal("temp file left behind")
		}
	}
	if len(s.List()) != 0 {
		t.Fatal("rejected upload listed")
	}
}

func TestReopenReconciles(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	m, err := s.Save("notes.txt", strings.NewReader("hello"), 0)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err := os.WriteFile(filepath.Join(dir, "manual.pdf"), []byte("%PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".upload-123"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s2 := open(t, dir)
	l := s2.List()
	if len(l) != 2 {
		t.Fatalf("list = %+v", l)
	}
	if got, ok := s2.Get(m.Name); !ok || got.Original != "notes.txt" {
		t.Fatalf("original lost: %+v", got)
	}
	if got, _ := s2.Get("manual.pdf"); got.MIME != "application/pdf" {
		t.Fatalf("mime = %q", got.MIME)
	}
	if _, err := os.Stat(filepath.Join(dir, ".upload-123")); err == nil {
		t.Fatal("stale temp not cleaned")
	}
	if free, ok := s2.FreeBytes(); !ok || free == 0 {
		t.Fatalf("free = %d %v", free, ok)
	}
}

func TestSaveSniffsMIMEWithoutExtension(t *testing.T) {
	s := open(t, t.TempDir())
	m, err := s.Save("README", strings.NewReader("plain words here"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.MIME, "text/plain") {
		t.Fatalf("mime = %q", m.MIME)
	}
}
