// Package wifi manages the WiFi connection and AP fallback for the picture frame.
package wifi

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
)

type Mode string

const (
	ModeConnected    Mode = "connected"
	ModeAP           Mode = "ap"
	ModeDisconnected Mode = "disconnected"
	ModeConnecting   Mode = "connecting"
)

type WiFiState struct { //nolint:revive // name drives generated TypeScript type
	Mode Mode   `json:"mode"`
	SSID string `json:"ssid"`
	IP   string `json:"ip"`
	// Signal (0-100) and Security describe the active link; zero/empty when not connected.
	Signal    int    `json:"signal"`
	Security  string `json:"security"`
	APEnabled bool   `json:"ap_enabled"`
	// APSSID/APHasPassword expose the configured hotspot identity (independent of
	// the live SSID) so the settings UI can pre-fill the form. Password is never sent.
	APSSID        string `json:"ap_ssid"`
	APHasPassword bool   `json:"ap_has_password"`
	Hostname      string `json:"hostname"`
	// LastConnectError is non-empty when the most recent connect attempt failed;
	// cleared when a new attempt starts or succeeds. LastConnectSSID names it.
	LastConnectError string `json:"last_connect_error,omitempty"`
	LastConnectSSID  string `json:"last_connect_ssid,omitempty"`
}

// connectFailedMsg is the generic connect-failure string. nmcli's error text is
// unreliable (wrong password may report "could not be found"), so we never branch on it.
const connectFailedMsg = "Could not connect, check the password and try again."

type WiFiNetwork struct { //nolint:revive // name drives generated TypeScript type
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal"`
	Security string `json:"security"`
	Known    bool   `json:"known"`
	Hidden   bool   `json:"hidden"`
}

type Config struct {
	APTimeoutMinutes    int
	ScanIntervalMinutes int
	APSSID              string
	APPassword          string
	Store               *config.Store
}

var ErrBusy = errors.New("wifi manager busy")

const commandsBufferSize = 4

type cmdAction int

const (
	actConnect cmdAction = iota
	actForget
	actScan
	actConfigure
)

type scanResult struct {
	networks []WiFiNetwork
	err      error
}

type command struct {
	action cmdAction
	ssid   string
	pass   string
	// passSet distinguishes "no password supplied" (keep stored) from explicit ""
	// (clear it / open AP). Only meaningful for actConfigure.
	passSet bool
	// hidden signals a non-broadcasting network NM won't find in a scan.
	hidden  bool
	enabled bool
	result  chan error
	scan    chan scanResult
}

// Manager serialises all nmcli operations through a single goroutine.
type Manager struct {
	log      *slog.Logger
	cfg      Config
	nmcli    Commander
	commands chan command

	mu    sync.RWMutex
	state WiFiState

	// The fields below are owned by the Run goroutine and need no lock.

	// apCooldownUntil suppresses the AP scan tick after the AP is raised,
	// preventing radio thrashing.
	apCooldownUntil time.Time
	// booted is false until the first poll resolves real state; resetting it
	// (e.g. after forgetting the active network) re-runs the boot fast-track.
	booted bool
	// apTimeoutUntil is non-zero while counting down to AP activation.
	apTimeoutUntil time.Time
	// iface is the detected WiFi device name, resolved lazily and cached. install.sh
	// may use a non-wlan0 adapter; falls back to wlan0 on error.
	iface string
}

// New creates a Manager. cfg.OverridesPath is used by Configure to persist changes.
func New(log *slog.Logger, cfg Config) *Manager {
	hostname, _ := os.Hostname()
	return &Manager{
		log:      log,
		cfg:      cfg,
		nmcli:    execCommander{},
		commands: make(chan command, commandsBufferSize),
		state: WiFiState{
			Mode:          ModeConnecting,
			APEnabled:     cfg.APSSID != "",
			APSSID:        cfg.APSSID,
			APHasPassword: cfg.APPassword != "",
			Hostname:      hostname,
		},
	}
}

func (m *Manager) setState(s WiFiState) {
	m.mu.Lock()
	m.state = s
	m.mu.Unlock()
}

func (m *Manager) Status() WiFiState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Connect enqueues a connection attempt. Returns nil (HTTP 202) or ErrBusy (HTTP 503).
func (m *Manager) Connect(_ context.Context, ssid, pass string, hidden bool) error {
	select {
	case m.commands <- command{action: actConnect, ssid: ssid, pass: pass, hidden: hidden}:
		return nil
	default:
		return ErrBusy
	}
}

// Forget is synchronous (unlike Connect); waits for the Run goroutine to complete it.
func (m *Manager) Forget(ctx context.Context, ssid string) error {
	cmd := command{action: actForget, ssid: ssid, result: make(chan error, 1)}
	select {
	case m.commands <- cmd:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-cmd.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Scan(ctx context.Context) ([]WiFiNetwork, error) {
	cmd := command{action: actScan, scan: make(chan scanResult, 1)}
	select {
	case m.commands <- cmd:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case r := <-cmd.scan:
		return r.networks, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Configure changes ap_enabled/ssid/password; persists via SaveWiFiOverrides.
// A nil pw keeps the stored password; a non-nil pw (including "") replaces it.
func (m *Manager) Configure(ctx context.Context, enabled bool, ssid string, password *string) error {
	cmd := command{
		action:  actConfigure,
		ssid:    ssid,
		enabled: enabled,
		result:  make(chan error, 1),
	}
	if password != nil {
		cmd.pass = *password
		cmd.passSet = true
	}
	select {
	case m.commands <- cmd:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-cmd.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

const (
	radioSettle     = 3 * time.Second
	apCooldown      = 60 * time.Second
	pollInterval    = 30 * time.Second
	nmRetryInterval = 5 * time.Second
	// recoverPollInterval re-polls fast while disconnected so an NM autoconnect shows up quickly.
	recoverPollInterval = 5 * time.Second
)

// Run is the single writer of state; uses real time calls (testable via synctest).
func (m *Manager) Run(ctx context.Context) {
	poll := time.NewTimer(0)
	defer poll.Stop()
	scan := time.NewTicker(time.Duration(m.cfg.ScanIntervalMinutes) * time.Minute)
	defer scan.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case cmd := <-m.commands:
			repollSoon := m.handleCommand(ctx, cmd)
			// A scan changes no state, so don't reset the poll (it would delay the next check).
			if cmd.action == actScan {
				continue
			}
			// A poll tick may have fired while the command ran; its NM read is now
			// stale relative to the state the command just set, discard it.
			if !poll.Stop() {
				select {
				case <-poll.C:
				default:
				}
			}
			// Connectivity-changing commands (e.g. forget) re-evaluate promptly.
			if repollSoon {
				poll.Reset(radioSettle)
			} else {
				poll.Reset(pollInterval)
			}

		case <-poll.C:
			poll.Reset(m.onPoll(ctx))

		case <-scan.C:
			m.onScanTick(ctx)
		}
	}
}

// handleCommand processes one user command and reports whether the Run loop
// should re-poll promptly because connectivity may have changed.
func (m *Manager) handleCommand(ctx context.Context, cmd command) (repollSoon bool) {
	switch cmd.action {
	case actConnect:
		m.doConnect(ctx, cmd.ssid, cmd.pass, cmd.hidden)

	case actForget:
		// Forgetting the active network drops the link. Reset booted so the next
		// poll re-runs the boot fast-track (reconnect or fast-raise AP) instead of
		// waiting out AP_TIMEOUT_MINUTES.
		current := m.Status()
		wasActive := current.Mode == ModeConnected && current.SSID == cmd.ssid
		err := m.forgetSSID(ctx, cmd.ssid)
		cmd.result <- err
		if err == nil && wasActive {
			m.booted = false
			return true
		}

	case actScan:
		nets, err := m.doScan(ctx)
		cmd.scan <- scanResult{networks: nets, err: err}

	case actConfigure:
		cmd.result <- m.doConfigure(ctx, cmd.enabled, cmd.ssid, cmd.pass, cmd.passSet)
	}
	return false
}

// onPoll reconciles WiFiState with NetworkManager and returns the delay until the
// next poll. It is the only place, besides the command handlers, that writes state.
func (m *Manager) onPoll(ctx context.Context) time.Duration {
	// Connect is in flight; skip so a mid-association NM read can't clobber state.
	if m.booted && m.Status().Mode == ModeConnecting {
		return pollInterval
	}

	if !m.booted {
		if err := m.reconcileHotspotProfile(ctx); err != nil {
			m.disableAPFallback(err)
		}
	}

	nmState, nmConnectivity, err := m.queryGeneralStatus(ctx)
	if err != nil {
		m.log.Warn("wifi: NM not ready", "err", err)
		if !m.booted {
			return nmRetryInterval
		}
		return pollInterval
	}

	next := m.Status()
	// Reflect current AP config in state (reconcile may have cleared it).
	next.APEnabled = m.cfg.APSSID != ""
	next.APSSID = m.cfg.APSSID
	next.APHasPassword = m.cfg.APPassword != ""
	// Only the connected branch repopulates these; reset so they never go stale.
	next.Signal = 0
	next.Security = ""

	if nmState == "connected" || nmConnectivity == "full" {
		return m.onConnected(ctx, next)
	}
	return m.onDisconnected(ctx, next)
}

// onConnected resolves state when NM reports connectivity: AP mode if our hotspot
// is the active link, otherwise the connected station.
func (m *Manager) onConnected(ctx context.Context, next WiFiState) time.Duration {
	m.booted = true
	// Active hotspot counts as AP mode even if NM reports "connected".
	if m.isAPActive(ctx) {
		m.setAPMode(next)
		return pollInterval
	}
	link, _ := m.queryActiveWiFi(ctx)
	m.apTimeoutUntil = time.Time{}
	next.Mode = ModeConnected
	next.SSID = link.ssid
	next.IP = link.ip
	next.Signal = link.signal
	next.Security = link.security
	m.setState(next)
	return pollInterval
}

// onDisconnected resolves state when NM reports no connectivity: AP mode if the
// hotspot is (still) up, the boot fast-track, or counting down to AP fallback.
func (m *Manager) onDisconnected(ctx context.Context, next WiFiState) time.Duration {
	// AP up but NM general status still shows disconnected (common briefly after
	// hotspot activation). Treat as AP to avoid clobbering state.
	if m.isAPActive(ctx) {
		m.booted = true
		m.apTimeoutUntil = time.Time{}
		m.setAPMode(next)
		return pollInterval
	}

	if !m.booted {
		m.booted = true
		if m.bootFastTrackAP(ctx, next) {
			return pollInterval
		}
	}

	next.Mode = ModeDisconnected
	next.SSID = ""
	next.IP = ""
	m.setState(next)

	if m.cfg.APSSID == "" {
		// No AP timeout to bound a fast cadence, so stay on the normal interval.
		m.apTimeoutUntil = time.Time{}
		return pollInterval
	}

	if m.apTimeoutUntil.IsZero() {
		m.apTimeoutUntil = time.Now().Add(time.Duration(m.cfg.APTimeoutMinutes) * time.Minute)
	}
	if time.Now().After(m.apTimeoutUntil) {
		m.log.Info("wifi: AP timeout elapsed, raising AP")
		m.activateAP(ctx)
		m.setAPMode(next)
		m.apTimeoutUntil = time.Time{}
		return pollInterval
	}
	// Counting down to AP: re-check soon to catch a reconnection (bounded by the timeout).
	return recoverPollInterval
}

// bootFastTrackAP raises the hotspot immediately at boot when no known network is
// in range, rather than waiting out the AP timeout. Reports whether it did.
func (m *Manager) bootFastTrackAP(ctx context.Context, next WiFiState) bool {
	if m.cfg.APSSID == "" {
		return false
	}
	nets, _ := m.doScan(ctx)
	if m.hasKnownNetwork(nets) {
		return false
	}
	m.log.Info("wifi: boot fast-track, no known networks, raising AP immediately")
	m.activateAP(ctx)
	m.setAPMode(next)
	return true
}

// setAPMode records the hotspot as the active link and publishes the state.
func (m *Manager) setAPMode(next WiFiState) {
	next.Mode = ModeAP
	next.SSID = m.cfg.APSSID
	next.IP = ""
	m.setState(next)
}

// disableAPFallback clears the AP config after the hotspot profile is found
// missing and publishes the cleared state, so the UI never offers a hotspot that
// can't actually be raised.
func (m *Manager) disableAPFallback(err error) {
	m.log.Warn("wifi: hotspot profile missing or unusable; AP fallback disabled", "err", err)
	m.cfg.APSSID = ""
	m.cfg.APPassword = ""
	next := m.Status()
	next.APEnabled = false
	next.APSSID = ""
	next.APHasPassword = false
	m.setState(next)
}
