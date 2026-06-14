package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/MateEke/picture-frame/internal/mqtt"
)

func gracefulShutdown(log *slog.Logger, srv *http.Server, pubDone <-chan struct{}, mqttHub *mqtt.Hub, notifyStopping bool) {
	log.Info("shutting down")
	// STOPPING means we're truly exiting. A re-exec keeps the PID, so sending it would leave
	// systemd waiting for an exit that never comes, then SIGKILL us, only real-exit paths set it.
	if notifyStopping {
		if _, err := daemon.SdNotify(false, daemon.SdNotifyStopping); err != nil {
			log.Error("failed to notify systemd stopping", "err", err)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// SSE connections are long-lived and never become idle, so Shutdown always
		// times out when clients are connected. Force-close remaining connections;
		// this cancels their request contexts and lets SSE handlers return cleanly.
		_ = srv.Close()
	}
	<-pubDone // bridge-offline must flush before disconnect
	if mqttHub != nil {
		mqttHub.Disconnect()
	}
}

func startSystemdWatchdog(log *slog.Logger) {
	interval, err := daemon.SdWatchdogEnabled(false)
	if err != nil || interval <= 0 {
		return
	}
	go func() {
		for {
			if _, err := daemon.SdNotify(false, daemon.SdNotifyWatchdog); err != nil {
				log.Error("failed to notify systemd watchdog", "err", err)
			}
			time.Sleep(interval / 2)
		}
	}()
}

// reexec replaces the current process image with the same binary and args.
// filepath.EvalSymlinks resolves the executable to its real path so the kernel
// receives a clean, non-symlinked path (required by gosec G702). On success it
// does not return, the process image is replaced.
func reexec() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("re-exec: resolve executable: %w", err)
	}
	// After the updater atomically swaps the binary, the original inode is unlinked and
	// Linux reports os.Executable() with a " (deleted)" suffix. Strip it so we exec the
	// new file now living at that path rather than the gone inode (which EvalSymlinks
	// would fail to resolve).
	exe = strings.TrimSuffix(exe, " (deleted)")
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("re-exec: resolve symlinks: %w", err)
	}
	// G702: exe is the current binary resolved through EvalSymlinks; os.Args are
	// the flags this process was launched with, not user-supplied HTTP input.
	//nolint:gosec
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("re-exec: %w", err)
	}
	return nil
}
