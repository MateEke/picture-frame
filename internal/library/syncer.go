package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/redact"
)

// Syncer reconciles a RemoteAlbum with a local directory and Library on a tick.
// Files are named "<asset-id>-<version>.jpg" so an edit upstream (new version)
// becomes a new local file: old version deleted, new version downloaded.
type Syncer struct {
	log      *slog.Logger
	remote   RemoteAlbum
	lib      *Library
	root     *os.Root
	interval time.Duration
	advance  Advancer
	trigger  chan struct{}
	aspect   *AspectStore // optional: caches per-image dimensions

	mu     sync.Mutex
	status Status
}

// SyncerOption configures optional Syncer collaborators.
type SyncerOption func(*Syncer)

// WithAspectStore records downloaded dimensions and clears them on removal.
func WithAspectStore(a *AspectStore) SyncerOption {
	return func(s *Syncer) { s.aspect = a }
}

// Status reports the latest sync outcome for the admin UI.
type Status struct {
	LastSync   time.Time
	AssetCount int
	LastError  string
}

// SyncerStatus is the read-and-trigger surface over a Syncer. Callers that may
// hold no syncer (the fs backend) keep it behind this interface so a nil syncer
// reads as a nil interface rather than a typed-nil.
type SyncerStatus interface {
	Status() Status
	Trigger()
}

// Advancer is poked when the syncer brings an empty library to non-empty.
type Advancer interface {
	Next()
}

func NewSyncer(log *slog.Logger, remote RemoteAlbum, lib *Library, root *os.Root, interval time.Duration, advance Advancer, opts ...SyncerOption) *Syncer {
	// Buffered so Trigger never blocks.
	s := &Syncer{log: log, remote: remote, lib: lib, root: root, interval: interval, advance: advance, trigger: make(chan struct{}, 1)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run syncs immediately then on each interval (or on Trigger) until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context) {
	s.syncOnce(ctx)
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.syncOnce(ctx)
		case <-s.trigger:
			s.syncOnce(ctx)
		}
	}
}

// Trigger requests an out-of-band sync. Non-blocking; coalesces if one is queued.
func (s *Syncer) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Status returns the latest sync outcome. Safe from any goroutine.
func (s *Syncer) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Syncer) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastSync = time.Now()
	s.status.LastError = safeErrorMessage(err.Error())
}

func (s *Syncer) setOK(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastSync = time.Now()
	s.status.AssetCount = count
	s.status.LastError = ""
}

// setPartial records a cycle where some downloads failed; the asset count
// reflects the remote, the error surfaces the per-cycle failure count.
func (s *Syncer) setPartial(count, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastSync = time.Now()
	s.status.AssetCount = count
	s.status.LastError = fmt.Sprintf("downloads failed: %d", failed)
}

func (s *Syncer) syncOnce(ctx context.Context) {
	remote, err := s.remote.List(ctx)
	if err != nil {
		s.log.Warn("library: remote list failed (keeping cache)", "err", err)
		s.setError(err)
		return
	}
	local, err := s.scanLocal()
	if err != nil {
		s.log.Error("library: local scan failed", "err", err)
		s.setError(err)
		return
	}
	s.cleanTmp()

	wasEmpty := s.lib.Len() == 0
	stale := local.staleAgainst(remote)
	for _, file := range stale {
		s.removeFile(file)
	}
	added, failed := 0, 0
	for _, a := range local.missingFrom(remote) {
		if err := s.download(ctx, a); err != nil {
			s.log.Warn("library: download failed", "id", a.ID, "err", err)
			failed++
			continue
		}
		added++
	}
	if (added > 0 || len(stale) > 0) && s.aspect != nil {
		if err := s.aspect.Flush(); err != nil {
			s.log.Warn("library: flush aspect index failed", "err", err)
		}
	}
	if wasEmpty && added > 0 && s.advance != nil {
		s.advance.Next()
	}
	if failed > 0 {
		s.setPartial(len(remote), failed)
	} else {
		s.setOK(len(remote))
	}
}

// localFile is the parsed form of a synced filename.
type localFile struct {
	name    string
	id      string
	version string
}

type localSet []localFile

func (ls localSet) byID() map[string]localFile {
	index := make(map[string]localFile, len(ls))
	for _, f := range ls {
		index[f.id] = f
	}
	return index
}

// staleAgainst returns local files whose ID is gone from remote or whose
// version differs from the remote one.
func (ls localSet) staleAgainst(remote []Asset) []localFile {
	want := make(map[string]string, len(remote))
	for _, a := range remote {
		want[a.ID] = a.Version
	}
	var out []localFile
	for _, f := range ls {
		if v, ok := want[f.id]; !ok || v != f.version {
			out = append(out, f)
		}
	}
	return out
}

// missingFrom returns remote assets that are absent locally (or stale, since
// the stale version is also deleted in the same cycle).
func (ls localSet) missingFrom(remote []Asset) []Asset {
	have := ls.byID()
	var out []Asset
	for _, a := range remote {
		if f, ok := have[a.ID]; !ok || f.version != a.Version {
			out = append(out, a)
		}
	}
	return out
}

var syncedNameRe = regexp.MustCompile(`^([0-9a-f-]{36})-([0-9a-f]+)\.jpg$`)

func (s *Syncer) scanLocal() (localSet, error) {
	d, err := s.root.OpenFile(".", os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open dir: %w", err)
	}
	defer d.Close()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var out localSet
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := syncedNameRe.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		out = append(out, localFile{name: entry.Name(), id: match[1], version: match[2]})
	}
	return out, nil
}

// cleanTmp removes leftover .tmp files from a previous crash.
func (s *Syncer) cleanTmp() {
	d, err := s.root.OpenFile(".", os.O_RDONLY, 0)
	if err != nil {
		s.log.Debug("library: cleanTmp open failed", "err", err)
		return
	}
	defer d.Close()
	entries, err := d.ReadDir(-1)
	if err != nil {
		s.log.Debug("library: cleanTmp read failed", "err", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tmp") {
			_ = s.root.Remove(e.Name())
		}
	}
}

func (s *Syncer) removeFile(f localFile) {
	if err := s.root.Remove(f.name); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Warn("library: delete failed", "name", f.name, "err", err)
		return
	}
	s.lib.Remove(f.name)
	if s.aspect != nil {
		s.aspect.Delete(f.name)
	}
}

func (s *Syncer) download(ctx context.Context, a Asset) error {
	name := SyncedFilename(a)
	tmp := name + ".tmp"
	if err := s.writeAtomic(ctx, a.ID, tmp, name); err != nil {
		_ = s.root.Remove(tmp)
		return err
	}
	s.lib.Add(name)
	if s.aspect != nil {
		// Decode the preview, not remote EXIF: the preview has orientation baked in,
		// so its dimensions are the displayed aspect (EXIF reports raw, pre-rotation).
		if w, h := ImageDimensions(s.root, name); w > 0 && h > 0 {
			s.aspect.Set(name, w, h)
		}
	}
	return nil
}

// maxAssetBytes caps a single download to bound disk + memory in case the
// remote returns an unexpectedly large response. Matches the upload limit so
// fs and remote backends share the same per-asset ceiling.
const maxAssetBytes int64 = 50 << 20

func (s *Syncer) writeAtomic(ctx context.Context, id, tmp, final string) error {
	body, err := s.remote.Fetch(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer body.Close()
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	n, err := io.Copy(f, io.LimitReader(body, maxAssetBytes+1))
	if err != nil {
		f.Close()
		return fmt.Errorf("copy: %w", err)
	}
	if n > maxAssetBytes {
		f.Close()
		return fmt.Errorf("asset exceeds %d bytes", maxAssetBytes)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := s.root.Rename(tmp, final); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// safeErrorMessage redacts filesystem paths and caps length so server-side error
// strings stay UI-safe and bounded.
func safeErrorMessage(s string) string { return redact.Path(s) }

// SyncedFilename returns the canonical local name for an asset. Panics on an
// empty Version, which would produce an unparseable name and loop in the diff.
func SyncedFilename(a Asset) string {
	if a.Version == "" {
		panic("library: SyncedFilename requires non-empty Version")
	}
	return a.ID + "-" + a.Version + ".jpg"
}

// IsSyncedName reports whether name matches the synced-file pattern.
func IsSyncedName(name string) bool {
	return syncedNameRe.MatchString(strings.ToLower(name))
}
