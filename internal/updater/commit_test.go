package updater

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCommitUpdateClearsState(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, bakName), "old binary")
	mustWrite(t, filepath.Join(dir, updatingMarker), "v1.3.1")

	if !markerPresent(dir) {
		t.Fatal("marker should be present before commit")
	}
	if err := commitUpdate(dir); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if markerPresent(dir) {
		t.Error("marker should be gone after commit")
	}
	if _, err := os.Stat(filepath.Join(dir, bakName)); !os.IsNotExist(err) {
		t.Error(".bak should be removed after commit")
	}
}

func TestCommitUpdateIdempotent(t *testing.T) {
	dir := t.TempDir()
	// No .bak, no marker (e.g. a swap-failure disarm, or a retried commit) → still succeeds.
	if err := commitUpdate(dir); err != nil {
		t.Fatalf("commit with nothing to clear: %v", err)
	}
}

func TestReadRolledBackVersion(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readRolledBackVersion(dir); ok {
		t.Error("no marker → ok=false")
	}
	mustWrite(t, filepath.Join(dir, rolledBackMarker), "v1.3.1\n")
	if v, ok := readRolledBackVersion(dir); !ok || v != "v1.3.1" {
		t.Errorf("got (%q, %v), want (v1.3.1, true)", v, ok)
	}
	if _, ok := readRolledBackVersion(dir); ok {
		t.Error("marker should be cleared after the first read")
	}
	// An empty marker still signals a rollback happened (the unit couldn't record the version).
	mustWrite(t, filepath.Join(dir, rolledBackMarker), "")
	if v, ok := readRolledBackVersion(dir); !ok || v != "" {
		t.Errorf("empty marker: got (%q, %v), want (\"\", true)", v, ok)
	}
}

func TestSkipAutoVersions(t *testing.T) {
	dir := t.TempDir()
	if got := skipAutoVersions(dir); got != nil {
		t.Errorf("no file → nil, got %v", got)
	}
	if err := recordSkipAuto(dir, "v1.3.1"); err != nil {
		t.Fatal(err)
	}
	if err := recordSkipAuto(dir, "v1.3.1"); err != nil { // duplicate → no-op
		t.Fatal(err)
	}
	if err := recordSkipAuto(dir, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if got := skipAutoVersions(dir); !slices.Equal(got, []string{"v1.3.1", "v2.0.0"}) {
		t.Errorf("got %v, want [v1.3.1 v2.0.0] with no duplicate", got)
	}
}

func TestExportedMarkerAndCommit(t *testing.T) {
	dir := t.TempDir()
	if MarkerPresent(dir) {
		t.Fatal("no marker yet")
	}
	mustWrite(t, filepath.Join(dir, bakName), "old binary")
	mustWrite(t, filepath.Join(dir, updatingMarker), "v1.3.1")
	if !MarkerPresent(dir) {
		t.Fatal("marker should be present")
	}
	if err := Commit(dir); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if MarkerPresent(dir) {
		t.Error("marker should be cleared after Commit")
	}
}

// The rollback unit hard-codes the same on-disk names as commit.go's constants; renaming a
// constant without updating the unit silently breaks marker gating. Assert they still agree.
func TestRollbackUnitReferencesMarkerNames(t *testing.T) {
	const unit = "../../deploy/systemd/picture-frame-rollback.service"
	data, err := os.ReadFile(unit)
	if err != nil {
		t.Fatalf("read %s: %v", unit, err)
	}
	body := string(data)
	for _, name := range []string{binaryName, bakName, updatingMarker, rolledBackMarker} {
		if !strings.Contains(body, name) {
			t.Errorf("%s does not reference %q, names drifted from commit.go", unit, name)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
