package wifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type activeLink struct {
	ssid     string
	ip       string
	signal   int
	security string
}

// queryGeneralStatus returns NM's STATE and CONNECTIVITY in a single nmcli call.
// SSID/IP are fetched separately via queryActiveWiFi to avoid extra forks per poll.
func (m *Manager) queryGeneralStatus(ctx context.Context) (nmState, connectivity string, err error) {
	out, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "STATE,CONNECTIVITY", "general", "status")
	if err != nil {
		return "", "", err
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := parseTerseFields(line)
		if len(fields) >= 2 {
			return fields[0], fields[1], nil
		}
	}
	return "", "", nil
}

// wifiInterface returns the NM-managed WiFi device name, detected once and cached.
// The adapter isn't guaranteed to be wlan0; on error it falls back to "wlan0"
// uncached, so a transient nmcli failure is retried next call.
func (m *Manager) wifiInterface(ctx context.Context) string {
	if m.iface != "" {
		return m.iface
	}
	out, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE", "device", "status")
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			fields := parseTerseFields(line)
			if len(fields) >= 2 && fields[1] == "wifi" {
				m.iface = fields[0]
				return m.iface
			}
		}
	}
	return "wlan0"
}

func (m *Manager) queryActiveWiFi(ctx context.Context) (link activeLink, err error) {
	out, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "ACTIVE,SSID,SIGNAL,SECURITY", "device", "wifi")
	if err != nil {
		return activeLink{}, err
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := parseTerseFields(line)
		if len(fields) >= 4 && fields[0] == "yes" {
			link.ssid = fields[1]
			link.signal, _ = strconv.Atoi(fields[2])
			link.security = fields[3]
			break
		}
	}
	if link.ssid == "" {
		return activeLink{}, nil
	}
	ipOut, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "IP4.ADDRESS", "device", "show", m.wifiInterface(ctx))
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(ipOut)), "\n") {
			fields := parseTerseFields(line)
			if len(fields) >= 2 && strings.HasPrefix(fields[0], "IP4.ADDRESS") {
				link.ip, _, _ = strings.Cut(fields[1], "/")
				break
			}
		}
	}
	return link, nil
}

func (m *Manager) doScan(ctx context.Context) ([]WiFiNetwork, error) {
	m.nmcli.Output(ctx, "nmcli", "device", "wifi", "rescan") //nolint:errcheck
	time.Sleep(radioSettle)

	listOut, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY", "device", "wifi", "list")
	if err != nil {
		return nil, err
	}

	knownSSIDs, _ := m.knownSSIDs(ctx)

	seen := map[string]bool{}
	var nets []WiFiNetwork
	for line := range strings.SplitSeq(strings.TrimSpace(string(listOut)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := parseTerseFields(line)
		if len(fields) < 8 {
			continue
		}
		ssid := fields[0]
		if ssid == "" || seen[ssid] {
			continue
		}
		seen[ssid] = true
		signal, _ := strconv.Atoi(fields[5])
		nets = append(nets, WiFiNetwork{
			SSID:     ssid,
			Signal:   signal,
			Security: fields[7],
			Known:    knownSSIDs[ssid],
		})
	}
	return nets, nil
}

// wifiProfiles returns saved WiFi profiles as connection-id → SSID, excluding the
// "hotspot" profile. It reads each profile's 802-11-wireless.ssid because
// provisioning tools (netplan, Pi imager) name profiles arbitrarily, so the
// connection-id rarely matches the SSID.
func (m *Manager) wifiProfiles(ctx context.Context) (map[string]string, error) {
	out, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "NAME,TYPE", "connection", "show")
	if err != nil {
		return nil, err
	}
	profiles := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := parseTerseFields(line)
		if len(fields) < 2 {
			continue
		}
		name, typ := fields[0], fields[1]
		// Accept both the canonical type and the nmcli alias (version-dependent).
		if typ != "802-11-wireless" && typ != "wifi" {
			continue
		}
		if name == "hotspot" {
			continue
		}
		if ssid := m.profileSSID(ctx, name); ssid != "" {
			profiles[name] = ssid
		}
	}
	return profiles, nil
}

func (m *Manager) knownSSIDs(ctx context.Context) (map[string]bool, error) {
	profiles, err := m.wifiProfiles(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(profiles))
	for _, ssid := range profiles {
		known[ssid] = true
	}
	return known, nil
}

// forgetSSID deletes every saved WiFi profile whose SSID matches. Deleting by
// SSID directly fails when the connection-id differs (e.g. "netplan-wlan0-Thor"
// for SSID "Thor"), so names are resolved first.
func (m *Manager) forgetSSID(ctx context.Context, ssid string) error {
	profiles, err := m.wifiProfiles(ctx)
	if err != nil {
		return err
	}
	var names []string
	for name, savedSSID := range profiles {
		if savedSSID == ssid {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("no saved network found for %q", ssid)
	}
	for _, name := range names {
		out, derr := m.nmcli.Output(ctx, "nmcli", "connection", "delete", name)
		if derr == nil {
			continue
		}
		// Deleting the active connection can exceed nmcli's 10s timeout (exit 3)
		// while NM tears the link, yet still remove the profile. Treat as success.
		if m.profileExists(ctx, name) {
			return fmt.Errorf("delete %q: %w (%s)", name, derr, strings.TrimSpace(string(out)))
		}
		m.log.Warn("wifi: connection delete reported an error but the profile is gone; treating as forgotten",
			"name", name, "err", derr)
	}
	return nil
}

// profileExists reports whether a profile exists (nmcli errors on missing ones).
func (m *Manager) profileExists(ctx context.Context, name string) bool {
	_, err := m.nmcli.Output(ctx, "nmcli", "-t", "-f", "NAME", "connection", "show", name)
	return err == nil
}

// profileSSID returns the SSID stored in a saved profile, or "".
// Uses -g so nmcli prints the bare value with no field key.
func (m *Manager) profileSSID(ctx context.Context, name string) string {
	out, err := m.nmcli.Output(ctx, "nmcli", "-g", "802-11-wireless.ssid", "connection", "show", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
