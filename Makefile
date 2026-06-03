# Pager — Development Lifecycle
# Usage: make dev | make build | make run | make clean

APP_NAME    := Pager
BIN_DIR     := bin
BRIDGE_BIN  := $(BIN_DIR)/pager-cc-bridge
APP_BIN     := $(BIN_DIR)/$(APP_NAME)
FRONTEND    := frontend

# Suppress macOS linker version mismatch warnings (Xcode 26 vs Go default min 11.0)
GO_LDFLAGS  := -ldflags="-extldflags '-Wl,-w'"
VITE_PORT   := 9245
HTTP_PORT   := 7421

.PHONY: dev build run clean test bridge install-bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop

# ─── Development ─────────────────────────────────────────────────────────────

## Run in dev mode (Wails hot-reload + Vite HMR)
## Automatically kills any previous dev processes on the same ports.
dev: stop install-bridge
	wails3 dev -config ./build/config.yml -port $(VITE_PORT)

## Run frontend dev server only (for UI iteration without Go rebuild)
dev-frontend:
	cd $(FRONTEND) && npm run dev

# ─── Build ───────────────────────────────────────────────────────────────────

## Build production .app bundle (output: bin/Pager.app)
##
## Wails v3 alpha.96 split the old `wails3 build` into two phases:
##   - `wails3 build`   produces a raw Mach-O at bin/Pager
##   - `wails3 package` wraps that into bin/Pager.app/Contents/{MacOS,Resources}
##                      and ad-hoc codesigns it ("- " identity)
## We want the bundle for `open` + macOS NotificationService bundle-id
## requirement, so this target uses package. (`-config` flag was removed
## upstream — wails.json + build/config.yml are picked up automatically.)
build: frontend-build install-bridge
	wails3 package

## Build Go binary only (no frontend rebuild)
build-go:
	go build $(GO_LDFLAGS) -o $(APP_BIN) .

## Build pager-cc-bridge binary
bridge:
	go build -o $(BRIDGE_BIN) ./cmd/bridge

## Build bridge and report deployment metadata
install-bridge: bridge
	@echo "✓ bridge built at $(BRIDGE_BIN)"
	@echo "  mtime: $$(date -r $(BRIDGE_BIN) '+%F %T')"
	@echo "  sha:   $$(shasum -a 256 $(BRIDGE_BIN) | cut -c1-12)"

## Install CC hooks into ~/.claude/settings.json
install-hooks: bridge
	./scripts/install-hooks.sh CC $(BRIDGE_BIN)

## Install CodeBuddy hooks into ~/.codebuddy/settings.json
install-hooks-codebuddy: bridge
	./scripts/install-hooks.sh --agent CodeBuddy --settings_file ~/.codebuddy/settings.json --bridge $(BRIDGE_BIN)

# ─── Run ─────────────────────────────────────────────────────────────────────

## Run the built binary directly
run: build-go install-bridge
	./$(APP_BIN)

# ─── Frontend ────────────────────────────────────────────────────────────────

## Install frontend dependencies
frontend-deps:
	cd $(FRONTEND) && npm install

## Build frontend for production
frontend-build:
	cd $(FRONTEND) && npm run build

# ─── Code Generation ─────────────────────────────────────────────────────────

## Regenerate Wails bindings (after changing Go services)
bindings:
	wails3 generate bindings

## Generate app icon assets
icon:
	go run ./cmd/icongen

# ─── Quality ─────────────────────────────────────────────────────────────────

## Run all Go tests
test:
	go test ./...

## Run Go vet + build check
lint:
	go vet ./...
	cd $(FRONTEND) && npx tsc --noEmit

# ─── Housekeeping ────────────────────────────────────────────────────────────

## Stop any running Pager dev processes (frees ports 9245 + 7421)
stop:
	@lsof -ti:$(VITE_PORT) | xargs kill -9 2>/dev/null || true
	@lsof -ti:$(HTTP_PORT) | xargs kill -9 2>/dev/null || true
	@sleep 0.5

## Clean build artifacts
clean:
	rm -rf $(BIN_DIR) $(FRONTEND)/dist

## Full rebuild from scratch
rebuild: clean frontend-deps build

# ─── Help ────────────────────────────────────────────────────────────────────

## Show available targets
help:
	@echo "Pager Development Commands:"
	@echo ""
	@echo "  make dev            Run in dev mode (auto-kills previous instance)"
	@echo "  make stop           Stop running dev processes (free ports)"
	@echo "  make dev-frontend   Run frontend only (Vite HMR)"
	@echo "  make build          Build production .app"
	@echo "  make build-go       Build Go binary only"
	@echo "  make bridge         Build pager-cc-bridge"
	@echo "  make install-bridge Build bridge with deployment verification"
	@echo "  make install-hooks  Install CC hooks into ~/.claude/settings.json"
	@echo "  make install-hooks-codebuddy  Install CodeBuddy hooks into ~/.codebuddy/settings.json"
	@echo "  make run            Build and run"
	@echo "  make frontend-deps  Install frontend npm deps"
	@echo "  make bindings       Regenerate Wails bindings"
	@echo "  make icon           Generate app icon"
	@echo "  make test           Run Go tests"
	@echo "  make lint           Go vet + TypeScript check"
	@echo "  make clean          Remove build artifacts"
	@echo "  make rebuild        Clean + full rebuild"
