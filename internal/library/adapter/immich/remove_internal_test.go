package immich

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const (
	removeTestAlbum = "1e4bb746-6072-4aec-9a20-a37b130cde4b"
	removeTestAsset = "9f7e71ab-a630-449d-b7c1-c52fc8b44733"
)

func newRemovalClient(t *testing.T, handler http.HandlerFunc) *APIClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewAPIClient(APIConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		AlbumIDs: []string{removeTestAlbum},
	})
	if err != nil {
		t.Fatalf("NewAPIClient: %v", err)
	}
	// Pretend the album was listed, so invalidation is observable.
	c.albums[0].etag = `"v1"`
	c.albums[0].loaded = true
	return c
}

func TestRemoveFromAlbumSendsBulkDeleteAndInvalidatesCache(t *testing.T) {
	var gotMethod, gotPath, gotKey string
	var gotIDs []string
	c := newRemovalClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotKey = r.Method, r.URL.Path, r.Header.Get("x-api-key")
		var body struct {
			IDs []string `json:"ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotIDs = body.IDs
		_, _ = w.Write([]byte(`[{"id":"` + removeTestAsset + `","success":true}]`))
	})

	if err := c.RemoveFromAlbum(t.Context(), removeTestAlbum, removeTestAsset); err != nil {
		t.Fatalf("RemoveFromAlbum: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if want := "/api/albums/" + removeTestAlbum + "/assets"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotKey != "test-key" {
		t.Errorf("x-api-key = %q, want the configured key", gotKey)
	}
	if len(gotIDs) != 1 || gotIDs[0] != removeTestAsset {
		t.Errorf("ids = %v, want [%s]", gotIDs, removeTestAsset)
	}
	if c.albums[0].etag != "" || c.albums[0].loaded {
		t.Errorf("album cache not invalidated: etag=%q loaded=%v", c.albums[0].etag, c.albums[0].loaded)
	}
}

// Removing a photo must only break the album–asset link. Immich deletes
// assets at DELETE /api/assets; that endpoint must never be touched here.
func TestRemoveFromAlbumNeverDeletesTheAsset(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	c := newRemovalClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.URL.Path != "/api/albums/"+removeTestAlbum+"/assets" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"` + removeTestAsset + `","success":true}]`))
	})

	if err := c.RemoveFromAlbum(t.Context(), removeTestAlbum, removeTestAsset); err != nil {
		t.Fatalf("RemoveFromAlbum: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := "DELETE /api/albums/" + removeTestAlbum + "/assets"
	if len(requests) != 1 || requests[0] != want {
		t.Fatalf("requests = %v, want exactly [%s]; the asset itself must not be deleted", requests, want)
	}
}

func TestRemoveFromAlbumTreatsNotFoundAsRemoved(t *testing.T) {
	c := newRemovalClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"` + removeTestAsset + `","success":false,"error":"not_found"}]`))
	})
	if err := c.RemoveFromAlbum(t.Context(), removeTestAlbum, removeTestAsset); err != nil {
		t.Fatalf("RemoveFromAlbum: %v", err)
	}
}

func TestRemoveFromAlbumReturnsPerAssetFailure(t *testing.T) {
	c := newRemovalClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"` + removeTestAsset + `","success":false,"error":"no_permission"}]`))
	})
	err := c.RemoveFromAlbum(t.Context(), removeTestAlbum, removeTestAsset)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "no_permission") {
		t.Errorf("error = %q, want the Immich failure reason", err)
	}
	if c.albums[0].etag == "" {
		t.Error("failed removal must keep the album cache")
	}
}

func TestRemoveFromAlbumAcceptsNoContent(t *testing.T) {
	c := newRemovalClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.RemoveFromAlbum(t.Context(), removeTestAlbum, removeTestAsset); err != nil {
		t.Fatalf("RemoveFromAlbum: %v", err)
	}
}

func TestRemoveFromAlbumSurfacesStatusError(t *testing.T) {
	c := newRemovalClient(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	err := c.RemoveFromAlbum(t.Context(), removeTestAlbum, removeTestAsset)
	var statusErr StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %v, want a StatusError", err)
	}
	if statusErr.HTTPStatus() != http.StatusForbidden {
		t.Errorf("status = %d, want 403", statusErr.HTTPStatus())
	}
}
