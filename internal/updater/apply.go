package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/MateEke/picture-frame/internal/redact"
)

// Downloader fetches a URL to a local file.
type Downloader interface {
	Download(ctx context.Context, url, dest string) error
}

// PreflightRunner runs `<binPath> --health-check` to prove a downloaded binary runs here
// before the swap.
type PreflightRunner interface {
	Check(binPath string) error
}

const maxBinaryBytes int64 = 64 << 20 // extraction cap; the binary is ~15MB

// applyTarget runs the full update: download, verify, pre-flight, swap, restart. On
// success the process re-execs into the new binary and never returns here; on any
// failure the running binary is left untouched and a UI-safe reason is recorded.
func (u *Updater) applyTarget(ctx context.Context, t *target) {
	if err := u.doApply(ctx, t); err != nil {
		u.log.Warn("updater: apply failed", "version", t.version, "err", err)
		u.setLastResult("failed: " + redact.Path(err.Error()))
	}
	u.setPhase(PhaseIdle)
}

func (u *Updater) doApply(ctx context.Context, t *target) error {
	// Work + staging live under installDir so the final rename is same-filesystem (atomic).
	work, err := os.MkdirTemp(u.installDir, ".update-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(work)

	u.setPhase(PhaseDownloading)
	// Name the tarball after its asset so Verify's basename lookup matches checksums.txt.
	tarball := filepath.Join(work, assetName(t.version, u.platform))
	checksums := filepath.Join(work, checksumsAsset)
	sig := filepath.Join(work, sigAsset)
	for _, d := range []struct{ url, dest string }{
		{t.url, tarball}, {t.checksumsURL, checksums}, {t.sigURL, sig},
	} {
		if err := u.downloader.Download(ctx, d.url, d.dest); err != nil {
			return fmt.Errorf("download: %w", err)
		}
	}

	u.setPhase(PhaseVerifying)
	if err := Verify(tarball, checksums, sig, u.pubKey); err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	u.setPhase(PhaseApplying)
	staged := filepath.Join(work, binaryName)
	if err := extractBinary(tarball, staged); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := u.preflight.Check(staged); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	live := filepath.Join(u.installDir, binaryName)
	bak := filepath.Join(u.installDir, bakName)
	if err := copyFile(live, bak); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	if err := writeMarker(u.installDir, t.version); err != nil {
		os.Remove(bak)
		return fmt.Errorf("marker: %w", err)
	}
	if err := os.Rename(staged, live); err != nil {
		// Disarm (.bak + marker) so a later unrelated crash can't roll back the unchanged binary.
		_ = commitUpdate(u.installDir)
		return fmt.Errorf("swap: %w", err)
	}
	u.log.Info("updater: applied; restarting into new version", "version", t.version)
	return u.restart() // re-exec into the new binary; the kiosk reloads on the version change
}

func (u *Updater) setLastResult(msg string) {
	u.mu.Lock()
	u.status.LastResult = msg
	u.status.LastResultSeq++
	u.mu.Unlock()
}

// extractBinary writes the "picture-frame" file from a .tar.gz to dest.
func extractBinary(tarball, dest string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%q not found in archive", binaryName)
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != binaryName {
			continue
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(tr, maxBinaryBytes)); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}
