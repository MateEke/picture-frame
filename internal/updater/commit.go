package updater

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// On-disk names under the install dir. The systemd rollback unit references the same
// names, so they must stay in sync with deploy/systemd/picture-frame-rollback.service.
const (
	binaryName       = "picture-frame"
	bakName          = "picture-frame.bak"
	updatingMarker   = ".updating"    // present while a new binary is being verified
	rolledBackMarker = ".rolled-back" // written by the rollback unit after restoring .bak
)

// skipAutoFile lists versions a rollback hit; app-only (the systemd unit never touches it).
const skipAutoFile = ".skip-auto"

// MarkerPresent reports whether a new binary is mid-verification in installDir.
func MarkerPresent(installDir string) bool { return markerPresent(installDir) }

// Commit clears the verification state after a successful update (removes .bak + marker).
func Commit(installDir string) error { return commitUpdate(installDir) }

func writeMarker(installDir, version string) error {
	return os.WriteFile(filepath.Join(installDir, updatingMarker), []byte(version), 0o600)
}

func markerPresent(installDir string) bool {
	_, err := os.Stat(filepath.Join(installDir, updatingMarker))
	return err == nil
}

// commitUpdate clears the verification state once the new binary proved healthy:
// it removes the backup and the marker so the rollback unit can no longer fire.
func commitUpdate(installDir string) error {
	if err := os.Remove(filepath.Join(installDir, bakName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(filepath.Join(installDir, updatingMarker)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// readRolledBackVersion returns (and clears) the version a systemd rollback recorded on the
// last boot; ok is false when no rollback happened. The unit copies the attempted version
// into the marker (empty if it couldn't).
func readRolledBackVersion(installDir string) (version string, ok bool) {
	p := filepath.Join(installDir, rolledBackMarker)
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	_ = os.Remove(p)
	return strings.TrimSpace(string(data)), true
}

// skipAutoVersions lists versions a prior rollback recorded. The updater won't auto-apply
// them again (a published-but-broken release would otherwise apply→rollback every day),
// while manual updates ignore the list so a retry stays possible.
func skipAutoVersions(installDir string) []string {
	data, err := os.ReadFile(filepath.Join(installDir, skipAutoFile))
	if err != nil {
		return nil
	}
	var out []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if v := strings.TrimSpace(line); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// recordSkipAuto appends version to the auto-skip list (no-op if already listed).
func recordSkipAuto(installDir, version string) error {
	if slices.Contains(skipAutoVersions(installDir), version) {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(installDir, skipAutoFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(version + "\n")
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
