package startup_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/startup"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestHealthCheckPasses(t *testing.T) {
	if err := startup.HealthCheck(testutil.NopLogger()); err != nil {
		t.Fatalf("health check should pass for a well-formed binary: %v", err)
	}
}
