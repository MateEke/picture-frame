package library

import (
	"log/slog"
	"os"
	"slices"
	"testing"
)

func openTestRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func TestOrderStoreSaveLoadRoundTrip(t *testing.T) {
	root := openTestRoot(t)
	s, names, err := LoadOrderStore(slog.Default(), root)
	if err != nil || names != nil {
		t.Fatalf("fresh load: names=%v err=%v", names, err)
	}
	want := []string{"b.jpg", "a.jpg", "c.png"}
	if err := s.Save(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	_, got, err := LoadOrderStore(slog.Default(), root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestOrderStoreCorruptRecoversEmpty(t *testing.T) {
	root := openTestRoot(t)
	if err := os.WriteFile(root.Name()+"/.order.json", []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	_, names, err := LoadOrderStore(slog.Default(), root)
	if err != nil {
		t.Fatalf("corrupt load should recover, got err %v", err)
	}
	if names != nil {
		t.Fatalf("corrupt load should yield nil names, got %v", names)
	}
}
