package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadWritesBodyWithToken(t *testing.T) {
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte("release-bytes"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := NewHTTPDownloader("ghp_secret").Download(context.Background(), srv.URL, dest); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "release-bytes" {
		t.Errorf("dest content: %q", got)
	}
	if gotAuth != "Bearer ghp_secret" {
		t.Errorf("token → Authorization: got %q", gotAuth)
	}
	if gotAccept != "application/octet-stream" {
		t.Errorf("must request the raw asset, got Accept %q", gotAccept)
	}
}

func TestDownloadErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out")
	if err := NewHTTPDownloader("").Download(context.Background(), srv.URL, dest); err == nil {
		t.Error("404 should error")
	}
}
