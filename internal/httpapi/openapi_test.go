package httpapi

import (
	"strings"
	"testing"
)

// OpenAPISpec builds a bare server, so it catches route registration that
// assumes NewServer-only initialization (a nil kioskPaths map panicked here).
func TestOpenAPISpec(t *testing.T) {
	spec, err := OpenAPISpec()
	if err != nil {
		t.Fatalf("OpenAPISpec: %v", err)
	}
	if !strings.Contains(string(spec), `"/api/heartbeat"`) {
		t.Error("spec is missing the heartbeat route")
	}
}
