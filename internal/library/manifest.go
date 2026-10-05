package library

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"
)

// The leading dot keeps this out of the fs loader, syncedNameRe, and cleanTmp.
const manifestIndexName = ".manifest.json"

// ManifestEntry describes one cached file.
type ManifestEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Bytes   int64  `json:"bytes"`
	// AddedAt is the unix time the file entered the cache. Recorded for a
	// future LRU eviction; nothing reads it yet.
	AddedAt int64 `json:"added_at"`
}

// Manifest is the source of truth for what the syncer holds on disk: which
// remote asset each file belongs to and how many bytes it costs. It lets the
// syncer reconcile without re-scanning semantics and enforces the storage
// budget. Safe for concurrent use.
//
// A nil root means memory-only (used when no persistence is wired, e.g. in
// tests): all reads/writes work, Flush is a no-op and no file is created.
type Manifest struct {
	log     *slog.Logger
	root    *os.Root
	mu      sync.Mutex
	files   map[string]ManifestEntry
	flushMu sync.Mutex // serializes Flush's file I/O against the shared tmp file
}

// LoadManifest reads the sidecar index from root. A missing or corrupt index
// yields an empty manifest (the syncer re-adopts what is on disk), so a
// corrupt file can never brick syncing.
func LoadManifest(log *slog.Logger, root *os.Root) *Manifest {
	m := &Manifest{log: log, root: root, files: make(map[string]ManifestEntry)}
	f, err := root.OpenFile(manifestIndexName, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return m
	}
	if err != nil {
		log.Warn("library: open manifest failed, starting empty", "err", err)
		return m
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Warn("library: read manifest failed, starting empty", "err", err)
		return m
	}
	if err := json.Unmarshal(data, &m.files); err != nil {
		log.Warn("library: parse manifest failed, starting empty", "err", err)
		m.files = make(map[string]ManifestEntry)
	}
	return m
}

// NewMemoryManifest returns a Manifest with no persistence.
func NewMemoryManifest(log *slog.Logger) *Manifest {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Manifest{log: log, files: make(map[string]ManifestEntry)}
}

// Set records name in memory; call Flush to persist.
func (m *Manifest) Set(name string, e ManifestEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.AddedAt == 0 {
		e.AddedAt = time.Now().Unix()
	}
	m.files[name] = e
}

// Delete drops name in memory; call Flush to persist.
func (m *Manifest) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, name)
}

// Has reports whether name is tracked.
func (m *Manifest) Has(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[name]
	return ok
}

// TotalBytes sums the tracked file sizes.
func (m *Manifest) TotalBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, e := range m.files {
		total += e.Bytes
	}
	return total
}

// DropMissing removes entries whose file is no longer on disk (crashed
// deletes, manual removals). Call Flush to persist.
func (m *Manifest) DropMissing(disk map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name := range m.files {
		if !disk[name] {
			delete(m.files, name)
		}
	}
}

// Missing returns remote assets with no tracked file at the wanted version.
// Tracked files are the source of truth: a file adopted on disk is Set before
// this runs, so anything returned here genuinely needs downloading.
func (m *Manifest) Missing(remote []Asset) []Asset {
	m.mu.Lock()
	defer m.mu.Unlock()
	byID := make(map[string]ManifestEntry, len(m.files))
	for _, e := range m.files {
		byID[e.ID] = e
	}
	var out []Asset
	for _, a := range remote {
		if e, ok := byID[a.ID]; !ok || e.Version != a.Version {
			out = append(out, a)
		}
	}
	return out
}

// Flush writes the index atomically (tmp + rename); a no-op without a root.
func (m *Manifest) Flush() error {
	if m.root == nil {
		return nil
	}
	m.mu.Lock()
	data, err := json.Marshal(m.files)
	m.mu.Unlock()
	if err != nil {
		return errors.New("library: marshal manifest: " + err.Error())
	}
	return writeFileAtomic(m.root, manifestIndexName, &m.flushMu, data)
}
