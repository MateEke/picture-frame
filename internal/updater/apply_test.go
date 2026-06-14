package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aead.dev/minisign"

	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeDownloader struct{ fail bool }

func (f *fakeDownloader) Download(_ context.Context, _, dest string) error {
	if f.fail {
		return errors.New("dial tcp: no route to host")
	}
	src := map[string]string{
		"picture-frame_1.3.1_linux_armv6.tar.gz": "testdata/picture-frame_1.3.1_linux_armv6.tar.gz",
		checksumsAsset:                           "testdata/checksums.txt",
		sigAsset:                                 "testdata/checksums.txt.minisig",
	}[filepath.Base(dest)]
	if src == "" {
		return errors.New("no fixture for " + dest)
	}
	return copyFile(src, dest)
}

type fakePreflight struct{ err error }

func (f fakePreflight) Check(string) error { return f.err }

func testTarget() *target {
	return &target{
		version:      "v1.3.1",
		url:          "https://x/release.tar.gz",
		checksumsURL: "https://x/checksums.txt",
		sigURL:       "https://x/checksums.txt.minisig",
	}
}

// applyFixture builds an Updater over a temp install dir holding a fake live binary.
// The returned *bool records whether the re-exec restart was triggered.
func applyFixture(t *testing.T, pub minisign.PublicKey, dl Downloader, pf PreflightRunner) (*Updater, string, *bool) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "picture-frame"), []byte("OLD BINARY"), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted := new(bool)
	u := New(Options{
		Log: testutil.NopLogger(), Downloader: dl, Preflight: pf, PubKey: pub, InstallDir: dir,
		Platform: "linux_armv6",
		Restart:  func() error { *restarted = true; return nil },
	})
	return u, dir, restarted
}

func TestApplyHappyPath(t *testing.T) {
	pub, err := minisign.PublicKeyFromFile("testdata/test.pub")
	if err != nil {
		t.Fatal(err)
	}
	u, dir, restarted := applyFixture(t, pub, &fakeDownloader{}, fakePreflight{})

	u.applyTarget(context.Background(), testTarget())

	if !*restarted {
		t.Error("expected a restart to be triggered")
	}
	if bak, _ := os.ReadFile(filepath.Join(dir, "picture-frame.bak")); string(bak) != "OLD BINARY" {
		t.Errorf(".bak: got %q", bak)
	}
	if marker, _ := os.ReadFile(filepath.Join(dir, ".updating")); string(marker) != "v1.3.1" {
		t.Errorf("marker: got %q", marker)
	}
	if live, _ := os.ReadFile(filepath.Join(dir, "picture-frame")); !strings.Contains(string(live), "picture-frame fixture") {
		t.Errorf("live binary not swapped: %q", live)
	}
	if r := u.Status().LastResult; strings.HasPrefix(r, "failed") {
		t.Errorf("LastResult: %q", r)
	}
}

func TestApplyAbortsOnBadSignature(t *testing.T) {
	prod, err := EmbeddedKey() // production key, won't match the test-key-signed fixture
	if err != nil {
		t.Fatal(err)
	}
	u, dir, restarted := applyFixture(t, prod, &fakeDownloader{}, fakePreflight{})

	u.applyTarget(context.Background(), testTarget())

	if *restarted {
		t.Error("bad signature must not restart")
	}
	if _, err := os.Stat(filepath.Join(dir, "picture-frame.bak")); !os.IsNotExist(err) {
		t.Error("bad signature must not create .bak (no swap)")
	}
	if live, _ := os.ReadFile(filepath.Join(dir, "picture-frame")); string(live) != "OLD BINARY" {
		t.Error("bad signature must leave the live binary untouched")
	}
	if !strings.HasPrefix(u.Status().LastResult, "failed") {
		t.Errorf("LastResult should report failure: %q", u.Status().LastResult)
	}
}

func TestApplyAbortsOnDownloadFailure(t *testing.T) {
	pub, err := minisign.PublicKeyFromFile("testdata/test.pub")
	if err != nil {
		t.Fatal(err)
	}
	u, dir, restarted := applyFixture(t, pub, &fakeDownloader{fail: true}, fakePreflight{})

	u.applyTarget(context.Background(), testTarget())

	if *restarted {
		t.Error("download failure must not restart")
	}
	if _, err := os.Stat(filepath.Join(dir, ".updating")); !os.IsNotExist(err) {
		t.Error("download failure must not write the marker (no swap)")
	}
	if !strings.HasPrefix(u.Status().LastResult, "failed") {
		t.Errorf("LastResult should report failure: %q", u.Status().LastResult)
	}
}

func TestExtractBinaryErrors(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")

	notGzip := filepath.Join(dir, "plain.tar.gz")
	if err := os.WriteFile(notGzip, []byte("not a gzip stream"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractBinary(notGzip, dest); err == nil {
		t.Error("non-gzip input should error")
	}

	// A valid .tar.gz that doesn't contain a "picture-frame" entry.
	empty := filepath.Join(dir, "empty.tar.gz")
	writeTarGz(t, empty, "README", []byte("hi"))
	if err := extractBinary(empty, dest); err == nil {
		t.Error("archive without the binary should error")
	}
}

func writeTarGz(t *testing.T, path, name string, content []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyAbortsOnPreflightFail(t *testing.T) {
	pub, err := minisign.PublicKeyFromFile("testdata/test.pub")
	if err != nil {
		t.Fatal(err)
	}
	u, dir, restarted := applyFixture(t, pub, &fakeDownloader{}, fakePreflight{err: errors.New("crashed on start")})

	u.applyTarget(context.Background(), testTarget())

	if *restarted {
		t.Error("preflight failure must not restart")
	}
	if _, err := os.Stat(filepath.Join(dir, ".updating")); !os.IsNotExist(err) {
		t.Error("preflight failure must not write the marker (no swap)")
	}
	if live, _ := os.ReadFile(filepath.Join(dir, "picture-frame")); string(live) != "OLD BINARY" {
		t.Error("preflight failure must leave the live binary untouched")
	}
}

// LastResultSeq must strictly increase so the UI can detect a repeated outcome.
func TestLastResultSeqBumpsPerOutcome(t *testing.T) {
	u := New(Options{Log: testutil.NopLogger()})
	u.setLastResult("failed: x")
	u.setLastResult("failed: x")
	if got := u.Status().LastResultSeq; got != 2 {
		t.Errorf("LastResultSeq after two outcomes: got %d, want 2", got)
	}
}
