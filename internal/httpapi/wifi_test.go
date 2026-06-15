package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
	"github.com/MateEke/picture-frame/internal/wifi"
)

type fakeWiFiMgr struct {
	wifiState     wifi.WiFiState
	networks      []wifi.WiFiNetwork
	scanErr       error
	connectErr    error
	forgetErr     error
	configErr     error
	forgotSSID    string // records the ssid passed to Forget
	connectHidden bool   // records the hidden flag passed to Connect
}

func (f *fakeWiFiMgr) Status() wifi.WiFiState { return f.wifiState }
func (f *fakeWiFiMgr) Scan(_ context.Context) ([]wifi.WiFiNetwork, error) {
	return f.networks, f.scanErr
}
func (f *fakeWiFiMgr) Connect(_ context.Context, _, _ string, hidden bool) error {
	f.connectHidden = hidden
	return f.connectErr
}
func (f *fakeWiFiMgr) Forget(_ context.Context, ssid string) error {
	f.forgotSSID = ssid
	return f.forgetErr
}
func (f *fakeWiFiMgr) Configure(_ context.Context, _ bool, _ string, _ *string) error {
	return f.configErr
}

func makeWiFiServer(mgr httpapi.WiFiManager) http.Handler {
	return httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		WiFi:        mgr,
		Production:  false,
	})
}

func TestWiFiStatusNilManager(t *testing.T) {
	srv := makeWiFiServer(nil)
	r := httptest.NewRequest(http.MethodGet, "/api/wifi/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestWiFiStatusOK(t *testing.T) {
	mgr := &fakeWiFiMgr{
		wifiState: wifi.WiFiState{Mode: wifi.ModeConnected, SSID: "HomeNet", APEnabled: true},
	}
	srv := makeWiFiServer(mgr)
	r := httptest.NewRequest(http.MethodGet, "/api/wifi/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	var s wifi.WiFiState
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Mode != wifi.ModeConnected || s.SSID != "HomeNet" {
		t.Errorf("unexpected state: %+v", s)
	}
}

func TestWiFiNetworksScanError(t *testing.T) {
	mgr := &fakeWiFiMgr{scanErr: errors.New("hw error")}
	srv := makeWiFiServer(mgr)
	r := httptest.NewRequest(http.MethodGet, "/api/wifi/networks", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", w.Code)
	}
}

func TestWiFiNetworksOK(t *testing.T) {
	mgr := &fakeWiFiMgr{
		networks: []wifi.WiFiNetwork{
			{SSID: "A", Signal: 80, Security: "WPA2", Known: true},
			{SSID: "B", Signal: 40, Security: "WPA3", Known: false},
		},
	}
	srv := makeWiFiServer(mgr)
	r := httptest.NewRequest(http.MethodGet, "/api/wifi/networks", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var nets []wifi.WiFiNetwork
	if err := json.NewDecoder(w.Body).Decode(&nets); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(nets) != 2 || nets[0].SSID != "A" {
		t.Errorf("unexpected networks: %+v", nets)
	}
}

func TestWiFiConnectAccepted(t *testing.T) {
	srv := makeWiFiServer(&fakeWiFiMgr{})
	body := `{"ssid":"HomeNet","password":"secret"}`
	r := httptest.NewRequest(http.MethodPost, "/api/wifi/connect", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusAccepted {
		t.Errorf("got %d, want 202", w.Code)
	}
}

func TestWiFiConnectBusy(t *testing.T) {
	mgr := &fakeWiFiMgr{connectErr: wifi.ErrBusy}
	srv := makeWiFiServer(mgr)
	body := `{"ssid":"HomeNet"}`
	r := httptest.NewRequest(http.MethodPost, "/api/wifi/connect", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestWiFiConnectMissingSSID(t *testing.T) {
	srv := makeWiFiServer(&fakeWiFiMgr{})
	body := `{"password":"x"}`
	r := httptest.NewRequest(http.MethodPost, "/api/wifi/connect", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("got %d, want 422", w.Code)
	}
}

func TestWiFiConnectForwardsHidden(t *testing.T) {
	mgr := &fakeWiFiMgr{}
	srv := makeWiFiServer(mgr)
	body := `{"ssid":"SecretNet","password":"x","hidden":true}`
	r := httptest.NewRequest(http.MethodPost, "/api/wifi/connect", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", w.Code)
	}
	if !mgr.connectHidden {
		t.Error("hidden flag was not forwarded to the manager")
	}
}

func TestWiFiForgetNoContent(t *testing.T) {
	mgr := &fakeWiFiMgr{}
	srv := makeWiFiServer(mgr)
	r := httptest.NewRequest(http.MethodDelete, "/api/wifi/network/OldNet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("got %d, want 204", w.Code)
	}
	if mgr.forgotSSID != "OldNet" {
		t.Errorf("forgot ssid: got %q, want OldNet", mgr.forgotSSID)
	}
}

func TestWiFiForgetDecodesEscapedSSID(t *testing.T) {
	// An SSID containing '/' is percent-encoded by the client as %2F; chi keeps it
	// raw in the param, so the handler must url.PathUnescape it before nmcli.
	mgr := &fakeWiFiMgr{}
	srv := makeWiFiServer(mgr)
	r := httptest.NewRequest(http.MethodDelete, "/api/wifi/network/Guest%2FIoT", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", w.Code)
	}
	if mgr.forgotSSID != "Guest/IoT" {
		t.Errorf("forgot ssid: got %q, want Guest/IoT (decoded)", mgr.forgotSSID)
	}
}

func TestWiFiAPConfigureOK(t *testing.T) {
	mgr := &fakeWiFiMgr{wifiState: wifi.WiFiState{APEnabled: true, SSID: "PF"}}
	srv := makeWiFiServer(mgr)
	body := `{"enabled":true,"ssid":"PF","password":""}`
	r := httptest.NewRequest(http.MethodPut, "/api/wifi/ap", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestCaptiveProbeAPMode(t *testing.T) {
	mgr := &fakeWiFiMgr{wifiState: wifi.WiFiState{Mode: wifi.ModeAP}}
	srv := makeWiFiServer(mgr)
	for _, path := range []string{"/generate_204", "/hotspot-detect.html", "/ncsi.txt", "/success.txt"} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("%s: got %d, want 200", path, w.Code)
			}
			if !strings.Contains(w.Body.String(), "/admin/wifi") {
				t.Errorf("%s: body should redirect to /admin/wifi, got: %s", path, w.Body.String())
			}
		})
	}
}

func TestWiFiConnectRejectsOversizedBody(t *testing.T) {
	srv := makeWiFiServer(&fakeWiFiMgr{})
	// MaxBodyBytes is 4096 for wifi-connect; exceed it.
	body := `{"ssid":"` + strings.Repeat("a", 8192) + `"}`
	r := httptest.NewRequest(http.MethodPost, "/api/wifi/connect", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413 for oversized body", w.Code)
	}
}

func TestCaptiveProbeNormalMode(t *testing.T) {
	mgr := &fakeWiFiMgr{wifiState: wifi.WiFiState{Mode: wifi.ModeConnected}}
	srv := makeWiFiServer(mgr)

	cases := []struct {
		path string
		code int
	}{
		{"/generate_204", http.StatusNoContent},
		{"/hotspot-detect.html", http.StatusOK},
		{"/ncsi.txt", http.StatusOK},
		{"/success.txt", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Errorf("%s: got %d, want %d", tc.path, w.Code, tc.code)
			}
			// In normal mode, body must NOT redirect to /admin/wifi.
			if strings.Contains(w.Body.String(), "/admin/wifi") {
				t.Errorf("%s: should not contain /admin/wifi redirect in normal mode", tc.path)
			}
		})
	}
}
