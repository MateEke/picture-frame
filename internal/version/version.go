package version

// Version is injected at build time via -ldflags "-X .../version.Version=<tag>".
var Version = "dev"

// Platform is the build target (e.g. linux_armv6), injected via -ldflags like Version.
// The self-updater needs it: the Go runtime exposes GOARCH but not the GOARM variant.
var Platform = "dev"

// UpdateRepo is the default GitHub release source ("owner/name") the self-updater tracks,
// injected at build time (the repo it was built from). The [updater].github_repo config
// overrides it; empty (e.g. dev builds) disables auto-updates unless config sets one.
var UpdateRepo = ""
