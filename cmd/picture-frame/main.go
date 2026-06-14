package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/MateEke/picture-frame/internal/config"
	displaypkg "github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/kioskwatch"
	libadapter "github.com/MateEke/picture-frame/internal/library/adapter"
	"github.com/MateEke/picture-frame/internal/mqtt"
	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/slideshow"
	"github.com/MateEke/picture-frame/internal/startup"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/version"
	"github.com/MateEke/picture-frame/internal/weather"
	"github.com/MateEke/picture-frame/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
}

// run wires up every subsystem and blocks until shutdown or a restart request.
// Returning an error (not os.Exit) lets deferred cleanup run and routes failures through main.
//
//nolint:funlen // composition root: a linear wiring sequence read top to bottom.
func run() error {
	configPath := flag.String("config", "config.toml", "path to user config file")
	overridesPath := flag.String("overrides", "runtime-overrides.toml", "path to runtime overrides file")
	screenStatePath := flag.String("screen-state", "screen-state", "path to the persisted manual screen on/off state")
	healthCheck := flag.Bool("health-check", false, "run a startup self-test and exit (the updater's pre-flight)")
	hashPW := flag.Bool("hash-password", false, "read a password from stdin, print its bcrypt hash, and exit (used by install.sh)")
	flag.Parse()

	// Before any logging, so stdout carries only the hash.
	if *hashPW {
		return hashPassword(os.Stdin, os.Stdout)
	}

	var levelVar slog.LevelVar
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &levelVar}))
	log.Info("starting picture-frame", "version", version.Version, "platform", version.Platform)
	// A release whose embedded frontend was stamped with a different version fails every
	// self-update: the commit gate matches the kiosk heartbeat's version to this binary's.
	if version.Version != "dev" {
		if baked := web.BakedVersion(); baked != version.Version {
			log.Error("frontend/backend version mismatch; self-updates will roll back, rebuild with matching PUBLIC_APP_VERSION",
				"frontend", baked, "backend", version.Version)
		}
	}

	cfg, err := config.Load(*configPath, *overridesPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	// Updater pre-flight: config parsed + validated above, now prove the server serves.
	// Exits without starting the long-running services (no socket, MQTT, or display).
	if *healthCheck {
		return startup.HealthCheck(log)
	}
	startup.ApplyConfiguredLogLevel(&levelVar, cfg, log)
	log.Info("config loaded", "addr", cfg.Addr, "sensors", len(cfg.Sensors), "logLevel", levelVar.Level())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// restartCh carries re-exec requests from POST /api/system/restart. Re-exec is
	// a healthy restart (syscall.Exec) and deliberately bypasses the kiosk
	// watchdog's os.Exit path, which counts against systemd's StartLimitBurst.
	restartCh := make(chan struct{}, 1)
	production := isProduction()

	display, intentStore, err := newDisplay(production, log, cfg, *screenStatePath)
	if err != nil {
		return fmt.Errorf("init display: %w", err)
	}

	// State bus: producer events fan out to every SSE client.
	bus := state.NewBus()
	// Seed the kiosk snapshot so it reaches every new SSE client on connect.
	weatherActive := startup.WeatherEnabled(cfg, production)
	bus.Publish(state.Event{Kind: state.KindKiosk, Payload: httpapi.KioskEventPayload(*cfg, weatherActive)})

	// Motion policy owns panel power. Start reconciles to persisted intent + actual
	// state rather than blindly powering on (see Policy).
	policy := displaypkg.NewPolicy(displaypkg.PolicyConfig{
		Log:             log,
		Display:         display,
		Bus:             bus,
		Store:           intentStore,
		BlankAfter:      cfg.Display.BlankAfter.Duration,
		MotionAvailable: config.HasMotionSensor(cfg.Sensors), // idle-blank needs a sensor to wake it
	})
	policy.Start(ctx)
	go policy.Run(ctx)

	// Screen: manual on/off shared by the HTTP API and MQTT bridge.
	screen := displaypkg.NewScreen(policy)

	libDir := libraryDir(cfg)
	lib, err := libadapter.Load(log, libDir, cfg.Slideshow.Randomize)
	if err != nil {
		return fmt.Errorf("load image library: %w", err)
	}
	log.Info("image library loaded", "count", lib.Len(), "dir", libDir, "backend", libraryBackend(cfg))

	imagesRoot, err := os.OpenRoot(libDir)
	if err != nil {
		return fmt.Errorf("open images root: %w", err)
	}
	defer imagesRoot.Close()

	slides := slideshow.New(log, lib, bus, cfg.Slideshow.Interval.Duration)
	go slides.Run(ctx)

	librarySyncer, err := startLibrarySyncer(ctx, log, cfg, lib, imagesRoot, slides)
	if err != nil {
		return fmt.Errorf("start library syncer: %w", err)
	}

	store := config.NewStore(*cfg, *overridesPath)

	wifiMgr, err := buildWiFiManager(ctx, log, cfg, production, store)
	if err != nil {
		return fmt.Errorf("init wifi: %w", err)
	}

	var weatherPoller *weather.Poller
	if fetcher := startup.BuildWeatherFetcher(log, cfg, production); fetcher != nil {
		weatherPoller = weather.New(log, fetcher, bus, cfg.Weather.PollInterval.Duration, cfg.Weather.RetryInterval.Duration)
		go weatherPoller.Run(ctx)
	}

	// Kiosk heartbeat watchdog: on loss, exit (→ systemd restart) in prod, no-op in dev.
	kioskWatch := kioskwatch.New(log, 5*time.Minute, kioskTimeoutFunc(production, log))
	go kioskWatch.Run(ctx)

	// If we booted into a just-applied binary, commit it the moment the kiosk renders the
	// new frontend (first heartbeat); if that never comes within the window, exit so
	// systemd's StartLimit → OnFailure rollback restores the previous binary.
	if production {
		watchUpdateCommit(ctx, log, kioskWatch)
	}

	// MQTT must come before sources (which register subs) and before Connect.
	mqttHub, pubDone := setupMQTT(ctx, log, cfg, bus, screen)

	sources := buildSources(log, cfg, mqttHub)
	roles := config.SensorRoles(cfg.Sensors)
	registry := sensors.NewRegistry(log, sources, func(reading sensors.Reading) {
		bus.Publish(state.Event{
			Kind: state.KindSensor,
			Payload: state.SensorPayload{
				DeviceID:  reading.DeviceID,
				Role:      roles[reading.DeviceID],
				Kind:      string(reading.Kind),
				Value:     reading.Value,
				Timestamp: reading.Timestamp,
			},
		})
	})

	if mqttHub != nil {
		go mqttHub.Connect(ctx)
	}

	liveCfg := &liveConfigImpl{slideshow: slides, policy: policy, weather: weatherPoller, logLevel: &levelVar, log: log}

	restartFn := startup.MakeRestartFunc(restartCh)
	updaterSvc := startUpdater(ctx, log, cfg, production, restartFn)

	srv := &http.Server{
		Addr:              cfg.Addr,
		ReadHeaderTimeout: 10 * time.Second,
		Handler: httpapi.NewServer(httpapi.Config{
			Log:           log,
			Screen:        screen,
			Bus:           bus,
			Library:       lib,
			Slideshow:     slides,
			ImagesRoot:    imagesRoot,
			KioskBeater:   kioskWatch,
			Backend:       libraryBackend(cfg),
			Syncer:        startup.SyncerStatus(librarySyncer),
			Updater:       updaterSvc,
			WiFi:          wifiMgr,
			Production:    production,
			WeatherActive: weatherActive,
			Store:         store,
			RunningConfig: *cfg,
			LiveConfig:    liveCfg,
			Restart:       restartFn,
		}),
	}

	startSystemdWatchdog(log)

	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("bind %s: %w", srv.Addr, err)
	}

	if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		log.Error("failed to notify systemd ready", "err", err)
	}

	go registry.Run(ctx)

	serveErr := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", srv.Addr)
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	return awaitShutdown(ctx, stop, log, srv, serveErr, restartCh, pubDone, mqttHub)
}

// awaitShutdown blocks until the server fails, the context is cancelled, or a
// restart is requested, then shuts down gracefully. It returns a non-nil error
// only on server failure or a failed re-exec.
func awaitShutdown(ctx context.Context, stop context.CancelFunc, log *slog.Logger, srv *http.Server, serveErr <-chan error, restartCh <-chan struct{}, pubDone <-chan struct{}, mqttHub *mqtt.Hub) error {
	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		gracefulShutdown(log, srv, pubDone, mqttHub, true)
		return nil
	case <-restartCh:
		stop() // cancel context so all subsystems begin winding down
		// false: a re-exec keeps the PID, so don't send STOPPING (see gracefulShutdown).
		gracefulShutdown(log, srv, pubDone, mqttHub, false)
		log.Info("re-executing for restart")
		return reexec()
	}
}
