package redact_test

import (
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/redact"
)

func TestPathRedactsAndCaps(t *testing.T) {
	if got := redact.Path("open /home/u/secret.toml: denied"); strings.Contains(got, "/home") {
		t.Errorf("path leaked: %q", got)
	}
	if got := redact.Path("no paths here"); got != "no paths here" {
		t.Errorf("clean string changed: %q", got)
	}
	if got := redact.Path(strings.Repeat("x", 200)); len(got) > 164 {
		t.Errorf("length not capped: %d", len(got))
	}
	if in := strings.Repeat("x", 160); redact.Path(in) != in {
		t.Error("exact-maxLen string must pass unchanged")
	}
}
