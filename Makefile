.PHONY: all clean build build-ui build-go embed-seed test test-e2e lint modernize mutation mutation-diff vuln sbom-go watch generate hooks help notices release-snapshot

# --- Variables ---
APP_NAME := picture-frame
BUILD_DIR := dist
UI_DIR := web
CMD_DIR := ./cmd/picture-frame

# npm ci in CI (deterministic, clean), npm i locally (fast, IDE-friendly)
NPM_INSTALL := $(if $(CI),npm ci,npm i)

# Extracts the current git tag or hash for versioning, defaults to "dev"
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default self-update source ("owner/name"), derived from the origin remote so it tracks
# the repo (incl. a future rename). goreleaser uses $GITHUB_REPOSITORY instead.
UPDATE_REPO := $(shell git config --get remote.origin.url 2>/dev/null | sed -E 's#.*github.com[:/]##; s#\.git$$##')

# Frontend build version (kiosk heartbeats carry it); overridable so goreleaser can match it
# to the version.Version ldflag — they must agree or updates roll back on the commit gate.
PUBLIC_APP_VERSION ?= $(VERSION)

# Linker flags: strip debug symbols (-s -w) and inject version + platform + default update
# source. build-go targets armv6 (Pi Zero W), so Platform is linux_armv6; goreleaser sets
# its own per-arch Platform. Without this the self-updater can't match its release asset.
LDFLAGS := -s -w \
	-X github.com/MateEke/picture-frame/internal/version.Version=$(VERSION) \
	-X github.com/MateEke/picture-frame/internal/version.Platform=linux_armv6 \
	-X github.com/MateEke/picture-frame/internal/version.UpdateRepo=$(UPDATE_REPO)

# The default target runs the full pipeline
all: clean lint test build

# --- Build Targets ---

build: build-ui build-go ## Build the complete production binary for the Pi Zero W

# //go:embed all:build needs web/build before ANY Go compile (vet, test, lint, gremlins,
# build). On fresh checkouts seed a build-shaped skeleton — incl. a fake immutable asset
# so httpapi's cache-header test has a target. A real build (has _app/) is left untouched.
embed-seed:
	@test -d $(UI_DIR)/build/_app || { mkdir -p $(UI_DIR)/build/_app/immutable && touch $(UI_DIR)/build/index.html $(UI_DIR)/build/_app/immutable/placeholder.js; }

build-ui: embed-seed ## Build the SvelteKit frontend
	@echo "==> Building frontend (SvelteKit)..."
	go generate ./internal/httpapi
	cd $(UI_DIR) && $(NPM_INSTALL) && PUBLIC_APP_VERSION="$(PUBLIC_APP_VERSION)" npm run build

build-go: embed-seed ## Build the static Go backend for ARMv6
	@echo "==> Building Go backend for linux/arm/v6..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@# build-go embeds web/build as-is; if it was baked with a different version the frame will
	@# reload-loop / roll back updates. Warn loudly — use `make build` to rebuild both in step.
	@baked=$$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' $(UI_DIR)/build/_app/version.json 2>/dev/null); \
	if [ "$$baked" != "$(VERSION)" ]; then \
		echo "WARNING: embedded frontend ($$baked) != backend ($(VERSION)) — run 'make build VERSION=$(VERSION)' to rebuild the UI, or the frame will reload-loop."; \
	fi

# --- Release Targets ---

# Regenerate the embedded internal/licenses/THIRD_PARTY_NOTICES.txt (Go deps via go-licenses,
# frontend via Vite's build.license, fonts appended). Assumes `make build-ui` already ran
# (it emits the frontend notices); the goreleaser before-hook runs build-ui then this.
notices: ## Regenerate the embedded THIRD_PARTY_NOTICES.txt (run after build-ui)
	@echo "==> Generating third-party notices..."
	bash scripts/gen-notices.sh > internal/licenses/THIRD_PARTY_NOTICES.txt
	@echo "    wrote internal/licenses/THIRD_PARTY_NOTICES.txt"

# Linked-modules-only (not the go.sum graph); release artifact — CI scans Go source
# via govulncheck instead, which adds reachability.
sbom-go: ## Generate the backend CycloneDX SBOM (linked Go modules)
	@mkdir -p tmp
	go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest app -json -output tmp/sbom.backend.cyclonedx.json -main cmd/picture-frame .

# Full multi-arch release locally without publishing (catches flag drift vs build-go).
# goreleaser v2 can't skip signing on snapshot, so we sign with a throwaway key in tmp/.
# Needs goreleaser + minisign on PATH (go install ...); CI uses goreleaser-action + real key.
release-snapshot: ## Build a local multi-arch snapshot (no publish; throwaway-signed)
	@command -v minisign >/dev/null 2>&1 || { echo "minisign not found: go install aead.dev/minisign/cmd/minisign"; exit 1; }
	@mkdir -p tmp
	@test -f tmp/minisign-snapshot.key || minisign -G -W -p tmp/minisign-snapshot.pub -s tmp/minisign-snapshot.key
	GITHUB_REPOSITORY=$(UPDATE_REPO) MINISIGN_KEY_FILE=tmp/minisign-snapshot.key goreleaser release --snapshot --clean

# --- Development Targets ---

generate: embed-seed ## Regenerate OpenAPI spec + TypeScript client
	go generate ./internal/httpapi
	cd $(UI_DIR) && $(NPM_INSTALL) && npx openapi-ts

hooks: ## Install git hooks via the pinned lefthook
	go tool lefthook install

watch: embed-seed ## Run development servers (Vite + Go proxy via Air)
	@echo "==> Starting dev environment..."
	@trap 'kill %1; exit' SIGINT; \
	go tool air & \
	until curl -sf http://localhost:8080/openapi.json >/dev/null 2>&1; do sleep 1; done && \
	cd $(UI_DIR) && BACKEND_URL=http://localhost:8080 npm run dev

# Internal packages to test — adapter packages are integration-tested on hardware.
# Lazy (=): go list compiles the embed, so it must run after embed-seed, not at parse.
TESTPKGS = $(shell go list ./internal/... | grep -v '/adapter')
COVERAGE_THRESHOLD := 80

# --- Quality Gates ---

test: embed-seed ## Run all tests with coverage gate (Go adapters excluded)
	@echo "==> Running Go tests..."
	go vet ./...
	go test -coverprofile=coverage.out $(TESTPKGS)
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/{gsub(/%/,""); print int($$3)}'); \
	echo "Coverage: $${total}% (threshold: $(COVERAGE_THRESHOLD)%)"; \
	if [ "$$total" -lt "$(COVERAGE_THRESHOLD)" ]; then \
		echo "FAIL: coverage $${total}% is below $(COVERAGE_THRESHOLD)%"; exit 1; \
	fi
	@echo "==> Running frontend tests..."
	cd $(UI_DIR) && npm run test:unit -- --run

test-e2e: ## Run Playwright E2E tests (chromium; full matrix runs in CI)
	@echo "==> Running E2E tests..."
	cd $(UI_DIR) && npm run test:e2e

# Opt-in mutation testing — verifies tests actually assert behaviour, not just
# execute it. Not part of `test`/`all`; the pre-push hook runs it diff-scoped.
# A run fails if the score drops below the per-config break threshold.
# Shared by the full and diff-scoped runs — tune in one place.
GREMLINS_FLAGS := --timeout-coefficient 30 --workers 4 --exclude-files adapter -S l

mutation: embed-seed ## Run mutation testing (full; frontend + Go)
	@echo "==> Frontend mutation testing (node 'server' + browser 'client' projects)..."
	cd $(UI_DIR) && npm run test:mutation
	@echo "==> Go mutation testing (internal/, hardware adapters excluded)..."
	go tool gremlins unleash $(GREMLINS_FLAGS) ./internal

mutation-diff: embed-seed ## Mutation testing scoped to changes vs BASE_REF (used by CI)
	cd $(UI_DIR) && npm run test:mutation:ci
	go tool gremlins unleash --diff $(BASE_REF) $(GREMLINS_FLAGS) ./internal

# osv-scanner is huge — compiling it via `go run` costs ~2min in CI, so CI downloads the
# prebuilt release binary onto PATH first; locally go run's build cache makes it cheap.
OSV_SCANNER := $(shell command -v osv-scanner 2>/dev/null || echo go run github.com/google/osv-scanner/v2/cmd/osv-scanner@latest)

vuln: embed-seed ## Scan dependencies for known vulnerabilities (reachable Go + bundled npm)
	@echo "==> Scanning Go dependencies (govulncheck, reachability-aware)..."
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	@echo "==> Building UI for the bundle-truth SBOM..."
	cd $(UI_DIR) && $(NPM_INSTALL) && npm run build
	@echo "==> Scanning bundled npm packages (osv-scanner)..."
	$(OSV_SCANNER) scan source -L $(UI_DIR)/.svelte-kit/output/client/cyclonedx/bom.json

lint: embed-seed ## Run linters (Go, shell, workflows, and Frontend)
	@echo "==> Linting Go code..."
	golangci-lint run
	@echo "==> Linting shell scripts..."
	@command -v shellcheck >/dev/null 2>&1 && shellcheck deploy/*.sh scripts/*.sh || echo "    shellcheck not installed; skipping"
	@echo "==> install.sh dry-run smoke (both backends, AP on/off, unattended on/off)..."
	@bash deploy/install.sh --dry-run --non-interactive --no-ap --no-unattended-upgrades >/dev/null 2>&1
	@bash deploy/install.sh --dry-run --non-interactive --ssid Smoke --ap-password pw --app-password app --display-backend vcgencmd >/dev/null 2>&1
	@echo "    install.sh dry-run OK"
	@echo "==> Linting GitHub workflows..."
	@command -v actionlint >/dev/null 2>&1 && actionlint || echo "    actionlint not installed; skipping"
	@echo "==> Linting frontend code..."
	cd $(UI_DIR) && npm run lint
	cd $(UI_DIR) && npm run check

# Suggests stdlib modernizations (e.g. strings.SplitSeq, slices.Contains). Opt-in
# rather than part of lint — not every suggestion is worth taking. Exits non-zero
# when it finds something. Run `make modernize FIX=1` to apply.
modernize: ## Suggest Go stdlib modernizations (FIX=1 applies them)
	@echo "==> Checking for Go modernizations..."
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest $(if $(FIX),-fix )./internal/... ./cmd/...

# --- Utilities ---

clean: ## Remove build artifacts
	@echo "==> Cleaning up..."
	rm -rf $(BUILD_DIR)
	rm -rf $(UI_DIR)/build
	rm -rf tmp/

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
