# Pager Config / Bindings / Build Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate three lifecycle inconsistencies in one pass: (1) move `frontend/bindings/` to build-time generation (no track), (2) split runtime data path into dev (`<repo>/.config/pager/` via `PAGER_CONFIG_DIR`) and prod (`~/Library/Application Support/Pager/`), (3) carry over two non-blocking polish items (DSL `//` negative test, drop unreachable `[认证]` defensive label).

**Architecture:** Single-process changes confined to `internal/infra/config/`, `internal/infra/store/` (caller side in `internal/wails/`), `internal/adapter/bridge/`, plus build-tooling (`Makefile`, `.gitignore`, `CLAUDE.md`). No new external dependencies. Config gains a shared `BaseDir()` resolver consulted by both `DefaultPath()` (settings.json) and a new `DefaultDBPath()` (pager.db). `make dev`/`run` inject `PAGER_CONFIG_DIR=$(PWD)/.config/pager`; `make build` ships an env-var-naive `.app` that uses the macOS-native default.

**Tech Stack:** Go 1.25, Wails v3 alpha.96, `wails3 generate bindings` CLI, existing `gopkg.in/yaml.v3`, `github.com/tidwall/gjson`. Pre-migrated runtime data already at `<repo>/.config/pager/` (66 sessions, 9782 events, integrity-verified).

---

## File Structure

### New files
- `internal/infra/config/config_test.go` — unit tests for `BaseDir()`

### Modified files
- `internal/infra/config/config.go` — add `BaseDir()`, refactor `DefaultPath()`, add `DefaultDBPath()`
- `internal/wails/app.go` — delete local `defaultDBPath()`, switch call site to `config.DefaultDBPath()`, prune unused imports (`os`, `path/filepath`)
- `internal/adapter/bridge/extractor.go` — delete the `case "auth_success"` arm in `notificationTypeLabel`
- `internal/adapter/bridge/extractor_test.go` — delete `TestExtractEventContent_Notification_AuthSuccess` (asserts unreachable branch)
- `internal/adapter/bridge/rules_test.go` — add `TestEvaluate_FallbackOperator_DocumentsHTTPURLLimitation` (pin `//` reserved-token semantics)
- `Makefile` — `.PHONY` add `bindings`; new `bindings:` target; wire `dev`/`run`/`build` to depend on it; inject `PAGER_CONFIG_DIR` for `dev`/`run`; help line
- `.gitignore` — add `.config/` and `frontend/bindings/`; delete obsolete `internal/infra/store/pager.db` line
- `CLAUDE.md` — insert two new sections ("Wails Bindings Lifecycle" + "Configuration Paths") between "Bridge Binary Lifecycle" and "Code Conventions"

### Git operations
- `git rm -rf --cached frontend/bindings/` (stops tracking 19 codegen files; working tree files preserved)

### Untouched
- `internal/domain/**`, `frontend/src/**`, all bridge core (extractor.go beyond the one delete, rules.go DSL engine)

---

## Task 1: Config path refactor + small polish

**Scope:** Pure code changes in two packages plus tests. Single commit. Spec sections covered: §3.2 (config + store path), §3.6.1 (DSL `//` test), §3.6.2 (drop `[认证]` label).

**Files:**
- Create: `internal/infra/config/config_test.go`
- Modify: `internal/infra/config/config.go`
- Modify: `internal/wails/app.go`
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`
- Modify: `internal/adapter/bridge/rules_test.go`

### Subtask 1A — Add `BaseDir()` and `DefaultDBPath()` to config

- [ ] **In `internal/infra/config/config.go`**, find the existing `DefaultPath` function (around line 47):

```go
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "settings.json")
}
```

Replace it with the following block (note: keep the existing `import` block; add no new imports):

```go
// BaseDir returns the directory under which all Pager runtime data
// (settings.json, pager.db, WAL files) lives. Resolution order:
//
//  1. PAGER_CONFIG_DIR env var (highest priority — used by `make dev`,
//     `make run`, and tests for isolation)
//  2. ~/Library/Application Support/Pager/ (macOS-native production default)
//
// The directory is NOT created here; callers should MkdirAll on first write.
func BaseDir() string {
	if envDir := os.Getenv("PAGER_CONFIG_DIR"); envDir != "" {
		return envDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Pager")
}

// DefaultPath returns the absolute path to settings.json under BaseDir().
func DefaultPath() string {
	return filepath.Join(BaseDir(), "settings.json")
}

// DefaultDBPath returns the absolute path to pager.db under BaseDir().
func DefaultDBPath() string {
	return filepath.Join(BaseDir(), "pager.db")
}
```

### Subtask 1B — Add unit tests for BaseDir

- [ ] **Create `internal/infra/config/config_test.go`** with this content:

```go
package config

import (
	"strings"
	"testing"
)

func TestBaseDir_Override(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "/tmp/pager-test")
	if got := BaseDir(); got != "/tmp/pager-test" {
		t.Errorf("BaseDir() = %q, want %q", got, "/tmp/pager-test")
	}
}

func TestBaseDir_Default(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "") // ensure no leakage from outer env
	got := BaseDir()
	if !strings.HasSuffix(got, "/Library/Application Support/Pager") {
		t.Errorf("BaseDir() = %q, want path ending in '/Library/Application Support/Pager'", got)
	}
}

func TestDefaultPath_UsesBaseDir(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "/tmp/pager-test")
	if got := DefaultPath(); got != "/tmp/pager-test/settings.json" {
		t.Errorf("DefaultPath() = %q, want %q", got, "/tmp/pager-test/settings.json")
	}
}

func TestDefaultDBPath_UsesBaseDir(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "/tmp/pager-test")
	if got := DefaultDBPath(); got != "/tmp/pager-test/pager.db" {
		t.Errorf("DefaultDBPath() = %q, want %q", got, "/tmp/pager-test/pager.db")
	}
}
```

`t.Setenv` is the stdlib idiom for scoped env-var override (auto-restored on test exit; available since Go 1.17).

### Subtask 1C — Switch app.go to use config.DefaultDBPath

- [ ] **In `internal/wails/app.go`**, find and DELETE the local `defaultDBPath` function (around lines 34-38):

```go
// defaultDBPath returns the SQLite database file path.
func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "pager.db")
}
```

- [ ] **Update the call site** (around line 50). Find:

```go
	eventStore, err := store.NewSQLiteStore(defaultDBPath())
```

Replace with:

```go
	eventStore, err := store.NewSQLiteStore(config.DefaultDBPath())
```

- [ ] **Verify imports**. After deleting `defaultDBPath`, the local `path/filepath` import in `app.go` may become unused. Run `go build ./internal/wails/` — if it errors with `imported and not used: path/filepath`, remove that import line. Same for `os` if it was used only by `defaultDBPath`.

(Do NOT remove `pager/internal/infra/config` — it's already imported for `config.LoadFrom(config.DefaultPath())` and remains in use.)

### Subtask 1D — Drop unreachable `[认证]` label

- [ ] **In `internal/adapter/bridge/extractor.go`**, find `notificationTypeLabel` (near the bottom of the file, around line 278):

```go
func notificationTypeLabel(ntype string) string {
	switch ntype {
	case "permission_prompt":
		return "[等授权] "
	case "idle_prompt":
		return "[等输入] "
	case "auth_success":
		return "[认证] " // Bug 1 already drops these at bridge entry; defensive.
	default:
		return ""
	}
}
```

DELETE the `auth_success` case so the function becomes:

```go
func notificationTypeLabel(ntype string) string {
	switch ntype {
	case "permission_prompt":
		return "[等授权] "
	case "idle_prompt":
		return "[等输入] "
	default:
		return ""
	}
}
```

### Subtask 1E — Delete the unreachable test

- [ ] **In `internal/adapter/bridge/extractor_test.go`**, find and DELETE the entire test function `TestExtractEventContent_Notification_AuthSuccess` (around the test list — search for "auth_success"):

```go
func TestExtractEventContent_Notification_AuthSuccess(t *testing.T) {
	in := &CCHookInput{NotificationType: "auth_success", Message: "auth_success: example-user"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[认证] auth_success: example-user" {
		t.Errorf("raw = %q", raw)
	}
}
```

Existing `TestExtractEventContent_Notification_UnknownType` covers the `default:` branch in `notificationTypeLabel` (`NotificationType: ""` → label `""`), so coverage is preserved.

### Subtask 1F — Pin DSL `//` reserved-token semantics

- [ ] **In `internal/adapter/bridge/rules_test.go`**, append this new test at the end of the file (after the existing `TestEvaluate_FilterChain_DefaultThenFirstline` or whatever comes last):

```go
// TestEvaluate_FallbackOperator_DocumentsHTTPURLLimitation pins the documented
// constraint that "//" inside a token body is reserved for the fallback
// operator. Future authors trying to use raw URL literals like "http://x" in
// a default: filter argument will trip this — by design.
//
// Trace for input "{$.url|default:http://x}" against payload "{}":
//   1. Token body "$.url|default:http://x" contains "//" → fallback chain
//   2. Split on "//": ["$.url|default:http:", "/x"]
//   3. Recurse on first part "$.url|default:http:":
//      - $.url path missing (exists=false)
//      - filter default:http: triggers, returns "http:"
//   4. First non-empty wins → "http:"
//
// The user wanted "http://x". They got "http:". The // got eaten as a
// fallback delimiter. This test exists so any future change to the parse
// order of // vs | gets caught and forces a deliberate decision.
func TestEvaluate_FallbackOperator_DocumentsHTTPURLLimitation(t *testing.T) {
	got := Evaluate("{$.url|default:http://x}", []byte(`{}`), "X")
	if got != "http:" {
		t.Errorf("got %q, want %q — // constraint may have changed; update doc + DSL", got, "http:")
	}
}
```

### Verification

- [ ] **Run all tests and vet**:
```bash
go test ./...
go vet ./...
```
Expected:
- `pager/internal/infra/config` package now has 4 PASS tests (BaseDir x2, DefaultPath x1, DefaultDBPath x1) — was 0 before.
- `pager/internal/adapter/bridge` package: net change is `-1 test (auth_success deleted) +1 test (DSL // negative) = 0`. All PASS.
- `pager/internal/wails` builds cleanly (no `defaultDBPath`, no unused imports).
- `go vet` clean.

If any failure: STOP and report `BLOCKED` with the failing output.

### Commit

- [ ] **Single commit**:
```bash
git add internal/infra/config/config.go \
        internal/infra/config/config_test.go \
        internal/wails/app.go \
        internal/adapter/bridge/extractor.go \
        internal/adapter/bridge/extractor_test.go \
        internal/adapter/bridge/rules_test.go
git commit -m "refactor(config): unify base dir for settings + DB; PAGER_CONFIG_DIR override

Adds config.BaseDir() consulted by both config.DefaultPath() (settings.json)
and new config.DefaultDBPath() (pager.db). Resolution: PAGER_CONFIG_DIR env
var > ~/Library/Application Support/Pager/ (macOS-native default, replaces
~/.config/pager/).

Move internal/wails/app.go from local defaultDBPath() to config.DefaultDBPath()
so settings + DB share the same base. dev mode (make dev/run, next commit) will
inject PAGER_CONFIG_DIR; .app builds get the macOS default with no env setup.

Bonus: drop the unreachable [认证] notificationTypeLabel arm (Bug 1's bridge-
entry shouldDrop already filters auth_success); pin DSL // reserved-token
semantics with TestEvaluate_FallbackOperator_DocumentsHTTPURLLimitation."
```

---

## Task 2: Build tooling + docs

**Scope:** Untrack `frontend/bindings/`, expand `.gitignore`, add `Makefile bindings:` target with prerequisite wiring + `PAGER_CONFIG_DIR` injection, document in `CLAUDE.md`. Single commit. Spec sections covered: §3.1 (bindings), §3.3 (Makefile), §3.4 (.gitignore), §3.5 (CLAUDE.md).

**Files:**
- Modify: `.gitignore`
- Modify: `Makefile`
- Modify: `CLAUDE.md`

**Git operations:** `git rm -rf --cached frontend/bindings/`

### Subtask 2A — Untrack frontend/bindings/

- [ ] **Run**:
```bash
git rm -rf --cached frontend/bindings/
```
Expected output: `rm 'frontend/bindings/encoding/json/index.js'` ... about 16 lines of `rm`. Working-tree files remain.

### Subtask 2B — .gitignore expansion

- [ ] **Modify `.gitignore`** with these specific edits.

Find the existing block:
```
# Frontend
frontend/node_modules/
frontend/dist/
```
Replace with:
```
# Frontend
frontend/node_modules/
frontend/dist/

# Wails bindings — codegen artifact, regenerated by `make bindings`
frontend/bindings/
```

Find the existing block:
```
# Test fixtures (local sandbox; not part of Go test suite)
test/
```
After it, insert:
```

# Application data (dev writes here via PAGER_CONFIG_DIR;
# prod writes to ~/Library/Application Support/Pager/)
.config/
```

Find and DELETE this block:
```
# Local app data (SQLite + settings live in ~/.config/pager/, but if cwd-relative
# DB files leak into the repo root or store/, ignore them)
internal/infra/store/pager.db
```
(After Task 1 + this task lands, the store always uses `config.BaseDir()` so the cwd-relative leak path is gone.)

### Subtask 2C — Makefile updates

- [ ] **In `Makefile`**, find the `.PHONY` line near the top (around line 15):
```makefile
.PHONY: dev build run clean test bridge install-bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop
```
Note `bindings` is already in `.PHONY` (no change needed — verify it's present; if missing add it). The current Makefile already has a `bindings:` target stub from before.

Inspect the existing `bindings` target (around line 78):
```makefile
## Regenerate Wails bindings (after changing Go services)
bindings:
	wails3 generate bindings
```
This is already correct. **No change to this target body**.

- [ ] **Wire dev/run/build to depend on bindings**.

Find:
```makefile
dev: stop install-bridge
	wails3 dev -config ./build/config.yml -port $(VITE_PORT)
```
Replace with:
```makefile
dev: stop install-bridge bindings
	PAGER_CONFIG_DIR="$(PWD)/.config/pager" \
		wails3 dev -config ./build/config.yml -port $(VITE_PORT)
```

Find:
```makefile
build: frontend-build install-bridge
	wails3 package
```
Replace with:
```makefile
build: frontend-build install-bridge bindings
	wails3 package
```

Find:
```makefile
run: build-go install-bridge
	./$(APP_BIN)
```
Replace with:
```makefile
run: build-go install-bridge bindings
	PAGER_CONFIG_DIR="$(PWD)/.config/pager" ./$(APP_BIN)
```

- [ ] **Help-target line**.

Find in the `help` target (around lines 121-122):
```makefile
	@echo "  make bridge         Build pager-cc-bridge"
	@echo "  make install-bridge Build bridge with deployment verification"
```
Insert between them, so the order becomes bridge → install-bridge → bindings. Actually a better placement is bridge → install-bridge → bindings, so insert AFTER the install-bridge line:

After:
```makefile
	@echo "  make install-bridge Build bridge with deployment verification"
```
Insert:
```makefile
	@echo "  make bindings       Regenerate Wails bindings (Go → TS, codegen)"
```

(There may already be a `bindings` line elsewhere in `help`; if present, leave it where it is and skip this step.)

### Subtask 2D — CLAUDE.md two new sections

- [ ] **In `CLAUDE.md`**, locate the existing `## Bridge Binary Lifecycle` section. Find the end of that section (just before the next `##` heading, which should be `## Code Conventions`).

Insert these two new sections immediately AFTER `## Bridge Binary Lifecycle`'s body and BEFORE `## Code Conventions`:

```markdown
## Wails Bindings Lifecycle

`frontend/bindings/` is build-output, generated by `wails3 generate bindings`
from Go services in `internal/wails/*_svc.go` and entities in
`internal/domain/entity/`. **Not tracked in git.**

`make dev` / `make run` / `make build` regenerate bindings as a prerequisite
(via the Makefile `bindings` target). If you change a `XxxBinding` method
signature or an entity that bindings reference, the next `make dev` picks it
up automatically.

If you ever see "binding X not found" in the frontend console, run
`make bindings` manually — fresh checkouts also need it before first dev.

## Configuration Paths

Pager runtime data (`settings.json`, `pager.db`, WAL files) lives under
a single base directory chosen at startup by `config.BaseDir()`:

| Mode                  | Trigger                              | BaseDir()                                  |
|-----------------------|--------------------------------------|--------------------------------------------|
| Development           | `make dev` / `make run`              | `<repo>/.config/pager/`                    |
| Production (.app)     | Double-click Pager.app               | `~/Library/Application Support/Pager/`     |
| Test / ad-hoc         | `PAGER_CONFIG_DIR=/tmp/X ./bin/Pager` | `/tmp/X/`                                  |

Resolution order in `config.BaseDir()`:
1. `PAGER_CONFIG_DIR` env var (highest priority)
2. `~/Library/Application Support/Pager/` (macOS-native default)

`make dev` / `make run` set `PAGER_CONFIG_DIR=$(PWD)/.config/pager` so dev
work never pollutes your installed Pager.app's data. The dev directory
`<repo>/.config/` is gitignored.

`make build` does NOT set the env var — the resulting `.app` ships to end
users who get the macOS-native default automatically.
```

### Verification

- [ ] **`make help`**:
```bash
make help
```
Expected: output includes both `make bindings` and `make install-bridge` lines.

- [ ] **`make bindings` standalone**:
```bash
make bindings
```
Expected: regenerates files under `frontend/bindings/` (or no-op if already up-to-date). No errors.

- [ ] **`git status`**:
```bash
git status --short
```
Expected: `frontend/bindings/` files no longer show. `.config/` (which has the migrated 132MB of data) does not show. The only modified files staged for commit should be `.gitignore`, `Makefile`, `CLAUDE.md`, plus the bunch of `D` (deleted from index) for `frontend/bindings/...`.

- [ ] **`grep` sanity**:
```bash
grep -A2 "Wails Bindings Lifecycle" CLAUDE.md | head -5
grep -A2 "Configuration Paths" CLAUDE.md | head -5
```
Both should print their section headings followed by the first paragraph.

### Commit

- [ ] **Single commit** combining the untrack + .gitignore + Makefile + CLAUDE.md changes:
```bash
git add .gitignore Makefile CLAUDE.md
# git rm --cached output is already staged from Subtask 2A
git commit -m "build(make+docs): bindings as build artifact + dev/prod path docs

Stop tracking frontend/bindings/ (codegen output of wails3 generate
bindings, was a 16-file mixed-tracking mess). Wire 'bindings' into
dev/run/build prerequisites alongside install-bridge. dev/run inject
PAGER_CONFIG_DIR=\$(PWD)/.config/pager so dev data never collides with
production Pager.app data.

.gitignore: add .config/ (dev runtime data) + frontend/bindings/.
Drop the obsolete internal/infra/store/pager.db rule (no longer
reachable now that store uses config.BaseDir()).

CLAUDE.md gets two new lifecycle sections — Wails Bindings Lifecycle
and Configuration Paths — alongside the existing Bridge Binary Lifecycle."
```

---

## Task 3: End-to-end verification

**Scope:** Verification only — no code changes unless a defect surfaces. Confirm `make dev` regenerates bindings + bridge automatically, Pager opens with the migrated 66 sessions visible, and the env-var path actually flows from Makefile → process → BaseDir(). Spec section: §4 (testing plan) end-to-end items.

**Files:** None (verification only)

### Subtask 3A — Cold-cache fresh make

- [ ] **Wipe codegen artifacts to prove regeneration**:
```bash
rm -rf frontend/bindings/
ls -la bin/pager-cc-bridge 2>&1 | head -1   # note current mtime
```

- [ ] **Run dev with PAGER process visible**. In one terminal:
```bash
make dev
```
Expected output sequence (within ~10 seconds):
1. `lsof -ti:9245 | xargs kill -9` (the `stop` prereq, no-op if nothing listening)
2. `go build -o bin/pager-cc-bridge ./cmd/bridge` (install-bridge)
3. `✓ bridge built at bin/pager-cc-bridge` + mtime + sha
4. `wails3 generate bindings` (the `bindings` prereq)
5. `wails3 dev -config ./build/config.yml -port 9245` running

If any of these steps is missing or out-of-order, STOP — Makefile prerequisite ordering is broken.

### Subtask 3B — Verify env-var injection reached the running process

- [ ] **In another terminal**, while `make dev` is running:
```bash
ps -E -p $(lsof -ti:7421 -sTCP:LISTEN) 2>/dev/null | tr ' ' '\n' | grep PAGER_CONFIG_DIR
```
Expected: line like `PAGER_CONFIG_DIR=/private/data/projects/github.com/sapaude/pager/.config/pager`

(The `-E` flag on macOS `ps` shows the process environment. If the running binary is the Wails dev wrapper rather than Pager itself, search by Pager's binary name instead:
```bash
ps -E -p $(pgrep -f 'Pager$' | head -1) 2>/dev/null | tr ' ' '\n' | grep PAGER_CONFIG_DIR
```
)

If `PAGER_CONFIG_DIR` is not set in the process environment, the Makefile env-var injection didn't reach the spawned binary — STOP and report `BLOCKED`.

### Subtask 3C — Verify migrated data is visible

- [ ] **Open the Pager popup window** (Alt+E global hotkey, or click the menubar icon).

- [ ] **Spot-check session count**. Confirm:
- The session list is NOT empty (would be empty if BaseDir() resolved to a path with no DB).
- At least one session shows the `2026-06-04` or earlier dates (proves DB at `<repo>/.config/pager/pager.db` is being read).

- [ ] **DB sanity from CLI**:
```bash
sqlite3 .config/pager/pager.db \
  "SELECT COUNT(*) FROM t_sessions WHERE deleted_at IS NULL"
```
Expected: positive number (originally 66 active sessions; may differ slightly if some were dismissed).

If the UI is empty but the DB has rows, the BaseDir resolution is wrong — STOP and report `BLOCKED`.

### Subtask 3D — Verify prod (env-var-naive) path works

This proves the macOS-native default code path; we don't need to actually ship and install the .app to validate it. A unit-level confirmation suffices.

- [ ] **Run BaseDir tests in env-naive mode**:
```bash
go test ./internal/infra/config/ -run "TestBaseDir_Default|TestDefaultPath_UsesBaseDir|TestDefaultDBPath_UsesBaseDir" -v
```
Expected: 3 PASS.

- [ ] **Manual sanity (optional, no commit)**: in a scratch shell, build and run without env var to prove no crash on missing dir:
```bash
unset PAGER_CONFIG_DIR
go build -o /tmp/pager-test .
ls ~/Library/Application\ Support/Pager/ 2>&1 | head -3   # may be empty / not exist before first run
# DON'T actually run /tmp/pager-test (it would create production data on your machine)
rm /tmp/pager-test
```
Expected: build succeeds without errors; `Library/Application Support/Pager/` may or may not exist yet (created on first prod write).

### Subtask 3E — Stop dev cleanly

- [ ] **Stop dev**:
```bash
make stop
```
Expected: kills :9245 and :7421, no error output.

### Subtask 3F — Cleanup commit (only if defect surfaced)

If any subtask above produced unexpected behavior:
- Investigate root cause.
- Make minimal fix (config.go, Makefile, etc.).
- Run `go test ./...` + `go vet ./...` to confirm green.
- Commit with `fix(...): minor follow-up from e2e verification` style message.

If all subtasks pass: T3 has no commit.

### Report

State explicitly in your report:
- ✅ make dev cold-cache invoked install-bridge + bindings + wails3 dev in correct order
- ✅ PAGER_CONFIG_DIR=<repo>/.config/pager in running process env
- ✅ Pager UI shows migrated session count (N sessions)
- ✅ TestBaseDir_Default + 2 path tests PASS in env-naive mode
- ✅ make stop terminates cleanly

OR identify the specific subtask that failed, with reproduction command + observed output.

---

## Self-Review

### Spec coverage

| Spec § | Implementing task |
|---|---|
| §3.1 Bindings (untrack + Makefile target + 3 prereqs + help line) | Task 2 (subtasks 2A, 2C) |
| §3.2 Config path (BaseDir + DefaultPath + DefaultDBPath + tests + caller-side switch) | Task 1 (subtasks 1A, 1B, 1C) |
| §3.3 Makefile changes | Task 2 (subtask 2C) |
| §3.4 .gitignore expansion | Task 2 (subtask 2B) |
| §3.5 CLAUDE.md two sections | Task 2 (subtask 2D) |
| §3.6.1 DSL `//` negative test | Task 1 (subtask 1F) |
| §3.6.2 Drop `[认证]` defensive label | Task 1 (subtasks 1D, 1E) |
| §4 testing plan (unit) | Task 1 verification step |
| §4 testing plan (e2e) | Task 3 |

No gaps.

### Placeholder scan

Searched for forbidden patterns: `TBD`, `TODO`, "implement later", "appropriate error handling", "similar to Task N". None present. Every code step contains complete, paste-ready code.

### Type / signature consistency

- `config.BaseDir() string` — defined Task 1A, called by `DefaultPath()` and `DefaultDBPath()` (1A), tested in 1B.
- `config.DefaultPath() string` — refactored to use `BaseDir()` in 1A; existing call site `config.LoadFrom(config.DefaultPath())` in `app.go` continues to compile.
- `config.DefaultDBPath() string` — new in 1A, called by `app.go` in 1C, tested in 1B.
- Local `defaultDBPath()` in `app.go` — DELETED in 1C (no later reference).
- `notificationTypeLabel(ntype string) string` — signature unchanged in 1D; one case removed; remaining cases still match `extractor_test.go` tests for `permission_prompt` and `idle_prompt`.
- `Evaluate(rule string, payload []byte, toolName string) string` — unchanged; new test in 1F uses existing signature.

No drift between task definitions and consumers.

### Notes for the implementer

- **Task 1 has 6 subtasks but only 1 commit**: do all the file changes first, run verification once, single commit. Splitting per-subtask would create commits that don't compile (e.g., 1C touches app.go but 1A/1B add the symbols it depends on).
- **Task 2's `frontend/bindings/` regeneration**: after `git rm -rf --cached`, the working-tree files stay. `make bindings` (run as part of verification or any later `make dev`) will overwrite them. The 16+3 file deletions in the index are STAGED automatically by `git rm --cached`; you don't need to `git add` them again.
- **Task 3 needs Pager NOT already running** when starting (otherwise `make dev`'s `stop` prereq kills the existing process; harmless but be aware). The user has confirmed Pager is currently stopped.
- **Filter argument with comma** in DSL filters (e.g., `bool:✓写入,✗失败`): unchanged — Task 1F's new test only exercises `default:http:` form, not bool. Existing `TestEvaluate_FilterBool_*` tests still cover bool comma-split.
- **`.app` build verification (Subtask 3D manual)** is intentionally read-only — actually running `/tmp/pager-test` without env var would create `~/Library/Application Support/Pager/` with empty data on your machine, which is harmless but non-deterministic. The unit tests + grep-the-code-path is sufficient evidence.
