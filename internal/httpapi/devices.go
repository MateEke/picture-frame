package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// hciAdapterRe matches a controller node ("hci0") but not a connection node
// ("hci1:71"), both of which live under /sys/class/bluetooth.
var hciAdapterRe = regexp.MustCompile(`^hci[0-9]+$`)

// SystemDevicesBody enumerates hardware the settings UI offers in device selects.
// Both lists may be empty (the device class is absent or the path is unreadable);
// the UI seeds an allow-custom combobox so a configured-but-absent device is kept.
type SystemDevicesBody struct {
	BluetoothAdapters []string `json:"bluetooth_adapters" doc:"HCI device IDs, e.g. hci0"`
	DisplayOutputs    []string `json:"display_outputs" doc:"Connected display connectors, e.g. HDMI-A-1"`
}

type getSystemDevicesOutput struct {
	Body SystemDevicesBody
}

func (s *server) registerDeviceRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-system-devices",
		Method:      http.MethodGet,
		Path:        "/api/system/devices",
		Summary:     "Enumerate Bluetooth adapters and display outputs from sysfs",
	}, func(_ context.Context, _ *struct{}) (*getSystemDevicesOutput, error) {
		return &getSystemDevicesOutput{Body: enumerateDevices(s.sysfsBase)}, nil
	})
}

// enumerateDevices reads adapter/output identifiers from a sysfs tree rooted at
// base (/sys/class in production, a temp tree in tests). A missing directory
// yields an empty list, the device simply isn't present, never an error.
func enumerateDevices(base string) SystemDevicesBody {
	return SystemDevicesBody{
		BluetoothAdapters: bluetoothAdapters(base),
		DisplayOutputs:    displayOutputs(base),
	}
}

// bluetoothAdapters lists hci controller names under <base>/bluetooth, skipping
// the per-connection nodes (hci<N>:<handle>) that also appear there.
func bluetoothAdapters(base string) []string {
	set := map[string]bool{}
	matches, _ := filepath.Glob(filepath.Join(base, "bluetooth", "hci*"))
	for _, m := range matches {
		name := filepath.Base(m)
		if !hciAdapterRe.MatchString(name) {
			continue
		}
		set[name] = true
	}
	return sortedKeys(set)
}

// displayOutputs lists DRM connectors with a display attached, under <base>/drm.
// A connector dir (card<N>-<connector>) exists per port whether or not anything
// is plugged in; its "status" file ("connected"/"disconnected") is the filter, so
// only "connected" ones are returned.
func displayOutputs(base string) []string {
	set := map[string]bool{}
	matches, _ := filepath.Glob(filepath.Join(base, "drm", "card*-*"))
	for _, m := range matches {
		_, connector, _ := strings.Cut(filepath.Base(m), "-")
		if connector == "" {
			continue
		}
		status, err := os.ReadFile(filepath.Join(m, "status"))
		if err != nil || strings.TrimSpace(string(status)) != "connected" {
			continue
		}
		set[connector] = true
	}
	return sortedKeys(set)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
