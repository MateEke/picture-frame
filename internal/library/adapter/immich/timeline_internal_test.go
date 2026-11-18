package immich

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
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

// Immich may report the bucket in ISO form on one server and accept only the
// bare date; the fallback must still send a different value.
func TestGetTimelineBucketFallsBackFromISOOn400(t *testing.T) {
	var calls []string
	get := func(_ context.Context, _ string, q url.Values, _ any) error {
		calls = append(calls, q.Get("timeBucket"))
		if len(calls) == 1 {
			return &httpError{Status: http.StatusBadRequest, Detail: "invalid bucket"}
		}
		return nil
	}
	if err := getTimelineBucket(context.Background(), get, "album", "2026-05-01T00:00:00.000Z", &timelineBucket{}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "2026-05-01T00:00:00.000Z" || calls[1] != "2026-05-01" {
		t.Errorf("calls = %v", calls)
	}
}

// An uninterpretable bucket has only one form; don't repeat a doomed request.
func TestGetTimelineBucketDoesNotRepeatUnchangedBucket(t *testing.T) {
	calls := 0
	get := func(_ context.Context, _ string, _ url.Values, _ any) error {
		calls++
		return &httpError{Status: http.StatusBadRequest}
	}
	if err := getTimelineBucket(context.Background(), get, "album", "not-a-date", &timelineBucket{}); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestHTTPErrorCarriesDetail(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("  bad\n bucket  ")),
	}
	err := statusError(resp)
	if got, want := err.HTTPStatus(), http.StatusBadRequest; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if got, want := err.Error(), "status 400: bad bucket"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}
