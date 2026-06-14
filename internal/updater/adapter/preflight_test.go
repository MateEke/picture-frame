package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeScript creates an executable shell script and returns its path.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-binary")
	// The pre-flight execs this path, so it must be executable (G306 is fine here).
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	return p
}

func TestPreflightPassesOnCleanExit(t *testing.T) {
	bin := writeScript(t, "exit 0")
	if err := (ExecPreflight{}).Check(bin); err != nil {
		t.Errorf("clean exit should pass: %v", err)
	}
}

func TestPreflightFailsAndIncludesOutput(t *testing.T) {
	bin := writeScript(t, `echo "boom on this device" >&2; exit 1`)
	err := (ExecPreflight{}).Check(bin)
	if err == nil {
		t.Fatal("non-zero exit should fail")
	}
	if !strings.Contains(err.Error(), "boom on this device") {
		t.Errorf("error should surface the binary's output: %v", err)
	}
}
