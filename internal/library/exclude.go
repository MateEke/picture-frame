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

// Leading dot for the same reason as orderIndexName.
const excludeIndexName = ".slideshow-exclude.json"

// ExcludeStore persists the names hidden from playback as a JSON array sidecar
// in the images directory. Safe for concurrent use.
type ExcludeStore struct {
	root    *os.Root
	flushMu sync.Mutex
}

// LoadExcludeStore reads the saved exclusions from root; returns nil names when absent or corrupt.
func LoadExcludeStore(log *slog.Logger, root *os.Root) (*ExcludeStore, []string, error) {
	s := &ExcludeStore{root: root}
	f, err := root.OpenFile(excludeIndexName, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("library: open exclude index: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Warn("library: read exclude index failed, ignoring", "err", err)
		return s, nil, nil
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		log.Warn("library: parse exclude index failed, ignoring", "err", err)
		return s, nil, nil
	}
	return s, names, nil
}

// Save writes the exclusions atomically through the images root.
func (s *ExcludeStore) Save(names []string) error {
	data, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("library: marshal exclude index: %w", err)
	}
	return writeFileAtomic(s.root, excludeIndexName, &s.flushMu, data)
}
