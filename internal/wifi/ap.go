package wifi

import (
	"context"
	"errors"
	"strings"
	"time"
)

// onScanTick runs in AP mode: tears down the AP, scans for known networks,
// reconnects if found, or re-raises the AP.
func (m *Manager) onScanTick(ctx context.Context) {
	current := m.Status()
	if current.Mode != ModeAP {
		return
	}
	if time.Now().Before(m.apCooldownUntil) {
		return
	}
	m.log.Info("wifi: AP mode scan tick, checking for known networks")
	m.deactivateAP(ctx)
	time.Sleep(radioSettle)

	nets, err := m.doScan(ctx)
	if err != nil {
		m.log.Warn("wifi: scan failed during AP scan tick", "err", err)
		m.activateAP(ctx)
		return
	}
	if m.hasKnownNetwork(nets) {
		m.log.Info("wifi: known network found during scan tick, attempting reconnect")
		if err := m.connectToFirstKnown(ctx, nets); err == nil {
			// Only a real NM "connected" counts; a lingering active row from the
			// just-torn-down AP could otherwise read as a false success.
			if nmState, _, gerr := m.queryGeneralStatus(ctx); gerr == nil && nmState == "connected" {
				link, _ := m.queryActiveWiFi(ctx)
				next := m.Status()
				next.Mode = ModeConnected
				next.SSID = link.ssid
				next.IP = link.ip
				next.Signal = link.signal
				next.Security = link.security
				m.setState(next)
				return
			}
		}
	}
	m.log.Info("wifi: no known networks found during scan tick, reactivating AP")
	m.activateAP(ctx)
}

// activateAP brings up the hotspot and arms the entry cooldown so the next scan
// tick doesn't immediately tear the freshly-raised AP back down.
func (m *Manager) activateAP(ctx context.Context) {
	if m.cfg.APSSID == "" {
		return
	}
	out, err := m.nmcli.Output(ctx, "nmcli", "connection", "up", "hotspot")
	if err != nil {
		m.log.Error("wifi: failed to activate AP", "err", err, "output", string(out))
		return
	}
	m.apCooldownUntil = time.Now().Add(apCooldown)
}

func (m *Manager) deactivateAP(ctx context.Context) {
	out, err := m.nmcli.Output(ctx, "nmcli", "connection", "down", "hotspot")
	if err != nil {
		m.log.Warn("wifi: failed to deactivate AP", "err", err, "output", string(out))
	}
}

func (m *Manager) isAPActive(ctx context.Context) bool {
	out, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "NAME,TYPE,STATE", "connection", "show", "--active")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := parseTerseFields(line)
		if len(fields) >= 3 && fields[0] == "hotspot" && fields[2] == "activated" {
			return true
		}
	}
	return false
}

// reconcileHotspotProfile verifies the hotspot NM profile exists and syncs its
// SSID/security to config. Returns error if the profile is absent.
func (m *Manager) reconcileHotspotProfile(ctx context.Context) error {
	if _, err := m.nmcli.Output(ctx, "nmcli", "-t", "connection", "show", "hotspot"); err != nil {
		return errors.New("hotspot NM profile not found")
	}
	if m.cfg.APSSID != "" {
		m.updateHotspotProfile(ctx, m.cfg.APSSID, m.cfg.APPassword)
	}
	return nil
}

func (m *Manager) updateHotspotProfile(ctx context.Context, ssid, pw string) {
	if out, err := m.nmcli.Output(ctx, "nmcli", "connection", "modify", "hotspot", "wifi.ssid", ssid); err != nil {
		m.log.Warn("wifi: failed to update hotspot SSID", "err", err, "output", string(out))
	}
	if pw != "" {
		// pmf=1 (disable 802.11w): brcmfmac AP mode can't do PMF. Re-set every time
		// because clearing the password removes the whole security section.
		args := []string{"connection", "modify", "hotspot",
			"wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", pw, "wifi-sec.pmf", "1"}
		if out, err := m.nmcli.Output(ctx, "nmcli", args...); err != nil {
			m.log.Warn("wifi: failed to set hotspot security", "err", err, "output", string(out))
		}
		return
	}
	// Remove the security section entirely, setting key-mgmt="" is rejected by NM.
	if out, err := m.nmcli.Output(ctx, "nmcli", "connection", "modify", "hotspot", "remove", "802-11-wireless-security"); err != nil {
		m.log.Warn("wifi: failed to clear hotspot security", "err", err, "output", string(out))
	}
}
