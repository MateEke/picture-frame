package updater

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aead.dev/minisign"
)

//go:embed minisign.pub
var embeddedPubKey string

var (
	errBadSignature     = errors.New("minisign signature verification failed")
	errChecksumMismatch = errors.New("artifact checksum mismatch")
	errNotInChecksums   = errors.New("artifact not listed in checksums.txt")
)

// EmbeddedKey parses the production minisign public key compiled into the binary.
func EmbeddedKey() (minisign.PublicKey, error) {
	var pub minisign.PublicKey
	if err := pub.UnmarshalText([]byte(embeddedPubKey)); err != nil {
		return pub, fmt.Errorf("parse embedded pubkey: %w", err)
	}
	return pub, nil
}

// Verify checks the minisign signature over checksums.txt, then that the artifact's
// sha256 matches its entry there. Authenticity (signature) before integrity (hash).
func Verify(artifactPath, checksumsPath, sigPath string, pub minisign.PublicKey) error {
	checksums, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	if !minisign.Verify(pub, checksums, sig) {
		return errBadSignature
	}
	name := filepath.Base(artifactPath)
	want, ok := checksumFor(checksums, name)
	if !ok {
		return fmt.Errorf("%w: %s", errNotInChecksums, name)
	}
	got, err := sha256File(artifactPath)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: %s", errChecksumMismatch, name)
	}
	return nil
}

// checksumFor returns the hex sha256 listed for name in sha256sum-format checksums.
// Tolerates the binary-mode "*" filename marker and spaces in the name.
func checksumFor(checksums []byte, name string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(checksums))
	for sc.Scan() {
		hash, rest, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			continue
		}
		if strings.TrimLeft(rest, " *") == name {
			return hash, true
		}
	}
	return "", false
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
