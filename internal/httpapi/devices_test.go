package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// fakeSysfs builds a sysfs-like tree under a temp dir and returns its base.
// Directory names mirror /sys/class entries the enumerator globs; DRM connectors
// carry a "status" file so the "connected"-only filter can be exercised.
func fakeSysfs(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	// connector dir -> status file contents ("" means no status file written).
	connectors := map[string]string{
		"card0-HDMI-A-1": "connected\n",
		"card0-eDP-1":    "connected\n",
		"card1-DP-1":     "disconnected\n", // plugged-in port absent: excluded
		"card1-HDMI-A-1": "connected\n",    // same connector, second card: deduped
		"card1-VGA-1":    "",               // no status file: excluded
		"card2-":         "connected\n",    // empty connector name: skipped
	}
	mkdir := func(rel string) {
		if err := os.MkdirAll(filepath.Join(base, rel), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	mkdir("bluetooth/hci0")
	mkdir("bluetooth/hci1")
	mkdir("bluetooth/hci1:71") // live connection node, not a controller: excluded
	mkdir("drm/card0")         // no connector suffix: not matched by card*-*
	mkdir("drm/renderD128")    // unrelated node
	for dir, status := range connectors {
		mkdir(filepath.Join("drm", dir))
		if status != "" {
			path := filepath.Join(base, "drm", dir, "status")
			if err := os.WriteFile(path, []byte(status), 0o600); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}
	}
	return base
}

func TestEnumerateDevicesViaRoute(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		SysfsBase:   fakeSysfs(t),
	})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/devices", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var body httpapi.SystemDevicesBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantBT := []string{"hci0", "hci1"}
	if !reflect.DeepEqual(body.BluetoothAdapters, wantBT) {
		t.Errorf("bluetooth_adapters: got %v, want %v", body.BluetoothAdapters, wantBT)
	}
	// Only connected connectors, sorted (ASCII: uppercase before lowercase) and
	// deduped across cards; DP-1 (disconnected) and VGA-1 (no status) are excluded.
	wantOut := []string{"HDMI-A-1", "eDP-1"}
	if !reflect.DeepEqual(body.DisplayOutputs, wantOut) {
		t.Errorf("display_outputs: got %v, want %v", body.DisplayOutputs, wantOut)
	}
}

func TestEnumerateDevicesMissingSysfsIsEmpty(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		SysfsBase:   filepath.Join(t.TempDir(), "absent"),
	})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/devices", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var body httpapi.SystemDevicesBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.BluetoothAdapters) != 0 {
		t.Errorf("bluetooth_adapters: got %v, want empty", body.BluetoothAdapters)
	}
	if len(body.DisplayOutputs) != 0 {
		t.Errorf("display_outputs: got %v, want empty", body.DisplayOutputs)
	}
}
