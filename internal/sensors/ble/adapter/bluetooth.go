//go:build linux

package adapter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"tinygo.org/x/bluetooth"

	"github.com/MateEke/picture-frame/internal/sensors/ble"
)

// Bluetooth implements ble.Adapter using tinygo.org/x/bluetooth (BlueZ/D-Bus on Linux).
type Bluetooth struct {
	adapter   *bluetooth.Adapter
	adapterID string // e.g. "hci0"
	// connectMu serializes scan+dial: sources share one adapter and tinygo's scan
	// state isn't concurrency-safe. Coupling: an absent device holds it for the
	// whole scan window, delaying other sources' reconnects (ok for a few sensors).
	connectMu sync.Mutex
}

// New enables the default system Bluetooth adapter (hci0).
func New() (*Bluetooth, error) {
	return NewWithID("hci0")
}

// NewWithID enables the named Bluetooth adapter.
func NewWithID(id string) (*Bluetooth, error) {
	a := bluetooth.NewAdapter(id)
	if err := a.Enable(); err != nil {
		return nil, fmt.Errorf("enable bluetooth adapter %s: %w", id, err)
	}
	return &Bluetooth{adapter: a, adapterID: id}, nil
}

// Connect scans until the peripheral advertises (BlueZ only creates the device
// object then), dials it and discovers the requested UUIDs. ctx bounds the scan.
func (b *Bluetooth) Connect(ctx context.Context, mac string, addressType string, uuids []string) (ble.Device, error) {
	addr, err := parseAddress(mac, addressType)
	if err != nil {
		return nil, err
	}

	// Released before the poll loop, so sources connect in turn but poll at once.
	b.connectMu.Lock()
	defer b.connectMu.Unlock()

	// Clear a phantom Connected:yes (a Disconnect that didn't land across a re-exec):
	// tinygo's Connect short-circuits on it and hands back a dead link. Disconnect first.
	b.clearStaleConnection(ctx, addr.String())

	if err := b.scanUntilSeen(ctx, addr); err != nil {
		return nil, fmt.Errorf("scan for %s: %w", mac, err)
	}

	device, err := b.adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", mac, err)
	}

	chars, err := discoverCharacteristics(device, uuids)
	if err != nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("discover characteristics on %s: %w", mac, err)
	}

	return &bleDevice{device: device, chars: chars}, nil
}

// clearStaleConnection disconnects mac if BlueZ still shows it Connected: the
// phantom left when a Disconnect didn't land across a re-exec. Best-effort.
func (b *Bluetooth) clearStaleConnection(ctx context.Context, mac string) {
	// Shared bus tinygo's adapter also holds; reuse it, never Close it.
	conn, err := dbus.SystemBus()
	if err != nil {
		return
	}
	obj := conn.Object("org.bluez", deviceObjectPath(b.adapterID, mac))

	// Bound the calls so a wedged BlueZ can't hang Connect indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var v dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		"org.bluez.Device1", "Connected").Store(&v); err != nil {
		return
	}
	if connected, ok := v.Value().(bool); !ok || !connected {
		return
	}
	_ = obj.CallWithContext(ctx, "org.bluez.Device1.Disconnect", 0).Err
}

// deviceObjectPath builds a peripheral's BlueZ object path, matching tinygo's
// format (uppercase MAC, colons to underscores).
func deviceObjectPath(adapterID, mac string) dbus.ObjectPath {
	devID := strings.ReplaceAll(strings.ToUpper(mac), ":", "_")
	return dbus.ObjectPath(fmt.Sprintf("/org/bluez/%s/dev_%s", adapterID, devID))
}

// scanUntilSeen scans until the target is seen advertising, populating BlueZ's
// device cache so Connect works.
func (b *Bluetooth) scanUntilSeen(ctx context.Context, target bluetooth.Address) error {
	targetMAC := strings.ToUpper(target.String())

	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	found := make(chan error, 1)
	go func() {
		err := b.adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			if strings.ToUpper(result.Address.String()) == targetMAC {
				// Stop before signalling so the next Connect doesn't hit InProgress.
				_ = a.StopScan()
				// Non-blocking: a second match before StopScan lands mustn't block here.
				select {
				case found <- nil:
				default:
				}
			}
		})
		// Scan() returns nil after StopScan; only forward real errors.
		if err != nil {
			select {
			case found <- err:
			default:
			}
		}
	}()

	select {
	case err := <-found:
		return err
	case <-scanCtx.Done():
		_ = b.adapter.StopScan()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("device %s not seen within 30s scan window", targetMAC)
	}
}

// Reset power-cycles the adapter via org.bluez.Adapter1 (tinygo-bluetooth has
// no reset API; godbus does).
func (b *Bluetooth) Reset(ctx context.Context) error {
	// Same lock as Connect: power-cycling under another source's scan/dial aborts it.
	b.connectMu.Lock()
	defer b.connectMu.Unlock()

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("dbus system bus: %w", err)
	}
	// Don't Close: SystemBus is the shared connection tinygo's adapter holds for
	// the process lifetime; closing it tears down the transport it scans over.

	obj := conn.Object("org.bluez", dbus.ObjectPath("/org/bluez/"+b.adapterID))

	if err := obj.SetProperty("org.bluez.Adapter1.Powered", dbus.MakeVariant(false)); err != nil {
		return fmt.Errorf("power off %s: %w", b.adapterID, err)
	}

	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := obj.SetProperty("org.bluez.Adapter1.Powered", dbus.MakeVariant(true)); err != nil {
		return fmt.Errorf("power on %s: %w", b.adapterID, err)
	}
	return nil
}

type bleDevice struct {
	device bluetooth.Device
	chars  map[string]bluetooth.DeviceCharacteristic
}

func (d *bleDevice) Subscribe(uuid string, handler func([]byte)) error {
	c, ok := d.chars[normalizeUUID(uuid)]
	if !ok {
		return fmt.Errorf("characteristic %s not found", uuid)
	}
	return c.EnableNotifications(handler)
}

func (d *bleDevice) Read(uuid string) ([]byte, error) {
	c, ok := d.chars[normalizeUUID(uuid)]
	if !ok {
		return nil, fmt.Errorf("characteristic %s not found", uuid)
	}
	buf := make([]byte, 64)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (d *bleDevice) Disconnect() error {
	return d.device.Disconnect()
}

func parseAddress(mac string, addressType string) (bluetooth.Address, error) {
	parsed, err := bluetooth.ParseMAC(mac)
	if err != nil {
		return bluetooth.Address{}, fmt.Errorf("invalid MAC %q: %w", mac, err)
	}
	var addr bluetooth.Address
	addr.MAC = parsed
	switch strings.ToLower(strings.TrimSpace(addressType)) {
	case "random":
		addr.SetRandom(true)
	case "public", "": // empty defaults to public
	default:
		return bluetooth.Address{}, fmt.Errorf("invalid address_type %q (want \"random\" or \"public\")", addressType)
	}
	return addr, nil
}

// discoverCharacteristics returns the characteristics matching wantUUIDs,
// erroring if any requested UUID is absent.
func discoverCharacteristics(device bluetooth.Device, wantUUIDs []string) (map[string]bluetooth.DeviceCharacteristic, error) {
	want := make(map[string]struct{}, len(wantUUIDs))
	for _, u := range wantUUIDs {
		want[normalizeUUID(u)] = struct{}{}
	}

	svcs, err := device.DiscoverServices(nil)
	if err != nil {
		return nil, fmt.Errorf("discover services: %w", err)
	}

	found := make(map[string]bluetooth.DeviceCharacteristic, len(wantUUIDs))
	for _, svc := range svcs {
		chars, err := svc.DiscoverCharacteristics(nil)
		if err != nil {
			return nil, fmt.Errorf("discover characteristics in service %s: %w", svc.UUID().String(), err)
		}
		for _, c := range chars {
			key := normalizeUUID(c.UUID().String())
			if _, ok := want[key]; ok {
				found[key] = c
			}
		}
	}

	for u := range want {
		if _, ok := found[u]; !ok {
			return nil, fmt.Errorf("characteristic %s not found on device", u)
		}
	}
	return found, nil
}

func normalizeUUID(u string) string {
	return strings.ToLower(u)
}
