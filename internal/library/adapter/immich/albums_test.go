package immich_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/library/adapter/immich"
)

func TestListAlbumsReturnsAlbums(t *testing.T) {
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"id":"a1","albumName":"Kitchen","assetCount":12},
			{"id":"a2","albumName":"Garden","assetCount":0}]`))
	}))
	defer srv.Close()

	albums, err := immich.ListAlbums(context.Background(), srv.URL+"/", testAPIKey, srv.Client())
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(albums) != 2 {
		t.Fatalf("albums: got %d, want 2", len(albums))
	}
	if albums[0].ID != "a1" || albums[0].AlbumName != "Kitchen" || albums[0].AssetCount != 12 {
		t.Errorf("first: got %+v", albums[0])
	}
	if gotKey != testAPIKey {
		t.Errorf("x-api-key: got %q, want %q", gotKey, testAPIKey)
	}
	// Trailing slash trimmed so the path doesn't double up.
	if gotPath != "/api/albums" {
		t.Errorf("path: got %q, want /api/albums", gotPath)
	}
}

func TestListAlbumsRejectsBadKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := immich.ListAlbums(context.Background(), srv.URL, "wrong", srv.Client())
	if err == nil {
		t.Fatal("want an error for a rejected key")
	}
	if !strings.Contains(err.Error(), "status 401") {
		t.Errorf("err: got %q, want a 401 status", err)
	}
}

func TestListAlbumsRequiresURLAndKey(t *testing.T) {
	if _, err := immich.ListAlbums(context.Background(), "", "k", nil); err == nil {
		t.Error("empty base url: want an error")
	}
	if _, err := immich.ListAlbums(context.Background(), "https://x.example", "", nil); err == nil {
		t.Error("empty key: want an error")
	}
}
