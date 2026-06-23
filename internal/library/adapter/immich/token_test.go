package immich

import "testing"

func TestThumbhashToken(t *testing.T) {
	if empty := thumbhashToken(""); empty != "0000000000000000" {
		t.Errorf("empty thumbhash token = %q, want 16 zeros", empty)
	}
	a := thumbhashToken("mQgOBQRoiIB3iHeIiIiYaF6PeAh3")
	if len(a) != 16 {
		t.Errorf("token len = %d, want 16", len(a))
	}
	if a == thumbhashToken("differenthashvalue==") {
		t.Error("distinct thumbhashes produced the same token")
	}
	if a != thumbhashToken("mQgOBQRoiIB3iHeIiIiYaF6PeAh3") {
		t.Error("token not stable for the same thumbhash")
	}
}
