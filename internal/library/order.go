package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"
)

// The leading dot keeps this out of the fs loader, syncedNameRe, and cleanTmp.
const orderIndexName = ".order.json"

// OrderStore persists the canonical image order as a JSON array sidecar in the
// images directory. Safe for concurrent use.
type OrderStore struct {
	root    *os.Root
	flushMu sync.Mutex
}

// LoadOrderStore reads the saved order from root; returns nil names when absent or corrupt.
func LoadOrderStore(log *slog.Logger, root *os.Root) (*OrderStore, []string, error) {
	s := &OrderStore{root: root}
	f, err := root.OpenFile(orderIndexName, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("library: open order index: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Warn("library: read order index failed, ignoring", "err", err)
		return s, nil, nil
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		log.Warn("library: parse order index failed, ignoring", "err", err)
		return s, nil, nil
	}
	return s, names, nil
}

// Save writes the order atomically (tmp + rename) through the images root.
func (s *OrderStore) Save(names []string) error {
	data, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("library: marshal order index: %w", err)
	}
	return writeFileAtomic(s.root, orderIndexName, &s.flushMu, data)
}

// writeFileAtomic writes data to name within root via a temp file + rename, so a
// reader never sees a partial file. mu serializes writers sharing the temp name.
func writeFileAtomic(root *os.Root, name string, mu *sync.Mutex, data []byte) error {
	mu.Lock()
	defer mu.Unlock()
	tmp := name + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("library: create %s: %w", name, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = root.Remove(tmp)
		return fmt.Errorf("library: write %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("library: close %s: %w", name, err)
	}
	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("library: rename %s: %w", name, err)
	}
	return nil
}
