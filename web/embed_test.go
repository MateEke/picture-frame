package web

import "testing"

// The embedded build carries _app/version.json (kit.version.name); the checked-in dev tree
// stamps "dev". A non-empty result proves the path + parse stay in step with SvelteKit's output.
func TestBakedVersion(t *testing.T) {
	if got := BakedVersion(); got == "" {
		t.Fatal("BakedVersion is empty, _app/version.json missing or unparseable in the embedded build")
	}
}
