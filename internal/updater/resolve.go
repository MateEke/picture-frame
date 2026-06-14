package updater

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// Gate decides which newer releases an auto-update may apply.
type Gate int

const (
	SameMajor Gate = iota // auto: only newer releases sharing the current major
	AnyMajor              // manual: any newer release
)

// Release is one GitHub release: its version, whether it's a pre-release, its release
// notes page, and its downloadable assets keyed by asset name.
type Release struct {
	Version    string // "v1.3.1" or "1.3.1"
	Prerelease bool
	NotesURL   string            // release page (GitHub html_url)
	Assets     map[string]string // asset name -> download URL
}

// ReleaseSource lists available releases (e.g. the GitHub Releases API).
type ReleaseSource interface {
	List(ctx context.Context) ([]Release, error)
}

// target is a resolved update: the chosen version and the asset URLs needed to fetch
// and verify it (the tarball plus the release's checksums file and its signature).
type target struct {
	version      string // canonical, with leading "v"
	url          string // platform tarball
	checksumsURL string
	sigURL       string
	notesURL     string // release notes page
}

const (
	checksumsAsset = "checksums.txt"
	sigAsset       = "checksums.txt.minisig"
)

// resolveLatest picks the newest non-pre-release that is newer than current, allowed by gate,
// and not in skip (versions a prior rollback recorded), then finds its asset for platform.
// Returns (nil, nil) when nothing applies; an error only when a matching release exists but
// lacks the platform's asset.
func resolveLatest(releases []Release, current, platform string, gate Gate, skip []string) (*target, error) {
	cur := canonical(current)
	skipped := make(map[string]bool, len(skip))
	for _, s := range skip {
		skipped[canonical(s)] = true
	}
	var best *Release
	var bestV string
	for i, r := range releases {
		if r.Prerelease {
			continue
		}
		v := canonical(r.Version)
		if !semver.IsValid(v) || semver.Compare(v, cur) <= 0 {
			continue
		}
		if gate == SameMajor && semver.Major(v) != semver.Major(cur) {
			continue
		}
		if skipped[v] {
			continue // rolled back before; don't auto-apply it again
		}
		if bestV == "" || semver.Compare(v, bestV) > 0 {
			best, bestV = &releases[i], v
		}
	}
	if best == nil {
		return nil, nil
	}
	asset := func(name string) (string, error) {
		url, ok := best.Assets[name]
		if !ok {
			return "", fmt.Errorf("release %s has no asset %q", bestV, name)
		}
		return url, nil
	}
	url, err := asset(assetName(bestV, platform))
	if err != nil {
		return nil, err
	}
	checksums, err := asset(checksumsAsset)
	if err != nil {
		return nil, err
	}
	sig, err := asset(sigAsset)
	if err != nil {
		return nil, err
	}
	return &target{version: bestV, url: url, checksumsURL: checksums, sigURL: sig, notesURL: best.NotesURL}, nil
}

// assetName is the release artifact name for a version + platform, e.g.
// picture-frame_1.3.1_linux_armv6.tar.gz (the version drops the leading "v").
func assetName(version, platform string) string {
	return fmt.Sprintf("picture-frame_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), platform)
}

// canonical normalizes a version to semver canonical form (ensures a "v" prefix).
func canonical(v string) string {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return semver.Canonical(v)
}
