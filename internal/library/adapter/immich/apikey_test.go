package immich_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/MateEke/picture-frame/internal/library/adapter/immich"
)

const (
	testAPIBase   = "https://photos.example.com"
	testAPIKey    = "test-api-key"
	testAPIAlbum  = "1e4bb746-6072-4aec-9a20-a37b130cde4b"
	testAPIAlbumB = "2e4bb746-6072-4aec-9a20-a37b130cde4c"
)

type recordedAPIRequest struct {
	method string
	path   string
	key    string // x-api-key header value
	size   string
}

type fakeAPIImmich struct {
	mu       sync.Mutex
	requests []recordedAPIRequest
	assets   []fakeAsset // default album (testAPIAlbum) timeline contents
	etag     string      // default album ETag; "" disables conditional caching
	preview  []byte
	status   int
	// albums, when non-nil, replaces assets/etag with per-album contents;
	// unknown album IDs 404. etags maps album ID → ETag ("" disables the gate).
	albums map[string][]fakeAsset
	etags  map[string]string
}

func newFakeAPIImmich() *fakeAPIImmich {
	return &fakeAPIImmich{
		etag:    `"etag-v1"`,
		preview: []byte("PREVIEW-BYTES"),
		status:  http.StatusOK,
	}
}

func (f *fakeAPIImmich) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, recordedAPIRequest{
			method: r.Method,
			path:   r.URL.Path,
			key:    r.Header.Get("x-api-key"),
			size:   r.URL.Query().Get("size"),
		})
		status := f.status
		f.mu.Unlock()

		if r.Header.Get("x-api-key") != testAPIKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("password") != "" || r.URL.Query().Get("key") != "" {
			http.Error(w, "credentials must travel in the x-api-key header", http.StatusBadRequest)
			return
		}
		if status != http.StatusOK {
			http.Error(w, "boom", status)
			return
		}
		if id, ok := strings.CutPrefix(r.URL.Path, "/api/albums/"); ok {
			if _, known := f.albumAssets(id); !known {
				http.NotFound(w, r)
				return
			}
			serveGate(w, r, f.albumETag(id))
			return
		}
		albumID := r.URL.Query().Get("albumId")
		switch r.URL.Path {
		case "/api/timeline/buckets":
			assets, known := f.albumAssets(albumID)
			if !known {
				http.NotFound(w, r)
				return
			}
			if len(assets) == 0 {
				writeJSON(w, `[]`)
				return
			}
			writeJSON(w, `[{"timeBucket":"2026-05-01","count":`+strconv.Itoa(len(assets))+`}]`)
		case "/api/timeline/bucket":
			assets, known := f.albumAssets(albumID)
			if !known {
				http.NotFound(w, r)
				return
			}
			var cols struct {
				ID        []string `json:"id"`
				IsImage   []bool   `json:"isImage"`
				Thumbhash []string `json:"thumbhash"`
			}
			for _, a := range assets {
				cols.ID = append(cols.ID, a.id)
				cols.IsImage = append(cols.IsImage, a.isImage)
				cols.Thumbhash = append(cols.Thumbhash, a.thumbhash)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cols)
		default:
			if id, isThumb := thumbnailAssetID(r.URL.Path); isThumb {
				if r.URL.Query().Get("albumId") != "" {
					http.Error(w, "unexpected album param", http.StatusBadRequest)
					return
				}
				_ = id
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write(f.preview)
				return
			}
			http.NotFound(w, r)
		}
	})
}

// albumAssets returns the timeline contents for an album: per-album when the
// albums map is set, otherwise the default album only.
func (f *fakeAPIImmich) albumAssets(id string) ([]fakeAsset, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.albums != nil {
		assets, ok := f.albums[id]
		return assets, ok
	}
	if id != testAPIAlbum {
		return nil, false
	}
	return append([]fakeAsset(nil), f.assets...), true
}

func (f *fakeAPIImmich) albumETag(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.etags != nil {
		return f.etags[id]
	}
	if id != testAPIAlbum {
		return ""
	}
	return f.etag
}

// thumbnailAssetID matches /api/assets/{id}/thumbnail.
func thumbnailAssetID(path string) (string, bool) {
	if !strings.HasPrefix(path, "/api/assets/") || !strings.HasSuffix(path, "/thumbnail") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/assets/"), "/thumbnail")
	return id, id != ""
}

func newAPIClient(t *testing.T, srv *httptest.Server) *immich.APIClient {
	t.Helper()
	return newAPIClientFor(t, srv, testAPIAlbum)
}

func newAPIClientFor(t *testing.T, srv *httptest.Server, albums ...string) *immich.APIClient {
	t.Helper()
	c, err := immich.NewAPIClient(immich.APIConfig{
		BaseURL:  srv.URL,
		APIKey:   testAPIKey,
		AlbumIDs: albums,
		HTTP:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewAPIClientConfigErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  immich.APIConfig
	}{
		{"empty base", immich.APIConfig{APIKey: "k", AlbumIDs: []string{"a"}}},
		{"bad scheme", immich.APIConfig{BaseURL: "ftp://host", APIKey: "k", AlbumIDs: []string{"a"}}},
		{"missing host", immich.APIConfig{BaseURL: "https://", APIKey: "k", AlbumIDs: []string{"a"}}},
		{"empty key", immich.APIConfig{BaseURL: testAPIBase, AlbumIDs: []string{"a"}}},
		{"no albums", immich.APIConfig{BaseURL: testAPIBase, APIKey: "k"}},
		{"empty album id", immich.APIConfig{BaseURL: testAPIBase, APIKey: "k", AlbumIDs: []string{""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := immich.NewAPIClient(tc.cfg); err == nil {
				t.Error("expected error")
			}
		})
	}
}

// A trailing slash on the base URL must not produce double-slash paths.
func TestNewAPIClientTrimsBaseSlash(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.assets = []fakeAsset{{id: assetA, isImage: true, thumbhash: "ha"}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c, err := immich.NewAPIClient(immich.APIConfig{
		BaseURL:  srv.URL + "/",
		APIKey:   testAPIKey,
		AlbumIDs: []string{testAPIAlbum},
		HTTP:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, r := range fake.requests {
		if strings.Contains(r.path, "//") {
			t.Errorf("%s: double slash in path", r.path)
		}
	}
}

func TestAPIClientListReturnsImageAssetsOnly(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.assets = []fakeAsset{
		{id: assetA, isImage: true, thumbhash: "ha"},
		{id: "video", isImage: false, thumbhash: "hv"},
		{id: assetB, isImage: true, thumbhash: "hb"},
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClient(t, srv)

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d assets, want 2 (video filtered)", len(got))
	}
	if got[0].ID != assetA || got[1].ID != assetB {
		t.Errorf("ids: %v", got)
	}
	if got[0].Version == got[1].Version {
		t.Error("distinct thumbhashes should yield distinct tokens")
	}
}

// Every request carries the key in the header; a wrong key fails the List.
func TestAPIClientAuthenticatesWithHeader(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.assets = []fakeAsset{{id: assetA, isImage: true, thumbhash: "ha"}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClient(t, srv)

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) == 0 {
		t.Fatal("no requests recorded")
	}
	for _, r := range fake.requests {
		if r.key != testAPIKey {
			t.Errorf("%s: x-api-key=%q, want %q", r.path, r.key, testAPIKey)
		}
	}

	wrong, err := immich.NewAPIClient(immich.APIConfig{
		BaseURL:  srv.URL,
		APIKey:   "wrong",
		AlbumIDs: []string{testAPIAlbum},
		HTTP:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.List(context.Background()); err == nil {
		t.Error("expected error on 401 with a wrong key")
	}
}

func TestAPIClientReusesCacheOn304(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.assets = []fakeAsset{{id: assetA, isImage: true, thumbhash: "ha"}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClient(t, srv)

	for range 2 {
		if _, err := c.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	buckets := 0
	for _, r := range fake.requests {
		if r.path == "/api/timeline/bucket" {
			buckets++
		}
	}
	if buckets != 1 {
		t.Errorf("timeline/bucket fetched %d times, want 1 (second List served from 304)", buckets)
	}
}

func TestAPIClientListPropagatesHTTPError(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.status = http.StatusNotFound
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClient(t, srv)

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error on 404")
	}
}

func TestAPIClientFetchReturnsPreviewBody(t *testing.T) {
	fake := newFakeAPIImmich()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClient(t, srv)

	body, err := c.Fetch(context.Background(), assetA)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "PREVIEW-BYTES" {
		t.Errorf("body: %q", got)
	}
	var thumbReq *recordedAPIRequest
	for i := range fake.requests {
		if strings.HasSuffix(fake.requests[i].path, "/thumbnail") {
			thumbReq = &fake.requests[i]
		}
	}
	if thumbReq == nil {
		t.Fatal("no thumbnail request recorded")
	}
	if thumbReq.size != "preview" {
		t.Errorf("size=%q, want preview", thumbReq.size)
	}
	if thumbReq.key != testAPIKey {
		t.Errorf("x-api-key=%q, want %q", thumbReq.key, testAPIKey)
	}
}

func TestAPIClientFetchPropagatesHTTPError(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.status = http.StatusForbidden
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClient(t, srv)

	if _, err := c.Fetch(context.Background(), assetA); err == nil {
		t.Error("expected error on 403")
	}
}

func bucketCalls(fake *fakeAPIImmich) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	n := 0
	for _, r := range fake.requests {
		if r.path == "/api/timeline/bucket" {
			n++
		}
	}
	return n
}

// Two albums merge in config order; a photo in both downloads once.
func TestAPIClientMergesMultipleAlbums(t *testing.T) {
	fake := newFakeAPIImmich()
	shared := fakeAsset{id: "shared-id", isImage: true, thumbhash: "hs"}
	fake.albums = map[string][]fakeAsset{
		testAPIAlbum:  {{id: assetA, isImage: true, thumbhash: "ha"}, shared},
		testAPIAlbumB: {shared, {id: assetB, isImage: true, thumbhash: "hb"}},
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClientFor(t, srv, testAPIAlbum, testAPIAlbumB)

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{assetA, shared.id, assetB}
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("asset[%d] = %s, want %s (full: %v)", i, got[i].ID, id, got)
		}
	}
}

// An unchanged album serves from its 304 while a changed one re-enumerates.
func TestAPIClientReusesCachePerAlbum(t *testing.T) {
	fake := newFakeAPIImmich()
	fake.albums = map[string][]fakeAsset{
		testAPIAlbum:  {{id: assetA, isImage: true, thumbhash: "ha"}},
		testAPIAlbumB: {{id: assetB, isImage: true, thumbhash: "hb"}},
	}
	fake.etags = map[string]string{testAPIAlbum: `"e1"`, testAPIAlbumB: `"e2"`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClientFor(t, srv, testAPIAlbum, testAPIAlbumB)

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := bucketCalls(fake); n != 2 {
		t.Fatalf("bucket calls = %d after two Lists, want 2 (both 304 on second)", n)
	}

	// Only album B changes: only B re-enumerates.
	fake.mu.Lock()
	fake.etags[testAPIAlbumB] = `"e3"`
	fake.mu.Unlock()
	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("assets = %v, want both albums' assets", got)
	}
	if n := bucketCalls(fake); n != 3 {
		t.Errorf("bucket calls = %d, want 3 (only B re-enumerated)", n)
	}
}

// A failing album fails the whole List; the syncer keeps its cache.
func TestAPIClientUnknownAlbumErrors(t *testing.T) {
	fake := newFakeAPIImmich()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newAPIClientFor(t, srv, testAPIAlbum, "00000000-0000-4000-8000-000000000000")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error for an unknown album")
	}
}
