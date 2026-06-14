package wifi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeCommander struct {
	mu      sync.Mutex
	replies map[string]fakeReply
	calls   []string
}

type fakeReply struct {
	out []byte
	err error
}

func newFake() *fakeCommander {
	return &fakeCommander{replies: map[string]fakeReply{}}
}

func (f *fakeCommander) set(prefix string, out string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies[prefix] = fakeReply{out: []byte(out), err: err}
}

func (f *fakeCommander) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	f.mu.Lock()
	f.calls = append(f.calls, key)
	f.mu.Unlock()

	f.mu.Lock()
	defer f.mu.Unlock()
	// Match longest prefix first.
	best, bestLen := fakeReply{}, -1
	for prefix, reply := range f.replies {
		if strings.HasPrefix(key, prefix) && len(prefix) > bestLen {
			best, bestLen = reply, len(prefix)
		}
	}
	return best.out, best.err
}

func (f *fakeCommander) called(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

// bootWait advances the fake clock past radioSettle so post-boot assertions don't race.
func bootWait(t *testing.T) {
	t.Helper()
	synctest.Wait()
	time.Sleep(radioSettle + time.Second) //nolint:forbidigo
	synctest.Wait()
}

func newTestManager(cfg Config, fake Commander) *Manager {
	m := New(testutil.NopLogger(), cfg)
	m.nmcli = fake
	return m
}

func defaultCfg() Config {
	return Config{
		APTimeoutMinutes:    3,
		ScanIntervalMinutes: 5,
		APSSID:              "PictureFrame",
	}
}

func setupConnectedFake(f *fakeCommander, ssid string) {
	f.set("nmcli -t -f STATE,CONNECTIVITY general status", "connected\nconnected:full", nil)
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "yes:"+ssid+":80:WPA2", nil)
	f.set("nmcli -t -f NAME,TYPE,STATE connection show --active", "hotspot:wifi:deactivated", nil)
	f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:192.168.1.5/24", nil)
	f.set("nmcli connection show hotspot", "connection.id: hotspot", nil)
	f.set("nmcli connection modify hotspot", "", nil)
}

func setupDisconnectedFake(f *fakeCommander) {
	f.set("nmcli -t -f STATE,CONNECTIVITY general status", "disconnected:none", nil)
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "", nil)
	f.set("nmcli -t -f NAME,TYPE,STATE connection show --active", "", nil)
	f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "", nil)
	f.set("nmcli connection show hotspot", "connection.id: hotspot", nil)
	f.set("nmcli connection modify hotspot", "", nil)
	f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
	f.set("nmcli device wifi rescan", "", nil)
	f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list", "", nil)
}

func TestStatusInitialState(t *testing.T) {
	m := newTestManager(defaultCfg(), newFake())
	s := m.Status()
	if s.Mode != ModeConnecting {
		t.Errorf("initial mode: got %q, want %q", s.Mode, ModeConnecting)
	}
	if !s.APEnabled {
		t.Error("ap_enabled should be true when APSSID is set")
	}
}

func TestStatusAPEnabledFalseWhenNoSSID(t *testing.T) {
	cfg := defaultCfg()
	cfg.APSSID = ""
	m := newTestManager(cfg, newFake())
	if m.Status().APEnabled {
		t.Error("ap_enabled should be false when APSSID is empty")
	}
}

func TestStatusAPHasPasswordReflectsConfig(t *testing.T) {
	cfg := defaultCfg()
	cfg.APPassword = "secret"
	if !newTestManager(cfg, newFake()).Status().APHasPassword {
		t.Error("ap_has_password should be true when an AP password is set")
	}
	cfg.APPassword = ""
	if newTestManager(cfg, newFake()).Status().APHasPassword {
		t.Error("ap_has_password should be false when no AP password is set")
	}
}

func TestRunConnectedOnBoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupConnectedFake(f, "HomeWiFi")
		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		s := m.Status()
		if s.Mode != ModeConnected {
			t.Errorf("mode: got %q, want %q", s.Mode, ModeConnected)
		}
		if s.SSID != "HomeWiFi" {
			t.Errorf("ssid: got %q, want HomeWiFi", s.SSID)
		}
		if !s.APEnabled {
			t.Error("ap_enabled should remain true")
		}
	})
}

func TestRunDisablesAPFallbackWhenHotspotProfileMissing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		// The hotspot NM profile is absent, so AP fallback can't be raised and
		// must be cleared from both config and the published state.
		f.set("nmcli -t connection show hotspot", "", errors.New("not found"))

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		s := m.Status()
		if s.APEnabled {
			t.Error("ap_enabled should be false once the hotspot profile is found missing")
		}
		if s.APSSID != "" {
			t.Errorf("ap_ssid: got %q, want empty", s.APSSID)
		}
		if s.APHasPassword {
			t.Error("ap_has_password should be false after AP fallback is disabled")
		}
	})
}

func TestRunBootFastTrackNoKnownNetworks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		// No known networks in scan.
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
		f.set("nmcli connection up hotspot", "", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		s := m.Status()
		if s.Mode != ModeAP {
			t.Errorf("boot fast-track: mode = %q, want %q", s.Mode, ModeAP)
		}
		if s.SSID != "PictureFrame" {
			t.Errorf("boot fast-track: ssid = %q, want PictureFrame", s.SSID)
		}
	})
}

func TestRunBootFastTrackKnownNetworkPresent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		// First poll: disconnected.
		f.set("nmcli -t -f STATE,CONNECTIVITY general status", "disconnected:none", nil)
		f.set("nmcli -t -f NAME,TYPE,STATE connection show --active", "", nil)
		f.set("nmcli connection show hotspot", "connection.id: hotspot", nil)
		f.set("nmcli connection modify hotspot", "", nil)
		// Scan returns a known network.
		f.set("nmcli device wifi rescan", "", nil)
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list",
			`HomeWiFi:AA\:BB\:CC\:DD\:EE\:FF:Infra:6:130 Mbit/s:80:▂▄▆█:WPA2`, nil)
		// Profile is named after the provisioning tool, not the SSID.
		f.set("nmcli -t -f NAME,TYPE connection show", "netplan-wlan0-HomeWiFi:802-11-wireless\nhotspot:802-11-wireless", nil)
		f.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-HomeWiFi", "HomeWiFi", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		s := m.Status()
		// Should stay disconnected, not fast-track to AP, because known network is in range.
		if s.Mode == ModeAP {
			t.Error("should not fast-track to AP when known network is in range")
		}
		if f.called("connection up hotspot") {
			t.Error("AP should not be activated when known network is in range")
		}
	})
}

func TestRunDormantNeverRaisesAP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		cfg := defaultCfg()
		cfg.APSSID = ""
		cfg.APTimeoutMinutes = 0

		f.set("nmcli -t -f STATE,CONNECTIVITY general status", "disconnected:none", nil)
		f.set("nmcli -t -f NAME,TYPE,STATE connection show --active", "", nil)
		f.set("nmcli connection show hotspot", "", errors.New("not found"))

		m := newTestManager(cfg, f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		// Advance well past any AP timeout.
		time.Sleep(10 * time.Minute)
		synctest.Wait()

		if f.called("connection up hotspot") {
			t.Error("dormant manager must not activate AP")
		}
		s := m.Status()
		if s.Mode == ModeAP {
			t.Errorf("mode: got %q, should never be AP in dormant mode", s.Mode)
		}
	})
}

func TestRunAPActivatesAfterTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		cfg := defaultCfg()
		cfg.APTimeoutMinutes = 1

		// Always disconnected; hotspot profile present.
		f.set("nmcli -t -f STATE,CONNECTIVITY general status", "disconnected:none", nil)
		f.set("nmcli -t -f NAME,TYPE,STATE connection show --active", "", nil)
		f.set("nmcli connection show hotspot", "connection.id: hotspot", nil)
		f.set("nmcli connection modify hotspot", "", nil)
		f.set("nmcli device wifi rescan", "", nil)
		// Scan: one known network but don't make it "in range"; return empty list.
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list", "", nil)
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
		f.set("nmcli connection up hotspot", "", nil)

		m := newTestManager(cfg, f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		// Boot fast-track fires and activates AP since no known networks in range.
		bootWait(t)

		s := m.Status()
		if s.Mode != ModeAP {
			t.Errorf("after timeout: mode = %q, want %q", s.Mode, ModeAP)
		}
	})
}

func TestConnectEnqueuesAndReturnsBusy(t *testing.T) {
	m := newTestManager(defaultCfg(), newFake())
	// Fill command channel.
	for range commandsBufferSize {
		err := m.Connect(context.Background(), "net", "pass")
		if err != nil && !errors.Is(err, ErrBusy) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// Next call must return ErrBusy.
	if err := m.Connect(context.Background(), "net", "pass"); !errors.Is(err, ErrBusy) {
		t.Errorf("expected ErrBusy, got %v", err)
	}
}

func TestForgetDeletesProfileByResolvedSSID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupConnectedFake(f, "OldNet")
		// The saved profile's connection-id differs from the SSID (netplan/imager
		// naming); Forget must resolve the SSID and delete by the real name.
		f.set("nmcli -t -f NAME,TYPE connection show", "netplan-wlan0-OldNet:802-11-wireless\nhotspot:802-11-wireless", nil)
		f.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-OldNet", "OldNet", nil)
		f.set("nmcli connection delete netplan-wlan0-OldNet", "", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		if err := m.Forget(ctx, "OldNet"); err != nil {
			t.Fatalf("Forget: %v", err)
		}
		if !f.called("connection delete netplan-wlan0-OldNet") {
			t.Error("should delete the profile by its resolved connection-id")
		}
		if f.called("connection delete OldNet") {
			t.Error("must not try to delete by the raw SSID")
		}
	})
}

func TestForgetUnknownSSIDErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupConnectedFake(f, "HomeWiFi")
		// Only the hotspot profile exists; nothing matches "Ghost".
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		if err := m.Forget(ctx, "Ghost"); err == nil {
			t.Error("expected an error when no saved profile matches the SSID")
		}
	})
}

func TestForgetTreatsDeleteTimeoutAsSuccessWhenProfileGone(t *testing.T) {
	// Deleting the active connection can time out (exit 3) while NM tears the link
	// down, yet still remove the profile. If it's actually gone, that's success.
	f := newFake()
	f.set("nmcli -t -f NAME,TYPE connection show", "netplan-wlan0-Thor:802-11-wireless", nil)
	f.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-Thor", "Thor", nil)
	f.set("nmcli connection delete netplan-wlan0-Thor", "Error: Timeout expired (10 seconds)", errors.New("exit status 3"))
	// Verification: the profile no longer exists (nmcli show errors).
	f.set("nmcli -t -f NAME connection show netplan-wlan0-Thor", "", errors.New("exit status 10"))

	m := newTestManager(defaultCfg(), f)
	if err := m.forgetSSID(context.Background(), "Thor"); err != nil {
		t.Errorf("delete timeout but profile gone should succeed, got %v", err)
	}
}

func TestForgetFailsWhenDeleteFailsAndProfileRemains(t *testing.T) {
	f := newFake()
	f.set("nmcli -t -f NAME,TYPE connection show", "netplan-wlan0-Thor:802-11-wireless", nil)
	f.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-Thor", "Thor", nil)
	f.set("nmcli connection delete netplan-wlan0-Thor", "boom", errors.New("exit status 1"))
	// Verification: the profile is still present → genuine failure.
	f.set("nmcli -t -f NAME connection show netplan-wlan0-Thor", "netplan-wlan0-Thor", nil)

	m := newTestManager(defaultCfg(), f)
	if err := m.forgetSSID(context.Background(), "Thor"); err == nil {
		t.Error("expected error when delete fails and the profile remains")
	}
}

// forgetScenario drives general-status / device-wifi / profile-list responses
// off whether the active connection has been deleted yet, so a forget-then-poll
// flow sees the device drop offline.
type forgetScenario struct {
	*statefulFake
	mu      sync.Mutex
	deleted bool
}

func (c *forgetScenario) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if strings.Contains(key, "connection delete netplan-wlan0-Thor") {
		c.mu.Lock()
		c.deleted = true
		c.mu.Unlock()
	}
	c.mu.Lock()
	deleted := c.deleted
	c.mu.Unlock()
	switch {
	case strings.Contains(key, "STATE,CONNECTIVITY general status"):
		if deleted {
			return []byte("disconnected:none"), nil
		}
		return []byte("connected:full"), nil
	case strings.Contains(key, "ACTIVE,SSID,SIGNAL,SECURITY device wifi"):
		if deleted {
			return []byte(""), nil
		}
		return []byte("yes:Thor:80:WPA2"), nil
	case strings.Contains(key, "-f NAME,TYPE connection show"):
		if deleted {
			return []byte("hotspot:802-11-wireless"), nil
		}
		return []byte("netplan-wlan0-Thor:802-11-wireless\nhotspot:802-11-wireless"), nil
	}
	return c.statefulFake.Output(ctx, name, args...)
}

func TestForgetActiveNetworkFastTracksAP(t *testing.T) {
	// Forgetting the connected network with no other known network in range must
	// raise the AP promptly (boot fast-track), not after AP_TIMEOUT_MINUTES.
	synctest.Test(t, func(t *testing.T) {
		c := &forgetScenario{statefulFake: newStatefulFake()}
		c.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-Thor", "Thor", nil)
		c.set("nmcli connection delete netplan-wlan0-Thor", "", nil)
		c.set("nmcli connection show hotspot", "connection.id: hotspot", nil)
		c.set("nmcli connection modify hotspot", "", nil)
		c.set("nmcli device wifi rescan", "", nil)
		c.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list", "", nil)
		c.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:192.168.1.5/24", nil)

		m := newTestManager(defaultCfg(), c)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		if m.Status().Mode != ModeConnected {
			t.Fatalf("precondition: expected connected, got %q", m.Status().Mode)
		}

		if err := m.Forget(ctx, "Thor"); err != nil {
			t.Fatalf("Forget: %v", err)
		}
		// The fast-track repoll fires after radioSettle, and its scan sleeps another
		// radioSettle, well under the 3-minute AP timeout.
		synctest.Wait()
		time.Sleep(2*radioSettle + 2*time.Second)
		synctest.Wait()

		if m.Status().Mode != ModeAP {
			t.Errorf("expected AP raised promptly after forgetting active network, got %q", m.Status().Mode)
		}
	})
}

func TestScanReturnsParsedNetworks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupConnectedFake(f, "HomeWiFi")
		f.set("nmcli device wifi rescan", "", nil)
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list",
			"HomeWiFi:AA\\:BB\\:CC\\:DD\\:EE\\:FF:Infra:6:130 Mbit/s:80:▂▄▆█:WPA2\nGuestNet:BE\\:EF\\:CA\\:FE\\:00\\:01:Infra:11:54 Mbit/s:45:▂▄__:WPA3", nil)
		// HomeWiFi's saved profile is named after the provisioning tool, not the
		// SSID: Known must be keyed on the resolved SSID, not the connection NAME.
		// "hotspot" is a wifi profile too but is excluded from known networks.
		f.set("nmcli -t -f NAME,TYPE connection show", "netplan-wlan0-HomeWiFi:802-11-wireless\nhotspot:802-11-wireless", nil)
		f.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-HomeWiFi", "HomeWiFi", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		nets, err := m.Scan(ctx)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(nets) != 2 {
			t.Fatalf("got %d networks, want 2", len(nets))
		}
		home := nets[0]
		if home.SSID != "HomeWiFi" || home.Signal != 80 || home.Security != "WPA2" || !home.Known {
			t.Errorf("HomeWiFi should be Known (keyed on resolved SSID): %+v", home)
		}
		guest := nets[1]
		if guest.SSID != "GuestNet" || guest.Security != "WPA3" || guest.Known {
			t.Errorf("GuestNet: %+v", guest)
		}
	})
}

// statefulFake wraps fakeCommander and tracks hotspot up/down to answer
// isAPActive queries correctly, avoiding poll-clobber after doConnect/onScanTick.
type statefulFake struct {
	*fakeCommander
	mu       sync.Mutex
	apActive bool
}

func (s *statefulFake) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	s.mu.Lock()
	if strings.Contains(key, "connection up hotspot") {
		s.apActive = true
	} else if strings.Contains(key, "connection down hotspot") {
		s.apActive = false
	}
	ap := s.apActive
	s.mu.Unlock()

	// Override isAPActive query to reflect tracked state.
	if strings.Contains(key, "connection show --active") {
		if ap {
			return []byte("hotspot:wifi:activated"), nil
		}
		return []byte(""), nil
	}
	return s.fakeCommander.Output(ctx, name, args...)
}

func newStatefulFake() *statefulFake {
	return &statefulFake{fakeCommander: newFake()}
}

func setupDisconnectedStateful(f *statefulFake) {
	f.set("nmcli -t -f STATE,CONNECTIVITY general status", "disconnected:none", nil)
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "", nil)
	f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "", nil)
	f.set("nmcli connection show hotspot", "connection.id: hotspot", nil)
	f.set("nmcli connection modify hotspot", "", nil)
	f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
	f.set("nmcli device wifi rescan", "", nil)
	f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list", "", nil)
	f.set("nmcli connection up hotspot", "", nil)
	f.set("nmcli connection down hotspot", "", nil)
}

func TestDoConnectSuccessFromConnected(t *testing.T) {
	// doConnect called directly: no Run(), no poll interference.
	f := newFake()
	f.set("nmcli --wait 30 device wifi connect NewNet password secret", "", nil)
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "yes:NewNet:80:WPA2", nil)
	f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:10.0.0.1/24", nil)

	m := newTestManager(defaultCfg(), f)
	// Seed a stale failure to prove a subsequent success clears it.
	m.setState(WiFiState{Mode: ModeConnected, SSID: "HomeWiFi", APEnabled: true, LastConnectError: "stale", LastConnectSSID: "Old"})

	m.doConnect(context.Background(), "NewNet", "secret")

	// The previous network must be kept (forgetting is an explicit user action).
	if f.called("connection delete HomeWiFi") {
		t.Error("must not delete the previous network's profile on a successful switch")
	}
	s := m.Status()
	if s.Mode != ModeConnected {
		t.Errorf("mode: got %q, want connected", s.Mode)
	}
	if s.SSID != "NewNet" {
		t.Errorf("ssid: got %q, want NewNet", s.SSID)
	}
	if s.LastConnectError != "" || s.LastConnectSSID != "" {
		t.Errorf("connect error should be cleared on success, got %q/%q", s.LastConnectError, s.LastConnectSSID)
	}
}

func TestDoConnectFailureRevertsToPreviousNetwork(t *testing.T) {
	// Wrong password: the attempt fails, NM reverts wlan0 to the previous network,
	// and the state must reflect "connected to HomeWiFi" (not a misleading
	// "disconnected" with a stale IP) while still recording the failure.
	f := newFake()
	f.set("nmcli --wait 30 device wifi connect BadNet password wrong", "", errors.New("activation failed"))
	f.set("nmcli connection delete BadNet", "", nil)
	f.set("nmcli --wait 20 device connect wlan0", "", nil)
	f.set("nmcli -t -f STATE,CONNECTIVITY general status", "connected:full", nil)
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "yes:HomeWiFi:80:WPA2", nil)
	f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:192.168.1.5/24", nil)

	m := newTestManager(defaultCfg(), f)
	m.setState(WiFiState{Mode: ModeConnected, SSID: "HomeWiFi", IP: "192.168.1.5", APEnabled: true})

	m.doConnect(context.Background(), "BadNet", "wrong")

	if !f.called("connection delete BadNet") {
		t.Error("should delete leftover profile on failure")
	}
	if !f.called("device connect wlan0") {
		t.Error("station fallback (NM autoconnect) should run when a previous network exists")
	}
	s := m.Status()
	if s.Mode != ModeConnected {
		t.Errorf("expected ModeConnected (reverted to previous), got %q", s.Mode)
	}
	if s.SSID != "HomeWiFi" {
		t.Errorf("ssid: got %q, want HomeWiFi", s.SSID)
	}
	if s.IP != "192.168.1.5" {
		t.Errorf("ip: got %q, want 192.168.1.5", s.IP)
	}
	if s.LastConnectError == "" {
		t.Error("LastConnectError should be set even though we reverted to the old network")
	}
	if s.LastConnectSSID != "BadNet" {
		t.Errorf("LastConnectSSID: got %q, want BadNet", s.LastConnectSSID)
	}
}

func TestDoConnectFailurePreviousGoneStaysDisconnected(t *testing.T) {
	// Wrong password AND the previous network is no longer reachable: NM can't
	// reactivate anything, so the state is genuinely disconnected with no stale IP.
	f := newFake()
	f.set("nmcli --wait 30 device wifi connect BadNet password wrong", "", errors.New("activation failed"))
	f.set("nmcli connection delete BadNet", "", nil)
	f.set("nmcli --wait 20 device connect wlan0", "", errors.New("no suitable connection"))
	f.set("nmcli -t -f STATE,CONNECTIVITY general status", "disconnected:none", nil)

	m := newTestManager(defaultCfg(), f)
	m.setState(WiFiState{Mode: ModeConnected, SSID: "HomeWiFi", IP: "192.168.1.5", APEnabled: true})

	m.doConnect(context.Background(), "BadNet", "wrong")

	s := m.Status()
	if s.Mode != ModeDisconnected {
		t.Errorf("expected ModeDisconnected, got %q", s.Mode)
	}
	if s.IP != "" {
		t.Errorf("stale IP should be cleared when disconnected, got %q", s.IP)
	}
	if s.LastConnectError == "" || s.LastConnectSSID != "BadNet" {
		t.Errorf("failure should be recorded: %q/%q", s.LastConnectError, s.LastConnectSSID)
	}
}

func TestDoConnectFromAPMode(t *testing.T) {
	// doConnect from AP mode involves a time.Sleep(radioSettle); use synctest.
	synctest.Test(t, func(t *testing.T) {
		f := newStatefulFake()
		setupDisconnectedStateful(f)
		f.set("nmcli --wait 30 device wifi connect HomeNet password mypass", "", nil)
		f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "yes:HomeNet:80:WPA2", nil)
		f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:192.168.1.10/24", nil)

		m := newTestManager(defaultCfg(), f)
		m.setState(WiFiState{Mode: ModeAP, SSID: "PictureFrame", APEnabled: true})

		done := make(chan struct{})
		go func() {
			m.doConnect(context.Background(), "HomeNet", "mypass")
			close(done)
		}()
		// doConnect sleeps radioSettle after deactivating AP.
		synctest.Wait()
		time.Sleep(radioSettle + time.Second)
		synctest.Wait()
		<-done

		if !f.called("connection down hotspot") {
			t.Error("AP should be torn down before connect attempt")
		}
		s := m.Status()
		if s.Mode != ModeConnected || s.SSID != "HomeNet" {
			t.Errorf("expected connected to HomeNet, got mode=%q ssid=%q", s.Mode, s.SSID)
		}
	})
}

func TestDoConnectFromAPFailureReactivatesAP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newStatefulFake()
		setupDisconnectedStateful(f)
		f.set("nmcli --wait 30 device wifi connect BadNet password x", "", errors.New("failed"))
		f.set("nmcli connection delete BadNet", "", nil)

		m := newTestManager(defaultCfg(), f)
		m.setState(WiFiState{Mode: ModeAP, SSID: "PictureFrame", APEnabled: true})

		done := make(chan struct{})
		go func() {
			m.doConnect(context.Background(), "BadNet", "x")
			close(done)
		}()
		synctest.Wait()
		time.Sleep(radioSettle + time.Second)
		synctest.Wait()
		<-done

		if !f.called("connection delete BadNet") {
			t.Error("should clean up leftover profile")
		}
		s := m.Status()
		if s.Mode != ModeAP {
			t.Errorf("should reactivate AP after failed connect; got mode=%q", s.Mode)
		}
	})
}

// onScanTick is driven directly here (manager put into AP mode, then called)
// rather than racing the Run loop through boot + AP-timeout + scan timing, which
// is non-deterministic and would otherwise force the test to skip.

func TestOnScanTickIgnoredOutsideAPMode(t *testing.T) {
	f := newFake()
	m := newTestManager(defaultCfg(), f)
	m.setState(WiFiState{Mode: ModeConnected, SSID: "HomeWiFi"})

	m.onScanTick(context.Background())

	if f.called("connection down hotspot") {
		t.Error("scan tick should be a no-op when not in AP mode")
	}
	if m.Status().Mode != ModeConnected {
		t.Errorf("mode changed: got %q, want Connected", m.Status().Mode)
	}
}

func TestOnScanTickSkippedDuringCooldown(t *testing.T) {
	f := newFake()
	m := newTestManager(defaultCfg(), f)
	m.setState(WiFiState{Mode: ModeAP, SSID: "PictureFrame", APEnabled: true})
	m.apCooldownUntil = time.Now().Add(time.Hour) // freshly raised; still cooling down

	m.onScanTick(context.Background())

	if f.called("connection down hotspot") {
		t.Error("scan tick should not tear down a freshly-raised AP during cooldown")
	}
}

func TestOnScanTickNoKnownNetworkReactivatesAP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		f.set("nmcli connection down hotspot", "", nil)
		f.set("nmcli connection up hotspot", "", nil)
		f.set("nmcli device wifi rescan", "", nil)
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list", "", nil)
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)

		m := newTestManager(defaultCfg(), f)
		m.setState(WiFiState{Mode: ModeAP, SSID: "PictureFrame", APEnabled: true})

		m.onScanTick(context.Background())

		if !f.called("connection up hotspot") {
			t.Error("AP should be re-raised when the scan finds no known network")
		}
		if m.Status().Mode != ModeAP {
			t.Errorf("mode: got %q, want AP", m.Status().Mode)
		}
	})
}

func TestOnScanTickScanFailureReactivatesAP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		f.set("nmcli connection down hotspot", "", nil)
		f.set("nmcli connection up hotspot", "", nil)
		f.set("nmcli device wifi rescan", "", nil)
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list", "", errors.New("scan failed"))

		m := newTestManager(defaultCfg(), f)
		m.setState(WiFiState{Mode: ModeAP, SSID: "PictureFrame", APEnabled: true})

		m.onScanTick(context.Background())

		if !f.called("connection up hotspot") {
			t.Error("AP should be re-raised after a scan failure")
		}
		if m.Status().Mode != ModeAP {
			t.Errorf("mode: got %q, want AP", m.Status().Mode)
		}
	})
}

func TestOnScanTickReconnectsToKnownNetwork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		f.set("nmcli connection down hotspot", "", nil)
		f.set("nmcli device wifi rescan", "", nil)
		// A known network (resolved from a netplan-named profile) is back in range.
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list",
			`HomeWiFi:AA\:BB\:CC\:DD\:EE\:FF:Infra:6:130 Mbit/s:80:▂▄▆█:WPA2`, nil)
		f.set("nmcli -t -f NAME,TYPE connection show", "netplan-wlan0-HomeWiFi:802-11-wireless\nhotspot:802-11-wireless", nil)
		f.set("nmcli -g 802-11-wireless.ssid connection show netplan-wlan0-HomeWiFi", "HomeWiFi", nil)
		f.set("nmcli --wait 30 device wifi connect HomeWiFi", "", nil)
		f.set("nmcli -t -f STATE,CONNECTIVITY general status", "connected:full", nil)
		f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi", "yes:HomeWiFi:80:WPA2", nil)
		f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:192.168.1.5/24", nil)

		m := newTestManager(defaultCfg(), f)
		m.setState(WiFiState{Mode: ModeAP, SSID: "PictureFrame", APEnabled: true})

		m.onScanTick(context.Background())

		s := m.Status()
		if s.Mode != ModeConnected {
			t.Fatalf("mode: got %q, want Connected", s.Mode)
		}
		if s.SSID != "HomeWiFi" || s.IP != "192.168.1.5" {
			t.Errorf("link: got ssid=%q ip=%q, want HomeWiFi / 192.168.1.5", s.SSID, s.IP)
		}
		if f.called("connection up hotspot") {
			t.Error("AP should not be re-raised after a successful reconnect")
		}
	})
}

func TestConnectToFirstKnownSkipsNonKnown(t *testing.T) {
	// connectToFirstKnown is a direct call: no Run() needed.
	f := newFake()
	f.set("nmcli --wait 30 device wifi connect HomeWiFi", "", nil)

	m := newTestManager(defaultCfg(), f)

	nets := []WiFiNetwork{
		{SSID: "GuestNet", Signal: 90, Security: "WPA2", Known: false},
		{SSID: "HomeWiFi", Signal: 80, Security: "WPA2", Known: true},
	}
	err := m.connectToFirstKnown(context.Background(), nets)
	if err != nil {
		t.Errorf("connectToFirstKnown: %v", err)
	}
	if f.called("connect GuestNet") {
		t.Error("should not attempt to connect to unknown network")
	}
	if !f.called("connect HomeWiFi") {
		t.Error("should connect to known network HomeWiFi")
	}
}

func TestConnectToFirstKnownAllFail(t *testing.T) {
	f := newFake()
	f.set("nmcli --wait 30 device wifi connect Net1", "", errors.New("failed"))
	f.set("nmcli connection delete Net1", "", nil)

	m := newTestManager(defaultCfg(), f)

	nets := []WiFiNetwork{
		{SSID: "Net1", Signal: 60, Security: "WPA2", Known: true},
	}
	err := m.connectToFirstKnown(context.Background(), nets)
	if err == nil {
		t.Error("expected error when all known networks fail")
	}
}

func TestForgetContextCancelled(t *testing.T) {
	m := newTestManager(defaultCfg(), newFake())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err := m.Forget(ctx, "SomeNet")
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestScanContextCancelled(t *testing.T) {
	m := newTestManager(defaultCfg(), newFake())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.Scan(ctx)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestConfigureContextCancelled(t *testing.T) {
	m := newTestManager(defaultCfg(), newFake())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.Configure(ctx, true, "PF", nil)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestConfigureDisablesTeardownAP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
		f.set("nmcli connection up hotspot", "", nil)
		f.set("nmcli connection down hotspot", "", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		// Start in AP mode (boot fast-track).
		if m.Status().Mode != ModeAP {
			t.Skip("not in AP mode, fast-track did not fire, test not applicable")
		}

		if err := m.Configure(ctx, false, "", nil); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		if m.Status().APEnabled {
			t.Error("ap_enabled should be false after disabling")
		}
	})
}

func TestConfigureKeepsExistingPasswordWhenOmitted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
		f.set("nmcli connection up hotspot", "", nil)
		f.set("nmcli connection modify hotspot", "", nil)

		cfg := defaultCfg()
		cfg.APPassword = "secret-pass"
		m := newTestManager(cfg, f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		// Rename the hotspot without supplying a password.
		if err := m.Configure(ctx, true, "NewName", nil); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		s := m.Status()
		if !s.APHasPassword {
			t.Error("omitting the password should preserve the stored key")
		}
		if s.APSSID != "NewName" {
			t.Errorf("APSSID = %q, want NewName", s.APSSID)
		}
		if !f.called("wifi-sec.psk secret-pass") {
			t.Error("expected the preserved password to be written to the NM profile")
		}
		if !f.called("wifi-sec.pmf 1") {
			t.Error("expected PMF to be disabled (brcmfmac AP mode can't do 802.11w)")
		}
	})
}

func TestConfigureClearsPasswordWhenEmptyProvided(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		f.set("nmcli -t -f NAME,TYPE connection show", "hotspot:802-11-wireless", nil)
		f.set("nmcli connection up hotspot", "", nil)
		f.set("nmcli connection modify hotspot", "", nil)

		cfg := defaultCfg()
		cfg.APPassword = "secret-pass"
		m := newTestManager(cfg, f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		// Explicit empty password downgrades to an open AP.
		if err := m.Configure(ctx, true, "OpenAP", new("")); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		if m.Status().APHasPassword {
			t.Error("an explicit empty password should clear the stored key")
		}
		if !f.called("remove 802-11-wireless-security") {
			t.Error("expected the security section to be removed for an open AP")
		}
	})
}

func TestQueryActiveWiFiReturnsFirstActive(t *testing.T) {
	f := newFake()
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi",
		"no:Neighbor:60:WPA2\nyes:HomeWiFi:80:WPA2", nil)
	f.set("nmcli -t -f IP4.ADDRESS device show wlan0", "IP4.ADDRESS[1]:192.168.1.5/24", nil)
	f.set("nmcli -t -f DEVICE,TYPE device status", "wlan0:wifi", nil)

	m := newTestManager(defaultCfg(), f)
	link, err := m.queryActiveWiFi(context.Background())
	if err != nil {
		t.Fatalf("queryActiveWiFi: %v", err)
	}
	if link.ssid != "HomeWiFi" {
		t.Errorf("ssid: got %q, want HomeWiFi", link.ssid)
	}
	if link.ip != "192.168.1.5" {
		t.Errorf("ip: got %q, want 192.168.1.5", link.ip)
	}
	if link.signal != 80 {
		t.Errorf("signal: got %d, want 80", link.signal)
	}
	if link.security != "WPA2" {
		t.Errorf("security: got %q, want WPA2", link.security)
	}
}

func TestQueryActiveWiFiReturnsEmptyWhenNoneActive(t *testing.T) {
	f := newFake()
	f.set("nmcli -t -f ACTIVE,SSID,SIGNAL,SECURITY device wifi",
		"no:HomeWiFi:80:WPA2\nno:Neighbor:60:WPA2", nil)
	f.set("nmcli -t -f DEVICE,TYPE device status", "wlan0:wifi", nil)

	m := newTestManager(defaultCfg(), f)
	link, err := m.queryActiveWiFi(context.Background())
	if err != nil {
		t.Fatalf("queryActiveWiFi: %v", err)
	}
	if link.ssid != "" {
		t.Errorf("ssid: got %q, want empty", link.ssid)
	}
}

func TestWiFiInterfaceDetectsNonWlan0(t *testing.T) {
	f := newFake()
	f.set("nmcli -t -f DEVICE,TYPE device status", "lo:loopback\nwlan1:wifi\neth0:ethernet", nil)
	m := newTestManager(defaultCfg(), f)
	if got := m.wifiInterface(context.Background()); got != "wlan1" {
		t.Errorf("wifiInterface = %q, want wlan1", got)
	}
	// Second call is served from the cache, not re-queried.
	if got := m.wifiInterface(context.Background()); got != "wlan1" {
		t.Errorf("cached wifiInterface = %q, want wlan1", got)
	}
}

func TestWiFiInterfaceFallsBackToWlan0(t *testing.T) {
	f := newFake()
	f.set("nmcli -t -f DEVICE,TYPE device status", "", errors.New("nm down"))
	m := newTestManager(defaultCfg(), f)
	if got := m.wifiInterface(context.Background()); got != "wlan0" {
		t.Errorf("wifiInterface fallback = %q, want wlan0", got)
	}
}

// The AP raise must wait out the full configured timeout, not fire early.
func TestRunAPTimeoutCountsConfiguredMinutes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		cfg := defaultCfg()
		cfg.APTimeoutMinutes = 2

		setupDisconnectedFake(f)
		// A known network is in range, so the boot fast-track declines and the
		// countdown path is exercised instead.
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list",
			`HomeWiFi:AA\:BB\:CC\:DD\:EE\:FF:Infra:6:130 Mbit/s:80:▂▄▆█:WPA2`, nil)
		f.set("nmcli -t -f NAME,TYPE connection show", "HomeWiFi:802-11-wireless\nhotspot:802-11-wireless", nil)
		f.set("nmcli -g 802-11-wireless.ssid connection show HomeWiFi", "HomeWiFi", nil)
		f.set("nmcli connection up hotspot", "", nil)

		m := newTestManager(cfg, f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)

		time.Sleep(65 * time.Second) // half-way: countdown armed, not elapsed
		synctest.Wait()
		// Assert on the nmcli call, not just Mode: a premature raise would be
		// masked by the next poll demoting state again.
		if f.called("connection up hotspot") {
			t.Fatal("AP raised before the configured timeout")
		}
		time.Sleep(90 * time.Second) // past the deadline + next poll
		synctest.Wait()
		if !f.called("connection up hotspot") {
			t.Fatal("AP not raised after the timeout")
		}
		if s := m.Status(); s.Mode != ModeAP {
			t.Fatalf("after the timeout: mode %q, want %q", s.Mode, ModeAP)
		}
	})
}

// A hotspot already active in NM (e.g. raised before a backend restart) must be
// recognized as AP mode from the active-connection list.
func TestRunDetectsExternallyActiveHotspot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		f.set("nmcli -t -f NAME,TYPE,STATE connection show --active", "hotspot:wifi:activated", nil)
		// A known network in range keeps the boot fast-track from raising the AP
		// itself: ModeAP must come from detecting the active hotspot row.
		f.set("nmcli -t -f SSID,BSSID,MODE,CHAN,RATE,SIGNAL,BARS,SECURITY device wifi list",
			`HomeWiFi:AA\:BB\:CC\:DD\:EE\:FF:Infra:6:130 Mbit/s:80:▂▄▆█:WPA2`, nil)
		f.set("nmcli -t -f NAME,TYPE connection show", "HomeWiFi:802-11-wireless\nhotspot:802-11-wireless", nil)
		f.set("nmcli -g 802-11-wireless.ssid connection show HomeWiFi", "HomeWiFi", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		if s := m.Status(); s.Mode != ModeAP {
			t.Fatalf("active hotspot not detected: mode %q", s.Mode)
		}
		if !f.called("wifi.ssid PictureFrame") {
			t.Error("boot reconcile should sync the configured SSID into the hotspot profile")
		}
	})
}

func TestConfigureReflectsAPEnabled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupConnectedFake(f, "HomeWiFi")
		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t)

		if err := m.Configure(ctx, true, "NewName", nil); err != nil {
			t.Fatalf("Configure enable: %v", err)
		}
		if s := m.Status(); !s.APEnabled || s.APSSID != "NewName" {
			t.Fatalf("after enable: %+v", s)
		}
		if err := m.Configure(ctx, false, "NewName", nil); err != nil {
			t.Fatalf("Configure disable: %v", err)
		}
		if s := m.Status(); s.APEnabled {
			t.Fatal("APEnabled still true after disable")
		}
	})
}

// Disabling the AP while it is the active link must tear the hotspot down.
func TestConfigureDisableTearsDownActiveAP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		setupDisconnectedFake(f)
		f.set("nmcli connection up hotspot", "", nil)
		f.set("nmcli connection down hotspot", "", nil)

		m := newTestManager(defaultCfg(), f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go m.Run(ctx)
		bootWait(t) // no known networks → boot fast-track raises the AP

		if s := m.Status(); s.Mode != ModeAP {
			t.Fatalf("precondition: mode %q, want %q", s.Mode, ModeAP)
		}
		if err := m.Configure(ctx, false, "PictureFrame", nil); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		if !f.called("connection down hotspot") {
			t.Error("disabling the AP while active must deactivate the hotspot")
		}
		if s := m.Status(); s.Mode != ModeDisconnected {
			t.Errorf("after teardown: mode %q, want %q", s.Mode, ModeDisconnected)
		}
	})
}
