package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// fakeImmich serves GET /api/albums with a fixed body; the header must carry the key.
func fakeImmich(t *testing.T, albumsJSON string, requireKey string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requireKey != "" && r.Header.Get("x-api-key") != requireKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api/albums" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(albumsJSON))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func immichConfig(t *testing.T, url, key string, albumIDs ...string) config.Config {
	t.Helper()
	cfg := defaultSaved()
	cfg.Library.Backend = config.BackendImmich
	cfg.Immich = config.ImmichConfig{
		URL:          url,
		APIKey:       key,
		AlbumIDs:     albumIDs,
		SyncInterval: config.Duration{Duration: 15 * time.Minute},
	}
	return cfg
}

func albumsServer(t *testing.T, saved config.Config) http.Handler {
	t.Helper()
	return httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, t.TempDir()+"/overrides.toml"),
		RunningConfig: saved,
	})
}

func getAlbums(t *testing.T, srv http.Handler) (int, []httpapi.ImmichAlbum, string) {
	t.Helper()
	return postAlbums(t, srv, "", "")
}

// postAlbums calls the picker endpoint; a blank field falls back to the saved
// config, a non-blank one overrides it (the unsaved-first-use path).
func postAlbums(t *testing.T, srv http.Handler, url, key string) (int, []httpapi.ImmichAlbum, string) {
	t.Helper()
	body, err := json.Marshal(httpapi.ImmichAlbumsRequest{URL: url, APIKey: key})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/immich/albums", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec.Code, nil, rec.Body.String()
	}
	var albums []httpapi.ImmichAlbum
	if err := json.Unmarshal(rec.Body.Bytes(), &albums); err != nil {
		t.Fatalf("decode: %v; body: %s", err, rec.Body)
	}
	return rec.Code, albums, ""
}

const albumsJSON = `[{"id":"a1","albumName":"Kitchen","assetCount":12},{"id":"a2","albumName":"Garden","assetCount":3}]`

func TestListImmichAlbumsListsFromKey(t *testing.T) {
	immich := fakeImmich(t, albumsJSON, "secret-key")
	srv := albumsServer(t, immichConfig(t, immich.URL, "secret-key", "a1"))

	code, albums, body := getAlbums(t, srv)
	if code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", code, body)
	}
	if len(albums) != 2 {
		t.Fatalf("albums: got %d, want 2", len(albums))
	}
	if albums[0].ID != "a1" || albums[0].Name != "Kitchen" || albums[0].AssetCount != 12 {
		t.Errorf("first album: got %+v", albums[0])
	}
	if albums[1].ID != "a2" || albums[1].Name != "Garden" {
		t.Errorf("second album: got %+v", albums[1])
	}
}

func TestListImmichAlbumsRequiresConfiguredConnection(t *testing.T) {
	// No url/key configured: the picker has nothing to authenticate with.
	srv := albumsServer(t, immichConfig(t, "", ""))

	if code, _, _ := getAlbums(t, srv); code != http.StatusConflict {
		t.Errorf("no connection: got %d, want 409", code)
	}
}

// A wrong key must not read as a generic gateway failure; the admin needs to
// know the key is the problem.
func TestListImmichAlbumsSurfacesUnauthorizedAs401(t *testing.T) {
	immich := fakeImmich(t, albumsJSON, "the-right-key")
	srv := albumsServer(t, immichConfig(t, immich.URL, "wrong-key", "a1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/immich/albums", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401; body: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "API key") {
		t.Errorf("body should name the API key as the cause: %s", rec.Body)
	}
}

// The API key must never reach the wire in the query string.
func TestListImmichAlbumsKeepsStoredKeyOutOfURL(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.String()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	frame := albumsServer(t, immichConfig(t, srv.URL, "secret-key", "a1"))
	if code, _, _ := getAlbums(t, frame); code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", code)
	}
	if strings.Contains(seen, "secret-key") {
		t.Errorf("API key leaked into URL: %s", seen)
	}
}

// The picker writes album_ids through the normal config PUT; a stored key must
// survive the round-trip when the field is left blank (write-only secret).
func TestPutConfigPersistsImmichAlbumSelection(t *testing.T) {
	immich := fakeImmich(t, albumsJSON, "secret-key")
	saved := immichConfig(t, immich.URL, "secret-key")
	srv, _, _ := makeConfigServer(t, saved)

	body := putBody(t, saved, func(dto *map[string]any) {
		api := (*dto)["library"].(map[string]any)["immich_api_key"].(map[string]any)
		if api["api_key"] != nil && api["api_key"] != "" {
			t.Error("immich api_key must not be returned by GET")
		}
		api["album_ids"] = []string{"a1", "a2"}
		api["sync_interval"] = "30m"
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config: got %d, want 200; body: %s", rec.Code, rec.Body)
	}

	after := httptest.NewRecorder()
	srv.ServeHTTP(after, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var got struct {
		Library struct {
			ApiKey struct {
				APIKeySet bool     `json:"api_key_set"`
				AlbumIDs  []string `json:"album_ids"`
			} `json:"immich_api_key"`
		} `json:"library"`
	}
	if err := json.Unmarshal(after.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Library.ApiKey.APIKeySet {
		t.Error("stored API key was cleared by a PUT that left the field blank")
	}
	if len(got.Library.ApiKey.AlbumIDs) != 2 || got.Library.ApiKey.AlbumIDs[0] != "a1" {
		t.Errorf("album_ids: got %v, want [a1 a2]", got.Library.ApiKey.AlbumIDs)
	}
}

// Album selection is not Tier-1: it changes the syncer, so it needs a restart.
func TestPutConfigAlbumSelectionNeedsRestart(t *testing.T) {
	immich := fakeImmich(t, albumsJSON, "secret-key")
	saved := immichConfig(t, immich.URL, "secret-key")
	srv, _, _ := makeConfigServer(t, saved)

	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["library"].(map[string]any)["immich_api_key"].(map[string]any)["album_ids"] = []string{"a1"}
	})

	if rec := doJSONPut(srv, "/api/config", body); rec.Code != http.StatusOK {
		t.Fatalf("PUT: got %d, want 200; body: %s", rec.Code, rec.Body)
	}
	after := httptest.NewRecorder()
	srv.ServeHTTP(after, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var got struct {
		RestartPending bool `json:"restart_pending"`
	}
	if err := json.Unmarshal(after.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.RestartPending {
		t.Error("changing album_ids must flag restart_pending")
	}
}

// Saving api-key mode requires an album selection, so the picker must work with
// credentials the user typed but has not saved yet — otherwise the album list is
// unreachable on a fresh config.
func TestListImmichAlbumsUsesUnsavedCredentials(t *testing.T) {
	immich := fakeImmich(t, albumsJSON, "typed-key")
	// Nothing configured in the saved config at all.
	srv := albumsServer(t, immichConfig(t, "", ""))

	if code, _, _ := postAlbums(t, srv, "", ""); code != http.StatusConflict {
		t.Fatalf("no credentials: got %d, want 409", code)
	}

	code, albums, body := postAlbums(t, srv, immich.URL+"/", "typed-key")
	if code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", code, body)
	}
	if len(albums) != 2 {
		t.Errorf("albums: got %d, want 2", len(albums))
	}
}

// A key typed into the body must not leak into the URL the frame calls.
func TestListImmichAlbumsKeepsTypedKeyOutOfURL(t *testing.T) {
	var seen string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.String()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer fake.Close()

	srv := albumsServer(t, immichConfig(t, "", ""))
	if code, _, _ := postAlbums(t, srv, fake.URL, "typed-key"); code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", code)
	}
	if strings.Contains(seen, "typed-key") {
		t.Errorf("API key leaked into URL: %s", seen)
	}
}
