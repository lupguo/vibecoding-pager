# Pager UI & State Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor Pager v1.x to a single 4-value SessionStatus enum, add project collapse + per-project Clear, restore expanded card metadata, migrate all icons to lucide-react, fix multi-Space binding via Wails native API, harden Alt+E hotkey, and route notifications through Wails-native NotificationService with click-to-highlight deep-linking.

**Architecture:** Backend cleanup of two parallel state systems (`Status` × `AttentionLevel`) into one `SessionStatus` derived by a single pure function `session.DeriveStatus`. UI rebuild of `SessionCard` and `SessionList` using lucide icons + dual-coded (color + shape) state indicators. Window/notification behavior delegated to Wails v3 alpha.96 native APIs (`MacWindowCollectionBehavior`, `services/notifications`).

**Tech Stack:**
- Backend: Go 1.25, Wails v3 alpha.96, `golang.design/x/hotkey`, modernc.org/sqlite
- Frontend: React 18 + TypeScript, Tailwind CSS, zustand, `lucide-react`
- macOS-only (osascript replaced; `iTerm2` / `Apple_Terminal` jump preserved)

**Spec:** `docs/superpowers/specs/2026-06-01-pager-ui-and-state-redesign-design.md`

---

## File Structure

### Files to CREATE

| Path | Responsibility |
|------|----------------|
| `internal/domain/session/status.go` | `DeriveStatus` pure function + helper constants (single source of truth for event→status mapping) |
| `internal/domain/session/status_test.go` | Table-driven tests for `DeriveStatus` |
| `frontend/src/store/uistate.ts` | zustand store: `collapsedProjects: Set<string>`, `highlightedKey: string \| null`, plus mutators |

### Files to MODIFY

| Path | Change scope |
|------|--------------|
| `internal/domain/entity/event.go` | Remove `AttentionLevel` field & constants; add `SessionStatus` type + 4 new constants |
| `internal/domain/session/tracker.go` | Replace event-type switch with `DeriveStatus` call; remove `AttentionLevel` field on Session |
| `internal/domain/session/tracker_test.go` | Adapt assertions to new `SessionStatus` |
| `cmd/bridge/main.go` | Stop calling `bridge.DetermineAttentionLevel`; stop setting `e.AttentionLevel` |
| `internal/adapter/notify/notify.go` | Replace osascript with Wails `NotificationService`; rewrite predicates against new `SessionStatus` |
| `internal/adapter/notify/notify_test.go` | Adapt to mock `NotificationService` |
| `internal/wails/app.go` | Notification service registration; `MoveToActiveSpace`; hotkey app-active hook; lastHotkey diff; `updateTrayIcon` switch |
| `internal/wails/hotkey.go` | Add `IsHotkeyHealthy()` + structured slog logging |
| `internal/wails/session_svc.go` | New method `DismissSessionsByProject` |
| `internal/wails/window_svc.go` | New method `SetCollapsedProjects` |
| `internal/infra/config/config.go` | New field `CollapsedProjects []string` |
| `internal/infra/store/schema.sql` | Drop `attention_level` columns; default `status='working'`; CamelCase comment |
| `internal/infra/store/sqlite.go` | Update `migrate`; remove `attention_level` from queries; `statusFromEvent` delegates to `DeriveStatus` |
| `internal/infra/store/sqlite_test.go` | Add `TestMigrationV1ToV2` |
| `frontend/src/index.css` | Add `--c-{working,waiting,done,error}` + bg/border/pill tokens + 3 keyframes |
| `frontend/src/store/sessions.ts` | `Session.Status` union type; remove `AttentionLevel`; subscribe to `highlight-session` event |
| `frontend/src/components/SessionList.tsx` | Project header: chevron + Folder + count + Trash2; conditional render based on collapsed state; scrollIntoView on highlight |
| `frontend/src/components/SessionCard.tsx` | 4-state lucide indicators; expanded metadata rows (default + folded "更多"); Copy buttons; pulse-highlight class |
| `frontend/src/pages/SettingsPanel.tsx` | Replace 4 nav emojis with lucide |
| `frontend/src/pages/settings/AboutSettings.tsx` | Replace 📟 + › + ↗ with lucide |

### Files to DELETE

| Path | Reason |
|------|--------|
| `internal/adapter/bridge/attention.go` | Sole consumer was the now-removed `AttentionLevel` field |
| `internal/adapter/bridge/attention_test.go` | Tests for the deleted file |

---

## Constants (no magic numbers — per CLAUDE.md)

| Constant | Value | Location |
|----------|-------|----------|
| `toolAskUserQuestion` | `"AskUserQuestion"` | `internal/domain/session/status.go` |
| `permModeBypassPermissions` | `"bypassPermissions"` | `internal/domain/session/status.go` |
| `HIGHLIGHT_DURATION_MS` | `1500` | `frontend/src/store/uistate.ts` |
| `COLLAPSED_DEBOUNCE_MS` | `250` | `frontend/src/store/uistate.ts` |
| `COPY_FEEDBACK_MS` | `800` | `frontend/src/components/SessionCard.tsx` (top-of-file const) |
| `NOTIFICATION_ID_PREFIX` | `"evt-"` | `internal/adapter/notify/notify.go` |

---

# Phase 1 — Status Model Foundation (Spec §C)

### Task 1: Add `SessionStatus` type and constants to entity

**Files:**
- Modify: `internal/domain/entity/event.go`

- [ ] **Step 1: Edit `internal/domain/entity/event.go`** — replace the SessionStatus & AttentionLevel constant blocks (lines 27–40) with the new enum, and remove the old `AttentionLevel` field from `AgentEvent`.

Replace lines 27–40:

```go
// SessionStatus is the single source of truth for session UX state.
// Values are mutually exclusive; UI renders one tag per card.
type SessionStatus string

const (
	StatusWorking SessionStatus = "working" // active, no user action needed
	StatusWaiting SessionStatus = "waiting" // user action required (any reason)
	StatusDone    SessionStatus = "done"    // ended cleanly
	StatusError   SessionStatus = "error"   // ended with failure
)
```

Remove the `AttentionLevel` line in `AgentEvent` struct (line 57). Keep all other fields.

- [ ] **Step 2: Verify the package compiles standalone**

Run: `go build ./internal/domain/entity/...`

Expected: PASS (no other files in the package reference the deleted constants).

- [ ] **Step 3: Run a quick `grep` to scope downstream breakage**

Run: `grep -rn "AttentionLevel\|StatusFinished\|StatusActive\|AttentionAttention\|AttentionRunning\|AttentionDone" --include='*.go' .`

Expected output: a list of remaining call sites. They will be fixed in subsequent tasks. Save this list mentally — every site must be touched.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/entity/event.go
git commit -m "refactor(entity): introduce SessionStatus enum, remove AttentionLevel"
```

---

### Task 2: Implement `DeriveStatus` pure function (TDD)

**Files:**
- Create: `internal/domain/session/status.go`
- Create: `internal/domain/session/status_test.go`

- [ ] **Step 1: Write the failing test** in `internal/domain/session/status_test.go`

```go
package session

import (
	"testing"

	"pager/internal/domain/entity"
)

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
		toolName  string
		permMode  string
		hasAskUser bool
		want      entity.SessionStatus
	}{
		// Terminal failures
		{"StopFailure → error", "StopFailure", "", "", false, entity.StatusError},
		{"Error → error", "Error", "", "", false, entity.StatusError},

		// User-attention events
		{"PermissionRequest → waiting", "PermissionRequest", "", "", false, entity.StatusWaiting},
		{"PermissionDenied → waiting", "PermissionDenied", "", "", false, entity.StatusWaiting},
		{"Notification → waiting", "Notification", "", "", false, entity.StatusWaiting},
		{"Elicitation → waiting", "Elicitation", "", "", false, entity.StatusWaiting},
		{"PostToolUseFailure → waiting", "PostToolUseFailure", "", "", false, entity.StatusWaiting},

		// Clean termination
		{"Stop (no pending) → done", "Stop", "", "", false, entity.StatusDone},
		{"Stop (askUser pending) → waiting", "Stop", "", "", true, entity.StatusWaiting},
		{"SessionEnd → done", "SessionEnd", "", "", false, entity.StatusDone},
		{"SubagentStop → done", "SubagentStop", "", "", false, entity.StatusDone},

		// Tool-use
		{"PreToolUse AskUserQuestion → waiting", "PreToolUse", "AskUserQuestion", "default", false, entity.StatusWaiting},
		{"PreToolUse bypass → working", "PreToolUse", "Edit", "bypassPermissions", false, entity.StatusWorking},
		{"PreToolUse default → waiting", "PreToolUse", "Edit", "default", false, entity.StatusWaiting},
		{"PreToolUse acceptEdits → waiting (non-bypass)", "PreToolUse", "Edit", "acceptEdits", false, entity.StatusWaiting},

		// Default running
		{"PostToolUse → working", "PostToolUse", "Edit", "default", false, entity.StatusWorking},
		{"PostToolBatch → working", "PostToolBatch", "", "", false, entity.StatusWorking},
		{"SessionStart → working", "SessionStart", "", "", false, entity.StatusWorking},
		{"unknown → working", "TaskCreated", "", "", false, entity.StatusWorking},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveStatus(tc.eventType, tc.toolName, tc.permMode, tc.hasAskUser)
			if got != tc.want {
				t.Errorf("DeriveStatus(%q,%q,%q,%v) = %q; want %q",
					tc.eventType, tc.toolName, tc.permMode, tc.hasAskUser, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test — expect compile failure**

Run: `go test ./internal/domain/session/ -run TestDeriveStatus -v`

Expected: build fails with `undefined: DeriveStatus`.

- [ ] **Step 3: Implement `DeriveStatus`** in `internal/domain/session/status.go`

```go
package session

import "pager/internal/domain/entity"

// Constants kept here to avoid magic strings. Mirrored against CC-native event types.
const (
	toolAskUserQuestion       = "AskUserQuestion"
	permModeBypassPermissions = "bypassPermissions"
)

// DeriveStatus folds (event_type, tool_name, permission_mode, hasPendingAskUser) into a
// single SessionStatus. The function is the only place where event_type → status mapping
// lives.
func DeriveStatus(eventType, toolName, permissionMode string, hasPendingAskUser bool) entity.SessionStatus {
	switch eventType {
	// terminal failures
	case "StopFailure", "Error":
		return entity.StatusError

	// user-attention events
	case "PermissionRequest", "PermissionDenied", "Notification",
		"Elicitation", "PostToolUseFailure":
		return entity.StatusWaiting

	// clean termination — but if AskUserQuestion is still pending, the agent is
	// waiting on the user, not finished.
	case "Stop", "SessionEnd", "SubagentStop":
		if hasPendingAskUser {
			return entity.StatusWaiting
		}
		return entity.StatusDone

	// tool-use
	case "PreToolUse":
		if toolName == toolAskUserQuestion || permissionMode != permModeBypassPermissions {
			return entity.StatusWaiting
		}
		return entity.StatusWorking

	// running default — PostToolUse, PostToolBatch, SessionStart, TaskCreated, etc.
	default:
		return entity.StatusWorking
	}
}
```

- [ ] **Step 4: Run the test — expect pass**

Run: `go test ./internal/domain/session/ -run TestDeriveStatus -v`

Expected: PASS for all 19 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/session/status.go internal/domain/session/status_test.go
git commit -m "feat(session): add DeriveStatus pure function with full table-driven tests"
```

---

### Task 3: Refactor `tracker.go` to use `DeriveStatus`

**Files:**
- Modify: `internal/domain/session/tracker.go`
- Modify: `internal/domain/session/tracker_test.go`

- [ ] **Step 1: Update the `Session` struct** in `tracker.go` (lines 12–27): change `Status string` to `Status entity.SessionStatus` and remove the `AttentionLevel string` line.

```go
type Session struct {
	Key            string                       `json:"Key"`
	Agent          string                       `json:"Agent"`
	Host           string                       `json:"Host"`
	CWD            string                       `json:"CWD"`
	TTY            string                       `json:"TTY"`
	TermProgram    string                       `json:"TermProgram"`
	ITermSessionID string                       `json:"ITermSessionID"`
	Status         entity.SessionStatus         `json:"Status"`
	AgentLabel     string                       `json:"AgentLabel"`
	SessionID      string                       `json:"SessionID"`
	LastEvent      *entity.AgentEvent            `json:"LastEvent"`
	PendingTools   map[string]*entity.AgentEvent `json:"PendingTools"`
	UpdatedAt      time.Time                    `json:"UpdatedAt"`
}
```

- [ ] **Step 2: Replace the event-type switch** (lines 78–111) with a single `DeriveStatus` call

Replace the entire `switch e.EventType { ... }` block (and the lines 78–84 above it that read `e.AttentionLevel`) with:

```go
	// Maintain pending-tool bookkeeping (must run before DeriveStatus so the
	// hasPendingAskUser flag is accurate).
	switch e.EventType {
	case "PreToolUse":
		if e.ToolUseID != "" {
			s.PendingTools[e.ToolUseID] = e
		}
	case "PostToolUse":
		if e.ToolUseID != "" {
			delete(s.PendingTools, e.ToolUseID)
		}
	case "Stop", "SessionEnd":
		// Clean termination clears pending bookkeeping unless AskUserQuestion is in flight.
		if !hasAskUserPending(s.PendingTools) {
			s.PendingTools = make(map[string]*entity.AgentEvent)
		}
	}

	s.Status = DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, hasAskUserPending(s.PendingTools))
```

The existing `hasAskUserPending` helper (lines 174–182) stays.

- [ ] **Step 3: Update `tracker_test.go`** to assert on the new `SessionStatus`

Open `internal/domain/session/tracker_test.go` and search for any reference to `entity.StatusWaiting`, `entity.StatusActive`, `entity.StatusFinished`, `entity.StatusError`, or `entity.AttentionAttention`. Each one becomes:

| Old | New |
|-----|-----|
| `entity.StatusWaiting` | `entity.StatusWaiting` (same name, new type) |
| `entity.StatusActive` | `entity.StatusWorking` |
| `entity.StatusFinished` | `entity.StatusDone` |
| `entity.StatusError` | `entity.StatusError` (same name) |
| `entity.AttentionAttention` etc. | DELETE the assertions referring to AttentionLevel — that field no longer exists |

The existing tests likely use snake_case event types like `entity.EventPreToolUse`. Replace those literal usages with the CamelCase strings the bridge actually emits: `"PreToolUse"`, `"PostToolUse"`, `"Stop"`, `"StopFailure"`, `"PreToolUse"` (with `ToolName: "AskUserQuestion"`).

- [ ] **Step 4: Run the test**

Run: `go test ./internal/domain/session/ -v`

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/session/tracker.go internal/domain/session/tracker_test.go
git commit -m "refactor(tracker): use DeriveStatus, drop AttentionLevel field"
```

---

### Task 4: Delete `bridge/attention.go` and stop bridge from setting AttentionLevel

**Files:**
- Delete: `internal/adapter/bridge/attention.go`
- Delete: `internal/adapter/bridge/attention_test.go`
- Modify: `cmd/bridge/main.go`

- [ ] **Step 1: Delete the attention files**

```bash
git rm internal/adapter/bridge/attention.go internal/adapter/bridge/attention_test.go
```

- [ ] **Step 2: Edit `cmd/bridge/main.go`** to drop the AttentionLevel computation

Remove line 41 (`attentionLevel := bridge.DetermineAttentionLevel(...)`) and remove the `AttentionLevel: attentionLevel,` line from the `entity.AgentEvent{...}` literal (line 56).

- [ ] **Step 3: Verify the bridge binary still builds**

Run: `go build ./cmd/bridge/`

Expected: PASS.

- [ ] **Step 4: Run bridge tests**

Run: `go test ./cmd/bridge/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(bridge): delete attention.go, stop emitting AttentionLevel"
```

---

### Task 5: Update SQLite schema, migrate, and queries

**Files:**
- Modify: `internal/infra/store/schema.sql`
- Modify: `internal/infra/store/sqlite.go`
- Modify: `internal/infra/store/sqlite_test.go`

- [ ] **Step 1: Replace `internal/infra/store/schema.sql`** with the v2 schema (full file content)

```sql
-- Pager SQLite Schema v2
-- All tables use t_ prefix and id auto-increment primary key.

CREATE TABLE IF NOT EXISTS t_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key      TEXT    NOT NULL UNIQUE,            -- 会话唯一标识 (session_id 或 host:cwd:tty)
    session_id       TEXT    NOT NULL DEFAULT '',        -- CC 原生 session_id
    agent            TEXT    NOT NULL DEFAULT 'claude-code', -- 代理类型: claude-code / codex / codebuddy
    host             TEXT    NOT NULL DEFAULT '',
    cwd              TEXT    NOT NULL DEFAULT '',
    project_name     TEXT    NOT NULL DEFAULT '',        -- CWD 末段，分组排序键
    tty              TEXT    NOT NULL DEFAULT '',
    term_program     TEXT    NOT NULL DEFAULT '',        -- iTerm2 / Apple_Terminal / WezTerm
    iterm_session_id TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'working', -- 当前状态: working / waiting / done / error
    agent_label      TEXT    NOT NULL DEFAULT 'CC',
    created_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),
    deleted_at       TEXT             DEFAULT NULL       -- 软删除标记 (NULL=未删除)
);

CREATE INDEX IF NOT EXISTS idx_sessions_project ON t_sessions(project_name, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON t_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted ON t_sessions(deleted_at);

CREATE TABLE IF NOT EXISTS t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key     TEXT    NOT NULL,                   -- FK → t_sessions.session_key
    agent_label     TEXT    NOT NULL DEFAULT '',
    event_type      TEXT    NOT NULL,                   -- CC-native CamelCase: PreToolUse/PostToolUse/Stop/StopFailure/Notification/PermissionRequest/Elicitation/SessionStart/...
    tool_name       TEXT    NOT NULL DEFAULT '',
    tool_use_id     TEXT    NOT NULL DEFAULT '',
    content         TEXT    NOT NULL DEFAULT '',        -- 截断显示内容 (≤60 rune)
    content_raw     TEXT    NOT NULL DEFAULT '',        -- 完整内容摘要
    permission_mode TEXT    NOT NULL DEFAULT '',        -- bypassPermissions / default / etc.
    raw_payload     BLOB             DEFAULT NULL,      -- 完整 stdin JSON
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);

CREATE INDEX IF NOT EXISTS idx_events_session_ts ON t_events(session_key, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_type       ON t_events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp  ON t_events(timestamp DESC);
```

- [ ] **Step 2: Update `migrate(db)` in `sqlite.go`** (line 425)

Replace the function body with:

```go
func migrate(db *sqlx.DB) {
	// v1.1: agent_label on t_events (existing)
	_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN agent_label TEXT NOT NULL DEFAULT ''`)

	// v2.0: rewrite legacy status values to new enum
	_, _ = db.Exec(`UPDATE t_sessions SET status = 'working' WHERE status = 'active'`)
	_, _ = db.Exec(`UPDATE t_sessions SET status = 'done'    WHERE status = 'finished'`)

	// v2.0: drop obsolete attention_level columns (SQLite >= 3.35; modernc.org/sqlite v1.51 supports it)
	_, _ = db.Exec(`ALTER TABLE t_sessions DROP COLUMN attention_level`)
	_, _ = db.Exec(`ALTER TABLE t_events DROP COLUMN attention_level`)
}
```

- [ ] **Step 3: Strip `attention_level` from queries**

Open `sqlite.go` and apply the spec §5.4.4 cleanup:

| Line (current) | Change |
|---|---|
| 100 | Remove `e.attention_level,` from SELECT column list |
| 168 | Remove `attention_level,` from `SessionEvents` SELECT |
| 356 | Remove `attention_level,` from `t_sessions` INSERT/UPSERT column list |
| 359–360 | Remove the `attention_level = excluded.attention_level,` UPSERT clause |
| 368 | Remove `e.AttentionLevel,` from the `t_sessions` INSERT bind values list |
| 377 | Remove `attention_level,` from `t_events` INSERT column list and its bind value |

After edits, the `loadRecentEventsSQL` (line ~99) should read its columns without `attention_level`. Likewise the `INSERT INTO t_sessions ...` UPSERT and `INSERT INTO t_events ...`.

Also: in `LoadRecentSessions` (around line 99) and `SessionEvents` (around line 167), the row scan struct (or `db.Select`) must drop the corresponding scan target. If the package uses `sqlx.SelectContext` against the entity `AgentEvent`, and `AgentEvent.AttentionLevel` was deleted in Task 1, scans should already be aligned — the SELECT column count must match.

- [ ] **Step 4: Replace `statusFromEvent` (lines 394–410)** with a delegate to `session.DeriveStatus`

```go
import (
	// ... existing imports ...
	"pager/internal/domain/session"
)

// statusFromEvent derives the session status for storage. Replay only sees one event
// at a time, so hasPendingAskUser is conservatively false; the live tracker
// re-derives correctly after Replay.
func statusFromEvent(e *entity.AgentEvent) entity.SessionStatus {
	return session.DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, false)
}
```

The call site (line 368) was `statusFromEvent(e), e.AttentionLevel, ...`; now becomes `string(statusFromEvent(e)), ...` (cast to string for SQL bind, since the column type is TEXT) and the `e.AttentionLevel` argument is removed entirely.

- [ ] **Step 5: Add migration test** to `internal/infra/store/sqlite_test.go`

Append:

```go
func TestMigrationV1ToV2(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v1.db")

	// Manually create a v1-shaped DB
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	v1Schema := `
CREATE TABLE t_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key TEXT NOT NULL UNIQUE,
    session_id TEXT NOT NULL DEFAULT '',
    agent TEXT NOT NULL DEFAULT 'claude-code',
    host TEXT NOT NULL DEFAULT '',
    cwd TEXT NOT NULL DEFAULT '',
    project_name TEXT NOT NULL DEFAULT '',
    tty TEXT NOT NULL DEFAULT '',
    term_program TEXT NOT NULL DEFAULT '',
    iterm_session_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    attention_level TEXT NOT NULL DEFAULT 'running',
    agent_label TEXT NOT NULL DEFAULT 'CC',
    created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    deleted_at TEXT DEFAULT NULL
);
CREATE TABLE t_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    tool_name TEXT NOT NULL DEFAULT '',
    tool_use_id TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    content_raw TEXT NOT NULL DEFAULT '',
    attention_level TEXT NOT NULL DEFAULT '',
    permission_mode TEXT NOT NULL DEFAULT '',
    raw_payload BLOB DEFAULT NULL,
    timestamp TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);
INSERT INTO t_sessions (session_key, status) VALUES ('s-active', 'active');
INSERT INTO t_sessions (session_key, status) VALUES ('s-finished', 'finished');
INSERT INTO t_sessions (session_key, status) VALUES ('s-error', 'error');
`
	if _, err := raw.Exec(v1Schema); err != nil {
		t.Fatalf("seed v1 schema: %v", err)
	}
	_ = raw.Close()

	// Open via NewSQLiteStore which runs schemaDDL + migrate
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	rows, err := store.db.Queryx(`SELECT session_key, status FROM t_sessions ORDER BY session_key`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for rows.Next() {
		var k, s string
		if err := rows.Scan(&k, &s); err != nil {
			t.Fatal(err)
		}
		got[k] = s
	}
	rows.Close()

	want := map[string]string{
		"s-active":   "working",
		"s-finished": "done",
		"s-error":    "error",
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("status[%s] = %q; want %q", k, got[k], w)
		}
	}

	// attention_level column should be gone
	cols, _ := store.db.Queryx(`PRAGMA table_info(t_sessions)`)
	for cols.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		_ = cols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == "attention_level" {
			t.Errorf("t_sessions.attention_level still present after migration")
		}
	}
	cols.Close()
}
```

Add `"database/sql"` and `"path/filepath"` to imports if missing.

- [ ] **Step 6: Run the test**

Run: `go test ./internal/infra/store/ -run TestMigrationV1ToV2 -v`

Expected: PASS.

- [ ] **Step 7: Run all store tests**

Run: `go test ./internal/infra/store/ -v`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/infra/store/schema.sql internal/infra/store/sqlite.go internal/infra/store/sqlite_test.go
git commit -m "refactor(store): drop attention_level columns, migrate status enum, add v2 migration test"
```

---

### Task 6: Update `notify.go` ShouldNotify against new `SessionStatus`

**Files:**
- Modify: `internal/adapter/notify/notify.go`
- Modify: `internal/adapter/notify/notify_test.go`

This task only fixes the `ShouldNotify` predicate logic. Wails-native rewrite of the
`Show*` functions happens in Task 22.

- [ ] **Step 1: Open `internal/adapter/notify/notify.go`** and replace `ShouldNotify` (lines 32–51) with:

```go
// ShouldNotify determines whether an event should trigger a system notification
// based on the configured notification level.
func ShouldNotify(e *entity.AgentEvent, level string) bool {
	status := session.DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, false)
	switch level {
	case "all":
		// Notify on any state transition the user cares about.
		return status == entity.StatusWaiting || status == entity.StatusDone || status == entity.StatusError
	case "attention_only":
		return status == entity.StatusWaiting || status == entity.StatusError
	default:
		return false
	}
}
```

Add `"pager/internal/domain/session"` to the imports.

- [ ] **Step 2: Update `notify_test.go`** — every test case currently set up with `EventType: entity.EventPreToolUse` etc. should be reframed as `EventType: "PreToolUse"` and the assertions adjusted to match the new logic above. If existing tests assert on AttentionLevel field, delete those cases.

A minimum smoke test:

```go
func TestShouldNotify_AttentionOnly(t *testing.T) {
	e := &entity.AgentEvent{EventType: "StopFailure"}
	if !ShouldNotify(e, "attention_only") {
		t.Error("StopFailure should notify in attention_only")
	}
	e2 := &entity.AgentEvent{EventType: "Stop"}
	if ShouldNotify(e2, "attention_only") {
		t.Error("Stop (clean) should NOT notify in attention_only")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/adapter/notify/...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/notify/notify.go internal/adapter/notify/notify_test.go
git commit -m "refactor(notify): switch ShouldNotify on new SessionStatus"
```

---

### Task 7: Update `app.go::updateTrayIcon` switch

**Files:**
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Edit `updateTrayIcon`** (lines 231–252) — switch on the new SessionStatus values

```go
func (p *PagerApp) updateTrayIcon(sessions []*session.Session) {
	hasWaiting := false
	hasWorking := false

	for _, s := range sessions {
		switch s.Status {
		case entity.StatusWaiting:
			hasWaiting = true
		case entity.StatusWorking:
			hasWorking = true
		}
	}

	switch {
	case hasWaiting:
		p.tray.SetLabel("●")
	case hasWorking:
		p.tray.SetLabel("")
	default:
		p.tray.SetLabel("")
	}
}
```

- [ ] **Step 2: Verify the package compiles**

Run: `go build ./...`

Expected: PASS. (At this point the entire backend should compile cleanly with the new status model.)

- [ ] **Step 3: Run the full Go test suite**

Run: `make test` (or `go test ./...`)

Expected: PASS for everything that already had tests.

- [ ] **Step 4: Commit**

```bash
git add internal/wails/app.go
git commit -m "refactor(app): updateTrayIcon switches on SessionStatus"
```

---

# Phase 2 — Multi-Space Window (Spec §F)

### Task 8: Add `MoveToActiveSpace` to popup window

**Files:**
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Add `CollectionBehavior`** to the popup window's `MacWindow` options (lines 152–155)

```go
Mac: application.MacWindow{
	Backdrop:           application.MacBackdropTransparent,
	CollectionBehavior: application.MacWindowCollectionBehaviorMoveToActiveSpace,
},
```

- [ ] **Step 2: Build to confirm the constant resolves**

Run: `go build ./internal/wails/...`

Expected: PASS.

- [ ] **Step 3: Manual smoke (after building the .app)**

Run: `make build && open build/bin/Pager.app`

Then: switch to a different macOS Space, press Alt+E. The popup should appear on the active Space (not snap back to the original Space).

If satisfied, kill the running Pager via the tray menu before continuing.

- [ ] **Step 4: Commit**

```bash
git add internal/wails/app.go
git commit -m "fix(window): bind popup with MoveToActiveSpace for multi-Space behavior"
```

---

# Phase 3 — Hotkey Resilience (Spec §E)

### Task 9: Add `IsHotkeyHealthy` and structured logging to hotkey.go

**Files:**
- Modify: `internal/wails/hotkey.go`

- [ ] **Step 1: Replace the file contents** of `internal/wails/hotkey.go` with the resilient version:

```go
package wails

import (
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/hotkey"

	"pager/internal/infra/log"
)

var (
	currentHotkey   *hotkey.Hotkey
	goroutineAlive  atomic.Bool
	hotkeyLogger    = log.Module("hotkey")
)

var keyMap = map[string]hotkey.Key{
	"A": hotkey.KeyA, "B": hotkey.KeyB, "C": hotkey.KeyC, "D": hotkey.KeyD,
	"E": hotkey.KeyE, "F": hotkey.KeyF, "G": hotkey.KeyG, "H": hotkey.KeyH,
	"I": hotkey.KeyI, "J": hotkey.KeyJ, "K": hotkey.KeyK, "L": hotkey.KeyL,
	"M": hotkey.KeyM, "N": hotkey.KeyN, "O": hotkey.KeyO, "P": hotkey.KeyP,
	"Q": hotkey.KeyQ, "R": hotkey.KeyR, "S": hotkey.KeyS, "T": hotkey.KeyT,
	"U": hotkey.KeyU, "V": hotkey.KeyV, "W": hotkey.KeyW, "X": hotkey.KeyX,
	"Y": hotkey.KeyY, "Z": hotkey.KeyZ,
	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2, "3": hotkey.Key3,
	"4": hotkey.Key4, "5": hotkey.Key5, "6": hotkey.Key6, "7": hotkey.Key7,
	"8": hotkey.Key8, "9": hotkey.Key9,
	"SPACE": hotkey.KeySpace, "RETURN": hotkey.KeyReturn,
	"ESCAPE": hotkey.KeyEscape, "DELETE": hotkey.KeyDelete, "TAB": hotkey.KeyTab,
	"LEFT": hotkey.KeyLeft, "RIGHT": hotkey.KeyRight,
	"UP": hotkey.KeyUp, "DOWN": hotkey.KeyDown,
	"F1": hotkey.KeyF1, "F2": hotkey.KeyF2, "F3": hotkey.KeyF3,
	"F4": hotkey.KeyF4, "F5": hotkey.KeyF5, "F6": hotkey.KeyF6,
	"F7": hotkey.KeyF7, "F8": hotkey.KeyF8, "F9": hotkey.KeyF9,
	"F10": hotkey.KeyF10, "F11": hotkey.KeyF11, "F12": hotkey.KeyF12,
}

// IsHotkeyHealthy reports whether the global hotkey listener goroutine is alive.
// Used by the app-active hook to detect silent failure modes.
func IsHotkeyHealthy() bool {
	return currentHotkey != nil && goroutineAlive.Load()
}

// RegisterHotkey registers a global hotkey that toggles the popup window.
// Safe to call repeatedly; the previous registration is unwound first.
func RegisterHotkey(window *application.WebviewWindow, hotkeyStr string) {
	if currentHotkey != nil {
		currentHotkey.Unregister()
		hotkeyLogger.Info("hotkey unregistered", "key", hotkeyStr)
		currentHotkey = nil
		goroutineAlive.Store(false)
	}

	mods, key, ok := ParseHotkey(hotkeyStr)
	if !ok {
		hotkeyLogger.Warn("invalid hotkey string", "key", hotkeyStr)
		return
	}

	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		hotkeyLogger.Error("hotkey register failed", "err", err, "key", hotkeyStr)
		return
	}
	currentHotkey = hk
	hotkeyLogger.Info("hotkey registered", "key", hotkeyStr)

	go func() {
		goroutineAlive.Store(true)
		defer goroutineAlive.Store(false)
		for range hk.Keydown() {
			visible := window.IsVisible()
			hotkeyLogger.Debug("hotkey fired", "visible", visible)
			if visible {
				window.Hide()
			} else {
				window.Show()
				window.Focus()
			}
		}
		hotkeyLogger.Warn("hotkey goroutine exited; channel closed", "key", hotkeyStr)
	}()
}

// ParseHotkey converts "Alt+E" format to hotkey modifiers and key.
func ParseHotkey(s string) ([]hotkey.Modifier, hotkey.Key, bool) {
	parts := strings.Split(s, "+")
	if len(parts) < 2 {
		return nil, 0, false
	}

	var mods []hotkey.Modifier
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "alt", "option":
			mods = append(mods, hotkey.ModOption)
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "cmd", "command", "meta":
			mods = append(mods, hotkey.ModCmd)
		}
	}

	keyStr := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	key, ok := keyMap[keyStr]
	if !ok {
		return nil, 0, false
	}

	return mods, key, true
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/wails/...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/wails/hotkey.go
git commit -m "feat(hotkey): add IsHotkeyHealthy + structured slog logging"
```

---

### Task 10: Diff-based re-register at app.go call site

**Files:**
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Edit `app.go`** — replace the `settingsBinding := NewSettingsBinding(...)` block (lines 105–107) with the lastHotkey closure pattern

```go
var lastHotkey = initialCfg.HotkeyToggle
settingsBinding := NewSettingsBinding(func(cfg config.Settings) {
	if cfg.HotkeyToggle == lastHotkey {
		return
	}
	lastHotkey = cfg.HotkeyToggle
	RegisterHotkey(popupWindow, cfg.HotkeyToggle)
}, eventStore)
```

- [ ] **Step 2: Build**

Run: `go build ./internal/wails/...`

Expected: PASS.

- [ ] **Step 3: Manual smoke**

`make dev` → toggle theme / opacity in Settings → confirm via console logs that `RegisterHotkey` is NOT re-invoked. Change the hotkey itself → confirm exactly one `hotkey unregistered` + `hotkey registered` pair logs.

- [ ] **Step 4: Commit**

```bash
git add internal/wails/app.go
git commit -m "fix(hotkey): only re-register when HotkeyToggle actually changes"
```

---

### Task 11: App-active health check

**Files:**
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Add the hook** in `NewPagerApp` after the goroutine that calls `RegisterHotkey` (line ~209). Insert before `logger.Info("app assembled")`:

```go
wailsApp.Event.OnApplicationEvent(events.Mac.ApplicationDidBecomeActive, func(_ *application.ApplicationEvent) {
	if IsHotkeyHealthy() {
		return
	}
	cfg, _ := config.LoadFrom(config.DefaultPath())
	logger.Warn("hotkey unhealthy on app activate, re-registering", "key", cfg.HotkeyToggle)
	RegisterHotkey(popupWindow, cfg.HotkeyToggle)
})
```

Add `"github.com/wailsapp/wails/v3/pkg/events"` to imports if not already present (it should be — line 12 imports `wails/v3/pkg/events` for `events.Common.WindowLostFocus`).

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/wails/app.go
git commit -m "feat(hotkey): re-register on macOS ApplicationDidBecomeActive when unhealthy"
```

---

# Phase 4 — Project Grouping & Clear (Spec §A)

### Task 12: Add `CollapsedProjects` to settings + `WindowBinding.SetCollapsedProjects`

**Files:**
- Modify: `internal/infra/config/config.go`
- Modify: `internal/wails/window_svc.go`

- [ ] **Step 1: Edit `config.go`** — add the new field

In the `Settings` struct (after `NotificationEvents`):

```go
type Settings struct {
	Language           string              `json:"language"`
	Theme              string              `json:"theme"`
	Opacity            int                 `json:"opacity"`
	HotkeyToggle       string              `json:"hotkey_toggle"`
	NotificationLevel  string              `json:"notification_level"`
	PopupWidth         int                 `json:"popup_width"`
	PopupPinned        bool                `json:"popup_pinned"`
	SessionLoadHours   int                 `json:"session_load_hours"`
	NotificationEvents map[string][]string `json:"notification_events"`
	CollapsedProjects  []string            `json:"collapsed_projects"`
}
```

In `Defaults()` add:

```go
CollapsedProjects: []string{},
```

- [ ] **Step 2: Add a test** in `internal/infra/config/config_test.go` to verify round-trip

```go
func TestCollapsedProjectsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	cfg := Defaults()
	cfg.CollapsedProjects = []string{"foo", "bar"}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.CollapsedProjects) != 2 || loaded.CollapsedProjects[0] != "foo" {
		t.Errorf("CollapsedProjects = %v; want [foo bar]", loaded.CollapsedProjects)
	}
}
```

Run: `go test ./internal/infra/config/...`

Expected: PASS.

- [ ] **Step 3: Edit `internal/wails/window_svc.go`** — add the new binding method

```go
// SetCollapsedProjects persists the list of project names whose group is collapsed.
func (w *WindowBinding) SetCollapsedProjects(list []string) error {
	cfg, _ := config.LoadFrom(w.configPath)
	cfg.CollapsedProjects = list
	if err := config.SaveTo(w.configPath, cfg); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("settings-changed", cfg)
	}
	return nil
}
```

- [ ] **Step 4: Regenerate Wails bindings**

Run: `make bindings`

Expected: `frontend/bindings/pager/internal/wails/windowbinding.js` regenerates with the new `SetCollapsedProjects` export.

- [ ] **Step 5: Commit**

```bash
git add internal/infra/config/ internal/wails/window_svc.go frontend/bindings/
git commit -m "feat(settings): add CollapsedProjects + SetCollapsedProjects binding"
```

---

### Task 13: Add `Tracker.DismissByProject` + `SessionBinding.DismissSessionsByProject`

**Files:**
- Modify: `internal/domain/session/tracker.go`
- Modify: `internal/domain/session/tracker_test.go`
- Modify: `internal/wails/session_svc.go`

- [ ] **Step 1: Write the test** in `tracker_test.go`

```go
func TestDismissByProject(t *testing.T) {
	tr := NewTracker(nil)
	now := time.Now()
	makeEvt := func(key, cwd string) *entity.AgentEvent {
		return &entity.AgentEvent{
			SessionID:  key,
			CWD:        cwd,
			EventType:  "PreToolUse",
			ToolName:   "Edit",
			Timestamp:  now,
		}
	}
	tr.TrackEvent(makeEvt("s1", "/path/to/projA"))
	tr.TrackEvent(makeEvt("s2", "/path/to/projA"))
	tr.TrackEvent(makeEvt("s3", "/path/to/projB"))

	dismissed := tr.DismissByProject("projA")
	if len(dismissed) != 2 {
		t.Errorf("dismissed count = %d; want 2", len(dismissed))
	}

	remaining := tr.ListByRecent()
	if len(remaining) != 1 || remaining[0].SessionID != "s3" {
		t.Errorf("remaining sessions = %+v; want only s3", remaining)
	}
}
```

Run: `go test ./internal/domain/session/ -run TestDismissByProject -v`

Expected: build fails — `DismissByProject` undefined.

- [ ] **Step 2: Implement `DismissByProject`** in `tracker.go` (after `Dismiss`, around line 165)

```go
// DismissByProject removes all sessions whose CWD's last segment equals project.
// Returns the dismissed session keys for caller-side store sync.
func (t *Tracker) DismissByProject(project string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var keys []string
	for k, s := range t.sessions {
		if projectFromCWD(s.CWD) == project {
			keys = append(keys, k)
			delete(t.sessions, k)
		}
	}
	if len(keys) > 0 && t.onUpdate != nil {
		t.onUpdate(t.snapshotLocked())
	}
	return keys
}

// projectFromCWD returns the last non-empty path segment of cwd.
// Mirrors the frontend's projectFromCWD logic to keep grouping consistent.
func projectFromCWD(cwd string) string {
	for i := len(cwd) - 1; i >= 0; i-- {
		if cwd[i] == '/' {
			if i == len(cwd)-1 {
				continue
			}
			return cwd[i+1:]
		}
	}
	return cwd
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/domain/session/ -run TestDismissByProject -v`

Expected: PASS.

- [ ] **Step 4: Add `DismissSessionsByProject`** to `internal/wails/session_svc.go`

```go
// DismissSessionsByProject removes all sessions in the given project from the tracker
// and soft-deletes them in the store.
func (s *SessionBinding) DismissSessionsByProject(project string) error {
	if s.tracker == nil {
		return fmt.Errorf("tracker not initialised")
	}
	keys := s.tracker.DismissByProject(project)
	if s.store != nil {
		for _, k := range keys {
			_ = s.store.DismissSession(k)
		}
	}
	return nil
}
```

- [ ] **Step 5: Regenerate bindings**

Run: `make bindings`

- [ ] **Step 6: Commit**

```bash
git add internal/domain/session/ internal/wails/session_svc.go frontend/bindings/
git commit -m "feat(session): add DismissByProject for bulk per-project clear"
```

---

### Task 14: Create `frontend/src/store/uistate.ts`

**Files:**
- Create: `frontend/src/store/uistate.ts`

- [ ] **Step 1: Create the file** with the full content

```ts
import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { useSessionStore, projectFromCWD } from './sessions'

export const HIGHLIGHT_DURATION_MS = 1500
export const COLLAPSED_DEBOUNCE_MS = 250

interface UIState {
  collapsedProjects: Set<string>
  highlightedKey: string | null

  initCollapsed: (list: string[]) => void
  toggleCollapsed: (project: string) => void
  isCollapsed: (project: string) => boolean

  setHighlighted: (key: string) => void
  clearHighlighted: () => void
}

export const useUIStore = create<UIState>((set, get) => ({
  collapsedProjects: new Set(),
  highlightedKey: null,

  initCollapsed: (list) => set({ collapsedProjects: new Set(list) }),

  toggleCollapsed: (project) => {
    const next = new Set(get().collapsedProjects)
    if (next.has(project)) next.delete(project)
    else next.add(project)
    set({ collapsedProjects: next })
    persistCollapsed(Array.from(next))
  },

  isCollapsed: (project) => get().collapsedProjects.has(project),

  setHighlighted: (key) => {
    set({ highlightedKey: key })
    setTimeout(() => {
      if (get().highlightedKey === key) {
        set({ highlightedKey: null })
      }
    }, HIGHLIGHT_DURATION_MS)
  },

  clearHighlighted: () => set({ highlightedKey: null }),
}))

let persistTimer: ReturnType<typeof setTimeout> | null = null

function persistCollapsed(list: string[]) {
  if (persistTimer) clearTimeout(persistTimer)
  persistTimer = setTimeout(async () => {
    try {
      const { SetCollapsedProjects } = await import(
        '../../bindings/pager/internal/wails/windowbinding.js'
      )
      await SetCollapsedProjects(list)
    } catch (err) {
      console.error('[pager] SetCollapsedProjects failed:', err)
    }
  }, COLLAPSED_DEBOUNCE_MS)
}

/** Call once at app boot — wires the highlight-session event from Go. */
export function initUIState() {
  Events.On('highlight-session', (ev: any) => {
    const sessionID =
      ev?.data?.session_id ?? ev?.data?.[0]?.session_id ?? ev?.session_id
    if (!sessionID) return
    const sessions = useSessionStore.getState().sessions
    const session = sessions.find(
      (s) => s.Key === sessionID || s.SessionID === sessionID
    )
    if (!session) return

    // Auto-expand the project group containing this session
    const project = projectFromCWD(session.CWD)
    const ui = useUIStore.getState()
    if (ui.isCollapsed(project)) {
      ui.toggleCollapsed(project)
    }
    ui.setHighlighted(session.Key)
  })
}
```

- [ ] **Step 2: Export `projectFromCWD` from `sessions.ts`**

Open `frontend/src/store/sessions.ts` and change line 76 from `function projectFromCWD` to `export function projectFromCWD`. (It's currently private to the module.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/store/uistate.ts frontend/src/store/sessions.ts
git commit -m "feat(ui): add uistate store for collapsed projects + highlight"
```

---

### Task 15: Update `SessionList.tsx` with project header + collapse + Clear

**Files:**
- Modify: `frontend/src/components/SessionList.tsx`

- [ ] **Step 1: Replace the file contents**

```tsx
import { useEffect, useRef } from 'react'
import { ChevronDown, ChevronRight, Folder, Trash2 } from 'lucide-react'
import { useProjectGroups } from '../store/sessions'
import { useUIStore } from '../store/uistate'
import { useSettingsStore } from '../store/settings'
import SessionCard from './SessionCard'
import EmptyState from './EmptyState'

export default function SessionList() {
  const groups = useProjectGroups()
  const collapsedProjects = useUIStore((s) => s.collapsedProjects)
  const toggleCollapsed = useUIStore((s) => s.toggleCollapsed)
  const highlightedKey = useUIStore((s) => s.highlightedKey)
  const initCollapsed = useUIStore((s) => s.initCollapsed)
  const settings = useSettingsStore((s) => s.settings)
  const settingsLoaded = useSettingsStore((s) => s.loaded)

  // Hydrate collapsed state from settings once
  const hydrated = useRef(false)
  useEffect(() => {
    if (!hydrated.current && settingsLoaded) {
      initCollapsed(settings.collapsed_projects ?? [])
      hydrated.current = true
    }
  }, [settingsLoaded, settings.collapsed_projects, initCollapsed])

  // Scroll highlighted card into view
  const cardRefs = useRef(new Map<string, HTMLDivElement>())
  useEffect(() => {
    if (!highlightedKey) return
    const el = cardRefs.current.get(highlightedKey)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }, [highlightedKey])

  if (groups.length === 0) {
    return <EmptyState />
  }

  return (
    <div className="p-[10px] space-y-[12px]">
      {groups.map((group) => {
        const collapsed = collapsedProjects.has(group.project)
        return (
          <div key={group.project}>
            <div
              className="group flex items-center gap-[6px] px-[4px] mb-[6px] cursor-pointer select-none"
              onClick={() => toggleCollapsed(group.project)}>
              {collapsed ? (
                <ChevronRight size={10} className="text-[--pager-text-muted]" strokeWidth={2.5} />
              ) : (
                <ChevronDown size={10} className="text-[--pager-text-muted]" strokeWidth={2.5} />
              )}
              <Folder size={11} className="text-[--pager-text-muted] opacity-60" strokeWidth={2} />
              <span className="text-[11px] font-semibold text-[--pager-text-muted] tracking-[0.3px]">
                {group.project}
              </span>
              <span className="text-[9px] bg-[rgba(255,255,255,0.06)] px-[5px] py-[1px] rounded-[3px] text-[--pager-text-faint]">
                {group.sessions.length}
              </span>
              <span className="flex-1 h-px bg-[--pager-border] opacity-50" />
              <button
                className="opacity-0 group-hover:opacity-100 w-[22px] h-[22px] flex items-center justify-center rounded-[4px] text-[--pager-text-faint] hover:bg-[rgba(255,69,58,0.15)] hover:text-[--c-waiting] transition-colors"
                title="清理该项目历史会话"
                onClick={async (e) => {
                  e.stopPropagation()
                  try {
                    const { DismissSessionsByProject } = await import(
                      '../../bindings/pager/internal/wails/sessionbinding.js'
                    )
                    await DismissSessionsByProject(group.project)
                  } catch (err) {
                    console.error('[pager] DismissSessionsByProject failed:', err)
                  }
                }}>
                <Trash2 size={11} strokeWidth={2} />
              </button>
            </div>
            {!collapsed && (
              <div className="space-y-[5px]">
                {group.sessions.map((session) => (
                  <div
                    key={session.Key}
                    ref={(el) => {
                      if (el) cardRefs.current.set(session.Key, el)
                      else cardRefs.current.delete(session.Key)
                    }}>
                    <SessionCard session={session} highlighted={highlightedKey === session.Key} />
                  </div>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
```

- [ ] **Step 2: Add `collapsed_projects` to the `Settings` interface** in `frontend/src/store/settings.ts`

```ts
export interface Settings {
  // ... existing fields ...
  collapsed_projects: string[]
}
```

And update `DEFAULT_SETTINGS` to include `collapsed_projects: []`.

- [ ] **Step 3: Manual verification**

Run: `make bindings && make dev`

Verify:
- Project header shows chevron, Folder, name, count
- Clicking the header toggles collapse (cards hide/show)
- Hovering the header reveals a Trash2 button
- Clicking Trash2 → all that project's session cards vanish
- Restart the app → previously collapsed projects still collapsed

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/SessionList.tsx frontend/src/store/settings.ts
git commit -m "feat(ui): project group collapse/expand + per-project Clear button"
```

---

# Phase 5 — Status Visual + Expanded Metadata + Lucide on Card (Spec §B + §C UI + §D)

### Task 16: Add CSS tokens and animations to `index.css`

**Files:**
- Modify: `frontend/src/index.css`

- [ ] **Step 1: Add the new CSS variables and keyframes**

In the `:root` block (top of file, after `--pager-detail-text`), add:

```css
  /* Status colors — dual encoded with shape (lucide icon) */
  --c-working: #28a745;
  --c-waiting: #ff3b30;
  --c-done:    #5ac8fa;
  --c-error:   #ff9f0a;

  --bg-working: rgba(40, 167, 69, 0.05);
  --bd-working: rgba(40, 167, 69, 0.15);
  --pill-working: rgba(40, 167, 69, 0.12);

  --bg-waiting: rgba(255, 59, 48, 0.06);
  --bd-waiting: rgba(255, 59, 48, 0.20);
  --pill-waiting: rgba(255, 59, 48, 0.15);

  --bg-done: rgba(90, 200, 250, 0.05);
  --bd-done: rgba(90, 200, 250, 0.18);
  --pill-done: rgba(90, 200, 250, 0.14);

  --bg-error: rgba(255, 159, 10, 0.07);
  --bd-error: rgba(255, 159, 10, 0.22);
  --pill-error: rgba(255, 159, 10, 0.16);
```

In the `:root.dark` block and the `@media (prefers-color-scheme: dark)` block, add the
darker variants:

```css
  --c-working: #30d158;
  --c-waiting: #ff453a;
  --c-done:    #5ac8fa;
  --c-error:   #ff9f0a;

  --bg-working: rgba(48, 209, 88, 0.07);
  --bd-working: rgba(48, 209, 88, 0.22);
  --pill-working: rgba(48, 209, 88, 0.15);

  --bg-waiting: rgba(255, 69, 58, 0.10);
  --bd-waiting: rgba(255, 69, 58, 0.28);
  --pill-waiting: rgba(255, 69, 58, 0.18);

  --bg-done: rgba(90, 200, 250, 0.05);
  --bd-done: rgba(90, 200, 250, 0.18);
  --pill-done: rgba(90, 200, 250, 0.14);

  --bg-error: rgba(255, 159, 10, 0.08);
  --bd-error: rgba(255, 159, 10, 0.25);
  --pill-error: rgba(255, 159, 10, 0.18);
```

Below the existing `pulse-status` keyframe, add:

```css
@keyframes pulse-attn {
  0%, 100% { transform: scale(1); opacity: 1; }
  50%      { transform: scale(1.15); opacity: 0.7; }
}
.anim-pulse-attn { animation: pulse-attn 1.6s ease-in-out infinite; transform-origin: 50% 50%; }

@keyframes spin-loader {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
.anim-spin-loader { animation: spin-loader 1.4s linear infinite; transform-origin: 50% 50%; }

@keyframes pulse-highlight {
  0%, 33%, 66%, 100% { background-color: var(--pager-card-attention-bg); }
  16%, 50%, 83%      { background-color: rgba(10, 132, 255, 0.18); }
}
.pulse-highlight {
  animation: pulse-highlight 1.5s ease-in-out;
}
```

- [ ] **Step 2: Manual verification**

Run: `make dev`. Page should still render unchanged (no consumers yet). Open the browser dev tools, switch theme to confirm the new variables are defined under both light and dark roots.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/index.css
git commit -m "feat(ui): add 4-state color tokens, pulse/spin/highlight keyframes"
```

---

### Task 17: Refactor `SessionCard.tsx` — 4-state lucide indicators

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx`
- Modify: `frontend/src/store/sessions.ts`

- [ ] **Step 1: Update `Session` type** in `frontend/src/store/sessions.ts`

Change:
```ts
export type AttentionLevel = 'attention' | 'running' | 'done'
```
to:
```ts
export type SessionStatus = 'working' | 'waiting' | 'done' | 'error'
```

In the `Session` interface, change `Status: string` to `Status: SessionStatus` and **delete** the `AttentionLevel: AttentionLevel` line.

In `AgentEvent`, **delete** the `attention_level: AttentionLevel` field.

In `filterExpiredDone`, change `if (s.AttentionLevel !== 'done')` to `if (s.Status !== 'done')`.

- [ ] **Step 2: Replace `SessionCard.tsx`** with the new 4-state version

```tsx
import { useState } from 'react'
import {
  ArrowRight, AlertTriangle, CheckCircle2, HandHelping, Loader2,
} from 'lucide-react'
import type { Session, SessionStatus } from '../store/sessions'
import { JumpToTerminal } from '../../bindings/pager/internal/wails/sessionbinding.js'

interface Props {
  session: Session
  highlighted?: boolean
}

const STATUS_TAG: Record<SessionStatus, { text: string; bgClass: string; textClass: string }> = {
  working: { text: 'WORKING', bgClass: 'bg-[--pill-working]', textClass: 'text-[--c-working]' },
  waiting: { text: 'WAITING', bgClass: 'bg-[--pill-waiting]', textClass: 'text-[--c-waiting]' },
  done:    { text: 'DONE',    bgClass: 'bg-[--pill-done]',    textClass: 'text-[--c-done]' },
  error:   { text: 'ERROR',   bgClass: 'bg-[--pill-error]',   textClass: 'text-[--c-error]' },
}

const CARD_BG: Record<SessionStatus, string> = {
  working: 'bg-[--bg-working] border-[--bd-working]',
  waiting: 'bg-[--bg-waiting] border-[--bd-waiting] shadow-sm',
  done:    'bg-[--bg-done] border-[--bd-done] opacity-90',
  error:   'bg-[--bg-error] border-[--bd-error]',
}

function StatusIcon({ status }: { status: SessionStatus }) {
  const cls = `text-[--c-${status}]`
  switch (status) {
    case 'waiting': return <HandHelping size={14} strokeWidth={2} className={`${cls} anim-pulse-attn`} />
    case 'working': return <Loader2 size={14} strokeWidth={2.5} className={`${cls} anim-spin-loader`} />
    case 'done':    return <CheckCircle2 size={14} strokeWidth={2.5} className={cls} />
    case 'error':   return <AlertTriangle size={14} strokeWidth={2.5} className={cls} />
  }
}

export default function SessionCard({ session, highlighted }: Props) {
  const [jumping, setJumping] = useState(false)
  const [expanded, setExpanded] = useState(false)

  const status: SessionStatus = (session.Status as SessionStatus) || 'working'
  const tag = STATUS_TAG[status]
  const bg = CARD_BG[status]

  const content = session.LastEvent?.content ?? ''
  const toolName = session.LastEvent?.tool_name ?? ''
  const sessionPrefix = (session.SessionID || session.Key || '').slice(0, 8)
  const agentLabel = session.AgentLabel || 'CC'

  const handleJump = async (e: React.MouseEvent) => {
    e.stopPropagation()
    setJumping(true)
    try {
      await JumpToTerminal(session.Key)
    } catch (err) {
      console.error('Jump failed:', err)
    } finally {
      setJumping(false)
    }
  }

  const relativeTime = formatRelativeTime(session.UpdatedAt)

  return (
    <div
      className={`rounded-lg border cursor-pointer transition-all duration-300 hover:shadow-sm ${bg} ${highlighted ? 'pulse-highlight' : ''}`}
      onClick={() => setExpanded(!expanded)}>
      <div className="px-[10px] py-[8px]">
        <div className="flex items-center gap-[6px] mb-[4px]">
          <StatusIcon status={status} />
          <span className={`text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] tracking-wide ${tag.bgClass} ${tag.textClass}`}>
            {tag.text}
          </span>
          <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide uppercase">
            {agentLabel}
          </span>
          <span className="flex-1" />
          <span className="text-[9px] text-[--pager-text-faint] font-mono">{sessionPrefix}</span>
          <span className="text-[9px] text-[--pager-text-faint]">{relativeTime}</span>
        </div>
        <div className="flex items-center justify-between pl-[22px]">
          <div className="flex items-center gap-[6px] flex-1 min-w-0">
            {toolName && (
              <span className="text-[9px] font-mono font-medium px-[4px] py-[1px] bg-[--pager-tool-bg] rounded-[3px] text-[--pager-tool-text] shrink-0">
                {toolName.length > 12 ? toolName.slice(0, 12) : toolName}
              </span>
            )}
            <span className="text-[11px] text-[--pager-text-primary] whitespace-nowrap overflow-hidden text-ellipsis">
              {content}
            </span>
          </div>
          <button
            onClick={handleJump}
            disabled={jumping}
            title="Jump to terminal"
            className="ml-[6px] p-[3px] rounded text-[--pager-text-faint] hover:text-[--pager-blue] hover:bg-[--pager-filter-bg] disabled:opacity-30 shrink-0 transition-colors">
            {jumping ? (
              <Loader2 size={13} className="animate-spin" />
            ) : (
              <ArrowRight size={13} />
            )}
          </button>
        </div>
        {/* Expanded section is added in Task 18 */}
      </div>
    </div>
  )
}

function formatRelativeTime(iso: string): string {
  if (!iso) return ''
  const now = Date.now()
  const then = new Date(iso).getTime()
  if (isNaN(then)) return ''
  const diffSec = Math.floor((now - then) / 1000)
  if (diffSec < 5) return 'now'
  if (diffSec < 60) return `${diffSec}s`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}
```

- [ ] **Step 3: Manual verification**

Run: `make dev`

Send a `PreToolUse` `AskUserQuestion` event via the bridge: card shows red HandHelping pulsing + WAITING tag.
Send `PostToolUse`: card shows green spinning Loader2 + WORKING tag.
Send `Stop`: card shows light-blue CheckCircle2 + DONE tag.
Send `StopFailure`: card shows orange AlertTriangle + ERROR tag.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/SessionCard.tsx frontend/src/store/sessions.ts
git commit -m "feat(ui): 4-state lucide status indicators with dual-coding"
```

---

### Task 18: Add expanded card default metadata (PATH / SESSION / TIME)

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx`

- [ ] **Step 1: Add a `MetaRow` helper and the default expanded section**

Inside `SessionCard.tsx`, add (above the default export):

```tsx
import {
  ArrowRight, AlertTriangle, CheckCircle2, ChevronDown, ChevronUp,
  Clock, Copy, Check, FolderOpen, HandHelping, Hash, Loader2,
  ShieldCheck, Terminal, Wrench,
} from 'lucide-react'

const COPY_FEEDBACK_MS = 800

function MetaRow({
  Icon, label, value, copyable,
}: {
  Icon: typeof FolderOpen
  label: string
  value: string
  copyable?: boolean
}) {
  const [copied, setCopied] = useState(false)
  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!value) return
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), COPY_FEEDBACK_MS)
    } catch (err) {
      console.error('[pager] copy failed:', err)
    }
  }
  return (
    <div className="flex items-center gap-[6px] px-[8px] py-[3px] text-[10px] leading-[1.6] text-[--pager-text-secondary]">
      <Icon size={11} className="opacity-65 shrink-0 text-[--pager-text-muted]" strokeWidth={2} />
      <span className="text-[9px] uppercase tracking-[0.3px] text-[--pager-text-muted] w-[60px] shrink-0">
        {label}
      </span>
      <span className="font-mono text-[--pager-text-primary] truncate flex-1" title={value}>
        {value || '—'}
      </span>
      <button
        onClick={handleCopy}
        className={`w-[18px] h-[18px] flex items-center justify-center rounded-[3px] text-[--pager-text-faint] hover:bg-[rgba(255,255,255,0.06)] hover:text-[--pager-text-secondary] shrink-0 transition-colors ${
          !copyable || !value ? 'invisible' : ''
        }`}
        title="复制">
        {copied ? <Check size={11} strokeWidth={2.5} /> : <Copy size={11} strokeWidth={2} />}
      </button>
    </div>
  )
}

function formatTimestamp(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
```

- [ ] **Step 2: Replace the empty `{/* Expanded section is added in Task 18 */}` comment** with the actual expanded UI

```tsx
        {expanded && (
          <div className="mt-[6px] pl-[22px]">
            <div className="bg-[rgba(255,255,255,0.025)] border border-[rgba(255,255,255,0.04)] rounded-[6px] py-[4px] mb-[6px]">
              <MetaRow Icon={FolderOpen} label="PATH" value={session.CWD || ''} copyable />
              <MetaRow Icon={Hash} label="SESSION" value={session.SessionID || session.Key || ''} copyable />
              <MetaRow Icon={Clock} label="TIME" value={formatTimestamp(session.LastEvent?.timestamp ?? session.UpdatedAt)} />
            </div>
            {session.LastEvent?.content_raw && (
              <pre className="p-[6px] bg-[--pager-detail-bg] border border-[--pager-detail-border] rounded-[6px] text-[10px] leading-relaxed text-[--pager-detail-text] font-mono whitespace-pre-wrap break-all max-h-32 overflow-y-auto">
                {session.LastEvent.content_raw}
              </pre>
            )}
            {/* "更多" toggle is added in Task 19 */}
          </div>
        )}
```

- [ ] **Step 3: Manual verification**

Run: `make dev`. Click any card → expanded section shows PATH / SESSION / TIME with lucide icons and Copy buttons. Click Copy → see the icon swap to a checkmark for ~800ms then revert.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/SessionCard.tsx
git commit -m "feat(ui): expanded card shows PATH/SESSION/TIME metadata with copy"
```

---

### Task 19: Add "更多" / "收起" toggle and folded section

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx`

- [ ] **Step 1: Add the `showMore` state and the folded block**

In the `SessionCard` body (after `const [expanded, setExpanded] = useState(false)`):

```tsx
const [showMore, setShowMore] = useState(false)
```

Replace the `{/* "更多" toggle is added in Task 19 */}` comment with:

```tsx
            {showMore && (
              <div className="bg-[rgba(255,255,255,0.025)] border border-[rgba(255,255,255,0.04)] rounded-[6px] py-[4px] mb-[6px]">
                <MetaRow
                  Icon={Wrench}
                  label="TOOL ID"
                  value={session.LastEvent?.tool_use_id ?? ''}
                  copyable
                />
                <MetaRow
                  Icon={Terminal}
                  label="TTY"
                  value={
                    session.TTY
                      ? `${session.TTY}${session.TermProgram ? ' · ' + session.TermProgram : ''}`
                      : ''
                  }
                />
                <MetaRow
                  Icon={ShieldCheck}
                  label="PERM"
                  value={session.LastEvent?.permission_mode ?? ''}
                />
              </div>
            )}
            <button
              onClick={(e) => { e.stopPropagation(); setShowMore(!showMore) }}
              className="w-full flex items-center justify-center gap-[4px] py-[4px] px-[8px] mt-[4px] text-[10px] text-[--pager-text-muted] bg-transparent border border-dashed border-[rgba(255,255,255,0.08)] rounded-[5px] hover:text-[--pager-text-secondary] hover:border-[rgba(255,255,255,0.16)]">
              {showMore ? <ChevronUp size={11} /> : <ChevronDown size={11} />}
              {showMore ? '收起' : '更多'}
            </button>
```

- [ ] **Step 2: Manual verification**

`make dev` → expand any card → click "更多" → TOOL ID / TTY / PERM rows render → click again → collapse to "更多".

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/SessionCard.tsx
git commit -m "feat(ui): add 更多/收起 toggle for TOOL ID/TTY/PERM metadata"
```

---

# Phase 6 — Settings Panel Lucide (Spec §D)

### Task 20: Update `SettingsPanel.tsx` nav icons

**Files:**
- Modify: `frontend/src/pages/SettingsPanel.tsx`

- [ ] **Step 1: Update the imports**

```tsx
import { X, Settings as SettingsIcon, Bell, Database, Info } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
```

- [ ] **Step 2: Replace the `navItems` array**

```tsx
const navItems: { id: Page; Icon: LucideIcon; label: string }[] = [
  { id: 'general',       Icon: SettingsIcon, label: t('nav.general') },
  { id: 'notifications', Icon: Bell,         label: t('nav.notifications') },
  { id: 'data',          Icon: Database,     label: isZh ? '数据' : 'Data' },
  { id: 'about',          Icon: Info,         label: t('nav.about') },
]
```

And in the render, replace `<span className="text-[14px]">{item.icon}</span>` with:

```tsx
<item.Icon size={14} strokeWidth={2} className="shrink-0" />
```

- [ ] **Step 3: Manual verification**

`make dev` → open Settings → 4 nav items show lucide icons.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/SettingsPanel.tsx
git commit -m "feat(ui): replace settings nav emojis with lucide icons"
```

---

### Task 21: Update `AboutSettings.tsx` icons

**Files:**
- Modify: `frontend/src/pages/settings/AboutSettings.tsx`

- [ ] **Step 1: Replace the file contents**

```tsx
import { useTranslation } from 'react-i18next'
import { BellRing, ChevronRight, ExternalLink } from 'lucide-react'

export default function AboutSettings() {
  const { t } = useTranslation()

  const openURL = (url: string) => {
    window.open(url, '_blank')
  }

  return (
    <div className="flex flex-col items-center justify-center h-full">
      <div
        className="w-[72px] h-[72px] mb-3 rounded-[16px] flex items-center justify-center shadow-lg"
        style={{ background: 'linear-gradient(135deg, #007aff, #5856d6)' }}>
        <BellRing size={36} strokeWidth={2} className="text-white" />
      </div>
      <div className="text-[18px] font-semibold text-[--pager-text-primary] mb-0.5">Pager</div>
      <div className="text-[12px] text-[--pager-text-muted] mb-6">{t('about.description')}</div>

      <div className="w-full max-w-[320px] bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden mb-4">
        <div className="flex justify-between items-center px-3.5 py-2.5 border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)]">
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.version')}</span>
          <span className="text-[12px] text-[--pager-text-muted]">1.0.0 (build 1)</span>
        </div>
        <div className="flex justify-between items-center px-3.5 py-2.5">
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.author')}</span>
          <a
            className="text-[12px] text-[#007aff] cursor-pointer hover:underline"
            onClick={() => openURL('https://github.com/sapaude')}>
            sapaude
          </a>
        </div>
      </div>

      <div className="w-full max-w-[320px] bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden mb-4">
        <div
          className="flex justify-between items-center px-3.5 py-2.5 border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)] cursor-pointer hover:bg-[rgba(0,0,0,0.02)] dark:hover:bg-[rgba(255,255,255,0.02)]"
          onClick={() => openURL('https://github.com/sapaude/pager/releases')}>
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.checkUpdate')}</span>
          <ChevronRight size={12} strokeWidth={2} className="text-[--pager-text-faint]" />
        </div>
        <div
          className="flex justify-between items-center px-3.5 py-2.5 cursor-pointer hover:bg-[rgba(0,0,0,0.02)] dark:hover:bg-[rgba(255,255,255,0.02)]"
          onClick={() => openURL('https://github.com/sapaude/pager')}>
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.github')}</span>
          <ExternalLink size={12} strokeWidth={2} className="text-[--pager-text-faint]" />
        </div>
      </div>

      <div className="text-[11px] text-[--pager-text-faint]">{t('about.copyright')}</div>
    </div>
  )
}
```

- [ ] **Step 2: Manual verification**

Settings → About → Logo is BellRing, list arrows are lucide ChevronRight / ExternalLink.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/settings/AboutSettings.tsx
git commit -m "feat(ui): replace about-panel emojis with lucide icons"
```

---

# Phase 7 — Wails-Native Notifications + Click-to-Highlight (Spec §G)

### Task 22: Add NotificationService registration + replace notify.go

**Files:**
- Modify: `internal/wails/app.go`
- Modify: `internal/adapter/notify/notify.go`
- Modify: `internal/adapter/notify/notify_test.go`

- [ ] **Step 1: Replace `internal/adapter/notify/notify.go`** entirely

```go
package notify

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

const NotificationIDPrefix = "evt-"

// ShouldNotifyByConfig checks if an event should trigger a system notification
// based on the per-agent notification_events config.
func ShouldNotifyByConfig(e *entity.AgentEvent, notifEvents map[string][]string) bool {
	if notifEvents == nil {
		return false
	}
	events, ok := notifEvents[e.AgentLabel]
	if !ok {
		return false
	}
	for _, ev := range events {
		if ev == e.EventType {
			return true
		}
	}
	return false
}

// ShouldNotify determines whether an event should trigger a notification based on level.
func ShouldNotify(e *entity.AgentEvent, level string) bool {
	status := session.DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, false)
	switch level {
	case "all":
		return status == entity.StatusWaiting || status == entity.StatusDone || status == entity.StatusError
	case "attention_only":
		return status == entity.StatusWaiting || status == entity.StatusError
	default:
		return false
	}
}

// ShowEvent dispatches a Wails-native notification for an event.
// svc is the registered NotificationService; nil-safe (no-op).
func ShowEvent(svc *notifications.NotificationService, e *entity.AgentEvent, lang string) {
	if svc == nil {
		return
	}
	label := e.AgentLabel
	if label == "" {
		label = agentLabel(e.Agent)
	}
	title := fmt.Sprintf("%s · %s", label, lastPath(e.CWD))
	body := e.Content
	if body == "" {
		if e.ToolName != "" {
			body = e.ToolName
		} else {
			body = e.EventType
		}
	}
	body = sanitize(body)
	title = sanitize(title)

	id := fmt.Sprintf("%s%s-%d", NotificationIDPrefix, e.SessionKey(), time.Now().UnixNano())
	err := svc.SendNotification(notifications.NotificationOptions{
		ID:    id,
		Title: title,
		Body:  body,
		Data:  map[string]any{"session_id": e.SessionKey()},
	})
	if err != nil {
		slog.Warn("notification send failed", "module", "notify", "err", err, "event", e.EventType)
	}
}

func sanitize(s string) string {
	r := []rune(s)
	if len(r) > 100 {
		s = string(r[:100]) + "…"
	}
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func agentLabel(agent string) string {
	switch agent {
	case entity.AgentClaudeCode:
		return "Claude Code"
	case entity.AgentCodex:
		return "Codex"
	default:
		return agent
	}
}

func lastPath(p string) string {
	parts := strings.Split(strings.TrimRight(p, "/"), "/")
	if len(parts) == 0 || p == "" {
		return p
	}
	return parts[len(parts)-1]
}
```

- [ ] **Step 2: Update `notify_test.go`** — remove tests against `osascript`-based functions; rewrite around `ShouldNotify` and `ShouldNotifyByConfig` only

```go
package notify

import (
	"testing"

	"pager/internal/domain/entity"
)

func TestShouldNotify_AttentionOnly(t *testing.T) {
	cases := []struct {
		ev   *entity.AgentEvent
		want bool
	}{
		{&entity.AgentEvent{EventType: "StopFailure"}, true},
		{&entity.AgentEvent{EventType: "PermissionRequest"}, true},
		{&entity.AgentEvent{EventType: "Stop"}, false},
		{&entity.AgentEvent{EventType: "PostToolUse", ToolName: "Edit", PermissionMode: "default"}, false},
		{&entity.AgentEvent{EventType: "PreToolUse", ToolName: "Edit", PermissionMode: "default"}, true},
	}
	for _, tc := range cases {
		if got := ShouldNotify(tc.ev, "attention_only"); got != tc.want {
			t.Errorf("ShouldNotify(%q) = %v; want %v", tc.ev.EventType, got, tc.want)
		}
	}
}

func TestShouldNotifyByConfig(t *testing.T) {
	cfg := map[string][]string{
		"CC": {"PreToolUse", "Stop"},
	}
	if !ShouldNotifyByConfig(&entity.AgentEvent{AgentLabel: "CC", EventType: "Stop"}, cfg) {
		t.Error("CC + Stop should notify")
	}
	if ShouldNotifyByConfig(&entity.AgentEvent{AgentLabel: "CC", EventType: "Notification"}, cfg) {
		t.Error("CC + Notification should NOT notify")
	}
}
```

Run: `go test ./internal/adapter/notify/...` → expect PASS.

- [ ] **Step 3: Wire NotificationService in `app.go`**

Add to imports:
```go
"github.com/wailsapp/wails/v3/pkg/services/notifications"
```

Replace the existing `p.tracker = session.NewTracker(...)` callback (line 57–75) so it accepts a NotificationService reference. The `notify.ShouldNotifyByConfig` block currently calls `notify.ShowEvent(e, cfg.Language)`; it must now pass the svc handle.

The cleanest approach: declare `notifSvc` BEFORE `p.tracker` and capture it in the closure.

```go
notifSvc := notifications.New()
notifSvc.OnNotificationResponse(func(result notifications.NotificationResult) {
	if result.Error != nil {
		slog.Warn("notification response error", "module", "notify", "err", result.Error)
		return
	}
	sessionID, _ := result.Response.UserInfo["session_id"].(string)
	if sessionID == "" {
		return
	}
	if popupWindow != nil {
		popupWindow.Show()
		popupWindow.Focus()
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("highlight-session", map[string]any{"session_id": sessionID})
	}
})

p.tracker = session.NewTracker(func(sessions []*session.Session) {
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}
	wailsApp.Event.Emit("sessions-updated", sessions)

	if len(sessions) > 0 && sessions[0].LastEvent != nil {
		cfg, _ := config.LoadFrom(config.DefaultPath())
		e := sessions[0].LastEvent
		if notify.ShouldNotifyByConfig(e, cfg.NotificationEvents) {
			notify.ShowEvent(notifSvc, e, cfg.Language)
		}
	}

	if p.tray != nil {
		p.updateTrayIcon(sessions)
	}
})
```

Note: `popupWindow` is declared later in `NewPagerApp`. Move the `var popupWindow *application.WebviewWindow` declaration UP — before `notifSvc` setup. The original line 102 already declares it; just move it above the `p.tracker` block.

Also register `notifSvc` in the `Services` slice (line 116):

```go
Services: []application.Service{
	application.NewService(p),
	application.NewService(sessionBinding),
	application.NewService(settingsBinding),
	application.NewService(windowBinding),
	application.NewService(notifSvc),
},
```

- [ ] **Step 4: Build**

Run: `go build ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/notify/ internal/wails/app.go
git commit -m "feat(notify): replace osascript with Wails NotificationService + click handler"
```

---

### Task 23: Frontend listener for `highlight-session`

**Files:**
- Modify: `frontend/src/main.tsx` (or wherever app boot lives)

- [ ] **Step 1: Locate the boot file**

Run: `grep -rn "initSettings\|initSessionSync" /private/data/projects/github.com/sapaude/pager/frontend/src/`

Find which file calls `initSettings()` / `initSessionSync()`. (Likely `frontend/src/main.tsx`.)

- [ ] **Step 2: Add `initUIState()` invocation** alongside the existing init calls

Open the boot file. After `initSettings()` and `initSessionSync()` calls, add:

```ts
import { initUIState } from './store/uistate'
// ...
initUIState()
```

- [ ] **Step 3: Manual verification**

Run: `make build && open build/bin/Pager.app` (signed build required for notifications).

Send a `PermissionRequest` event via the bridge. Click the macOS notification banner. Expected:
- Pager popup appears on the active Space
- The project group containing the session expands if it was collapsed
- The corresponding session card pulses (3 flashes over 1.5s)
- The card scrolls into view

If the app is unsigned, `notifSvc.SendNotification` will log an error; the click flow won't trigger because no notification displayed. This is the documented dev-mode constraint.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/main.tsx
git commit -m "feat(ui): subscribe to highlight-session event for notification deep-link"
```

---

# Final Phase — Verification & Cleanup

### Task 24: Repo-wide AttentionLevel audit

**Files:** None (audit only)

- [ ] **Step 1: Run the audit grep**

```bash
grep -rn 'AttentionLevel\|attention_level\|StatusFinished\|StatusActive\|EventPreToolUse\|EventPostToolUse\|EventStop\b\|AttentionAttention\|AttentionRunning\|AttentionDone\|DetermineAttentionLevel\|statusFromEvent' \
  --include='*.go' --include='*.ts' --include='*.tsx' --include='*.sql' .
```

Expected: zero hits (besides perhaps `statusFromEvent` if you kept it as a thin wrapper inside `sqlite.go` — that's fine).

If anything else turns up, treat it as a follow-up fix in this same task and commit separately.

- [ ] **Step 2: Run all tests**

```bash
make test
make lint
```

Expected: all PASS.

- [ ] **Step 3: Manual end-to-end verification matrix**

For each row, perform the action and verify:

| Action | Expected |
|---|---|
| Send `PreToolUse` `AskUserQuestion` | Card: WAITING red HandHelping pulse; tray dot red |
| Send `PostToolUse` | Card: WORKING green Loader2 spin |
| Send `Stop` (no pending) | Card: DONE light-blue CheckCircle2 |
| Send `StopFailure` | Card: ERROR orange AlertTriangle |
| Toggle a project group | Cards collapse/expand; chevron rotates |
| Restart app | Collapsed projects remain collapsed |
| Click project Trash2 (hover-revealed) | All cards in that project disappear |
| Switch to other Space, press Alt+E | Window appears on active Space |
| `make dev`, change opacity 5 times | hotkey logs show NO "registered" lines |
| Click a notification banner (signed build) | Popup opens, project expands, card highlights, scrolls into view |

- [ ] **Step 4: If everything passes, commit any cleanup**

```bash
git status
# If only remaining changes are from prior tasks, no commit needed.
```

---

### Task 25: End-to-end regression test (deliverable)

This task produces the durable regression artifact that future contributors run before
shipping any change to status / hotkey / notification surfaces.

**Files:**
- Create: `internal/e2e_test.go` (Go integration test for the headless data path)
- Create: `docs/regression/2026-06-01-ui-state-regression.md` (manual UI verification checklist)

- [ ] **Step 1: Write the headless integration test** at `internal/e2e_test.go`

```go
//go:build integration

package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"pager/internal/adapter/httpapi"
	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/store"
)

// TestE2E_StatusModelDataPath drives the full bridge → HTTP → tracker → SQLite path
// for every status-relevant event combination and asserts the resulting Session.Status.
// It does NOT cover UI, hotkey, or notification surfaces — see the manual checklist
// at docs/regression/2026-06-01-ui-state-regression.md for those.
func TestE2E_StatusModelDataPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(filepath.Join(dir, "e2e.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	tr := session.NewTracker(nil)
	srv := httpapi.New(tr, httpapi.WithStore(st))
	srv.Start()
	defer srv.Stop()

	// Wait briefly for listener
	time.Sleep(150 * time.Millisecond)

	type expect struct {
		name      string
		event     entity.AgentEvent
		preceding []entity.AgentEvent // events to send before the assertion event
		want      entity.SessionStatus
	}
	cases := []expect{
		{
			name:  "PreToolUse AskUserQuestion → waiting",
			event: entity.AgentEvent{
				SessionID: "s-ask", CWD: "/p/a", EventType: "PreToolUse",
				ToolName: "AskUserQuestion", Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
		{
			name:  "PreToolUse Edit + bypass → working",
			event: entity.AgentEvent{
				SessionID: "s-bypass", CWD: "/p/b", EventType: "PreToolUse",
				ToolName: "Edit", PermissionMode: "bypassPermissions", Timestamp: time.Now(),
			},
			want: entity.StatusWorking,
		},
		{
			name:  "PreToolUse Edit + default → waiting",
			event: entity.AgentEvent{
				SessionID: "s-edit", CWD: "/p/c", EventType: "PreToolUse",
				ToolName: "Edit", PermissionMode: "default", Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
		{
			name:  "PostToolUse → working",
			event: entity.AgentEvent{
				SessionID: "s-post", CWD: "/p/d", EventType: "PostToolUse",
				ToolName: "Bash", Timestamp: time.Now(),
			},
			want: entity.StatusWorking,
		},
		{
			name:  "Stop (no pending) → done",
			event: entity.AgentEvent{
				SessionID: "s-stop", CWD: "/p/e", EventType: "Stop", Timestamp: time.Now(),
			},
			want: entity.StatusDone,
		},
		{
			name: "Stop (AskUser pending) → waiting",
			preceding: []entity.AgentEvent{
				{SessionID: "s-pending-ask", CWD: "/p/f", EventType: "PreToolUse",
					ToolName: "AskUserQuestion", ToolUseID: "tu-1", Timestamp: time.Now()},
			},
			event: entity.AgentEvent{
				SessionID: "s-pending-ask", CWD: "/p/f", EventType: "Stop",
				Timestamp: time.Now().Add(time.Second),
			},
			want: entity.StatusWaiting,
		},
		{
			name:  "StopFailure → error",
			event: entity.AgentEvent{
				SessionID: "s-fail", CWD: "/p/g", EventType: "StopFailure", Timestamp: time.Now(),
			},
			want: entity.StatusError,
		},
		{
			name:  "PermissionRequest → waiting",
			event: entity.AgentEvent{
				SessionID: "s-perm", CWD: "/p/h", EventType: "PermissionRequest",
				Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
		{
			name:  "Notification → waiting",
			event: entity.AgentEvent{
				SessionID: "s-notif", CWD: "/p/i", EventType: "Notification",
				Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
	}

	postEvent := func(t *testing.T, e entity.AgentEvent) {
		t.Helper()
		body, _ := json.Marshal(e)
		resp, err := http.Post(
			fmt.Sprintf("http://127.0.0.1%s/event", httpapi.ListenAddr),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST status: %d", resp.StatusCode)
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, pre := range tc.preceding {
				postEvent(t, pre)
			}
			postEvent(t, tc.event)

			// Allow tracker write to settle
			time.Sleep(50 * time.Millisecond)

			s, ok := tr.Session(tc.event.SessionID)
			if !ok {
				t.Fatalf("session %q not found", tc.event.SessionID)
			}
			if s.Status != tc.want {
				t.Errorf("session.Status = %q; want %q", s.Status, tc.want)
			}
		})
	}
}

// TestE2E_DismissByProjectFlow asserts the per-project bulk clear chain
// (binding → tracker → store soft-delete).
func TestE2E_DismissByProjectFlow(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(filepath.Join(dir, "e2e2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tr := session.NewTracker(nil)
	make := func(key, cwd string) *entity.AgentEvent {
		return &entity.AgentEvent{
			SessionID: key, CWD: cwd, EventType: "PreToolUse", ToolName: "Edit",
			PermissionMode: "bypassPermissions", Timestamp: time.Now(),
		}
	}
	tr.TrackEvent(make("s1", "/x/projA"))
	tr.TrackEvent(make("s2", "/x/projA"))
	tr.TrackEvent(make("s3", "/x/projB"))

	keys := tr.DismissByProject("projA")
	if len(keys) != 2 {
		t.Errorf("dismissed = %d; want 2", len(keys))
	}
	left := tr.ListByRecent()
	if len(left) != 1 || left[0].SessionID != "s3" {
		t.Errorf("remaining = %+v; want only s3", left)
	}
}
```

- [ ] **Step 2: Run the integration test**

```bash
go test -tags=integration ./internal/... -run 'TestE2E_' -v
```

Expected: PASS for all 9 sub-tests in `TestE2E_StatusModelDataPath` plus
`TestE2E_DismissByProjectFlow`.

- [ ] **Step 3: Write the manual verification checklist** at `docs/regression/2026-06-01-ui-state-regression.md`

```markdown
# UI / Window / Notification Regression Checklist

> Run before shipping any change that touches `internal/wails`, `internal/adapter/notify`,
> `internal/domain/session`, or any file under `frontend/src/`.

**Build:** `make build`
**Launch:** `open build/bin/Pager.app`

Each row = one click-through. Check ✅ when verified, ❌ if regressed.

## A. Status Visual (4-state)

Send each event via `pager-cc-bridge` (or test bridge harness) and verify card render:

| Step | Event | Expected card |
|---|---|---|
| 1 | `--event PreToolUse --agent CC` with stdin `{"tool_name":"AskUserQuestion","cwd":"/p/x","session_id":"r1"}` | WAITING tag (red), HandHelping pulsing icon |
| 2 | `--event PreToolUse` with `{"tool_name":"Edit","permission_mode":"bypassPermissions",...}` | WORKING tag (green), Loader2 spinning |
| 3 | `--event Stop` (same session) | DONE tag (light blue), CheckCircle2 |
| 4 | `--event StopFailure` (new session) | ERROR tag (orange), AlertTriangle |

## B. Project Grouping

| Step | Action | Expected |
|---|---|---|
| 1 | Click project header | Group collapses / chevron rotates from down to right |
| 2 | Click again | Group expands |
| 3 | Quit + relaunch | Previously collapsed projects still collapsed |
| 4 | Hover project header | Trash2 button fades in on the right |
| 5 | Click Trash2 | All cards in that project vanish; SQLite `t_sessions.deleted_at` populated |

## C. Expanded Card

| Step | Action | Expected |
|---|---|---|
| 1 | Click any card | PATH / SESSION / TIME rows render with FolderOpen / Hash / Clock icons |
| 2 | Click Copy on PATH | Icon swaps to Check for ~800ms; clipboard contains the CWD |
| 3 | Click "更多" | TOOL ID / TTY / PERM rows render with Wrench / Terminal / ShieldCheck |
| 4 | Click "收起" | Folded section hides; button text reverts to "更多" |

## D. Multi-Space Window

| Step | Action | Expected |
|---|---|---|
| 1 | Show popup, switch to a different macOS Space | Popup vanishes from current Space |
| 2 | Press Alt+E on the new Space | Popup appears on this Space (does NOT snap back) |
| 3 | Press Alt+E again | Popup hides |

## E. Hotkey Resilience

| Step | Action | Expected |
|---|---|---|
| 1 | Open Settings, change Theme | tail of stderr/log shows NO "hotkey unregistered" or "hotkey registered" lines |
| 2 | Open Settings, change Hotkey to `Ctrl+E` | log shows exactly one "hotkey unregistered" + one "hotkey registered" |
| 3 | Press the new hotkey | popup toggles |
| 4 | Switch to other apps for 30 minutes (or `caffeinate -d` test) and return | Alt+E still toggles popup; if log shows "hotkey unhealthy on app activate, re-registering", that's the recovery path firing as designed |

## F. Notifications (signed build only)

| Step | Action | Expected |
|---|---|---|
| 1 | Trigger a `PermissionRequest` event with `notification_events` configured | macOS notification banner appears |
| 2 | Click the banner | Pager popup appears on active Space; project group expands; target card pulse-highlights for 1.5s; card scrolls into view |
| 3 | Click banner for a session that has been Cleared | Popup appears, no card highlights, no error |

## G. Settings Panel Icons

| Step | Action | Expected |
|---|---|---|
| 1 | Open Settings | 4 nav items (通用 / 通知 / 数据 / 关于) each show a lucide icon (Settings/Bell/Database/Info), no emojis |
| 2 | Click 关于 | Logo is BellRing (lucide), list arrows are ChevronRight / ExternalLink |
| 3 | Click X to close | Settings hides (does not destroy) |

## H. Tray Icon

| Step | Action | Expected |
|---|---|---|
| 1 | All sessions in DONE/ERROR | Tray label is empty |
| 2 | Any session in WORKING (no WAITING) | Tray label empty |
| 3 | Any session in WAITING | Tray label is `●` (red dot) |

---

If any row fails, file an issue and reference this checklist. Do NOT ship until all
rows pass on a fresh `make build && open` from the target branch.
```

- [ ] **Step 4: Final verification — run the integration test from the regression doc**

```bash
go test -tags=integration ./internal/... -run 'TestE2E_' -v
make test
make lint
```

Expected: all PASS. If `make lint` flags anything (TypeScript or `go vet`), fix it before commit.

- [ ] **Step 5: Commit**

```bash
git add internal/e2e_test.go docs/regression/2026-06-01-ui-state-regression.md
git commit -m "test(e2e): add regression test suite + manual UI checklist"
```

---

## Self-Review Notes (writer-side checklist applied)

✅ **Spec coverage:**
- §A → Task 12 (settings), Task 13 (backend dismiss), Task 14 (uistate), Task 15 (UI)
- §B → Task 18 (default rows), Task 19 (folded "更多")
- §C → Tasks 1-7 (entire phase)
- §D → Tasks 17 (card icons), 18-19 (expanded card icons), 20 (settings nav), 21 (about)
- §E → Tasks 9, 10, 11
- §F → Task 8
- §G → Tasks 22, 23
- E2E regression artifact → Task 25 (integration test + manual checklist)

✅ **Type consistency:** `SessionStatus` is consistently a string union in TS (`'working'|'waiting'|'done'|'error'`) and a `type SessionStatus string` in Go with constants `StatusWorking/StatusWaiting/StatusDone/StatusError`. `DismissByProject` (tracker) ↔ `DismissSessionsByProject` (binding) — different names by design (tracker domain method vs Wails binding); both consistently used.

✅ **No placeholders:** Every code step contains the actual code. No "implement later" or "similar to Task N" hand-waves.

✅ **Constants named:** `HIGHLIGHT_DURATION_MS`, `COPY_FEEDBACK_MS`, `COLLAPSED_DEBOUNCE_MS`, `NotificationIDPrefix`, `toolAskUserQuestion`, `permModeBypassPermissions` — all named, no magic numbers.
