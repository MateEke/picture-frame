package config_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/MateEke/picture-frame/internal/config"
)

func TestStoreSnapshotIsDeepCopy(t *testing.T) {
	orig := config.Config{
		LogLevel: "info",
		Sensors: []config.SensorConfig{{
			ID:              "s1",
			Type:            "ble",
			Characteristics: []config.CharacteristicConfig{{UUID: "u", Kind: "temperature"}},
			MockReadings:    []config.MockReadingConfig{{Kind: "temperature", Value: 1}},
		}},
	}
	store := config.NewStore(orig, filepath.Join(t.TempDir(), "o.toml"))
	snap := store.Snapshot()

	snap.Sensors[0].ID = "mutated"
	snap.Sensors[0].Characteristics[0].Kind = "humidity"
	snap.Sensors[0].MockReadings[0].Value = 99

	fresh := store.Snapshot()
	if fresh.Sensors[0].ID != "s1" {
		t.Errorf("sensor id leaked: %q", fresh.Sensors[0].ID)
	}
	if fresh.Sensors[0].Characteristics[0].Kind != "temperature" {
		t.Errorf("characteristic leaked: %q", fresh.Sensors[0].Characteristics[0].Kind)
	}
	if fresh.Sensors[0].MockReadings[0].Value != 1 {
		t.Errorf("mock reading leaked: %v", fresh.Sensors[0].MockReadings[0].Value)
	}
}

func TestStorePasswordHash(t *testing.T) {
	store := config.NewStore(config.Config{Auth: config.AuthConfig{PasswordHash: "abc"}}, filepath.Join(t.TempDir(), "o.toml"))
	if got := store.PasswordHash(); got != "abc" {
		t.Errorf("got %q, want abc", got)
	}
}

func TestStoreUpdatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "o.toml")
	store := config.NewStore(config.Config{}, path)

	if err := store.Update(func(c *config.Config) error {
		c.Addr = ":9999"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if snap := store.Snapshot(); snap.Addr != ":9999" {
		t.Errorf("in-memory: got %q, want :9999", snap.Addr)
	}

	loaded, err := config.Load("/nonexistent", path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Addr != ":9999" {
		t.Errorf("on-disk: got %q, want :9999", loaded.Addr)
	}
}

func TestStoreUpdateRollbackOnFnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "o.toml")
	store := config.NewStore(config.Config{Addr: ":8080"}, path)

	sentinel := errors.New("validation failed")
	err := store.Update(func(c *config.Config) error {
		c.Addr = ":broken"
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if snap := store.Snapshot(); snap.Addr != ":8080" {
		t.Errorf("rollback failed: got %q, want :8080", snap.Addr)
	}
}

func TestStoreUpdateRollbackOnWriteError(t *testing.T) {
	store := config.NewStore(config.Config{Addr: ":8080"}, t.TempDir())

	err := store.Update(func(c *config.Config) error {
		c.Addr = ":broken"
		return nil
	})
	if err == nil {
		t.Fatal("expected write error")
	}
	if snap := store.Snapshot(); snap.Addr != ":8080" {
		t.Errorf("rollback failed: got %q, want :8080", snap.Addr)
	}
}

func TestStoreConcurrentUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "o.toml")
	store := config.NewStore(config.Config{}, path)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_ = store.Update(func(c *config.Config) error {
				c.Addr = ":0"
				return nil
			})
			_ = store.Snapshot()
			_ = store.PasswordHash()
		})
	}
	wg.Wait()
}

// Fails if a new reference-typed Config field isn't deep-copied by cloneConfig.
func TestCloneConfigCoversEveryReferenceField(t *testing.T) {
	deepCopied := map[string]bool{
		"Config.Sensors":               true,
		"SensorConfig.Characteristics": true,
		"SensorConfig.MockReadings":    true,
	}

	configPkg := reflect.TypeFor[config.Config]().PkgPath()
	seen := map[reflect.Type]bool{}
	var missing []string

	var walk func(rt reflect.Type)
	walk = func(rt reflect.Type) {
		if seen[rt] {
			return
		}
		seen[rt] = true
		for f := range rt.Fields() {
			path := rt.Name() + "." + f.Name
			switch f.Type.Kind() {
			case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Chan, reflect.Func, reflect.UnsafePointer:
				if !deepCopied[path] {
					missing = append(missing, path)
				}
				if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.Struct {
					walk(f.Type.Elem())
				}
			case reflect.Struct:
				// Recurse only into our own structs; stdlib value types copy fine.
				if f.Type.PkgPath() == configPkg {
					walk(f.Type)
				}
			}
		}
	}
	walk(reflect.TypeFor[config.Config]())

	if len(missing) > 0 {
		t.Fatalf("cloneConfig must deep-copy these reference fields (update cloneConfig and this allowlist): %v", missing)
	}
}
