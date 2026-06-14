# Style guide

These are the conventions the codebase follows. The aim is consistency (code should read as
though a single author wrote it), so where this guide is silent, the surrounding code is the
reference.

Rather than restate them, we lean on three external guides: the
[Google Go Style Guide](https://google.github.io/styleguide/go/), the
[Google TypeScript Style Guide](https://google.github.io/styleguide/tsguide.html), and the
[Svelte docs](https://svelte.dev/docs/svelte/overview). The sections below cover the points worth
calling out and the few places we deviate.

## General principles

These hold in every language:

- **Comments cover the _why_.** A comment that restates the code is noise; one that records a
  non-obvious reason, a gotcha, or a decision saves the next reader from re-deriving it. A line or
  two is usually enough, and there's no need for decorative banners.
- **Variable names scale with scope.** A loop index can be `i`; a value carried across a long
  function deserves a descriptive name. Length should track how far the reader has to hold the
  name in mind.
- **Abstractions earn their place.** An interface, wrapper, or helper is worth adding when it
  removes real duplication or clarifies intent. A single caller usually isn't reason enough.

## Go

- Follow the Google Go Style Guide. Highlights we hold to:
  - Receivers are short (`m`, `s`) and consistent per type. Other names scale with scope.
  - No `util`/`helper`/`common` packages. Name a package for what it provides.
  - Prefer an option struct or functional options over a long parameter list. See
    `internal/library/library.go` for the functional-option pattern used here.
  - Doc comments start with the name and say only what's materially useful. Don't enumerate every
    field or restate the signature. Document sentinel errors and non-obvious concurrency; skip the
    obvious (context cancellation, read-only safety).
- Keep functions to one job. If a function needs section comments to be followed, split it.
- Errors: wrap with `%w` when callers inspect the cause, annotate with `%v` otherwise. Don't
  swallow errors silently. Handle or log them.

### Go tests

- Flat top-level `TestXxx`. Use `t.Run` only for table-driven subtests, not as an umbrella.
- Time-dependent code uses `testing/synctest`. Don't add clock-injection seams to production code
  just for tests.
- Shared helpers live in `internal/testutil`, not copied per package.

## TypeScript

- Follow the Google TypeScript Style Guide. Highlights:
  - Descriptive names; single letters only in a tiny scope. No Hungarian notation, type prefixes,
    or leading/trailing `_`.
  - `const` by default, `let` when you reassign, never `var`. `===` always (`== null` is the one
    allowed loose check).
  - Let the compiler infer obvious types; annotate where it aids the reader or pins a contract.
  - No `any`. Reach for `unknown` and narrow. No `as` / `as const` casts unless truly
    unavoidable; narrow with type guards instead. (`{#each ... as ...}` is unrelated and fine.)
  - Named exports only. JSDoc for exported/public API, line comments for the rest, sparingly.
- No nested ternaries. Extract a helper or use `if`/`else`.

### Vitest

- One umbrella `describe` per file wraps all tests; nested `describe`s inside it are fine.

## Svelte 5

Runes mode only. Follow the official best practices:

- `$state` is for values that drive the UI. Plain values stay plain. Use `$state.raw` for large
  objects you replace wholesale (e.g. API responses) rather than mutate.
- Compute with `$derived`, not an `$effect` that assigns. Treat `$effect` as an escape hatch: no
  state writes inside it; prefer event handlers, attachments (`{@attach}`), or `createSubscriber`.
- Values derived from props use `$derived` so they track prop changes.
- Keyed `{#each}` with a stable id, never the index.
- Forward HTML attributes with rest props typed via `svelte/elements` (e.g. spread
  `...rest: HTMLButtonAttributes`); don't invent bespoke props for things like `data-testid`.
- Prefer clsx-style `class` arrays/objects over the `class:` directive. Style children through CSS
  custom properties.
- Avoid legacy features: `export let`, `$:`, `on:click`, `<slot>`, and stores for shared state
  (use a `$state` class). Validate components with the Svelte MCP autofixer.
- When one control is rendered twice responsively (rail + bar), give each instance a distinct
  `data-testid`; list items that exist to be counted may share one.

## Shell

- `#!/usr/bin/env bash` with `set -euo pipefail`. Quote expansions. Must pass `shellcheck`.
- Comment the non-obvious (why a sleep, why a specific flag), not the mechanics.
