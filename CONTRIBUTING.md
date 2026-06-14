# Contributing

Thanks for your interest in the project. This document covers the local setup and the checks your
change needs to pass.

## Prerequisites

- Go (see the version in [`go.mod`](go.mod))
- Node.js 24+ and npm
- `golangci-lint` and `shellcheck` on your `PATH`
- `air` and `lefthook` are pinned as Go tools and run via `go tool`; no separate install needed.

## Setup

```sh
go mod download
make hooks        # install the git hooks (lefthook)
cd web && npm i
```

## Day-to-day

```sh
make watch        # Vite dev server + Go backend (Air live-reload) behind a proxy
make generate     # regenerate the OpenAPI spec and the typed TS client after API changes
```

The TypeScript API client under `web/src/lib/api/` is generated. Never edit it by hand; change the
Go handlers and run `make generate`.

## Checks

Run these before pushing; the git hooks and CI run the same.

```sh
make lint         # golangci-lint, ESLint, svelte-check, shellcheck
make test         # go vet, Go tests with the coverage gate, then Vitest
make test-e2e     # Playwright (chromium locally; the full matrix runs in CI)
```

- **Coverage gate:** Go tests for `internal/` must stay above the threshold in the
  [`Makefile`](Makefile). Hardware adapter packages (`.../adapter`) are integration-tested on the
  device and excluded.
- **Frontend:** `npm run check` must report zero warnings.

## Mutation testing

[Stryker](https://stryker-mutator.io/) (frontend) and [Gremlins](https://gremlins.dev/) (Go, pinned
as a `go tool`) check that tests assert behaviour, not just run it. Run `make mutation` on demand;
it's deliberately out of the git hooks (too slow per push). A surviving mutant is a weak/missing
assertion (fix the test), a real bug (fix the code), or a true equivalent (leave it). Config:
`web/stryker*.json` and the `mutation` target in the [`Makefile`](Makefile).

CI runs mutation testing on every PR and push to main, scoped to the diff (Stryker
`--incremental` anchored to the main baseline, gremlins `--diff`). It is informational.
A red mutation check flags weak tests but does not block merging.

## Security

`make vuln` scans for known CVEs: govulncheck (Go, reachability-aware) and osv-scanner
over the bundle-truth CycloneDX SBOM the UI build emits (SvelteKit devDependencies are
the *bundled* deps, and package.json sections don't reflect what ships). The same scan
gates CI and runs daily (`security.yml`), which also `npm audit`s the full tree for
build-toolchain hygiene. Releases attach both SBOMs (frontend bundle + linked Go
modules) for downstream scanning. Dependabot opens security PRs that auto-merge on
green; weekly grouped version-update PRs wait for a human.

## Code style

Read [`STYLE.md`](STYLE.md). Match the surrounding code, keep comments to the non-obvious why, and
let the linters guide you rather than suppressing them. Reach for `nolint` / `eslint-disable` only
as a genuine last resort, with a reason.

## Commits

Conventional Commits, matching the existing history: `type(scope): summary`
(e.g. `refactor(wifi): split manager into command, profile, and policy files`).
Common types here: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`.

## Releases & versioning

Strict [SemVer](https://semver.org). The in-app self-updater is on by default and
auto-applies any newer release **sharing the current major**, so a minor or patch
release MUST be backward-compatible. **Never ship a breaking change in a minor or
patch.** Anything that breaks config, the HTTP/MQTT contract, or on-disk state is a
**major**, which the updater surfaces but only applies on explicit confirmation.
When in doubt, bump the major.
