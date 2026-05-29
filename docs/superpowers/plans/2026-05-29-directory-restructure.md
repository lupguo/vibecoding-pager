# Directory Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize Pager into a four-quadrant layered architecture (domain/adapter/infra/wails) with slog logging, clear naming conventions, and a minimal root `main.go`.

**Architecture:** Move all Go business code from root into `internal/` sub-packages organized by layer. `domain/entity/` holds shared structs (AgentEvent), `domain/session/` holds the Registry, `adapter/` wraps external I/O, `infra/` provides logging and config, `wails/` handles Wails framework bindings. The new `main.go` does only assembly + Run.

**Tech Stack:** Go 1.25, Wails v3 alpha.96, log/slog (stdlib)

---

## File Structure

### New files to create
| File | Responsibility |
|------|----------------|
| `internal/domain/entity/event.go` | AgentEvent struct + all constants (moved from internal/event) |
| `internal/domain/session/registry.go` | Registry state machine (moved from internal/registry) |
| `internal/domain/session/registry_test.go` | Registry tests (moved) |
| `internal/adapter/httpapi/server.go` | HTTP server (moved from internal/server) |
| `internal/adapter/httpapi/server_test.go` | Server tests (moved) |
| `internal/adapter/notify/notify.go` | macOS notifications (moved) |
| `internal/adapter/terminal/jump.go` | Terminal tab jump (moved) |
| `internal/adapter/bridge/extractor.go` | Content extraction (moved) |
| `internal/adapter/bridge/extractor_test.go` | Extractor tests (moved) |
| `internal/adapter/bridge/attention.go` | Attention level logic (moved) |
| `internal/adapter/bridge/attention_test.go` | Attention tests (moved) |
| `internal/adapter/bridge/poster.go` | HTTP POST client (moved) |
| `internal/adapter/bridge/types.go` | Input types (moved) |
| `internal/infra/log/log.go` | slog initialization (new) |
| `internal/infra/config/config.go` | Settings persistence (moved from internal/settings) |
| `internal/infra/config/config_test.go` | Config tests (moved) |
| `internal/infra/store/store.go` | Store interface placeholder (new) |
| `internal/wails/app.go` | PagerApp lifecycle (moved from root app.go) |
| `internal/wails/hotkey.go` | HotkeyManager (moved from root hotkey.go) |
| `internal/wails/session_svc.go` | SessionBinding (moved from root service.go) |
| `internal/wails/settings_svc.go` | SettingsBinding (moved from root settings_service.go) |
| `internal/wails/assets/tray-icon.png` | Tray icon 1x (moved) |
| `internal/wails/assets/tray-icon@2x.png` | Tray icon 2x (moved) |

### Files to delete after migration
| File | Reason |
|------|--------|
| `app.go` | Moved to internal/wails/ |
| `service.go` | Moved to internal/wails/ |
| `settings_service.go` | Moved to internal/wails/ |
| `hotkey.go` | Moved to internal/wails/ |
| `internal/event/` | Moved to domain/entity/ |
| `internal/registry/` | Moved to domain/session/ |
| `internal/server/` | Moved to adapter/httpapi/ |
| `internal/notify/` | Moved to adapter/notify/ |
| `internal/terminal/` | Moved to adapter/terminal/ |
| `internal/bridge/` | Moved to adapter/bridge/ |
| `internal/settings/` | Moved to infra/config/ |
| `assets/` | Moved to internal/wails/assets/ |
| `bridge` (binary) | Build artifact |

---

## Task 1: Create infra/log — slog Module Logger

**Files:**
- Create: `internal/infra/log/log.go`

- [ ] **Step 1: Create the log package**

```go
// internal/infra/log/log.go
package log

import (
	"log/slog"
	"os"
)

var defaultLevel = &slog.LevelVar{}

// Init sets up the global slog handler with text output to stderr.
func Init(level slog.Level) {
	defaultLevel.Set(level)
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: defaultLevel,
	})
	slog.SetDefault(slog.New(handler))
}

// Module returns a logger with a "module" attribute for filtering.
func Module(name string) *slog.Logger {
	return slog.Default().With("module", name)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/infra/log/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/infra/log/
git commit -m "feat(infra): add slog logger module with Init and Module factory"
```

---

## Task 2: Create infra/config — Settings Persistence

**Files:**
- Create: `internal/infra/config/config.go`
- Create: `internal/infra/config/config_test.go`

- [ ] **Step 1: Create config.go (copy from internal/settings/settings.go with package rename)**

```go
// internal/infra/config/config.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds all user-configurable preferences.
type Settings struct {
	Language          string `json:"language"`
	Theme             string `json:"theme"`
	Opacity           int    `json:"opacity"`
	HotkeyToggle     string `json:"hotkey_toggle"`
	NotificationLevel string `json:"notification_level"`
}

// Defaults returns the default settings.
func Defaults() Settings {
	return Settings{
		Language:          "zh",
		Theme:             "system",
		Opacity:           75,
		HotkeyToggle:     "Alt+E",
		NotificationLevel: "attention_only",
	}
}

// DefaultPath returns ~/.config/pager/settings.json
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "settings.json")
}

// LoadFrom reads settings from the given path.
// Returns defaults if file does not exist.
func LoadFrom(path string) (Settings, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Defaults(), err
	}
	return cfg, nil
}

// SaveTo writes settings to the given path atomically (tmp + rename).
func SaveTo(path string, cfg Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 2: Create config_test.go (copy from internal/settings/settings_test.go with package rename)**

```go
// internal/infra/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenFileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Language != "zh" {
		t.Errorf("expected default language 'zh', got '%s'", cfg.Language)
	}
	if cfg.Theme != "system" {
		t.Errorf("expected default theme 'system', got '%s'", cfg.Theme)
	}
	if cfg.Opacity != 75 {
		t.Errorf("expected default opacity 75, got %d", cfg.Opacity)
	}
	if cfg.HotkeyToggle != "Alt+E" {
		t.Errorf("expected default hotkey 'Alt+E', got '%s'", cfg.HotkeyToggle)
	}
	if cfg.NotificationLevel != "attention_only" {
		t.Errorf("expected default notification_level 'attention_only', got '%s'", cfg.NotificationLevel)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to not be created on Load")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "settings.json")

	cfg := Settings{
		Language:          "en",
		Theme:             "dark",
		Opacity:           50,
		HotkeyToggle:     "Ctrl+Shift+P",
		NotificationLevel: "all",
	}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	if loaded != cfg {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", loaded, cfg)
	}
}

func TestLoadFromCorruptedFileReturnsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	os.WriteFile(path, []byte("{invalid json"), 0o644)

	cfg, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
	if cfg != Defaults() {
		t.Errorf("expected defaults on corruption, got %+v", cfg)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infra/config/ -v`
Expected: 3 tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infra/config/
git commit -m "feat(infra): add config package (moved from internal/settings)"
```

---

## Task 3: Create infra/store — Store Interface Placeholder

**Files:**
- Create: `internal/infra/store/store.go`

- [ ] **Step 1: Create store interface**

```go
// internal/infra/store/store.go
package store

// SessionStore abstracts session persistence.
// v1: in-memory (Registry itself serves as the store).
// v1.5: SQLite implementation.
type SessionStore interface {
	SaveSession(key string, data []byte) error
	LoadSession(key string) ([]byte, error)
	ListSessions() ([][]byte, error)
	RemoveSession(key string) error
}
```

Note: Using `[]byte` (serialized JSON) rather than importing domain types keeps infra independent of domain.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/infra/store/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/infra/store/
git commit -m "feat(infra): add store interface placeholder for future persistence"
```

---

## Task 4: Create domain/entity — Shared Domain Types

**Files:**
- Create: `internal/domain/entity/event.go`

- [ ] **Step 1: Create event.go (content from internal/event/types.go, package rename to entity)**

```go
// internal/domain/entity/event.go
package entity

import (
	"encoding/json"
	"time"
)

// EventType constants
const (
	EventPreToolUse  = "pre_tool_use"
	EventPostToolUse = "post_tool_use"
	EventStop        = "stop"
	EventError       = "error"
	EventNotification = "notification"
)

// Agent constants
const (
	AgentClaudeCode = "claude-code"
	AgentCodex      = "codex"
)

// SessionStatus constants
const (
	StatusWaiting  = "waiting"
	StatusActive   = "active"
	StatusFinished = "finished"
	StatusError    = "error"
)

// AttentionLevel constants
const (
	AttentionAttention = "attention"
	AttentionRunning   = "running"
	AttentionDone      = "done"
)

// AgentEvent is the single cross-layer data structure.
// Bridge fills all fields; server and UI are read-only consumers.
type AgentEvent struct {
	Agent          string `json:"agent"`
	Host           string `json:"host"`
	CWD            string `json:"cwd"`
	TTY            string `json:"tty"`
	SessionID      string `json:"session_id"`
	TermProgram    string `json:"term_program"`
	ITermSessionID string `json:"iterm_session_id,omitempty"`
	EventType      string `json:"event_type"`
	ToolName       string `json:"tool_name"`
	ToolUseID      string `json:"tool_use_id"`
	Content        string `json:"content"`
	ContentRaw     string `json:"content_raw"`
	AttentionLevel string `json:"attention_level"`
	AgentLabel     string `json:"agent_label"`
	PermissionMode string `json:"permission_mode,omitempty"`
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// SessionKey returns the unique session identifier.
// Prefers CC-native session_id; falls back to host:cwd:tty triple.
func (e *AgentEvent) SessionKey() string {
	if e.SessionID != "" {
		return e.SessionID
	}
	return e.Host + ":" + e.CWD + ":" + e.TTY
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/domain/entity/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/domain/entity/
git commit -m "feat(domain): add entity package with AgentEvent and constants"
```

---

## Task 5: Create domain/session — Registry State Machine

**Files:**
- Create: `internal/domain/session/registry.go`
- Create: `internal/domain/session/registry_test.go`

- [ ] **Step 1: Create registry.go (from internal/registry/registry.go, update imports to use domain/entity)**

The key changes from the original:
- Package name: `session` (was `registry`)
- Import `pager/internal/domain/entity` instead of `pager/internal/event`
- All references to `event.Xxx` become `entity.Xxx`
- The `Session` struct stays here (it's session-domain specific)

```go
// internal/domain/session/registry.go
package session

import (
	"sort"
	"sync"
	"time"

	"pager/internal/domain/entity"
)

// Session represents an active agent session.
type Session struct {
	Key            string                        `json:"Key"`
	Agent          string                        `json:"Agent"`
	Host           string                        `json:"Host"`
	CWD            string                        `json:"CWD"`
	TTY            string                        `json:"TTY"`
	TermProgram    string                        `json:"TermProgram"`
	ITermSessionID string                        `json:"ITermSessionID"`
	Status         string                        `json:"Status"`
	AttentionLevel string                        `json:"AttentionLevel"`
	AgentLabel     string                        `json:"AgentLabel"`
	SessionID      string                        `json:"SessionID"`
	LastEvent      *entity.AgentEvent            `json:"LastEvent"`
	PendingTools   map[string]*entity.AgentEvent `json:"PendingTools"`
	UpdatedAt      time.Time                     `json:"UpdatedAt"`
}

// Registry is a thread-safe session registry.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	onChange func(sessions []*Session)
}

// New creates a Registry with an onChange callback.
func New(onChange func([]*Session)) *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
		onChange: onChange,
	}
}

// Apply processes an AgentEvent and updates the Registry state.
func (r *Registry) Apply(e *entity.AgentEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := e.SessionKey()
	s, exists := r.sessions[key]
	if !exists {
		s = &Session{
			Key:          key,
			Agent:        e.Agent,
			Host:         e.Host,
			CWD:          e.CWD,
			TTY:          e.TTY,
			SessionID:    e.SessionID,
			PendingTools: make(map[string]*entity.AgentEvent),
		}
		r.sessions[key] = s
	}

	if e.TermProgram != "" {
		s.TermProgram = e.TermProgram
	}
	if e.ITermSessionID != "" {
		s.ITermSessionID = e.ITermSessionID
	}

	s.LastEvent = e
	s.UpdatedAt = e.Timestamp
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}

	if e.AttentionLevel != "" {
		s.AttentionLevel = e.AttentionLevel
	}
	if e.AgentLabel != "" {
		s.AgentLabel = e.AgentLabel
	}

	switch e.EventType {
	case entity.EventPreToolUse:
		s.Status = entity.StatusWaiting
		if e.ToolUseID != "" {
			s.PendingTools[e.ToolUseID] = e
		}
	case entity.EventPostToolUse:
		if e.ToolUseID != "" {
			delete(s.PendingTools, e.ToolUseID)
		}
		if len(s.PendingTools) == 0 {
			s.Status = entity.StatusActive
		}
	case entity.EventStop:
		s.Status = entity.StatusFinished
		s.PendingTools = make(map[string]*entity.AgentEvent)
	case entity.EventError:
		s.Status = entity.StatusError
	}

	if r.onChange != nil {
		r.onChange(r.listSortedLocked())
	}
}

// ListSorted returns sessions ordered by UpdatedAt descending.
func (r *Registry) ListSorted() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listSortedLocked()
}

func (r *Registry) listSortedLocked() []*Session {
	result := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// GetByKey finds a session by its key.
func (r *Registry) GetByKey(key string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[key]
	return s, ok
}

// Remove deletes a session by key.
func (r *Registry) Remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, key)
	if r.onChange != nil {
		r.onChange(r.listSortedLocked())
	}
}
```

- [ ] **Step 2: Create registry_test.go (update imports)**

Copy from `internal/registry/registry_test.go`, change:
- Package: `session`
- Import: `pager/internal/domain/entity` instead of `pager/internal/event`
- All `event.Xxx` → `entity.Xxx`
- All `registry.Xxx` → direct (same package)

- [ ] **Step 3: Run tests**

Run: `go test ./internal/domain/session/ -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/domain/session/
git commit -m "feat(domain): add session package with Registry (moved from internal/registry)"
```

---

## Task 6: Create adapter/httpapi — HTTP Server

**Files:**
- Create: `internal/adapter/httpapi/server.go`
- Create: `internal/adapter/httpapi/server_test.go`

- [ ] **Step 1: Create server.go (from internal/server/server.go, update imports)**

Changes:
- Package: `httpapi`
- Import: `pager/internal/domain/entity` (was `pager/internal/event`)
- Import: `pager/internal/domain/session` (was `pager/internal/registry`)
- Replace `event.AgentEvent` → `entity.AgentEvent`
- Replace `registry.Registry` → `session.Registry`
- Replace `[]*registry.Session` → `[]*session.Session` in function signatures
- Replace `log.Printf` → `slog` (import `log/slog`)

- [ ] **Step 2: Create server_test.go (update imports similarly)**

- [ ] **Step 3: Run tests**

Run: `go test ./internal/adapter/httpapi/ -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/httpapi/
git commit -m "feat(adapter): add httpapi server (moved from internal/server)"
```

---

## Task 7: Create adapter/notify — macOS Notifications

**Files:**
- Create: `internal/adapter/notify/notify.go`

- [ ] **Step 1: Create notify.go (from internal/notify/notify.go, update imports)**

Changes:
- Import: `pager/internal/domain/entity` (was `pager/internal/event`)
- All `event.Xxx` → `entity.Xxx`
- Replace `log` usage with `log/slog`

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/adapter/notify/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/notify/
git commit -m "feat(adapter): add notify package (moved from internal/notify)"
```

---

## Task 8: Create adapter/terminal — Terminal Jump

**Files:**
- Create: `internal/adapter/terminal/jump.go`

- [ ] **Step 1: Create jump.go (from internal/terminal/jump.go, unchanged package name)**

The package name is already `terminal`, just move the file. Replace `log` with `log/slog`.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/adapter/terminal/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/terminal/
git commit -m "feat(adapter): add terminal package (moved from internal/terminal)"
```

---

## Task 9: Create adapter/bridge — CC Hook Event Parsing

**Files:**
- Create: `internal/adapter/bridge/extractor.go`
- Create: `internal/adapter/bridge/extractor_test.go`
- Create: `internal/adapter/bridge/attention.go`
- Create: `internal/adapter/bridge/attention_test.go`
- Create: `internal/adapter/bridge/poster.go`
- Create: `internal/adapter/bridge/types.go`

- [ ] **Step 1: Copy all files from internal/bridge/ to internal/adapter/bridge/**

Changes:
- Import: `pager/internal/domain/entity` (was `pager/internal/event`)
- All `event.Xxx` constants → `entity.Xxx`

- [ ] **Step 2: Run tests**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/bridge/
git commit -m "feat(adapter): add bridge package (moved from internal/bridge)"
```

---

## Task 10: Create internal/wails — Framework Binding Layer

**Files:**
- Create: `internal/wails/app.go`
- Create: `internal/wails/hotkey.go`
- Create: `internal/wails/session_svc.go`
- Create: `internal/wails/settings_svc.go`
- Create: `internal/wails/assets/tray-icon.png`
- Create: `internal/wails/assets/tray-icon@2x.png`

- [ ] **Step 1: Create app.go — PagerApp with NewPagerApp(assets) assembler**

This is the largest file. It combines:
- The old `app.go` logic (registry onChange, tray icon, lifecycle)
- The assembly logic from `main.go` (creating windows, tray, menu)
- Export a `NewPagerApp(assets embed.FS) *application.App` function

Key struct:
```go
package wails

// PagerApp holds all Wails application state and orchestrates lifecycle.
type PagerApp struct {
    reg    *session.Registry
    srv    *httpapi.Server
    tray   *application.SystemTray
    app    *application.App
}
```

Import paths:
- `pager/internal/domain/entity`
- `pager/internal/domain/session`
- `pager/internal/adapter/httpapi`
- `pager/internal/adapter/notify`
- `pager/internal/adapter/terminal`
- `pager/internal/infra/config`
- `pager/internal/infra/log`

- [ ] **Step 2: Create hotkey.go (from root hotkey.go, package wails)**

Changes:
- Package: `wails`
- Keep all hotkey logic, just change package name
- Export `RegisterHotkey` and `ParseHotkey` (capitalize)

- [ ] **Step 3: Create session_svc.go (from root service.go)**

Changes:
- Package: `wails`
- Rename struct: `SessionService` → `SessionBinding`
- Import: `pager/internal/domain/session` (was `pager/internal/registry`)
- Import: `pager/internal/adapter/terminal`
- `*registry.Registry` → `*session.Registry`

- [ ] **Step 4: Create settings_svc.go (from root settings_service.go)**

Changes:
- Package: `wails`
- Rename struct: `SettingsService` → `SettingsBinding`
- Import: `pager/internal/infra/config` (was `pager/internal/settings`)
- `settings.Settings` → `config.Settings`
- `settings.LoadFrom` → `config.LoadFrom`
- `settings.SaveTo` → `config.SaveTo`
- `settings.DefaultPath` → `config.DefaultPath`

- [ ] **Step 5: Move tray icon assets**

```bash
mkdir -p internal/wails/assets
cp assets/tray-icon.png internal/wails/assets/
cp assets/tray-icon@2x.png internal/wails/assets/
```

- [ ] **Step 6: Verify the package compiles**

Run: `go build ./internal/wails/`
Expected: Success (may need stub main.go temporarily)

- [ ] **Step 7: Commit**

```bash
git add internal/wails/
git commit -m "feat(wails): add framework binding layer (moved from root)"
```

---

## Task 11: Rewrite main.go — Minimal Entry Point

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Rewrite main.go to the minimal version**

```go
package main

import (
	"embed"
	"log/slog"
	"os"

	infralog "pager/internal/infra/log"
	"pager/internal/wails"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	infralog.Init(slog.LevelInfo)

	app := wails.NewPagerApp(assets)
	if err := app.Run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify full build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "refactor: minimal main.go — delegates to internal/wails.NewPagerApp"
```

---

## Task 12: Update cmd/bridge — Fix Imports

**Files:**
- Modify: `cmd/bridge/main.go`

- [ ] **Step 1: Update imports**

Change:
- `"pager/internal/bridge"` → `"pager/internal/adapter/bridge"`
- `"pager/internal/event"` → `"pager/internal/domain/entity"`
- All `event.Xxx` → `entity.Xxx`
- All `bridge.Xxx` stays the same (package name unchanged)

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/bridge/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add cmd/bridge/
git commit -m "refactor(bridge): update imports to new package structure"
```

---

## Task 13: Delete Old Directories + Cleanup

**Files:**
- Delete: `app.go`, `service.go`, `settings_service.go`, `hotkey.go`
- Delete: `internal/event/`, `internal/registry/`, `internal/server/`, `internal/notify/`, `internal/terminal/`, `internal/bridge/`, `internal/settings/`
- Delete: `assets/` directory
- Delete: `bridge` binary
- Modify: `.gitignore`

- [ ] **Step 1: Remove old Go files from root**

```bash
rm app.go service.go settings_service.go hotkey.go
```

- [ ] **Step 2: Remove old internal directories**

```bash
rm -rf internal/event internal/registry internal/server internal/notify internal/terminal internal/bridge internal/settings
```

- [ ] **Step 3: Remove old assets directory and bridge binary**

```bash
rm -rf assets
rm -f bridge
```

- [ ] **Step 4: Update .gitignore**

Add:
```
bridge
.task/
```

- [ ] **Step 5: Verify everything still builds and tests pass**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove old directory structure (migrated to layered architecture)"
```

---

## Task 14: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Rewrite CLAUDE.md to reflect new architecture**

The CLAUDE.md should be updated to:
- Reference `ARCHITECTURE.md` for directory conventions
- Update the Architecture section to show the four-layer structure
- Update Key Files table to new paths
- Update Commands section (already done — Makefile)
- Keep Non-Negotiable Decisions, Session State Machine, Code Conventions
- Update Code Conventions to mention slog and new naming

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for new layered architecture"
```

---

## Task 15: Regenerate Wails Bindings + Final Verification

**Files:**
- Regenerated: `frontend/bindings/`

- [ ] **Step 1: Regenerate bindings**

Run: `wails3 generate bindings`
Expected: Success — bindings reflect new package paths

- [ ] **Step 2: Rebuild frontend**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 3: Full build + test**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add frontend/bindings/
git commit -m "chore: regenerate Wails bindings for new package structure"
```
