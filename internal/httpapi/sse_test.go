package httpapi_test

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func newSSEServer(t *testing.T) (*httptest.Server, *state.Bus) {
	t.Helper()
	bus := state.NewBus()
	srv := httptest.NewServer(httpapi.NewServer(httpapi.Config{
		Log:    testutil.NopLogger(),
		Screen: &mockScreen{},
		Bus:    bus,
	}))
	t.Cleanup(srv.Close)
	return srv, bus
}

func TestSSEHeaders(t *testing.T) {
	srv, _ := newSSEServer(t)

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", got)
	}
}

func TestSSEReconcilesOnConnect(t *testing.T) {
	m := &mockScreen{}
	srv := httptest.NewServer(httpapi.NewServer(httpapi.Config{
		Log:    testutil.NopLogger(),
		Screen: m,
		Bus:    state.NewBus(),
	}))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Headers arrive only after handleEvents runs Reconcile + flushes, so it has fired once.
	if got := m.reconciles.Load(); got != 1 {
		t.Errorf("expected exactly 1 reconcile on connect, got %d", got)
	}
}

func TestSSEEventStreaming(t *testing.T) {
	srv, bus := newSSEServer(t)

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Publish after a short delay so the handler has time to subscribe.
	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish(state.Event{
			Kind: state.KindSensor,
			Payload: state.SensorPayload{
				DeviceID: "d1",
				Role:     "inside",
				Kind:     "temperature",
				Value:    21.5,
			},
		})
	}()

	line := readSSELine(t, resp.Body, 2*time.Second)
	if !strings.Contains(line, "event: sensor") {
		t.Errorf("expected event: sensor line, got: %q", line)
	}
}

func TestSSEEventFormat(t *testing.T) {
	srv, bus := newSSEServer(t)

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish(state.Event{
			Kind:    state.KindSensor,
			Payload: state.SensorPayload{DeviceID: "d1", Kind: "temperature", Value: 23.45},
		})
	}()

	// Collect the full SSE block (id + event + data + blank line).
	scanner := bufio.NewScanner(resp.Body)
	got := collectSSEBlock(t, scanner, 2*time.Second)

	checks := []string{"id: 1", "event: sensor", `"value":23.45`}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in SSE block:\n%s", want, got)
		}
	}
}

func TestSSEMultipleSubscribers(t *testing.T) {
	srv, bus := newSSEServer(t)

	resp1, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()

	resp2, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish(state.Event{
			Kind:    state.KindSensor,
			Payload: state.SensorPayload{DeviceID: "d1", Kind: "temperature", Value: 20.0},
		})
	}()

	// Both connections should receive the event.
	for i, body := range []io.ReadCloser{resp1.Body, resp2.Body} {
		line := readSSELine(t, body, 2*time.Second)
		if !strings.Contains(line, "event: sensor") {
			t.Errorf("subscriber %d: expected sensor event, got %q", i+1, line)
		}
	}
}

// readSSELine reads lines from r until one contains "event:" (other than the
// bootstrap "event: ready" signal) or the deadline passes.
func readSSELine(t *testing.T, r io.Reader, timeout time.Duration) string {
	t.Helper()
	lines := make(chan string, 16)
	scanErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanErr <- scanner.Err()
	}()
	deadline := time.After(timeout)
	for {
		select {
		case line := <-lines:
			if strings.HasPrefix(line, "event:") && line != "event: ready" {
				return line
			}
		case err := <-scanErr:
			t.Fatalf("stream closed before a matching event (scanner err: %v)", err)
			return ""
		case <-deadline:
			t.Fatal("timeout waiting for SSE event line")
			return ""
		}
	}
}

// collectSSEBlock reads SSE blocks (terminated by a blank line) and returns
// the first block that is not the bootstrap "ready" event, or fails on timeout.
func collectSSEBlock(t *testing.T, scanner *bufio.Scanner, timeout time.Duration) string {
	t.Helper()
	done := make(chan string, 1)
	scanErr := make(chan error, 1)
	go func() {
		var sb strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			sb.WriteString(line)
			sb.WriteByte('\n')
			if line == "" {
				block := sb.String()
				if strings.Contains(block, "event: ready") {
					sb.Reset()
					continue
				}
				done <- block
				return
			}
		}
		scanErr <- scanner.Err()
	}()
	select {
	case block := <-done:
		return block
	case err := <-scanErr:
		t.Fatalf("stream closed before a complete block (scanner err: %v)", err)
		return ""
	case <-time.After(timeout):
		t.Fatal("timeout waiting for SSE block")
		return ""
	}
}
