package wifi

import (
	"context"
	"errors"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
)

func (m *Manager) doConnect(ctx context.Context, ssid, pass string) {
	current := m.Status()
	wasAP := current.Mode == ModeAP
	if wasAP {
		m.deactivateAP(ctx)
		time.Sleep(radioSettle)
	}
	prevSSID := current.SSID

	next := m.Status()
	next.Mode = ModeConnecting
	next.SSID = ssid
	next.LastConnectError = "" // clear any prior failure now that a new attempt starts
	next.LastConnectSSID = ""
	m.setState(next)

	args := []string{"--wait", "30", "device", "wifi", "connect", ssid}
	if pass != "" {
		args = append(args, "password", pass)
	}
	out, err := m.nmcli.Output(ctx, "nmcli", args...)
	if err != nil {
		m.log.Warn("wifi: connect failed", "ssid", ssid, "err", err, "output", string(out))
		m.recoverFromFailedConnect(ctx, next, ssid, prevSSID, wasAP)
		return
	}

	// Success. Keep the previous network's profile so the device can auto-rejoin
	// it later; removing a network is an explicit Forget.
	link, _ := m.queryActiveWiFi(ctx)
	next.Mode = ModeConnected
	next.SSID = ssid
	next.IP = link.ip
	next.Signal = link.signal
	next.Security = link.security
	m.setState(next)
}

// recoverFromFailedConnect cleans up after a failed connect and resolves state:
// back to the hotspot if the attempt interrupted AP mode, otherwise to whatever
// network NM autoconnect brings back (or disconnected).
func (m *Manager) recoverFromFailedConnect(ctx context.Context, next WiFiState, ssid, prevSSID string, wasAP bool) {
	// Clean up leftover profile unconditionally (confirmed required on Trixie).
	m.nmcli.Output(ctx, "nmcli", "connection", "delete", ssid) //nolint:errcheck
	next.LastConnectError = connectFailedMsg
	next.LastConnectSSID = ssid

	if wasAP {
		m.activateAP(ctx) // re-arms the cooldown
		next.Mode = ModeAP
		next.SSID = m.cfg.APSSID
		next.IP = ""
		m.setState(next)
		return
	}

	// Station fallback: a failed attempt leaves wlan0 down; ask NM to reactivate
	// its best known connection. `device connect` uses NM autoconnect, so the
	// previous profile's name (rarely equal to the SSID) is not needed.
	if prevSSID != "" && prevSSID != ssid {
		m.nmcli.Output(ctx, "nmcli", "--wait", "20", "device", "connect", m.wifiInterface(ctx)) //nolint:errcheck
	}
	// Report real post-fallback state, not a misleading "disconnected" while the
	// radio is in fact back on the previous network.
	if nmState, _, gerr := m.queryGeneralStatus(ctx); gerr == nil && nmState == "connected" {
		link, _ := m.queryActiveWiFi(ctx)
		if link.ssid == "" {
			link.ssid = prevSSID
		}
		next.Mode = ModeConnected
		next.SSID = link.ssid
		next.IP = link.ip
		next.Signal = link.signal
		next.Security = link.security
	} else {
		next.Mode = ModeDisconnected
		next.SSID = ""
		next.IP = ""
		next.Signal = 0
		next.Security = ""
	}
	m.setState(next)
}

func (m *Manager) doConfigure(ctx context.Context, enabled bool, ssid, password string, passwordSet bool) error {
	current := m.Status()
	wasActive := current.Mode == ModeAP && current.APEnabled

	newSSID := ssid
	if !enabled {
		newSSID = ""
	}

	// Omitting the password keeps the stored key so a SSID-only edit can't
	// silently strip WPA2 and open the hotspot. A supplied value (including "")
	// is applied verbatim.
	newPassword := m.cfg.APPassword
	if passwordSet {
		newPassword = password
	}

	wifiCfg := config.WiFiConfig{
		APTimeoutMinutes:    m.cfg.APTimeoutMinutes,
		ScanIntervalMinutes: m.cfg.ScanIntervalMinutes,
		APSSID:              newSSID,
		APPassword:          newPassword,
	}
	if m.cfg.Store != nil {
		if err := m.cfg.Store.Update(func(c *config.Config) error {
			c.WiFi = wifiCfg
			return nil
		}); err != nil {
			return err
		}
	}

	m.cfg.APSSID = newSSID
	m.cfg.APPassword = newPassword

	next := m.Status()
	next.APEnabled = enabled && newSSID != ""
	next.APSSID = newSSID
	next.APHasPassword = newPassword != ""

	if wasActive && !enabled {
		// Tear down the AP. Recovery relies on NM autoconnect; if no known network
		// is in range the device stays disconnected (user warned by the UI dialog).
		m.deactivateAP(ctx)
		next.Mode = ModeDisconnected
		next.SSID = ""
	} else if enabled && newSSID != "" {
		m.updateHotspotProfile(ctx, newSSID, newPassword)
	}
	m.setState(next)
	return nil
}

func (m *Manager) hasKnownNetwork(nets []WiFiNetwork) bool {
	for _, network := range nets {
		if network.Known {
			return true
		}
	}
	return false
}

func (m *Manager) connectToFirstKnown(ctx context.Context, nets []WiFiNetwork) error {
	for _, network := range nets {
		if !network.Known {
			continue
		}
		args := []string{"--wait", "30", "device", "wifi", "connect", network.SSID}
		out, err := m.nmcli.Output(ctx, "nmcli", args...)
		if err != nil {
			m.log.Warn("wifi: auto-reconnect failed", "ssid", network.SSID, "err", err, "output", string(out))
			m.nmcli.Output(ctx, "nmcli", "connection", "delete", network.SSID) //nolint:errcheck
			continue
		}
		return nil
	}
	return errors.New("no known networks reachable")
}
