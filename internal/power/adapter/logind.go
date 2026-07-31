package adapter

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/MateEke/picture-frame/internal/power"
)

const (
	logindDest  = "org.freedesktop.login1"
	logindPath  = dbus.ObjectPath("/org/freedesktop/login1")
	logindIface = "org.freedesktop.login1.Manager"
)

// Logind implements power.Logind over godbus' shared system bus, which is
// deliberately never closed: the BLE adapter broke when it was.
type Logind struct{ obj dbus.BusObject }

func NewLogind() (*Logind, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect system bus: %w", err)
	}
	return &Logind{obj: conn.Object(logindDest, logindPath)}, nil
}

// Can returns logind's polkit-aware verdict: "yes", "no", "challenge" or "na".
func (l *Logind) Can(ctx context.Context, a power.Action) (string, error) {
	var verdict string
	call := l.obj.CallWithContext(ctx, canMethod(a), 0)
	if call.Err != nil {
		return "", fmt.Errorf("%s: %w", canMethod(a), call.Err)
	}
	if err := call.Store(&verdict); err != nil {
		return "", fmt.Errorf("%s: decode reply: %w", canMethod(a), err)
	}
	return verdict, nil
}

// interactive=false: an auth agent this service never has would hang the call.
func (l *Logind) Do(ctx context.Context, a power.Action) error {
	if call := l.obj.CallWithContext(ctx, doMethod(a), 0, false); call.Err != nil {
		return fmt.Errorf("%s: %w", doMethod(a), call.Err)
	}
	return nil
}

func canMethod(a power.Action) string { return logindIface + ".Can" + string(a) }
func doMethod(a power.Action) string  { return logindIface + "." + string(a) }
