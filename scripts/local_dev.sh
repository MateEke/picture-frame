#!/usr/bin/env bash
# Local dev loop: Air (live-reload Go backend) + Vite (live-reload SvelteKit UI).
# Hardware-free by construction — GO_ENV is left non-prod, which swaps the display,
# rotator, wifi, updater and logind adapters for in-tree mocks (cmd/picture-frame/wiring.go).
# Equivalent to `make watch`, plus the prerequisites that target assumes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="config.dev.toml"
BACKEND="http://localhost:8080"
VITE="http://localhost:5173"

cd "$ROOT"

for tool in go node npm; do
	command -v "$tool" >/dev/null 2>&1 || { echo "ERROR: '$tool' not found in PATH" >&2; exit 1; }
done

# //go:embed all:build needs web/build to exist before any Go compile (Makefile:embed-seed).
make embed-seed

# .air.toml passes -config config.dev.toml; config.dev.toml is gitignored (*.toml), so
# seed it from the template on first run rather than failing the Air build.
if [ ! -f "$CONFIG" ]; then
	cp config.dev.example.toml "$CONFIG"
	echo "==> Seeded $CONFIG from config.dev.example.toml (edit freely; it is gitignored)"
fi

# An empty library is a valid kiosk state but not a useful one — borrow the e2e fixtures
# so slide planning, split-screen pairing and the aspect index all have something to chew on.
IMAGES="images"
if [ ! -d "$IMAGES" ] || [ -z "$(ls -A "$IMAGES" 2>/dev/null)" ]; then
	mkdir -p "$IMAGES"
	cp web/e2e/fixtures/images/*.jpg "$IMAGES"/
	echo "==> Seeded $IMAGES/ with e2e fixtures"
fi

[ -d web/node_modules ] || (cd web && npm i)

echo "==> Backend  $BACKEND   (http://localhost:8080/kiosk)"
echo "==> UI       $VITE"

# Air in the background, Vite in the foreground; Ctrl-C must take both down, and
# Air's own children (the built binary) go with it via the process group.
go tool air &
AIR_PID=$!
trap 'kill "$AIR_PID" 2>/dev/null || true' EXIT INT TERM

# Vite proxies /api, /img, /events and /healthz to the backend (web/vite.config.ts), so
# wait for the backend before starting it or the first proxied request 502s.
until curl -sf "$BACKEND/healthz" >/dev/null 2>&1; do
	kill -0 "$AIR_PID" 2>/dev/null || { echo "ERROR: Air exited before the backend came up" >&2; exit 1; }
	sleep 1
done
echo "==> Backend ready"

# Deliberately no BACKEND_URL: its default already points at $BACKEND, and setting it
# makes hey-api regenerate src/lib/api from the live backend, pinning baseUrl to a
# literal host and dirtying the tree on every run. `make generate` covers that on purpose.
cd web && npm run dev
