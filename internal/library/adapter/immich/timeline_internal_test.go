package immich

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestBucketParam(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"2026-05-01", "2026-05-01T00:00:00.000Z"},
		{"2026-05-01T00:00:00.000Z", "2026-05-01T00:00:00.000Z"},
		{"", ""},
		{"not-a-date", "not-a-date"},
	}
	for _, tt := range tests {
		if got := bucketParam(tt.in); got != tt.want {
			t.Errorf("bucketParam(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// A v3 server rejects the legacy date; the retry must use it on 400 only.
func TestGetTimelineBucketFallsBackOn400(t *testing.T) {
	var calls []string
	get := func(_ context.Context, path string, q url.Values, _ any) error {
		if path != "/api/timeline/bucket" {
			t.Errorf("path = %q", path)
		}
		v := q.Get("timeBucket")
		calls = append(calls, v)
		if len(calls) == 1 {
			return &httpError{Status: http.StatusBadRequest}
		}
		return nil
	}
	if err := getTimelineBucket(context.Background(), get, "album", "2026-05-01", &timelineBucket{}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "2026-05-01T00:00:00.000Z" || calls[1] != "2026-05-01" {
		t.Errorf("calls = %v", calls)
	}
}

func TestGetTimelineBucketDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	get := func(_ context.Context, _ string, _ url.Values, _ any) error {
		calls++
		return &httpError{Status: http.StatusInternalServerError}
	}
	if err := getTimelineBucket(context.Background(), get, "album", "2026-05-01", &timelineBucket{}); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}
