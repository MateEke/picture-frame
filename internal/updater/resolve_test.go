package updater

import "testing"

func assetsFor(version string) map[string]string {
	m := map[string]string{
		"checksums.txt":         "https://example.test/" + version + "/checksums.txt",
		"checksums.txt.minisig": "https://example.test/" + version + "/checksums.txt.minisig",
	}
	for _, p := range []string{"linux_armv6", "linux_armv7", "linux_arm64"} {
		name := "picture-frame_" + version + "_" + p + ".tar.gz"
		m[name] = "https://example.test/" + name
	}
	return m
}

func TestResolveLatestSameMajor(t *testing.T) {
	releases := []Release{
		{Version: "v1.2.0", Assets: assetsFor("1.2.0")},
		{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
		{Version: "v2.0.0", Assets: assetsFor("2.0.0")},
		{Version: "v1.4.0-rc.1", Prerelease: true, Assets: assetsFor("1.4.0-rc.1")},
	}

	// Same major from 1.2.0 → newest non-prerelease 1.x = 1.3.1.
	got, err := resolveLatest(releases, "1.2.0", "linux_armv6", SameMajor, nil)
	if err != nil {
		t.Fatalf("same-major: %v", err)
	}
	if got == nil || got.version != "v1.3.1" {
		t.Fatalf("same-major version: %+v", got)
	}
	if got.url != "https://example.test/picture-frame_1.3.1_linux_armv6.tar.gz" {
		t.Errorf("same-major url: %q", got.url)
	}
	if got.checksumsURL == "" || got.sigURL == "" {
		t.Errorf("checksums/sig urls not resolved: %+v", got)
	}

	// Any major → 2.0.0.
	got, err = resolveLatest(releases, "1.2.0", "linux_armv6", AnyMajor, nil)
	if err != nil || got == nil || got.version != "v2.0.0" {
		t.Fatalf("any-major: %+v, %v", got, err)
	}

	// Pick the highest regardless of list order (the max must not just be the last entry).
	unordered := []Release{
		{Version: "v2.0.0", Assets: assetsFor("2.0.0")},
		{Version: "v1.3.1", Assets: assetsFor("1.3.1")},
	}
	got, err = resolveLatest(unordered, "1.2.0", "linux_armv6", AnyMajor, nil)
	if err != nil || got == nil || got.version != "v2.0.0" {
		t.Fatalf("unordered any-major should pick the highest (2.0.0): %+v, %v", got, err)
	}

	// Nothing newer → (nil, nil).
	got, err = resolveLatest(releases, "2.0.0", "linux_armv6", AnyMajor, nil)
	if err != nil || got != nil {
		t.Fatalf("up-to-date: want (nil,nil), got (%+v,%v)", got, err)
	}

	// A newer release exists but lacks this platform's asset → error.
	noAsset := []Release{{Version: "v1.3.1", Assets: map[string]string{}}}
	if _, err := resolveLatest(noAsset, "1.2.0", "linux_armv6", SameMajor, nil); err == nil {
		t.Error("missing asset: want error, got nil")
	}
}

func TestResolveLatestSkipsRolledBackVersions(t *testing.T) {
	releases := []Release{
		{Version: "v1.3.0", Assets: assetsFor("1.3.0")},
		{Version: "v1.3.1", Assets: assetsFor("1.3.1")}, // newest, but it rolled back
	}
	// Auto-apply skips the rolled-back version and falls back to the next-newest.
	got, err := resolveLatest(releases, "1.2.0", "linux_armv6", SameMajor, []string{"v1.3.1"})
	if err != nil || got == nil || got.version != "v1.3.0" {
		t.Fatalf("skip should fall back to v1.3.0: %+v, %v", got, err)
	}
	// Skipping every newer release → nothing to auto-apply.
	if got, _ := resolveLatest(releases, "1.2.0", "linux_armv6", SameMajor, []string{"v1.3.0", "v1.3.1"}); got != nil {
		t.Errorf("all skipped → want nil, got %+v", got)
	}
	// Manual (no skip) still offers the rolled-back version, so a retry is possible.
	if got, _ := resolveLatest(releases, "1.2.0", "linux_armv6", AnyMajor, nil); got == nil || got.version != "v1.3.1" {
		t.Errorf("manual should still offer v1.3.1: %+v", got)
	}
}
