package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installShim puts a fake wlr-randr on PATH: logs argv per call, prints
// queryOutput for no-arg queries.
func installShim(t *testing.T, queryOutput string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args.log")
	queryFile := filepath.Join(dir, "query.txt")
	if err := os.WriteFile(queryFile, []byte(queryOutput), 0600); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n[ $# -eq 0 ] && cat %q\nexit 0\n", argsFile, queryFile)
	if err := os.WriteFile(filepath.Join(dir, "wlr-randr"), []byte(script), 0o700); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func shimCalls(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	// Trailing newline only: a no-arg query logs an empty line that must survive.
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

const queryNormal = `HDMI-A-1 "Mock Panel"
  Enabled: yes
  Modes:
    1920x1080 px, 60.000000 Hz (preferred, current)
  Position: 0,0
  Transform: normal
  Scale: 1.000000
`

const query90 = `HDMI-A-1 "Mock Panel"
  Enabled: yes
  Transform: 90
`

func newTestRotator(t *testing.T, desired int) (*WlrRandr, string) {
	t.Helper()
	runtimeDir := t.TempDir()
	r := NewWlrRandr("HDMI-A-1", desired, runtimeDir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	return r, filepath.Join(runtimeDir, "rotation")
}

func TestWlrRandrSetInvokesTransform(t *testing.T) {
	argsFile := installShim(t, queryNormal)
	r, intentPath := newTestRotator(t, 0)

	if err := r.Set(context.Background(), 90); err != nil {
		t.Fatalf("Set: %v", err)
	}
	calls := shimCalls(t, argsFile)
	if len(calls) != 1 || calls[0] != "--output HDMI-A-1 --transform 90" {
		t.Errorf("calls = %q, want one '--output HDMI-A-1 --transform 90'", calls)
	}
	intent, err := os.ReadFile(intentPath)
	if err != nil || string(intent) != "90" {
		t.Errorf("intent file = %q, %v; want \"90\"", intent, err)
	}
}

func TestWlrRandrSetMapsZeroToNormal(t *testing.T) {
	argsFile := installShim(t, queryNormal)
	r, _ := newTestRotator(t, 90)

	if err := r.Set(context.Background(), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	calls := shimCalls(t, argsFile)
	if len(calls) != 1 || calls[0] != "--output HDMI-A-1 --transform normal" {
		t.Errorf("calls = %q, want '--output HDMI-A-1 --transform normal'", calls)
	}
}

func TestWlrRandrSetRejectsInvalidDegrees(t *testing.T) {
	installShim(t, queryNormal)
	r, _ := newTestRotator(t, 0)
	if err := r.Set(context.Background(), 45); err == nil {
		t.Fatal("expected error for 45 degrees")
	}
}

func TestWlrRandrSetSkipsWhenAlreadyApplied(t *testing.T) {
	argsFile := installShim(t, queryNormal)
	r, _ := newTestRotator(t, 0)

	if err := r.Set(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	if err := r.Set(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	if calls := shimCalls(t, argsFile); len(calls) != 1 {
		t.Errorf("got %d exec calls, want 1 (second Set is a no-op)", len(calls))
	}
}

func TestWlrRandrReconcileAppliesOnDrift(t *testing.T) {
	argsFile := installShim(t, queryNormal) // compositor says normal
	r, _ := newTestRotator(t, 90)           // desired 90

	r.Reconcile(context.Background())
	calls := shimCalls(t, argsFile)
	// One query (logged as an empty line) + one set.
	want := "--output HDMI-A-1 --transform 90"
	if len(calls) != 2 || strings.TrimSpace(calls[0]) != "" || calls[1] != want {
		t.Errorf("calls = %q, want [query, %q]", calls, want)
	}
}

func TestWlrRandrReconcileSkipsWhenUnsupported(t *testing.T) {
	argsFile := installShim(t, queryNormal)
	r, _ := newTestRotator(t, 90)

	t.Setenv("PATH", t.TempDir()) // wlr-randr gone
	r.Reconcile(context.Background())
	if calls := shimCalls(t, argsFile); len(calls) != 0 {
		t.Errorf("calls = %q, want none when unsupported", calls)
	}
}

func TestWlrRandrReconcileNoopWithoutDrift(t *testing.T) {
	argsFile := installShim(t, query90)
	r, _ := newTestRotator(t, 90)

	r.Reconcile(context.Background())
	calls := shimCalls(t, argsFile)
	if len(calls) != 1 || strings.TrimSpace(calls[0]) != "" {
		t.Errorf("calls = %q, want only the query", calls)
	}
}

func TestWlrRandrIntentWrittenOnConstruction(t *testing.T) {
	installShim(t, queryNormal)
	_, intentPath := newTestRotator(t, 270)
	intent, err := os.ReadFile(intentPath)
	if err != nil || string(intent) != "270" {
		t.Errorf("intent file = %q, %v; want \"270\" at construction", intent, err)
	}
}

func TestWlrRandrSupported(t *testing.T) {
	r, _ := newTestRotator(t, 0)
	t.Setenv("PATH", t.TempDir())
	if r.Supported() {
		t.Error("Supported() = true with empty PATH")
	}
	installShim(t, queryNormal)
	if !r.Supported() {
		t.Error("Supported() = false with shim on PATH (must re-check per call)")
	}
}

func TestParseTransform(t *testing.T) {
	cases := []struct {
		name, out, connector, want string
		wantErr                    bool
	}{
		{"normal", queryNormal, "HDMI-A-1", "normal", false},
		{"rotated", query90, "HDMI-A-1", "90", false},
		{"other connector only", query90, "HDMI-A-2", "", true},
		{"empty", "", "HDMI-A-1", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTransform([]byte(tc.out), tc.connector)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
