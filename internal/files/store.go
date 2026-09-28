// Package files stores arbitrary (non-slideshow) uploads, e.g. videos, PDFs and
// documents, in a directory next to the image library. Files keep a sanitized
// copy of their original name; metadata lives in a JSON sidecar.
package files

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// NamePattern is the canonical stored-name rule; the HTTP routes embed it as a
// huma `pattern` tag. No leading dot keeps sidecars and temp files unreachable.
const NamePattern = `^[a-zA-Z0-9_~-][a-zA-Z0-9_.~-]*$`

var nameRe = regexp.MustCompile(NamePattern)

// ValidName reports whether name is a servable stored filename.
func ValidName(name string) bool {
	return len(name) <= maxNameLen && nameRe.MatchString(name)
}

const (
	indexName  = ".files-index.json"
	tmpPrefix  = ".upload-"
	maxNameLen = 120
	sniffBytes = 512
)

// ErrNotFound is returned for names that aren't in the store.
var ErrNotFound = errors.New("files: not found")

// ErrTooLarge is returned when an upload exceeds the size limit.
var ErrTooLarge = errors.New("files: too large")

// Meta describes one stored file.
type Meta struct {
	Name     string    `json:"name"`
	Original string    `json:"original"`
	Size     int64     `json:"size"`
	MIME     string    `json:"mime"`
	Added    time.Time `json:"added"`
}

// Store is safe for concurrent use.
type Store struct {
	log     *slog.Logger
	root    *os.Root
	mu      sync.Mutex
	byName  map[string]Meta
	flushMu sync.Mutex
}

// Open creates dir if missing, loads the sidecar index and reconciles it with
// what's on disk (files copied in by hand appear; deleted ones drop out).
func Open(log *slog.Logger, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("files: create dir %q: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("files: open root: %w", err)
	}
	s := &Store{log: log, root: root, byName: map[string]Meta{}}
	s.loadIndex()
	s.reconcile()
	return s, nil
}

// Close releases the directory handle.
func (s *Store) Close() error { return s.root.Close() }

func (s *Store) loadIndex() {
	f, err := s.root.Open(indexName)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.log.Warn("files: open index failed, rebuilding", "err", err)
		}
		return
	}
	defer f.Close()
	var list []Meta
	if err := json.NewDecoder(f).Decode(&list); err != nil {
		s.log.Warn("files: parse index failed, rebuilding", "err", err)
		return
	}
	for _, m := range list {
		if ValidName(m.Name) {
			s.byName[m.Name] = m
		}
	}
}

func (s *Store) reconcile() {
	d, err := s.root.Open(".")
	if err != nil {
		s.log.Warn("files: read dir failed", "err", err)
		return
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		s.log.Warn("files: read dir failed", "err", err)
		return
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, tmpPrefix) {
			_ = s.root.Remove(name) // leftover from an interrupted upload
			continue
		}
		if e.IsDir() || !ValidName(name) {
			continue
		}
		onDisk[name] = true
		if _, ok := s.byName[name]; ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		s.byName[name] = Meta{
			Name: name, Original: name, Size: info.Size(),
			MIME: s.detectMIME(name), Added: info.ModTime(),
		}
	}
	for name := range s.byName {
		if !onDisk[name] {
			delete(s.byName, name)
		}
	}
	if err := s.flush(); err != nil {
		s.log.Warn("files: write index failed", "err", err)
	}
}

// List returns every file, newest first.
func (s *Store) List() []Meta {
	s.mu.Lock()
	out := make([]Meta, 0, len(s.byName))
	for _, m := range s.byName {
		out = append(out, m)
	}
	s.mu.Unlock()
	slices.SortFunc(out, func(a, b Meta) int {
		if c := b.Added.Compare(a.Added); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

// Get returns the metadata for name.
func (s *Store) Get(name string) (Meta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.byName[name]
	return m, ok
}

// Save streams r to disk under a name derived from original, never buffering
// the whole file. At most limit bytes are accepted (0 = unlimited).
func (s *Store) Save(original string, r io.Reader, limit int64) (Meta, error) {
	tmp, err := s.createTemp()
	if err != nil {
		return Meta{}, err
	}
	tmpName := filepath.Base(tmp.Name())
	src := r
	if limit > 0 {
		src = io.LimitReader(r, limit+1)
	}
	n, copyErr := io.Copy(tmp, src)
	closeErr := tmp.Close()
	if copyErr == nil && closeErr == nil && limit > 0 && n > limit {
		copyErr = ErrTooLarge
	}
	if copyErr != nil || closeErr != nil {
		_ = s.root.Remove(tmpName)
		if errors.Is(copyErr, ErrTooLarge) {
			return Meta{}, ErrTooLarge
		}
		return Meta{}, fmt.Errorf("files: write: %w", errors.Join(copyErr, closeErr))
	}

	s.mu.Lock()
	name := s.uniqueName(Sanitize(original))
	if err := s.root.Rename(tmpName, name); err != nil {
		s.mu.Unlock()
		_ = s.root.Remove(tmpName)
		return Meta{}, fmt.Errorf("files: rename: %w", err)
	}
	m := Meta{
		Name:     name,
		Original: displayName(original, name),
		Size:     n,
		MIME:     s.detectMIME(name),
		Added:    time.Now(),
	}
	s.byName[name] = m
	s.mu.Unlock()

	if err := s.flush(); err != nil {
		s.log.Warn("files: write index failed", "err", err)
	}
	return m, nil
}

// Delete removes name from disk and the index.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	if _, ok := s.byName[name]; !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	if err := s.root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.mu.Unlock()
		return fmt.Errorf("files: remove: %w", err)
	}
	delete(s.byName, name)
	s.mu.Unlock()
	if err := s.flush(); err != nil {
		s.log.Warn("files: write index failed", "err", err)
	}
	return nil
}

// OpenFile opens name for reading; the caller closes it.
func (s *Store) OpenFile(name string) (*os.File, Meta, error) {
	m, ok := s.Get(name)
	if !ok {
		return nil, Meta{}, ErrNotFound
	}
	f, err := s.root.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, Meta{}, ErrNotFound
		}
		return nil, Meta{}, err
	}
	return f, m, nil
}

// FreeBytes reports the space available to unprivileged writers in the store's
// filesystem; ok is false when it can't be determined.
func (s *Store) FreeBytes() (free uint64, ok bool) {
	return diskFree(s.root.Name())
}

func (s *Store) createTemp() (*os.File, error) {
	for range 5 {
		name := fmt.Sprintf("%s%d", tmpPrefix, time.Now().UnixNano())
		f, err := s.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("files: create temp: %w", err)
		}
	}
	return nil, errors.New("files: could not allocate temp file")
}

// uniqueName appends -1, -2, … before the extension until name is free.
// Caller holds s.mu.
func (s *Store) uniqueName(name string) string {
	taken := func(n string) bool {
		if _, ok := s.byName[n]; ok {
			return true
		}
		_, err := s.root.Stat(n)
		return err == nil
	}
	if !taken(name) {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", base, i, ext)
		if !taken(cand) {
			return cand
		}
	}
}

func (s *Store) detectMIME(name string) string {
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); t != "" {
		return t
	}
	f, err := s.root.Open(name)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, sniffBytes)
	n, _ := io.ReadFull(f, buf)
	return http.DetectContentType(buf[:n])
}

func (s *Store) flush() error {
	list := s.List()
	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	tmp := indexName + ".tmp"
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = s.root.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = s.root.Remove(tmp)
		return err
	}
	return s.root.Rename(tmp, indexName)
}

// transliterate maps common accented letters (Finnish/Swedish first) to ASCII
// so "Kesä 2024.mp4" becomes "Kesa_2024.mp4" rather than "Kes_2024.mp4".
var transliterate = strings.NewReplacer(
	"ä", "a", "Ä", "A", "ö", "o", "Ö", "O", "å", "a", "Å", "A",
	"é", "e", "É", "E", "è", "e", "ü", "u", "Ü", "U", "ß", "ss",
)

// Sanitize turns an arbitrary client filename into a safe stored name.
func Sanitize(original string) string {
	base := filepath.Base(strings.ReplaceAll(original, `\`, "/"))
	base = transliterate.Replace(base)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range base {
		ok := r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_.~-", r))
		if !ok {
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
			continue
		}
		b.WriteRune(r)
		lastUnderscore = r == '_'
	}
	name := strings.TrimLeft(b.String(), "._")
	ext := strings.ToLower(filepath.Ext(name))
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if len(ext) > 16 {
		ext = ""
		stem = name
	}
	if stem == "" || stem == "_" {
		stem = "tiedosto"
	}
	if len(stem)+len(ext) > maxNameLen {
		stem = stem[:maxNameLen-len(ext)]
	}
	return stem + ext
}

func displayName(original, stored string) string {
	base := filepath.Base(strings.ReplaceAll(original, `\`, "/"))
	if base == "" || base == "." || base == "/" {
		return stored
	}
	return base
}
