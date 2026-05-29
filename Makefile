# Pager — Development Lifecycle
# Usage: make dev | make build | make run | make clean

APP_NAME    := Pager
BIN_DIR     := bin
BRIDGE_BIN  := $(BIN_DIR)/pager-cc-bridge
APP_BIN     := $(BIN_DIR)/$(APP_NAME)
FRONTEND    := frontend
VITE_PORT   := 9245

.PHONY: dev build run clean test bridge frontend-deps frontend-build bindings icon lint

# ─── Development ─────────────────────────────────────────────────────────────

## Run in dev mode (Wails hot-reload + Vite HMR)
dev:
	wails3 dev -config ./build/config.yml -port $(VITE_PORT)

## Run frontend dev server only (for UI iteration without Go rebuild)
dev-frontend:
	cd $(FRONTEND) && npm run dev

# ─── Build ───────────────────────────────────────────────────────────────────

## Build production .app bundle
build: frontend-build
	wails3 build -config ./build/config.yml

## Build Go binary only (no frontend rebuild)
build-go:
	go build -o $(APP_BIN) .

## Build pager-cc-bridge binary
bridge:
	go build -o $(BRIDGE_BIN) ./cmd/bridge

# ─── Run ─────────────────────────────────────────────────────────────────────

## Run the built binary directly
run: build-go
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
	@echo "  make dev            Run in dev mode (Wails + Vite hot reload)"
	@echo "  make dev-frontend   Run frontend only (Vite HMR)"
	@echo "  make build          Build production .app"
	@echo "  make build-go       Build Go binary only"
	@echo "  make bridge         Build pager-cc-bridge"
	@echo "  make run            Build and run"
	@echo "  make frontend-deps  Install frontend npm deps"
	@echo "  make bindings       Regenerate Wails bindings"
	@echo "  make icon           Generate app icon"
	@echo "  make test           Run Go tests"
	@echo "  make lint           Go vet + TypeScript check"
	@echo "  make clean          Remove build artifacts"
	@echo "  make rebuild        Clean + full rebuild"
