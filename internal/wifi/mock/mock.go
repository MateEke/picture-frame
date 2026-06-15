// Package mock is an in-memory wifi.Manager for development and UI testing:
// Connect/Forget/Configure mutate held state, touching no real networking.
package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/MateEke/picture-frame/internal/wifi"
)

// Manager is an in-memory WiFiManager.
type Manager struct {
	mu       sync.Mutex
	state    wifi.WiFiState
	networks []wifi.WiFiNetwork
}

// New seeds the mock with an initial state and scan list.
func New(state wifi.WiFiState, networks []wifi.WiFiNetwork) *Manager {
	return &Manager{state: state, networks: networks}
}

// NewDefault seeds a connected station plus a scan list spanning saved/available,
// open, and WPA3-only cases, so the admin WiFi page renders all its states.
func NewDefault(hostname string) *Manager {
	return New(
		wifi.WiFiState{
			Mode:          wifi.ModeConnected,
			SSID:          "Home-WiFi",
			IP:            "192.168.1.42",
			Signal:        82,
			Security:      "WPA2",
			APEnabled:     true,
			APSSID:        "PictureFrame",
			APHasPassword: true,
			Hostname:      hostname,
		},
		[]wifi.WiFiNetwork{
			{SSID: "Home-WiFi", Signal: 82, Security: "WPA2", Known: true},
			{SSID: "Home-WiFi-5G", Signal: 64, Security: "WPA2", Known: false},
			{SSID: "Neighbour_2.4", Signal: 38, Security: "WPA2", Known: false},
			{SSID: "Coffee Shop", Signal: 25, Security: "", Known: false},
			{SSID: "OldRouter", Signal: 12, Security: "WPA2", Known: true},
			{SSID: "Hidden-Office", Security: "WPA2", Known: true, Hidden: true},
			{SSID: "SecureCorp", Signal: 55, Security: "WPA3", Known: false},
		},
	)
}

func (m *Manager) Status() wifi.WiFiState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) Scan(context.Context) ([]wifi.WiFiNetwork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]wifi.WiFiNetwork(nil), m.networks...), nil
}

// Connect simulates a successful association, adopting the target's signal/security.
func (m *Manager) Connect(_ context.Context, ssid, _ string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Mode = wifi.ModeConnected
	m.state.SSID = ssid
	m.state.LastConnectError = ""
	m.state.LastConnectSSID = ""
	for i := range m.networks {
		if m.networks[i].SSID == ssid {
			m.state.Signal = m.networks[i].Signal
			m.state.Security = m.networks[i].Security
			m.networks[i].Known = true
		}
	}
	return nil
}

// Forget clears the saved flag, drops the link if it was active, and errors when
// nothing matches (as the real manager does).
func (m *Manager) Forget(_ context.Context, ssid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := false
	for i := range m.networks {
		if m.networks[i].SSID == ssid && m.networks[i].Known {
			m.networks[i].Known = false
			found = true
		}
	}
	if !found {
		return fmt.Errorf("no saved network found for %q", ssid)
	}
	if m.state.Mode == wifi.ModeConnected && m.state.SSID == ssid {
		m.state.Mode = wifi.ModeDisconnected
		m.state.SSID = ""
		m.state.IP = ""
		m.state.Signal = 0
		m.state.Security = ""
	}
	return nil
}

// Configure updates the hotspot identity; a nil pw keeps the stored key.
func (m *Manager) Configure(_ context.Context, enabled bool, ssid string, pw *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if enabled {
		m.state.APSSID = ssid
	} else {
		m.state.APSSID = ""
	}
	m.state.APEnabled = enabled && ssid != ""
	if pw != nil {
		m.state.APHasPassword = *pw != ""
	}
	return nil
}
