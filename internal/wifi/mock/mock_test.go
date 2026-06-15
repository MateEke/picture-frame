package mock

import (
	"context"
	"testing"

	"github.com/MateEke/picture-frame/internal/wifi"
)

func seeded() *Manager {
	return New(
		wifi.WiFiState{Mode: wifi.ModeConnected, SSID: "Home", IP: "10.0.0.5", Signal: 70, Security: "WPA2"},
		[]wifi.WiFiNetwork{
			{SSID: "Home", Signal: 70, Security: "WPA2", Known: true},
			{SSID: "Cafe", Signal: 40, Security: "", Known: false},
		},
	)
}

func TestNewDefaultIsSeeded(t *testing.T) {
	m := NewDefault("frame")
	s := m.Status()
	if s.Mode != wifi.ModeConnected || s.Hostname != "frame" {
		t.Errorf("state = %+v, want connected with hostname frame", s)
	}
	nets, _ := m.Scan(context.Background())
	if len(nets) == 0 {
		t.Error("default network list should not be empty")
	}
}

func TestStatusReturnsSeededState(t *testing.T) {
	m := seeded()
	if got := m.Status(); got.SSID != "Home" || got.Signal != 70 {
		t.Errorf("Status = %+v, want Home/70", got)
	}
}

func TestScanReturnsCopyOfNetworks(t *testing.T) {
	m := seeded()
	nets, err := m.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(nets) != 2 {
		t.Fatalf("len = %d, want 2", len(nets))
	}
	// returned slice must be a copy, not the mock's own.
	nets[0].SSID = "tampered"
	if again, _ := m.Scan(context.Background()); again[0].SSID != "Home" {
		t.Errorf("Scan returned aliased slice: %q", again[0].SSID)
	}
}

func TestConnectAdoptsNetworkSignalAndMarksKnown(t *testing.T) {
	m := seeded()
	if err := m.Connect(context.Background(), "Cafe", "", false); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	s := m.Status()
	if s.Mode != wifi.ModeConnected || s.SSID != "Cafe" {
		t.Errorf("state = %+v, want connected/Cafe", s)
	}
	if s.Signal != 40 || s.Security != "" {
		t.Errorf("signal/security = %d/%q, want 40/''", s.Signal, s.Security)
	}
	nets, _ := m.Scan(context.Background())
	if !nets[1].Known {
		t.Error("Cafe should be marked known after connect")
	}
}

func TestConnectToUnlistedNetworkLeavesSignalUntouched(t *testing.T) {
	m := seeded()
	if err := m.Connect(context.Background(), "Unlisted", "pw", false); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if s := m.Status(); s.SSID != "Unlisted" || s.Signal != 70 {
		t.Errorf("state = %+v, want SSID Unlisted, signal kept at 70", s)
	}
}

func TestForgetActiveNetworkDropsLink(t *testing.T) {
	m := seeded()
	if err := m.Forget(context.Background(), "Home"); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	s := m.Status()
	if s.Mode != wifi.ModeDisconnected || s.SSID != "" || s.Signal != 0 || s.Security != "" {
		t.Errorf("state = %+v, want disconnected/empty", s)
	}
	if nets, _ := m.Scan(context.Background()); nets[0].Known {
		t.Error("Home should no longer be known")
	}
}

func TestForgetInactiveKnownNetworkKeepsLink(t *testing.T) {
	m := New(
		wifi.WiFiState{Mode: wifi.ModeConnected, SSID: "Home"},
		[]wifi.WiFiNetwork{
			{SSID: "Home", Known: true},
			{SSID: "Old", Known: true},
		},
	)
	if err := m.Forget(context.Background(), "Old"); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if s := m.Status(); s.Mode != wifi.ModeConnected || s.SSID != "Home" {
		t.Errorf("active link changed: %+v", s)
	}
}

func TestForgetUnknownNetworkErrors(t *testing.T) {
	m := seeded()
	if err := m.Forget(context.Background(), "Cafe"); err == nil {
		t.Error("forgetting an unsaved network should error")
	}
}

func TestConfigureEnablesHotspot(t *testing.T) {
	m := seeded()
	pw := "secret"
	if err := m.Configure(context.Background(), true, "Frame", &pw); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	s := m.Status()
	if !s.APEnabled || s.APSSID != "Frame" || !s.APHasPassword {
		t.Errorf("state = %+v, want enabled Frame with password", s)
	}
}

func TestConfigureClearsPasswordWithEmptyString(t *testing.T) {
	m := seeded()
	empty := ""
	if err := m.Configure(context.Background(), true, "Frame", &empty); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if s := m.Status(); s.APHasPassword {
		t.Error("empty password should clear ap_has_password")
	}
}

func TestConfigureNilPasswordKeepsStoredKey(t *testing.T) {
	m := New(wifi.WiFiState{APHasPassword: true}, nil)
	if err := m.Configure(context.Background(), true, "Frame", nil); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if s := m.Status(); !s.APHasPassword {
		t.Error("nil password should keep the stored key")
	}
}

func TestConfigureDisableClearsSSID(t *testing.T) {
	m := seeded()
	if err := m.Configure(context.Background(), false, "ignored", nil); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if s := m.Status(); s.APEnabled || s.APSSID != "" {
		t.Errorf("state = %+v, want disabled with empty SSID", s)
	}
}
