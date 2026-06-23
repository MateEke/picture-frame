package immich_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/MateEke/picture-frame/internal/library/adapter/immich"
)

const (
	testKey      = "test-key-123"
	testPassword = "Almafa123"
	testAlbumID  = "1e4bb746-6072-4aec-9a20-a37b130cde4b"
	assetA       = "9b7b87ad-f032-442d-a7c3-046fed72e7bc"
	assetB       = "ab7b87ad-f032-442d-a7c3-046fed72e7bd"
)

type recordedRequest struct {
	method string
	path   string
	key    string
	pw     string
	size   string
	cookie string // immich_shared_link_token value, if sent
}

type fakeAsset struct {
	id        string
	isImage   bool
	thumbhash string
}

type fakeImmich struct {
	mu       sync.Mutex
	requests []recordedRequest
	album    string      // /shared-links/me response
	assets   []fakeAsset // timeline contents
	etag     string      // album ETag; "" disables conditional caching
	preview  []byte
	status   int
	// loginStatus drives POST /api/shared-links/login: 0 mimics a pre-2.6 server
	// (404, no such endpoint); 201 issues loginToken as the token cookie.
	loginStatus int
	loginToken  string
}

func newFakeImmich(t *testing.T) *fakeImmich {
	t.Helper()
	return &fakeImmich{
		album:   `{"album":{"id":"` + testAlbumID + `"}}`,
		etag:    `"etag-v1"`,
		preview: []byte("PREVIEW-BYTES"),
		status:  http.StatusOK,
	}
}

func (f *fakeImmich) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cookie := ""
	if ck, err := r.Cookie("immich_shared_link_token"); err == nil {
		cookie = ck.Value
	}
	f.requests = append(f.requests, recordedRequest{
		method: r.Method,
		path:   r.URL.Path,
		key:    r.URL.Query().Get("key"),
		pw:     r.URL.Query().Get("password"),
		size:   r.URL.Query().Get("size"),
		cookie: cookie,
	})
}

func (f *fakeImmich) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		f.mu.Lock()
		status, loginStatus, loginToken, etag := f.status, f.loginStatus, f.loginToken, f.etag
		f.mu.Unlock()
		if r.URL.Path == "/api/shared-links/login" {
			f.serveLogin(w, r, loginStatus, loginToken)
			return
		}
		if status != http.StatusOK {
			http.Error(w, "boom", status)
			return
		}
		switch {
		case r.URL.Path == "/api/shared-links/me":
			writeJSON(w, f.album)
		case r.URL.Path == "/api/albums/"+testAlbumID:
			serveGate(w, r, etag)
		case r.URL.Path == "/api/timeline/buckets":
			f.serveBuckets(w)
		case r.URL.Path == "/api/timeline/bucket":
			f.serveBucket(w)
		case strings.HasPrefix(r.URL.Path, "/api/assets/") && strings.HasSuffix(r.URL.Path, "/thumbnail"):
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(f.preview)
		default:
			http.NotFound(w, r)
		}
	})
}

func (f *fakeImmich) serveLogin(w http.ResponseWriter, r *http.Request, loginStatus int, loginToken string) {
	switch loginStatus {
	case 0:
		http.NotFound(w, r)
	case http.StatusOK, http.StatusCreated:
		http.SetCookie(w, &http.Cookie{Name: "immich_shared_link_token", Value: loginToken, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(loginStatus)
	default:
		http.Error(w, "denied", loginStatus)
	}
}

func serveGate(w http.ResponseWriter, r *http.Request, etag string) {
	if etag != "" {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
	}
	writeJSON(w, `{"id":"`+testAlbumID+`"}`)
}

func (f *fakeImmich) serveBuckets(w http.ResponseWriter) {
	f.mu.Lock()
	n := len(f.assets)
	f.mu.Unlock()
	if n == 0 {
		writeJSON(w, `[]`)
		return
	}
	writeJSON(w, `[{"timeBucket":"2026-05-01","count":`+strconv.Itoa(n)+`}]`)
}

func (f *fakeImmich) serveBucket(w http.ResponseWriter) {
	f.mu.Lock()
	assets := append([]fakeAsset(nil), f.assets...)
	f.mu.Unlock()
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
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func newClient(t *testing.T, srv *httptest.Server, password string) *immich.Client {
	t.Helper()
	c, err := immich.New(immich.Config{
		ShareURL: srv.URL + "/share/" + testKey,
		Password: password,
		HTTP:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseShareURLErrors(t *testing.T) {
	cases := []struct{ name, in string }{
		{"empty key", "https://host/share/"},
		{"bad path", "https://host/album/x"},
		{"missing host", "https:///share/x"},
		{"wrong scheme", "ftp://host/share/x"},
		{"unparseable", "://nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := immich.New(immich.Config{ShareURL: tc.in}); err == nil {
				t.Errorf("expected error for %q", tc.in)
			}
		})
	}
}

// Trailing query/fragment shouldn't corrupt the parsed key.
func TestParseShareURLIgnoresQueryAndFragment(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	cases := []string{
		srv.URL + "/share/" + testKey + "?utm_source=email",
		srv.URL + "/share/" + testKey + "#preview",
		srv.URL + "/share/" + testKey + "?x=1#y",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			fake.requests = nil
			c, err := immich.New(immich.Config{ShareURL: in, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.List(context.Background()); err != nil {
				t.Fatal(err)
			}
			for _, r := range fake.requests {
				if r.key != testKey {
					t.Errorf("key = %q, want %q (input %q)", r.key, testKey, in)
				}
			}
		})
	}
}

func TestListReturnsImageAssetsOnly(t *testing.T) {
	fake := newFakeImmich(t)
	fake.assets = []fakeAsset{
		{id: assetA, isImage: true, thumbhash: "ha"},
		{id: "video", isImage: false, thumbhash: "hv"},
		{id: assetB, isImage: true, thumbhash: "hb"},
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

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
	for _, a := range got {
		if len(a.Version) != 16 {
			t.Errorf("version %q is not 16 hex chars", a.Version)
		}
	}
	if got[0].Version == got[1].Version {
		t.Error("distinct thumbhashes should yield distinct tokens")
	}
}

// A missing thumbhash (not yet generated) maps to the sentinel token.
func TestListTokenizesThumbhash(t *testing.T) {
	fake := newFakeImmich(t)
	fake.assets = []fakeAsset{
		{id: assetA, isImage: true, thumbhash: ""},
		{id: assetB, isImage: true, thumbhash: "hb"},
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Version != "0000000000000000" {
		t.Errorf("empty thumbhash token = %q, want sentinel", got[0].Version)
	}
	if got[1].Version == "0000000000000000" {
		t.Error("non-empty thumbhash should not map to the sentinel")
	}
}

// The adapter logs whether a tick re-enumerated or was served from the 304 cache.
func TestListLogsCacheVsEnumerate(t *testing.T) {
	fake := newFakeImmich(t)
	fake.assets = []fakeAsset{{id: assetA, isImage: true, thumbhash: "ha"}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	var buf bytes.Buffer
	c, err := immich.New(immich.Config{
		ShareURL: srv.URL + "/share/" + testKey,
		HTTP:     srv.Client(),
		Logger:   slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := c.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if out := buf.String(); !strings.Contains(out, "enumerated album") || !strings.Contains(out, "served from cache") {
		t.Errorf("missing enumerate/cache log lines:\n%s", out)
	}
}

// A second List with an unchanged album ETag is served from cache: no second
// timeline enumeration.
func TestListReusesCacheOn304(t *testing.T) {
	fake := newFakeImmich(t)
	fake.assets = []fakeAsset{{id: assetA, isImage: true, thumbhash: "ha"}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

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

// A pre-2.6 server has no login endpoint (404): the client falls back to the
// legacy password query parameter on every request.
func TestLegacyServerFallsBackToPasswordQuery(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, testPassword)

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	logins, others := 0, 0
	for _, r := range fake.requests {
		if r.key != testKey {
			t.Errorf("%s: key=%q, want %q", r.path, r.key, testKey)
		}
		if r.path == "/api/shared-links/login" {
			logins++
			continue
		}
		others++
		if r.pw != testPassword {
			t.Errorf("%s: password=%q, want %q", r.path, r.pw, testPassword)
		}
	}
	if logins != 1 {
		t.Errorf("login probed %d times, want 1", logins)
	}
	if others < 2 {
		t.Fatalf("expected at least 2 data requests, got %d", others)
	}
}

// Immich >= 2.6.0: the password is exchanged once for the token cookie; data
// requests carry the cookie and never the password query parameter.
func TestLoginTokenFlow(t *testing.T) {
	fake := newFakeImmich(t)
	fake.loginStatus = http.StatusCreated
	fake.loginToken = "tok-abc123"
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, testPassword)

	for range 2 {
		if _, err := c.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	logins := 0
	for _, r := range fake.requests {
		if r.path == "/api/shared-links/login" {
			logins++
			if r.method != http.MethodPost {
				t.Errorf("login method=%s, want POST", r.method)
			}
			continue
		}
		if r.pw != "" {
			t.Errorf("%s: password leaked into query: %q", r.path, r.pw)
		}
		if r.cookie != "tok-abc123" {
			t.Errorf("%s: token cookie=%q, want tok-abc123", r.path, r.cookie)
		}
	}
	if logins != 1 {
		t.Errorf("login called %d times across two Lists, want 1 (token cached)", logins)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	fake := newFakeImmich(t)
	fake.loginStatus = http.StatusUnauthorized
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "wrong")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error when the share password is rejected")
	}
}

// A 401 mid-run (share password rotated upstream) clears the token; the retry
// re-runs the login exchange and succeeds with the fresh token.
func TestListRetriesLoginOn401(t *testing.T) {
	fake := newFakeImmich(t)
	fake.loginStatus = http.StatusCreated
	fake.loginToken = "tok-1"
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, testPassword)

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Invalidate the held token: data requests 401 until a re-login happens.
	fake.mu.Lock()
	fake.status = http.StatusUnauthorized
	fake.loginToken = "tok-2"
	fake.mu.Unlock()
	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("expected 401 while the server rejects the stale token")
	}
	fake.mu.Lock()
	fake.status = http.StatusOK
	fake.mu.Unlock()
	if _, err := c.List(context.Background()); err != nil {
		t.Fatalf("expected recovery after re-login, got %v", err)
	}
	last := fake.requests[len(fake.requests)-1]
	if last.cookie != "tok-2" {
		t.Errorf("final request used token %q, want the re-issued tok-2", last.cookie)
	}
}

func TestListOmitsPasswordWhenNotConfigured(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, r := range fake.requests {
		if r.pw != "" {
			t.Errorf("%s: password sent unexpectedly: %q", r.path, r.pw)
		}
	}
}

func TestListCachesAlbumID(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	for range 3 {
		if _, err := c.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	shareLinkCalls := 0
	for _, r := range fake.requests {
		if r.path == "/api/shared-links/me" {
			shareLinkCalls++
		}
	}
	if shareLinkCalls != 1 {
		t.Errorf("shared-links/me called %d times, want 1 (cached)", shareLinkCalls)
	}
}

func TestListPropagatesHTTPError(t *testing.T) {
	fake := newFakeImmich(t)
	fake.status = http.StatusUnauthorized
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error on 401")
	}
}

func TestFetchReturnsPreviewBody(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

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
	var thumbReq *recordedRequest
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
}

func TestFetchPropagatesHTTPError(t *testing.T) {
	fake := newFakeImmich(t)
	fake.status = http.StatusForbidden
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.Fetch(context.Background(), assetA); err == nil {
		t.Error("expected error on 403")
	}
}

func TestListInvalidatesAlbumIDOn404(t *testing.T) {
	fake := newFakeImmich(t)
	var gateCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/shared-links/me":
			writeJSON(w, fake.album)
		case "/api/albums/" + testAlbumID:
			gateCalls++
			if gateCalls == 1 {
				http.Error(w, "gone", http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", `"e"`)
			writeJSON(w, `{}`)
		case "/api/timeline/buckets":
			writeJSON(w, `[{"timeBucket":"2026-05-01","count":1}]`)
		case "/api/timeline/bucket":
			writeJSON(w, `{"id":["`+assetA+`"],"isImage":[true],"thumbhash":["h"]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newClient(t, srv, "")

	// First gate call 404s; the retry succeeds after re-resolving via /me.
	if _, err := c.List(context.Background()); err != nil {
		t.Fatalf("expected retry success, got %v", err)
	}
	if gateCalls != 2 {
		t.Errorf("expected 2 gate calls (404 → re-resolve → retry), got %d", gateCalls)
	}
}

func TestListErrorsOnMissingAlbumID(t *testing.T) {
	fake := newFakeImmich(t)
	fake.album = `{"album":{"id":""}}`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error for share with no album")
	}
}
