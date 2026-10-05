package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
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
// The Manifest is the source of truth for what is cached and enforces the
// storage budget (MaxBytes <= 0 means unlimited).
type Syncer struct {
	log      *slog.Logger
	remote   RemoteAlbum
	lib      *Library
	root     *os.Root
	interval time.Duration
	advance  Advancer
	trigger  chan struct{}
	aspect   *AspectStore // optional: caches per-image dimensions
	manifest *Manifest
	maxBytes int64
	// retryBase is the first retry delay after a failed cycle; it doubles per
	// consecutive failure. Non-positive selects the default.
	retryBase time.Duration

	mu     sync.Mutex
	status Status
}

// SyncerOption configures optional Syncer collaborators.
type SyncerOption func(*Syncer)

// WithAspectStore records downloaded dimensions and clears them on removal.
func WithAspectStore(a *AspectStore) SyncerOption {
	return func(s *Syncer) { s.aspect = a }
}

// WithManifest tracks cached files and their sizes across restarts. Without
// it the syncer uses a memory-only manifest (no persistence, no budget file).
func WithManifest(m *Manifest) SyncerOption {
	return func(s *Syncer) {
		if m != nil {
			s.manifest = m
		}
	}
}

// WithMaxBytes caps the cached bytes. Once over budget, further downloads are
// skipped until pruning frees space; the slideshow keeps serving what is
// cached. Non-positive means unlimited.
func WithMaxBytes(n int64) SyncerOption {
	return func(s *Syncer) { s.maxBytes = n }
}

// WithRetryBase sets the first retry delay after a failed cycle (default
// 30s). Primarily useful to shorten the loop in tests.
func WithRetryBase(d time.Duration) SyncerOption {
	return func(s *Syncer) { s.retryBase = d }
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
	s := &Syncer{log: log, remote: remote, lib: lib, root: root, interval: interval, advance: advance, trigger: make(chan struct{}, 1), manifest: NewMemoryManifest(log)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run syncs immediately then on each interval (or on Trigger) until ctx is
// cancelled. A failed cycle retries with exponential backoff (jittered, capped
// at the interval) instead of waiting out the full interval; a persistent
// failure therefore converges to the normal cadence, never faster.
func (s *Syncer) Run(ctx context.Context) {
	failures := 0
	delay := time.Duration(0) // first sync runs immediately
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.trigger:
			timer.Stop()
		case <-timer.C:
		}
		if s.syncOnce(ctx) {
			failures = 0
			delay = s.interval
		} else {
			failures++
			delay = backoffDelay(failures, s.retryBase, s.interval)
			s.log.Warn("library: sync failed, retrying with backoff", "failures", failures, "retry_in", delay)
		}
	}
}

// defaultRetryBase is the first retry delay after a failed sync cycle.
const defaultRetryBase = 30 * time.Second

// backoffDelay waits base after the first failure, doubling per consecutive
// failure, jittered into [d/2, d] so concurrent frames don't retry in
// lockstep, and capped at max (the normal sync interval).
func backoffDelay(failures int, base, max time.Duration) time.Duration {
	if base <= 0 {
		base = defaultRetryBase
	}
	if max <= 0 {
		max = base
	}
	d := base
	for i := 1; i < failures && d < max; i++ {
		d *= 2
		if d <= 0 || d > max {
			d = max
			break
		}
	}
	if d > max {
		d = max
	}
	half := int64(d) / 2
	return time.Duration(half + rand.N(half+1))
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

// syncOnce reconciles one cycle and reports whether it succeeded. A cycle
// counts as failed when the remote can't be listed, the disk can't be scanned,
// or any download fails; budget skips are deliberate, not failures.
func (s *Syncer) syncOnce(ctx context.Context) bool {
	remote, err := s.remote.List(ctx)
	if err != nil {
		s.log.Warn("library: remote list failed (keeping cache)", "err", err)
		s.setError(err)
		return false
	}
	local, err := s.scanLocal()
	if err != nil {
		s.log.Error("library: local scan failed", "err", err)
		s.setError(err)
		return false
	}
	s.cleanTmp()

	// Drop manifest entries whose file vanished (manual deletes, crashed
	// removals) so they re-download instead of looking cached.
	onDisk := make(map[string]bool, len(local))
	for _, f := range local {
		onDisk[f.name] = true
	}
	s.manifest.DropMissing(onDisk)

	wasEmpty := s.lib.Len() == 0
	stale := local.staleAgainst(remote)
	for _, file := range stale {
		s.removeFile(file)
	}
	adopted := s.adopt(local, remote)
	added, failed, skipped := 0, 0, 0
	budget := s.manifest.TotalBytes()
	for _, a := range s.manifest.Missing(remote) {
		if s.maxBytes > 0 && budget >= s.maxBytes {
			skipped++
			continue
		}
		n, err := s.download(ctx, a)
		if err != nil {
			s.log.Warn("library: download failed", "id", a.ID, "err", err)
			failed++
			continue
		}
		budget += n
		added++
		// Unpause the slideshow on the first image instead of waiting out the
		// whole album: download order is display order for a fresh cache.
		if wasEmpty && added == 1 && s.advance != nil {
			s.advance.Next()
		}
	}
	if skipped > 0 {
		s.log.Warn("library: cache over budget, downloads skipped", "skipped", skipped, "max_bytes", s.maxBytes, "cached_bytes", budget)
	}
	if (added > 0 || len(stale) > 0 || adopted > 0) && s.aspect != nil {
		if err := s.aspect.Flush(); err != nil {
			s.log.Warn("library: flush aspect index failed", "err", err)
		}
	}
	if added > 0 || len(stale) > 0 || adopted > 0 {
		if err := s.manifest.Flush(); err != nil {
			s.log.Warn("library: flush manifest failed", "err", err)
		}
	}
	if failed > 0 {
		s.setPartial(len(remote), failed)
		return false
	}
	s.setOK(len(remote))
	return true
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
	s.manifest.Delete(f.name)
	if s.aspect != nil {
		s.aspect.Delete(f.name)
	}
}

// adopt records scanned files that are wanted but untracked (pre-manifest
// caches, crash recovery) so Missing doesn't re-download them. It returns the
// adopted count. Stale files never reach here: they are deleted above.
func (s *Syncer) adopt(local localSet, remote []Asset) int {
	want := make(map[string]string, len(remote))
	for _, a := range remote {
		want[a.ID] = a.Version
	}
	adopted := 0
	for _, f := range local {
		if s.manifest.Has(f.name) {
			continue
		}
		if v, ok := want[f.id]; !ok || v != f.version {
			continue
		}
		var size int64
		if info, err := s.root.Stat(f.name); err == nil {
			size = info.Size()
		} else {
			s.log.Warn("library: adopt stat failed", "name", f.name, "err", err)
		}
		s.manifest.Set(f.name, ManifestEntry{ID: f.id, Version: f.version, Bytes: size})
		adopted++
	}
	return adopted
}

func (s *Syncer) download(ctx context.Context, a Asset) (int64, error) {
	name := SyncedFilename(a)
	tmp := name + ".tmp"
	n, err := s.writeAtomic(ctx, a.ID, tmp, name)
	if err != nil {
		_ = s.root.Remove(tmp)
		return 0, err
	}
	s.lib.Add(name)
	s.manifest.Set(name, ManifestEntry{ID: a.ID, Version: a.Version, Bytes: n})
	if s.aspect != nil {
		// Decode the preview, not remote EXIF: the preview has orientation baked in,
		// so its dimensions are the displayed aspect (EXIF reports raw, pre-rotation).
		if w, h := ImageDimensions(s.root, name); w > 0 && h > 0 {
			s.aspect.Set(name, w, h)
		}
	}
	return n, nil
}

// maxAssetBytes caps a single download to bound disk + memory in case the
// remote returns an unexpectedly large response. Matches the upload limit so
// fs and remote backends share the same per-asset ceiling.
const maxAssetBytes int64 = 50 << 20

func (s *Syncer) writeAtomic(ctx context.Context, id, tmp, final string) (int64, error) {
	body, err := s.remote.Fetch(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("fetch: %w", err)
	}
	defer body.Close()
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("create tmp: %w", err)
	}
	n, err := io.Copy(f, io.LimitReader(body, maxAssetBytes+1))
	if err != nil {
		f.Close()
		return 0, fmt.Errorf("copy: %w", err)
	}
	if n > maxAssetBytes {
		f.Close()
		return 0, fmt.Errorf("asset exceeds %d bytes", maxAssetBytes)
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("close tmp: %w", err)
	}
	if err := s.root.Rename(tmp, final); err != nil {
		return 0, fmt.Errorf("rename: %w", err)
	}
	return n, nil
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
