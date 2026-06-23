package library

import "testing"

func TestStaleAgainstVersionMismatch(t *testing.T) {
	const id = "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
	local := localSet{{name: id + "-1111.jpg", id: id, version: "1111"}}

	if stale := local.staleAgainst([]Asset{{ID: id, Version: "deadbeefdeadbeef"}}); len(stale) != 1 {
		t.Fatalf("stale = %d, want 1 (token changed)", len(stale))
	}
	if stale := local.staleAgainst([]Asset{{ID: id, Version: "1111"}}); len(stale) != 0 {
		t.Fatalf("stale = %d, want 0 (token unchanged)", len(stale))
	}
}
