package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	displaypkg "github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/kioskwatch"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/library/adapter/immich"
	"github.com/MateEke/picture-frame/internal/slideplan"
	"github.com/MateEke/picture-frame/internal/slideshow"
	"github.com/MateEke/picture-frame/internal/startup"
	"github.com/MateEke/picture-frame/internal/updater"
	updateradapter "github.com/MateEke/picture-frame/internal/updater/adapter"
	updatermock "github.com/MateEke/picture-frame/internal/updater/mock"
	"github.com/MateEke/picture-frame/internal/version"
	"github.com/MateEke/picture-frame/internal/weather"
	"github.com/MateEke/picture-frame/internal/wifi"
	wifimock "github.com/MateEke/picture-frame/internal/wifi/mock"
)

func isProduction() bool {
	env := os.Getenv("GO_ENV")
	return strings.EqualFold(env, "prod") || strings.EqualFold(env, "production")
}

// newDisplay returns the panel controller and intent store: real hardware plus a
// persisted intent store in prod, a no-op mock (no store) in development.
func newDisplay(production bool, log *slog.Logger, cfg *config.Config, screenStatePath string) (displaypkg.Controller, displaypkg.IntentStore, error) {
	if !production {
		log.Info("using no-op display controller (development mode)")
		m := &displaypkg.Mock{}
		m.SetOn(true)
		return m, nil, nil
	}
	display, err := startup.NewDisplayController(log, cfg.Display)
	if err != nil {
		return nil, nil, err
	}
	return display, displaypkg.NewFileIntentStore(screenStatePath), nil
}

// kioskTimeoutFunc returns the watchdog's on-loss action: exit (→ systemd
// restart) in prod, a warning no-op in dev.
func kioskTimeoutFunc(production bool, log *slog.Logger) func() {
	if production {
		return func() {
			log.Error("exiting due to lost kiosk heartbeat")
			os.Exit(1)
		}
	}
	return func() {
		log.Warn("kiosk heartbeat lost (no-op in dev; would exit in prod)")
	}
}

// buildWiFiManager returns the real nmcli manager in prod, an in-memory mock in
// dev, or nil (wifi routes then serve 503) when WIFI_MOCK=off.
func buildWiFiManager(ctx context.Context, log *slog.Logger, cfg *config.Config, production bool, store *config.Store) (httpapi.WiFiManager, error) {
	if !production {
		if strings.EqualFold(os.Getenv("WIFI_MOCK"), "off") {
			log.Info("wifi: disabled (development, WIFI_MOCK=off), routes serve 503")
			return nil, nil
		}
		log.Info("wifi: using in-memory mock (development mode)")
		hostname, _ := os.Hostname()
		return wifimock.NewDefault(hostname), nil
	}
	if cfg.WiFi.ScanIntervalMinutes <= 0 {
		return nil, fmt.Errorf("wifi.scan_interval_minutes must be positive")
	}
	mgr := wifi.New(log, wifi.Config{
		APTimeoutMinutes:    cfg.WiFi.APTimeoutMinutes,
		ScanIntervalMinutes: cfg.WiFi.ScanIntervalMinutes,
		APSSID:              cfg.WiFi.APSSID,
		APPassword:          cfg.WiFi.APPassword,
		Store:               store,
	})
	go mgr.Run(ctx)
	return mgr, nil
}

func libraryBackend(cfg *config.Config) string {
	if cfg.Library.Backend == "" {
		return config.BackendFS
	}
	return cfg.Library.Backend
}

func libraryDir(cfg *config.Config) string {
	if libraryBackend(cfg) == config.BackendImmich {
		return filepath.Join(cfg.Slideshow.ImagesDir, config.BackendImmich)
	}
	return cfg.Slideshow.ImagesDir
}

// startLibrarySyncer starts the remote-backend syncer in a goroutine when one
// is configured and returns it for status exposure. fs backend → (nil, nil).
func startLibrarySyncer(ctx context.Context, log *slog.Logger, cfg *config.Config, lib *library.Library, root *os.Root, slides *slideshow.Slideshow, aspect *library.AspectStore) (*library.Syncer, error) {
	if libraryBackend(cfg) != config.BackendImmich {
		return nil, nil
	}
	client, err := immich.New(immich.Config{
		ShareURL: cfg.Library.Immich.ShareURL,
		Password: cfg.Library.Immich.SharePassword,
	})
	if err != nil {
		return nil, err
	}
	interval := cfg.Library.Immich.SyncInterval.Duration
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	syncer := library.NewSyncer(log, client, lib, root, interval, slides, library.WithAspectStore(aspect))
	go syncer.Run(ctx)
	log.Info("library: immich syncer started", "share_url", cfg.Library.Immich.ShareURL, "interval", interval)
	return syncer, nil
}

// startUpdater wires the self-updater: real GitHub-backed adapters in production, a
// binary-free simulator in development. Returns nil (the API then reports no update
// available) when production has no github_repo configured.
func startUpdater(ctx context.Context, log *slog.Logger, cfg *config.Config, production bool, restart func() error) httpapi.UpdaterStatus {
	if !production {
		mock := updatermock.New(updatermock.Options{
			Current:  version.Version,
			Platform: version.Platform,
			Latest:   os.Getenv("UPDATER_MOCK_LATEST"),
			Outcome:  os.Getenv("UPDATER_MOCK_OUTCOME"),
			Offline:  os.Getenv("UPDATER_MOCK_OFFLINE") == "1",
			Delay:    mockUpdaterDelay(),
		})
		go mock.Run(ctx)
		log.Info("updater: using mock simulator (development mode)")
		return mock
	}
	// github_repo overrides the built-in default (the repo this binary was released from);
	// with neither, there's nothing to update from.
	repo := cfg.Updater.GithubRepo
	if repo == "" {
		repo = version.UpdateRepo
	}
	if repo == "" {
		log.Info("updater: no release source (github_repo) configured; self-update disabled")
		return nil
	}
	pubKey, err := updater.EmbeddedKey()
	if err != nil {
		log.Error("updater: embedded signing key failed to parse; self-update disabled", "err", err)
		return nil
	}
	// Optional GitHub token (config wins over env): authenticates a private release source
	// and lifts the unauthenticated rate limit. Unset for a public repo.
	token := cfg.Updater.GithubToken
	if token == "" {
		token = os.Getenv("PF_GITHUB_TOKEN")
	}
	u := updater.New(updater.Options{
		Log:        log,
		Source:     updateradapter.NewGitHub(repo, token),
		Downloader: updateradapter.NewHTTPDownloader(token),
		Preflight:  updateradapter.ExecPreflight{},
		PubKey:     pubKey,
		Current:    version.Version,
		Platform:   version.Platform,
		AutoUpdate: cfg.Updater.AutoUpdate,
		UpdateHour: cfg.Updater.UpdateHour,
		InstallDir: updaterInstallDir(),
		Restart:    restart,
	})
	go u.Run(ctx)
	log.Info("updater: started", "repo", repo, "authenticated", token != "",
		"auto_update", cfg.Updater.AutoUpdate, "hour", cfg.Updater.UpdateHour)
	return u
}

// mockUpdaterDelay is the per-phase delay for the dev simulator (800ms so phases are
// visible by hand); e2e sets UPDATER_MOCK_DELAY=0 for instant, deterministic transitions.
func mockUpdaterDelay() time.Duration {
	if v := os.Getenv("UPDATER_MOCK_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 800 * time.Millisecond
}

// liveConfigImpl calls Tier-1 setters on the running subsystems.
type liveConfigImpl struct {
	slideshow *slideshow.Slideshow
	policy    *displaypkg.Policy
	weather   *weather.Poller // may be nil if api_key not configured
	logLevel  *slog.LevelVar
	log       *slog.Logger
}

func (l *liveConfigImpl) ApplyLive(cfg config.Config) {
	l.slideshow.SetInterval(cfg.Slideshow.Interval.Duration)
	l.slideshow.SetRandomize(cfg.Slideshow.Randomize)
	l.slideshow.SetSplitConfig(cfg.Slideshow.SplitScreen, slideplan.Threshold{Factor: cfg.Slideshow.PairThreshold})
	l.policy.SetBlankAfter(cfg.Display.BlankAfter.Duration)
	if l.weather != nil {
		l.weather.SetIntervals(cfg.Weather.PollInterval.Duration, cfg.Weather.RetryInterval.Duration)
	}
	l.applyLogLevel(cfg.LogLevel)
}

// applyLogLevel updates the running slog level. An empty value resets to info;
// an unparseable value is logged and the current level is kept. The LOG_LEVEL
// env override only applies at startup, a live edit reflects the saved config.
func (l *liveConfigImpl) applyLogLevel(level string) {
	if level == "" {
		l.logLevel.Set(slog.LevelInfo)
		return
	}
	if err := l.logLevel.UnmarshalText([]byte(level)); err != nil {
		l.log.Warn("invalid log_level, keeping current", "value", level, "err", err)
	}
}

// watchUpdateCommit, when this process booted into a freshly-applied binary, commits the
// update on the first kiosk heartbeat (proof the new frontend renders) and otherwise exits
// within the window so systemd's StartLimit → OnFailure rollback restores the old binary.
func watchUpdateCommit(ctx context.Context, log *slog.Logger, watch *kioskwatch.Watch) {
	dir := updaterInstallDir()
	if !updater.MarkerPresent(dir) {
		return
	}
	committed := make(chan struct{})
	// Commit only once the *new* build beats: the old page keeps heartbeating its old version
	// until the SSE version-change reloads it, and committing then wouldn't prove the new UI renders.
	watch.OnBeat(func(beatVersion string) bool {
		if beatVersion != version.Version {
			return false // still the old frontend; keep waiting for the reload
		}
		if err := updater.Commit(dir); err != nil {
			log.Error("update: commit failed", "err", err)
		}
		log.Info("update verified by the new build's kiosk heartbeat; committed")
		close(committed)
		return true
	})
	go func() {
		const window = 2 * time.Minute
		select {
		case <-committed:
		case <-ctx.Done():
		case <-time.After(window):
			log.Error("update: no kiosk heartbeat in the verification window; exiting for rollback")
			os.Exit(1)
		}
	}()
}

// updaterInstallDir is the directory holding the running binary (and its .bak/markers).
// It strips Linux's " (deleted)" suffix that appears after the binary was swapped.
func updaterInstallDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(strings.TrimSuffix(exe, " (deleted)"))
}
