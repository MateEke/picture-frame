package library

import (
	"strings"
	"testing"
)

func TestSafeErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"redacts /var path", "open /var/lib/picture-frame/cache: denied", "open <path> denied"},
		{"redacts /home path", "stat /home/user/file: missing", "stat <path> missing"},
		{"redacts /tmp path", "rm /tmp/foo.tmp: busy", "rm <path> busy"},
		{"redacts /etc path", "read /etc.bak/secret: nope", "read <path> nope"},
		{"leaves bare message untouched", "status 404", "status 404"},
		{"leaves URL host alone", "Get https://immich.example.com/api/x: timeout", "Get https://immich.example.com/api/x: timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeErrorMessage(tc.in); got != tc.want {
				t.Errorf("safeErrorMessage(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSafeErrorMessageTruncates(t *testing.T) {
	in := strings.Repeat("x", 500)
	got := safeErrorMessage(in)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if len(got) > 200 { // 160 chars + 3-byte ellipsis
		t.Errorf("output too long: %d bytes", len(got))
	}
}

func TestSyncedFilenamePanicsOnZeroTime(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for zero UpdatedAt")
		}
	}()
	_ = SyncedFilename(Asset{ID: "x"})
}
