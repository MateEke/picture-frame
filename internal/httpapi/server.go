package httpapi

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/MateEke/picture-frame/internal/auth"
	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/web"
)

// ScreenController is the manual screen-control surface used by the HTTP API,
// implemented by *display.Screen.
type ScreenController interface {
	On(ctx context.Context) error
	Off(ctx context.Context) error
	State() bool
	Auto() bool
	// Reconcile re-asserts desired power; the SSE handler calls it on connect
	// (a kiosk reconnect may follow a compositor restart).
	Reconcile(ctx context.Context)
}

// SlideshowController allows the HTTP layer to trigger immediate advances.
type SlideshowController interface {
	Next()
}

// KioskBeater records a kiosk heartbeat (with the reporting build's version); implemented
// by *kioskwatch.Watch.
type KioskBeater interface {
	Beat(version string)
}

// SyncerStatus reports the current state of a remote library backend and lets
// the admin request an out-of-band sync. Aliased to the library-owned interface
// so producers (e.g. startup) can return it without importing this package.
type SyncerStatus = library.SyncerStatus

// Config holds the dependencies for the HTTP server.
type Config struct {
	Log         *slog.Logger
	Screen      ScreenController
	Bus         *state.Bus
	Library     *library.Library
	Slideshow   SlideshowController
	ImagesRoot  *os.Root
	KioskBeater KioskBeater // required
	// Backend is the active library backend, e.g. "fs" or "immich".
	Backend string
	// Syncer is non-nil for remote backends; exposes sync status to the UI.
	Syncer SyncerStatus
	// Updater is non-nil when the self-updater is wired; nil reads as no update available.
	Updater UpdaterStatus
	// WiFi is non-nil in production; nil in dev returns 503 for wifi routes.
	WiFi WiFiManager
	// Production serves the embedded build when true, else proxies to Vite.
	Production bool
	// WeatherActive gates the kiosk weather UI; see startup.WeatherEnabled.
	WeatherActive bool
	// SysfsBase roots device enumeration (Bluetooth adapters, display outputs);
	// "" defaults to /sys/class. Overridden in tests with a fake sysfs tree.
	SysfsBase string

	// Config editing
	Store         *config.Store // persisted config state; nil in tests that don't exercise config
	RunningConfig config.Config // initial snapshot of what live subsystems reflect (== saved at startup)
	LiveConfig    LiveConfig    // nil in dev/tests; applies Tier-1 changes in-process
	Restart       func() error  // nil in dev/tests; triggers an in-place re-exec.
}

type server struct {
	log           *slog.Logger
	screen        ScreenController
	bus           *state.Bus
	lib           *library.Library
	slideshow     SlideshowController
	imagesRoot    *os.Root
	kioskBeater   KioskBeater
	backend       string
	syncer        SyncerStatus
	updater       UpdaterStatus
	wifiMgr       WiFiManager
	sysfsBase     string
	weatherActive bool
	auth          *auth.Authenticator
	authMu        sync.Mutex // serializes bcrypt work; see checkPasswordGated

	// cookie-less kiosk routes; see kioskExempt
	kioskPaths    map[string]bool
	kioskPrefixes []string

	store      *config.Store
	mu         sync.RWMutex // guards running only
	running    config.Config
	liveConfig LiveConfig
	restart    func() error
}

// NewServer constructs the root HTTP handler. With Production set it serves the
// embedded SvelteKit build (SPA fallback); otherwise it proxies to Vite.
func NewServer(cfg Config) http.Handler {
	backend := cfg.Backend
	if backend == "" {
		backend = config.BackendFS
	}
	sysfsBase := cfg.SysfsBase
	if sysfsBase == "" {
		sysfsBase = "/sys/class"
	}
	s := &server{
		log:           cfg.Log,
		screen:        cfg.Screen,
		bus:           cfg.Bus,
		lib:           cfg.Library,
		slideshow:     cfg.Slideshow,
		imagesRoot:    cfg.ImagesRoot,
		kioskBeater:   cfg.KioskBeater,
		backend:       backend,
		syncer:        cfg.Syncer,
		updater:       cfg.Updater,
		wifiMgr:       cfg.WiFi,
		sysfsBase:     sysfsBase,
		weatherActive: cfg.WeatherActive,
		auth:          auth.New(),
		store:         cfg.Store,
		running:       cfg.RunningConfig,
		liveConfig:    cfg.LiveConfig,
		restart:       cfg.Restart,
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.requireAuth)

	humaConfig := huma.DefaultConfig("Picture Frame", "1.0.0")
	humaConfig.Info.Description = "Picture frame kiosk API"
	api := humachi.New(r, humaConfig)

	s.registerRoutes(api)

	// OS captive-portal probe URLs, redirect to /admin/wifi when AP is active.
	for _, path := range captiveProbes {
		r.Get(path, s.handleCaptiveProbe)
	}

	if cfg.Production {
		cfg.Log.Info("serving embedded build (production mode)")
		r.Handle("/*", newSPAHandler(cfg.Log))
	} else {
		cfg.Log.Info("proxying to Vite dev server (development mode)")
		r.Handle("/*", newViteProxy(cfg.Log))
	}

	return r
}

func (s *server) registerRoutes(api huma.API) {
	s.registerScreenRoutes(api)
	s.registerLibraryRoutes(api)
	s.registerImageRoutes(api)
	s.registerSlideshowRoutes(api)
	s.registerHeartbeatRoutes(api)
	s.registerWiFiRoutes(api)
	s.registerHealthRoutes(api)
	s.registerSSERoutes(api)
	s.registerConfigRoutes(api)
	s.registerDeviceRoutes(api)
	s.registerSystemInfoRoutes(api)
	s.registerUpdateRoutes(api)
	s.registerAuthRoutes(api)
}

func newSPAHandler(log *slog.Logger) http.Handler {
	sub, err := fs.Sub(web.BuildFS, "build")
	if err != nil {
		log.Error("failed to create sub FS from embedded build", "err", err)
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// The fallback serves index.html, never cacheable as the asset path.
			w.Header().Set("Cache-Control", "no-cache")
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		// embed.FS files have a zero modtime (no implicit validators), so set
		// caching explicitly: content-hashed assets forever, the rest revalidates.
		if strings.HasPrefix(path, "_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func newViteProxy(log *slog.Logger) http.Handler {
	target, _ := url.Parse("http://localhost:5173")
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error("vite proxy error", "path", r.URL.Path, "err", err)
		http.Error(w, "Vite dev server unavailable, run: cd web && npm run dev", http.StatusBadGateway)
	}
	return proxy
}
