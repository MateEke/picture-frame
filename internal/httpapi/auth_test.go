package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/auth"
	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

const sessionCookie = "pf_session"

func hashFor(t *testing.T, pw string) string {
	t.Helper()
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

// authServer builds a server whose saved/running config carry the given auth
func authServer(t *testing.T, hash string, store *config.Store) http.Handler {
	t.Helper()
	cfg := config.Config{Auth: config.AuthConfig{PasswordHash: hash}}
	if store == nil {
		store = config.NewStore(cfg, filepath.Join(t.TempDir(), "overrides.toml"))
	}
	return httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         store,
		RunningConfig: cfg,
	})
}

// addSession attaches a raw session-cookie header (avoids constructing an
// http.Cookie literal, which gosec G124 flags for the missing Secure flag).
func addSession(req *http.Request, value string) {
	req.Header.Set("Cookie", sessionCookie+"="+value)
}

// validToken forges a session token for hash, as the server's own Authenticator
// (default TTL) would.
func validToken(hash string) string {
	return auth.New().Issue(hash)
}

func sessionFromResponse(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func TestGatedRouteOpenWhenNoPassword(t *testing.T) {
	srv := authServer(t, "", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/screen", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("opt-in off should pass through: got %d", rec.Code)
	}
}

func TestGatedRouteRejectsWithoutCookie(t *testing.T) {
	srv := authServer(t, hashFor(t, "pw"), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/screen", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestGatedRouteAcceptsValidCookie(t *testing.T) {
	hash := hashFor(t, "pw")
	srv := authServer(t, hash, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/screen", nil)
	addSession(req, validToken(hash))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid cookie should pass: got %d", rec.Code)
	}
}

func TestGatedRouteRejectsInvalidCookie(t *testing.T) {
	srv := authServer(t, hashFor(t, "pw"), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/screen", nil)
	addSession(req, "garbage")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestAuthRoutesAndHealthNotGated(t *testing.T) {
	srv := authServer(t, hashFor(t, "pw"), nil)
	for _, path := range []string{"/healthz", "/api/auth/status"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("%s must not be gated, got 401", path)
		}
	}
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	hash := hashFor(t, "pw")
	srv := authServer(t, hash, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"pw"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	c := sessionFromResponse(rec)
	if c == nil || !auth.New().Verify(hash, c.Value) {
		t.Fatal("login did not set a valid session cookie")
	}
	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	srv := authServer(t, hashFor(t, "pw"), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	if sessionFromResponse(rec) != nil {
		t.Error("no cookie should be set on failed login")
	}
}

func TestLoginWhenNoPasswordSet(t *testing.T) {
	srv := authServer(t, "", nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with no password configured should 401, got %d", rec.Code)
	}
}

func TestStatusReportsState(t *testing.T) {
	// no password
	srv := authServer(t, "", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	if body := rec.Body.String(); !strings.Contains(body, `"required":false`) || !strings.Contains(body, `"authenticated":false`) {
		t.Fatalf("no-password status: %s", body)
	}

	// password set, no cookie
	hash := hashFor(t, "pw")
	srv = authServer(t, hash, nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	if body := rec.Body.String(); !strings.Contains(body, `"required":true`) || !strings.Contains(body, `"authenticated":false`) {
		t.Fatalf("password-no-cookie status: %s", body)
	}

	// password set, valid cookie
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	addSession(req, validToken(hash))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if body := rec.Body.String(); !strings.Contains(body, `"required":true`) || !strings.Contains(body, `"authenticated":true`) {
		t.Fatalf("password-cookie status: %s", body)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	srv := authServer(t, hashFor(t, "pw"), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	c := sessionFromResponse(rec)
	if c == nil {
		t.Fatal("logout should set a session cookie")
	}
	// Delete = empty value AND negative MaxAge (not a lingering empty cookie).
	if c.Value != "" || c.MaxAge >= 0 {
		t.Fatalf("logout should delete the cookie, got %+v", c)
	}
}

func TestSetPasswordInitial(t *testing.T) {
	dir := t.TempDir()
	overrides := filepath.Join(dir, "overrides.toml")
	srv := authServer(t, "", config.NewStore(config.Config{}, overrides))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString(`{"new":"newpw"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	if sessionFromResponse(rec) == nil {
		t.Error("setting the first password should log the caller in")
	}
	data, err := os.ReadFile(overrides)
	if err != nil {
		t.Fatalf("overrides not written: %v", err)
	}
	if !strings.Contains(string(data), "password_hash") {
		t.Errorf("overrides missing password_hash: %s", data)
	}

	// the gate is now active for a fresh request with no cookie
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/screen", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("gate should be active after setting a password, got %d", rec.Code)
	}
}

func TestChangePasswordRequiresCurrent(t *testing.T) {
	dir := t.TempDir()
	overrides := filepath.Join(dir, "overrides.toml")
	srv := authServer(t, hashFor(t, "old"), config.NewStore(config.Config{Auth: config.AuthConfig{PasswordHash: hashFor(t, "old")}}, overrides))

	post := func(body string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(`{"current":"wrong","new":"new2"}`); code != http.StatusForbidden {
		t.Fatalf("wrong current should 403, got %d", code)
	}
	if code := post(`{"current":"old","new":"new2"}`); code != http.StatusNoContent {
		t.Fatalf("correct current should 204, got %d", code)
	}

	// old password no longer logs in; new one does
	login := func(pw string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"`+pw+`"}`))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(rec, req)
		return rec.Code
	}
	if login("old") != http.StatusUnauthorized {
		t.Error("old password should be rejected after change")
	}
	if login("new2") != http.StatusNoContent {
		t.Error("new password should be accepted after change")
	}
}

func TestDisablePassword(t *testing.T) {
	dir := t.TempDir()
	overrides := filepath.Join(dir, "overrides.toml")
	srv := authServer(t, hashFor(t, "old"), config.NewStore(config.Config{Auth: config.AuthConfig{PasswordHash: hashFor(t, "old")}}, overrides))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString(`{"current":"old","new":""}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("disable should 204, got %d", rec.Code)
	}
	if c := sessionFromResponse(rec); c == nil || c.Value != "" || c.MaxAge >= 0 {
		t.Fatalf("disabling the password should delete the session cookie, got %+v", c)
	}

	// gate is off again
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/screen", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("gate should be off after disable, got %d", rec.Code)
	}
}

func TestSetPasswordTooLong(t *testing.T) {
	dir := t.TempDir()
	overrides := filepath.Join(dir, "overrides.toml")
	srv := authServer(t, "", config.NewStore(config.Config{}, overrides))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		bytes.NewBufferString(`{"new":"`+strings.Repeat("x", 73)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("overlong password should 422, got %d", rec.Code)
	}
}

func TestSetPasswordSaveError(t *testing.T) {
	// Pointing the overrides path at a directory makes the write fail.
	srv := authServer(t, "", config.NewStore(config.Config{}, t.TempDir()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString(`{"new":"pw"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("save failure should 500, got %d", rec.Code)
	}
}

func TestChangePasswordNotRestartPending(t *testing.T) {
	dir := t.TempDir()
	overrides := filepath.Join(dir, "overrides.toml")
	srv := authServer(t, "", config.NewStore(config.Config{}, overrides))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString(`{"new":"pw"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)

	// Reuse the cookie the server just issued (the bcrypt hash is unforgeable
	// externally) to reach the now-gated GET /api/config.
	session := sessionFromResponse(rec)
	if session == nil {
		t.Fatal("set-password should have issued a session cookie")
		return
	}
	cfgReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	addSession(cfgReq, session.Value)
	out := httptest.NewRecorder()
	srv.ServeHTTP(out, cfgReq)
	if out.Code != http.StatusOK {
		t.Fatalf("config get: %d", out.Code)
	}
	if !strings.Contains(out.Body.String(), `"restart_pending":false`) {
		t.Errorf("password change should not be restart-pending: %s", out.Body.String())
	}
}

// mediaServer is like authServer but with an images root, so /img/{name} can be
// reached (returns 404 for a missing file once it passes the gate).
func mediaServer(t *testing.T, hash string) http.Handler {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	cfg := config.Config{Auth: config.AuthConfig{PasswordHash: hash}}
	return httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		ImagesRoot:    root,
		Store:         config.NewStore(cfg, filepath.Join(t.TempDir(), "overrides.toml")),
		RunningConfig: cfg,
	})
}

func imgCode(t *testing.T, srv http.Handler, remoteAddr, token string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/img/missing.jpg", nil)
	req.RemoteAddr = remoteAddr
	if token != "" {
		addSession(req, token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Code
}

func TestKioskMediaGating(t *testing.T) {
	hash := hashFor(t, "pw")
	srv := mediaServer(t, hash)

	cases := []struct {
		name       string
		remoteAddr string
		token      string
		blocked    bool
	}{
		{"remote without cookie is blocked", "192.168.1.5:5000", "", true},
		{"loopback bypasses auth", "127.0.0.1:5000", "", false},
		{"loopback without a port still parses", "127.0.0.1", "", false},
		{"ipv6 loopback bypasses auth", "[::1]:5000", "", false},
		{"remote with a valid cookie passes", "192.168.1.5:5000", validToken(hash), false},
		{"non-IP remote is blocked", "weird-host:5000", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := imgCode(t, srv, tc.remoteAddr, tc.token)
			if tc.blocked && code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401 (blocked)", code)
			}
			if !tc.blocked && code == http.StatusUnauthorized {
				t.Errorf("got 401, want pass-through")
			}
		})
	}
}

func heartbeatCode(t *testing.T, srv http.Handler, remoteAddr, token string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/heartbeat", nil)
	req.RemoteAddr = remoteAddr
	if token != "" {
		addSession(req, token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Code
}

// The on-device kiosk heartbeats without a cookie; a configured password must
// not lock it out (401 would bounce the frame to the login page and starve
// kioskwatch).
func TestKioskHeartbeatGating(t *testing.T) {
	hash := hashFor(t, "pw")
	srv := mediaServer(t, hash)

	cases := []struct {
		name       string
		remoteAddr string
		token      string
		blocked    bool
	}{
		{"loopback without cookie passes", "127.0.0.1:5000", "", false},
		{"remote without cookie is blocked", "192.168.1.5:5000", "", true},
		{"remote with a valid cookie passes", "192.168.1.5:5000", validToken(hash), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := heartbeatCode(t, srv, tc.remoteAddr, tc.token)
			if tc.blocked && code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401 (blocked)", code)
			}
			if !tc.blocked && code == http.StatusUnauthorized {
				t.Errorf("got 401, want pass-through")
			}
		})
	}
}

func TestKioskMediaOpenWhenNoPassword(t *testing.T) {
	srv := mediaServer(t, "")
	if code := imgCode(t, srv, "192.168.1.5:5000", ""); code == http.StatusUnauthorized {
		t.Errorf("no password: /img should be open, got 401")
	}
}

func TestEventsRemoteRequiresAuth(t *testing.T) {
	srv := mediaServer(t, hashFor(t, "pw"))
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	req.RemoteAddr = "192.168.1.5:5000"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("remote no-cookie /events: got %d, want 401", rec.Code)
	}
}

// The kiosk's SSE connection must survive a configured password; this is the
// guard against an /events exemption silently going missing.
func TestEventsLoopbackBypassesAuth(t *testing.T) {
	srv := mediaServer(t, hashFor(t, "pw"))
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Error("loopback no-cookie /events: got 401, want pass-through")
	}
}

// The 4KB body cap on the auth endpoints must hold; they are reachable
// without a session.
func TestAuthBodyLimitRejectsOversized(t *testing.T) {
	srv := authServer(t, hashFor(t, "pw"), nil)
	big := `{"password":"` + strings.Repeat("x", 5000) + `"}`
	for _, path := range []string{"/api/auth/login", "/api/auth/password"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(big))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("%s: got %d, want 413", path, rec.Code)
		}
	}
}
