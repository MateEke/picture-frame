package updater

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"aead.dev/minisign"
)

func testPubKey(t *testing.T) minisign.PublicKey {
	t.Helper()
	pub, err := minisign.PublicKeyFromFile("testdata/test.pub")
	if err != nil {
		t.Fatalf("load test pubkey: %v", err)
	}
	return pub
}

func TestVerifyChecksumAndSignature(t *testing.T) {
	pub := testPubKey(t)
	const (
		artifact  = "testdata/picture-frame_1.3.1_linux_armv6.tar.gz"
		checksums = "testdata/checksums.txt"
		sig       = "testdata/checksums.txt.minisig"
	)

	if err := Verify(artifact, checksums, sig, pub); err != nil {
		t.Fatalf("valid release: %v", err)
	}

	// Tampered artifact: same (validly-signed) checksums and the same name, but the bytes
	// no longer match their listed sha256 → checksum error.
	dir := t.TempDir()
	bad := filepath.Join(dir, "picture-frame_1.3.1_linux_armv6.tar.gz")
	if err := os.WriteFile(bad, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(bad, checksums, sig, pub); err == nil {
		t.Error("tampered artifact: want checksum error, got nil")
	}

	// Tampered signature → signature error.
	sigBytes, _ := os.ReadFile(sig)
	sigBytes[len(sigBytes)-5] ^= 0xff
	badSig := filepath.Join(dir, "bad.minisig")
	// G703 false positive: badSig is derived from t.TempDir(), not external input.
	if err := os.WriteFile(badSig, sigBytes, 0o600); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	if err := Verify(artifact, checksums, badSig, pub); err == nil {
		t.Error("tampered signature: want signature error, got nil")
	}
}

func TestChecksumForTolerantParsing(t *testing.T) {
	lines := "abc123  picture-frame_1.0.0_linux_armv6.tar.gz\n" +
		"def456 *picture-frame_1.0.0_linux_arm64.tar.gz\n" // binary-mode marker
	if h, ok := checksumFor([]byte(lines), "picture-frame_1.0.0_linux_armv6.tar.gz"); !ok || h != "abc123" {
		t.Errorf("text mode: got %q %v", h, ok)
	}
	if h, ok := checksumFor([]byte(lines), "picture-frame_1.0.0_linux_arm64.tar.gz"); !ok || h != "def456" {
		t.Errorf("binary mode: got %q %v", h, ok)
	}
}

func TestEmbeddedKeyParses(t *testing.T) {
	if _, err := EmbeddedKey(); err != nil {
		t.Fatalf("embedded key should parse: %v", err)
	}
}

// The embedded production pubkey must stay byte-identical to the committed deploy copy:
// the same key is used at install time and by the updater.
func TestEmbeddedPubKeyMatchesDeployCopy(t *testing.T) {
	embedded, err := os.ReadFile("minisign.pub")
	if err != nil {
		t.Fatal(err)
	}
	deployed, err := os.ReadFile("../../deploy/minisign.pub")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(embedded, deployed) {
		t.Error("internal/updater/minisign.pub drifted from deploy/minisign.pub")
	}
}
