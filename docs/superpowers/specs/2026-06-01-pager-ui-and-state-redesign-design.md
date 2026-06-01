# Pager UI & State Redesign — Design Spec

**Date**: 2026-06-01
**Status**: Draft (awaiting user review)
**Scope**: v1.x UI consistency + state model cleanup + window/hotkey/notification robustness

---

## 1. Overview

Four interrelated improvements to Pager v1.x, surfaced from a brainstorming session on
2026-06-01. The work cuts across domain (status model), adapter (notification, terminal,
hotkey), wails (window collection behavior, bindings), and frontend (session list,
session card, settings panel) layers.

| ID | Topic | Type |
|----|-------|------|
| §A | Project grouping: collapse/expand + per-project Clear | Feature |
| §B | Expanded card metadata: PATH / SESSION / TIME (+ folded "更多") | Bug fix + UX |
| §C | Status model unification: 4-value enum WORKING/WAITING/DONE/ERROR | Refactor + bug fix |
| §D | Lucide icon migration across the entire app | UX consistency |
| §E | Hotkey resilience (Alt+E stops working after a while) | Bug fix |
| §F | Multi-Space window binding (window stuck on original Space) | Bug fix |
| §G | Wails-native notifications with click → popup highlight | Feature |

§D is a cross-cutting concern that supports §A–§C. §E + §F + §G together cover Topic ④
from brainstorming (hotkey + Spaces + notification interaction).

---

## 2. Problem Statement

### 2.1 §A — Project grouping today

`SessionList.tsx` already groups sessions by project (CWD last segment), but project
groups are always expanded and there is no way to bulk-clear historical sessions for one
project. Users with many active projects scroll a long flat list.

### 2.2 §B — Expanded card metadata regression

`SessionCard.tsx` expanded view currently renders only `<pre>{contentRaw}</pre>`. Earlier
versions surfaced `project_path` and `session_id`; both fields disappeared from the
expanded view, leaving users without a way to copy/inspect them.

### 2.3 §C — Two parallel status systems disagreeing

`internal/domain/entity/event.go` defines two parallel state systems:

- `Status` (`waiting / active / finished / error`)
- `AttentionLevel` (`attention / running / done`)

`tracker.go::TrackEvent` switches on `e.EventType` against snake_case constants
(`entity.EventPreToolUse = "pre_tool_use"`), but `cmd/bridge/main.go` posts CamelCase
event types (`"PreToolUse"`, `"Stop"`). As a consequence, **`Session.Status` is never
set in the live event path** — only `AttentionLevel` (set by
`bridge/attention.go::DetermineAttentionLevel`) drives UI rendering.

The hidden side effect: `app.go::updateTrayIcon` switches on
`s.Status == StatusWaiting / StatusActive`, so the tray icon never reflects current
state. `sqlite.go::statusFromEvent` already contains a CamelCase-aware compatibility
shim (used during replay), evidencing the inconsistency.

The 5-value system (`waiting/active/finished/error/attention`) mixes two orthogonal
dimensions — Phase (lifecycle) and Attention (user demand) — without alignment, with
`waiting` and `attention` semantically duplicating one another.

### 2.4 §D — Mixed icon styles

`SettingsPanel.tsx` navigation uses emoji (`⚙️ 🔔 💾 ℹ️`), while elsewhere components
already use `lucide-react`. `AboutSettings.tsx` uses `📟` for the app logo and Unicode
arrows (`›` / `↗`) for list affordances.

### 2.5 §E — Alt+E stops working after a while

`hotkey.go` registers a single global hotkey via `golang.design/x/hotkey` and runs a
goroutine consuming `hk.Keydown()`. Two reproducible failure modes:

1. **Window already on another Space (caused by §F)** — `window.IsVisible()` returns
   `true`, so Alt+E hides the (invisible-to-user) window; user perceives "Alt+E broken".
2. **Settings churn** — `SettingsBinding`'s `onChange` callback re-registers the hotkey
   on _every_ settings change (opacity, language, theme), risking races between the
   re-register sequence and the running goroutine.

Possible long-term-idle macOS Accessibility/Input-Monitoring permission revocation also
contributes; the current code has no recovery path.

### 2.6 §F — Multi-Space binding

The popup window does not declare `CollectionBehavior`, so Wails defaults to
`NSWindowCollectionBehaviorFullScreenPrimary`. Once shown on Space N, switching to Space
M and pressing Alt+E either snaps back to Space N or appears to do nothing.

### 2.7 §G — Notification click is a dead end

`internal/adapter/notify/notify.go` uses `osascript` `display notification`, which
provides no click callback. Users who see a Pager notification have no way to be
deep-linked back into the corresponding session card.

---

## 3. Goals & Non-Goals

### 3.1 Goals

1. Single source of truth for session status (one enum, one mapping function).
2. Visually distinct, color-blind-friendly indicators for the four states.
3. Project groups collapsible per project, with persistence.
4. One-click bulk dismiss per project, no modal.
5. Restore PATH and SESSION_ID visibility in expanded card.
6. Zero emojis in the UI; all icons from `lucide-react`.
7. Alt+E reliably re-summons the popup, regardless of which Space the user is on.
8. Notification click deep-links to the originating session card with visible highlight.
9. All changes use Wails v3 native APIs where available — no cgo / hack code.

### 3.2 Non-Goals (defer to v1.x or later)

- Agent tab bar in popup (the "dynamic agent tabs" mentioned in git log applies to
  `NotificationSettings` only).
- Auto-Show popup on notification _fire_ (this spec only auto-Shows on user _click_).
- Auto-jump-to-TTY on notification (user explicitly asked: notification → Pager
  popup highlight, NOT TTY jump).
- Action buttons (yes/no/snooze) on notifications — only deep-link click.
- Backward-compatible rename of `AttentionLevel` field via migration; we delete and
  reset (no production data to migrate at v1.x).
- Linux/Windows hotkey behavior — macOS-only.

---

## 4. Architecture & Design

### 4.1 §C — Status Model

#### 4.1.1 New canonical enum

Defined in `internal/domain/entity/event.go`:

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

`AttentionLevel` field and `AttentionAttention/Running/Done` constants are **deleted**.
The legacy `Status` constants (`StatusFinished`, `StatusActive`) are also deleted.

#### 4.1.2 Centralized derivation

A single pure function in `internal/domain/session/status.go`:

```go
// DeriveStatus folds an event + session context into a single SessionStatus.
// The function is the only place where event_type → status mapping lives.
func DeriveStatus(eventType, toolName, permissionMode string, hasPendingAskUser bool) entity.SessionStatus {
    switch eventType {
    // — terminal failures —
    case "StopFailure", "Error":
        return entity.StatusError

    // — user-attention events —
    case "PermissionRequest", "PermissionDenied", "Notification",
         "Elicitation", "PostToolUseFailure":
        return entity.StatusWaiting

    // — clean termination —
    case "Stop", "SessionEnd", "SubagentStop":
        if hasPendingAskUser {
            return entity.StatusWaiting
        }
        return entity.StatusDone

    // — tool-use —
    case "PreToolUse":
        if toolName == toolAskUserQuestion || permissionMode != permModeBypassPermissions {
            return entity.StatusWaiting
        }
        return entity.StatusWorking

    // — running default —
    default: // PostToolUse / PostToolBatch / SessionStart / TaskCreated / etc.
        return entity.StatusWorking
    }
}
```

Constants `toolAskUserQuestion = "AskUserQuestion"` and
`permModeBypassPermissions = "bypassPermissions"` live alongside `DeriveStatus`. No
magic strings.

`tracker.go::TrackEvent` calls `DeriveStatus(...)` once per event and writes the result
to `Session.Status`. The old switch on snake_case constants is removed.

`statusFromEvent` in `sqlite.go` is replaced with `DeriveStatus` (with `hasPendingAskUser=false`
during single-event replay; tracker re-evaluates after Replay).

`updateTrayIcon` in `app.go` switches on the new `SessionStatus` values. `Working` and
`Waiting` raise the menubar dot; `Done`/`Error` do not.

#### 4.1.3 UI mapping

| `SessionStatus` | Tag text | Color token | Lucide icon | Animation |
|-----------------|----------|-------------|-------------|-----------|
| `working`       | WORKING  | `--c-working` = `#30d158` | `Loader2` | spin 1.4s |
| `waiting`       | WAITING  | `--c-waiting` = `#ff453a` | `HandHelping` | pulse 1.6s |
| `done`          | DONE     | `--c-done` = `#5ac8fa`    | `CheckCircle2` | none |
| `error`         | ERROR    | `--c-error` = `#ff9f0a`   | `AlertTriangle` | none |

Color decisions:

- `done` is **light blue** (not green) to avoid clashing with `working` green.
- `error` is **orange** (not red) so the **red** namespace is reserved for `waiting`,
  keeping the user's attention reflex sharp.

Color + shape dual-encoding satisfies color-blind accessibility.

CSS tokens are declared in `frontend/src/index.css` alongside existing
`--pager-card-attention-bg` etc.; the legacy bg/border tokens are renamed to match the
new state names but keep their numeric values (no visual regression for working /
waiting; new tokens added for `done` and `error`).

### 4.2 §A — Project Grouping (Collapse + Clear)

#### 4.2.1 Collapse state persistence

Settings (`internal/infra/config/config.go`):

```go
type Settings struct {
    // ... existing fields ...

    // CollapsedProjects lists project names (CWD last segment) whose group
    // is collapsed in the popup. Default empty (all expanded).
    CollapsedProjects []string `json:"collapsed_projects"`
}
```

Frontend (`frontend/src/store/sessions.ts` or new `frontend/src/store/uistate.ts`)
exposes:

```ts
interface UIState {
    collapsedProjects: Set<string>
    toggleCollapsed: (project: string) => void
    isCollapsed: (project: string) => boolean
}
```

`toggleCollapsed` calls a new Wails binding `WindowBinding.SetCollapsedProjects(list []string)`
which writes to `settings.json` (debounced ~250ms in frontend).

Initial load: `useSettingsStore` already loads settings on mount; copy into
`collapsedProjects` set on first hydration.

#### 4.2.2 Project header markup

`frontend/src/components/SessionList.tsx`:

```tsx
<div className="pheader" onClick={() => toggleCollapsed(project)}>
  {/* Chevron rotates -90deg when collapsed */}
  {collapsed ? <ChevronRight size={10} /> : <ChevronDown size={10} />}
  <Folder size={11} className="text-[--pager-text-muted] opacity-60" />
  <span>{project}</span>
  <span className="count">{group.sessions.length}</span>
  <span className="divider" />
  <button
    className="clear-btn"  // opacity-0 by default; group-hover:opacity-100
    onClick={(e) => { e.stopPropagation(); clearProject(project); }}
    title="清理该项目历史会话">
    <Trash2 size={11} />
  </button>
</div>
{!collapsed && <SessionCardList sessions={group.sessions} />}
```

Hover behavior: Clear button is `opacity-0` until pointer enters the project header.
Hovering Clear itself turns it into the danger color (`--c-waiting` red). No tooltip
modal, no confirmation dialog (per Q2 decision).

#### 4.2.3 Clear semantics

New backend method on `session.Tracker`:

```go
// DismissByProject removes all sessions whose CWD's last segment equals project.
// Returns the list of dismissed session keys for caller-side logging / store sync.
func (t *Tracker) DismissByProject(project string) []string
```

New Wails binding method on `SessionBinding`:

```go
func (s *SessionBinding) DismissSessionsByProject(project string) error {
    if s.tracker == nil { return errTrackerNotInitialised }
    keys := s.tracker.DismissByProject(project)
    if s.store != nil {
        for _, k := range keys {
            _ = s.store.DismissSession(k) // soft-delete, errors ignored — same pattern as DismissSession
        }
    }
    return nil
}
```

No filter on `SessionStatus` (per Q2 decision: `working` / `waiting` sessions are also
cleared; if the agent emits new events afterwards, the session reappears with a new
notification — that is the desired behavior).

### 4.3 §B — Expanded Card Metadata

#### 4.3.1 Default expanded section (always visible when expanded)

Rendered in `SessionCard.tsx` when `expanded === true`, before the existing
`<pre>{contentRaw}</pre>` block:

| Row | Lucide icon | Label | Value source | Copy |
|-----|-------------|-------|--------------|------|
| 1   | `FolderOpen` | PATH    | `session.CWD`              | yes |
| 2   | `Hash`       | SESSION | `session.SessionID` (or `session.Key` fallback) | yes |
| 3   | `Clock`      | TIME    | formatted `LastEvent.timestamp` (`YYYY/MM/DD HH:mm:ss`) | no |

Empty values render the row but with `<span class="meta-val">—</span>` and a hidden Copy
button (visibility:hidden to preserve layout).

#### 4.3.2 Folded "更多" section

Below content_raw, a button:

```tsx
<button onClick={() => setShowMore(!showMore)} className="more-btn">
  {showMore ? <ChevronUp size={11} /> : <ChevronDown size={11} />}
  {showMore ? '收起' : '更多'}
</button>
```

Button text is exactly `更多` / `收起` (no "诊断信息" suffix per user request).

When `showMore === true`, three additional rows render in a second `meta-block` above
the button:

| Row | Lucide icon  | Label   | Value source                                            | Copy |
|-----|--------------|---------|---------------------------------------------------------|------|
| 4   | `Wrench`     | TOOL ID | `LastEvent.tool_use_id` (rendered only when non-empty)  | yes  |
| 5   | `Terminal`   | TTY     | `${session.TTY} · ${session.TermProgram}`               | no   |
| 6   | `ShieldCheck`| PERM    | `LastEvent.permission_mode` (rendered only when non-empty) | no |

The "more" expanded state is **per-card, in-memory only** (does not persist across app
restarts, does not persist in `settings.json`).

#### 4.3.3 Copy button

Lucide `Copy` icon, 18×18 hit target, 11×11 visual. Click triggers
`navigator.clipboard.writeText(value)` and a transient checkmark (icon swap to `Check`
for 800ms) as feedback. No toast, no alert.

### 4.4 §D — Lucide Icon Migration

Every emoji or Unicode glyph used as UI affordance is replaced with a `lucide-react`
component. Full mapping table:

| File | Element | Old | New (lucide-react) |
|------|---------|-----|--------------------|
| `pages/SettingsPanel.tsx` | nav: 通用 | `⚙️` | `Settings` |
| `pages/SettingsPanel.tsx` | nav: 通知 | `🔔` | `Bell` |
| `pages/SettingsPanel.tsx` | nav: 数据 | `💾` | `Database` |
| `pages/SettingsPanel.tsx` | nav: 关于 | `ℹ️` | `Info` |
| `pages/settings/AboutSettings.tsx` | App logo | `📟` | `BellRing` (white stroke on existing gradient bg) |
| `pages/settings/AboutSettings.tsx` | Right chevron | `›` | `ChevronRight` |
| `pages/settings/AboutSettings.tsx` | External link arrow | `↗` | `ExternalLink` |
| `components/SessionList.tsx` | Project chevron | (none) | `ChevronDown` / `ChevronRight` |
| `components/SessionList.tsx` | Project clear button | (new) | `Trash2` |
| `components/SessionCard.tsx` | Status WAITING indicator | red dot | `HandHelping` (animate-pulse) |
| `components/SessionCard.tsx` | Status WORKING indicator | green dot | `Loader2` (animate-spin) |
| `components/SessionCard.tsx` | Status DONE indicator | gray dot | `CheckCircle2` |
| `components/SessionCard.tsx` | Status ERROR indicator | (new) | `AlertTriangle` |
| `components/SessionCard.tsx` | Expand: PATH icon | (none) | `FolderOpen` |
| `components/SessionCard.tsx` | Expand: SESSION icon | (none) | `Hash` |
| `components/SessionCard.tsx` | Expand: TIME icon | (none) | `Clock` |
| `components/SessionCard.tsx` | Expand: TOOL ID icon | (none) | `Wrench` |
| `components/SessionCard.tsx` | Expand: TTY icon | (none) | `Terminal` |
| `components/SessionCard.tsx` | Expand: PERM icon | (none) | `ShieldCheck` |
| `components/SessionCard.tsx` | Copy button | (none) | `Copy` (swap to `Check` 800ms after click) |
| `components/SessionCard.tsx` | More/collapse toggle | (none) | `ChevronDown` / `ChevronUp` |

Already lucide (no change): `App.tsx` Pin/Settings, `EmptyState.tsx` Clock,
`SessionCard.tsx` ArrowRight (jump button), `SettingsPanel.tsx` X.

Rationale for icon choices:

- `Hash` for SESSION instead of `KeyRound`: session_id is an identifier; `#` is the
  international convention for IDs; `KeyRound` overstates security semantics.
- `BellRing` for app logo instead of `Radio`/`Pager`: BellRing's vibration motion
  matches Pager's "alert" semantics; lucide does not have a dedicated `Pager` icon.
- `HandHelping` for WAITING: "raised hand" universally signals "I need help",
  more precise than `Bell` (which signals notification) for a state meaning
  "user must respond".

### 4.5 §F — Multi-Space Binding

Set `CollectionBehavior` on the popup window when creating it
(`internal/wails/app.go`):

```go
popupWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
    // ... existing fields ...
    Mac: application.MacWindow{
        Backdrop:           application.MacBackdropTransparent,
        CollectionBehavior: application.MacWindowCollectionBehaviorMoveToActiveSpace,
    },
})
```

`MoveToActiveSpace` (= `1<<1`) is preferred over `CanJoinAllSpaces` (= `1<<0`):

- `CanJoinAllSpaces` makes the window present on every Space at all times — visually
  noisy.
- `MoveToActiveSpace` only relocates the window to the active Space when `Show()`
  is called — the "summon to me" semantics users expect from Alt+E.

The settings window keeps default behavior (no `CollectionBehavior` set) since users
open it deliberately and aren't surprised by Space-binding there.

### 4.6 §E — Hotkey Resilience

Three improvements to `internal/wails/hotkey.go`, all standard library + Wails:

#### 4.6.1 Re-register only on real change

The `SettingsBinding.onChange(cfg)` signature is **unchanged**. The diff happens at the
call site in `app.go` via a closure-captured `lastHotkey`:

```go
// internal/wails/app.go
var lastHotkey string
settingsBinding := NewSettingsBinding(func(cfg config.Settings) {
    if cfg.HotkeyToggle == lastHotkey {
        return
    }
    lastHotkey = cfg.HotkeyToggle
    RegisterHotkey(popupWindow, cfg.HotkeyToggle)
}, eventStore)
// Initialize lastHotkey to current value so first explicit settings change still triggers
lastHotkey = initialCfg.HotkeyToggle
```

This is the minimal-impact change: `SettingsBinding` does not need a new constructor
parameter, and the dedup logic stays where the consumer lives. `RegisterHotkey` is no
longer called on opacity / theme / language / notification changes.

#### 4.6.2 Health-check on app activation

Listen for the macOS app-active event and verify the hotkey goroutine is alive:

```go
// internal/wails/app.go (after popupWindow + RegisterHotkey)
wailsApp.Event.OnApplicationEvent(events.Mac.ApplicationDidBecomeActive, func(_ *application.ApplicationEvent) {
    if !IsHotkeyHealthy() {
        cfg, _ := config.LoadFrom(config.DefaultPath())
        slog.Warn("hotkey unhealthy on app activate, re-registering", "module", "hotkey")
        RegisterHotkey(popupWindow, cfg.HotkeyToggle)
    }
})
```

`IsHotkeyHealthy()` checks `currentHotkey != nil` and that the goroutine is still
listening (we add a `goroutineAlive atomic.Bool` set to `true` inside the goroutine and
reset to `false` when the channel closes).

#### 4.6.3 Structured logging

Every state transition in `hotkey.go` gets a `slog` log line via
`infra/log.Module("hotkey")`:

- `Register` — success: `level=info "hotkey registered" key=Alt+E`; failure:
  `level=error "hotkey register failed" err=... key=...`
- `Unregister` — `level=info "hotkey unregistered" key=...`
- `Keydown received` — `level=debug "hotkey fired" visible=true|false`
- `Goroutine exit` — `level=warn "hotkey goroutine exited; channel closed" key=...`

These logs are the diagnostic substrate for any future "Alt+E doesn't work" reports.

#### 4.6.4 Hotkey library choice

Wails v3 alpha.96 exposes window-scoped `KeyBinding.Add(...)`, but no system-wide
global hotkey API. `golang.design/x/hotkey` remains the right choice and is **not**
considered a hack — Wails docs do not provide an alternative for this use case.

### 4.7 §G — Wails-Native Notifications + Click → Highlight

#### 4.7.1 Service registration

Replace `internal/adapter/notify/notify.go`'s `osascript` implementation with
`pkg/services/notifications.NotificationService`. The service is registered as a Wails
service in `app.go`:

```go
notificationSvc := notifications.New() // returns *notifications.NotificationService
notificationSvc.OnNotificationResponse(func(result notifications.NotificationResult) {
    if result.Error != nil {
        slog.Warn("notification response error", "module", "notify", "err", result.Error)
        return
    }
    sessionID, _ := result.Response.UserInfo["session_id"].(string)
    if sessionID == "" { return }
    popupWindow.Show()
    popupWindow.Focus()
    wailsApp.Event.Emit("highlight-session", map[string]any{"session_id": sessionID})
})

wailsApp := application.New(application.Options{
    // ...
    Services: []application.Service{
        application.NewService(p),
        application.NewService(sessionBinding),
        application.NewService(settingsBinding),
        application.NewService(windowBinding),
        application.NewService(notificationSvc),
    },
})
```

If notification service initialization fails (most commonly: app not signed),
Pager **logs an error and continues without notifications** — the app does not crash.
The error is surfaced to the user via a Settings → Notifications banner ("通知服务
不可用：应用未签名"). No osascript fallback (per Q11 simplification).

#### 4.7.2 Sending a notification

`notify.ShowEvent(e, lang)` is rewritten:

```go
func ShowEvent(svc *notifications.NotificationService, e *entity.AgentEvent, lang string) {
    if svc == nil { return }
    title := fmt.Sprintf("%s · %s", agentLabel(e), lastPath(e.CWD))
    body  := bodyText(e, lang)
    err := svc.SendNotification(notifications.NotificationOptions{
        ID:    notificationID(e),  // "evt-<sessionkey>-<unix-ns>"
        Title: title,
        Body:  body,
        Data:  map[string]any{"session_id": e.SessionKey()},
    })
    if err != nil {
        slog.Warn("notification send failed", "module", "notify", "err", err)
    }
}
```

`notificationID(e)` is unique per call (sessionKey + nanosecond timestamp) so multiple
notifications don't merge.

#### 4.7.3 Frontend highlight handler

`frontend/src/store/sessions.ts`:

```ts
export const useUIStore = create<{ highlightedKey: string | null }>(...)

Events.On('highlight-session', (ev) => {
  const sessionID = ev?.data?.session_id ?? ev?.session_id
  if (!sessionID) return
  // Auto-expand the project group containing this session
  const session = useSessionStore.getState().sessions.find(s => s.Key === sessionID || s.SessionID === sessionID)
  if (!session) return
  const project = projectFromCWD(session.CWD)
  useUIStore.getState().setExpanded(project)
  // Trigger highlight (self-clearing after 1.5s)
  useUIStore.getState().setHighlighted(session.Key)
  setTimeout(() => useUIStore.getState().clearHighlighted(session.Key), HIGHLIGHT_DURATION_MS)
})
```

`SessionCard.tsx` reads `useUIStore.highlightedKey` and applies a
`pulse-highlight` class (3 background-color flashes over 1.5s using a CSS keyframe).
After the timer, the class is removed.

`SessionList.tsx` uses a `ref` map keyed by session key; when `highlightedKey` becomes
non-null, it calls `el.scrollIntoView({ behavior: 'smooth', block: 'center' })`.

Constants:

```ts
const HIGHLIGHT_DURATION_MS = 1500
const HIGHLIGHT_PULSE_COUNT = 3
```

(per CLAUDE.md "禁止魔数")

Edge case: if `session_id` doesn't match any current session (already dismissed,
or evicted by 30-minute filter), the popup is still shown but no card is highlighted.
No error.

---

## 5. Data Model Changes

### 5.1 `internal/domain/entity/event.go`

- **Remove** `AttentionLevel` field from `AgentEvent`.
- **Remove** constants `AttentionAttention / AttentionRunning / AttentionDone`.
- **Remove** legacy status constants `StatusFinished / StatusActive`.
- **Add** type `SessionStatus string` and constants `StatusWorking / StatusWaiting /
  StatusDone / StatusError`.
- `Session.Status` field type changes from `string` to `SessionStatus`.

### 5.2 `internal/domain/session/`

- **New** `status.go` containing `DeriveStatus` + helper constants
  (`toolAskUserQuestion`, `permModeBypassPermissions`).
- `tracker.go::TrackEvent` rewritten: removes the snake-case switch, calls
  `DeriveStatus(...)` once, writes result into `Session.Status`.
- `Session` struct loses `AttentionLevel string` field.

### 5.3 `internal/adapter/bridge/attention.go`

- **Deleted** (its sole consumer was the now-removed `AttentionLevel` chain). The bridge
  no longer computes status pre-emptively; tracker derives status from the raw
  event_type/tool_name/permission_mode trio it receives.
- `cmd/bridge/main.go` no longer calls `bridge.DetermineAttentionLevel(...)` and no
  longer sets `e.AttentionLevel`. Only event_type, tool_name, tool_use_id,
  permission_mode are preserved on the wire.

### 5.4 `internal/infra/store/sqlite.go` + `schema.sql`

The SQLite schema currently uses tables `t_sessions` and `t_events`; both have an
`attention_level` column. Schema cleanup is unified to match the new 4-value status
enum and to drop the obsolete attention dimension entirely.

#### 5.4.1 New `schema.sql` (fresh installs)

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

#### 5.4.2 Field-level cleanup checklist

| Table | Column | Action | Reason |
|-------|--------|--------|--------|
| `t_sessions` | `attention_level` | **DROP** | Field deleted in §5.1 |
| `t_sessions` | `status` | DEFAULT `'active'` → `'working'` | Align with new enum |
| `t_sessions` | `status` (comment) | `waiting/active/finished/error` → `working/waiting/done/error` | Align with new enum |
| `t_events` | `attention_level` | **DROP** | Field deleted in §5.1 |
| `t_events` | `event_type` (comment) | snake_case list → CamelCase list | Bridge has emitted CamelCase since 17dc051 |

#### 5.4.3 Migration for existing databases

`migrate(db)` (the function on line 425 of `sqlite.go`) is extended; existing
"errors ignored" idempotent pattern is preserved:

```go
func migrate(db *sqlx.DB) {
    // v1.1: existing
    _, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN agent_label TEXT NOT NULL DEFAULT ''`)

    // v2.0: rewrite legacy status values to new enum
    _, _ = db.Exec(`UPDATE t_sessions SET status = 'working' WHERE status = 'active'`)
    _, _ = db.Exec(`UPDATE t_sessions SET status = 'done'    WHERE status = 'finished'`)

    // v2.0: drop obsolete attention_level columns
    // (SQLite >= 3.35 supports DROP COLUMN; modernc.org/sqlite v1.51.0 includes it)
    _, _ = db.Exec(`ALTER TABLE t_sessions DROP COLUMN attention_level`)
    _, _ = db.Exec(`ALTER TABLE t_events DROP COLUMN attention_level`)
}
```

#### 5.4.4 Code-level cleanup in `sqlite.go`

| Line (current) | What to change |
|---------------|----------------|
| `100`  | Remove `e.attention_level` from `LoadRecentSessions` SELECT column list |
| `168`  | Remove `attention_level` from `SessionEvents` SELECT column list |
| `356`  | Remove `attention_level` from `t_sessions` INSERT/UPSERT column list and the corresponding `excluded.attention_level` reference |
| `359-360` | Remove the `attention_level = excluded.attention_level` UPSERT clause |
| `368`  | Remove `e.AttentionLevel` from the `t_sessions` INSERT bind values |
| `377`  | Remove `attention_level` from `t_events` INSERT column list |
| `394–410` | Replace `statusFromEvent(e *entity.AgentEvent) string` body with a call to `session.DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, false)`. The wrapper function may stay for callers that already use it, but its body delegates to the new domain function. |

#### 5.4.5 Other code sites that reference `attention_level`

A repo-wide audit (`grep -rn 'attention_level\|AttentionLevel'`) will run as part of
the implementation; expected hits and resolutions:

- `internal/domain/entity/event.go` — field deleted (§5.1)
- `internal/adapter/bridge/types.go` — JSON unmarshal field deleted (no longer sent
  by bridge per §5.3)
- `internal/adapter/bridge/attention.go` — file deleted (§5.3)
- `internal/adapter/bridge/attention_test.go` — deleted with the file
- `internal/adapter/notify/notify.go` — `ShouldNotify`'s `entity.AttentionAttention`
  reference removed; logic rewritten against the new SessionStatus
- `internal/wails/app.go` — `updateTrayIcon` switch updated to new values
- `frontend/src/store/sessions.ts` — `AttentionLevel` type alias and `Session` field
  removed; `Status: 'working'|'waiting'|'done'|'error'`
- `frontend/src/components/SessionCard.tsx` — `isAttention`/`isRunning` derivations
  replaced with direct `Session.Status` switch
- `frontend/src/store/sessions.ts::filterExpiredDone` — predicate updated to
  `s.Status === 'done'` (was `s.AttentionLevel === 'done'`)

### 5.5 `internal/infra/config/config.go`

```go
type Settings struct {
    // existing fields...
    CollapsedProjects []string `json:"collapsed_projects"` // NEW
}
```

Default: empty slice (everything expanded). No migration needed.

---

## 6. Public API / Wails Bindings

### 6.1 New bindings

| Binding | Method | Purpose |
|---------|--------|---------|
| `SessionBinding` | `DismissSessionsByProject(project string) error` | §A.2.3 bulk clear |
| `WindowBinding`  | `SetCollapsedProjects(list []string) error` | §A.2.1 persist collapse state |

### 6.2 New Wails events

| Event name | Direction | Payload | Triggered by |
|------------|-----------|---------|--------------|
| `highlight-session` | Go → Frontend | `{ session_id: string }` | Notification click callback |

Existing events `sessions-updated` and `settings-changed` are unchanged.

### 6.3 Removed/changed bindings

- `SessionBinding.ListSessions` returns the same struct shape, but `Status` field now
  serializes as one of the new four values, and `AttentionLevel` field is gone.

Frontend bindings auto-regenerate via `make bindings`.

---

## 7. Frontend Component Changes

### 7.1 New / modified files

| File | Change |
|------|--------|
| `frontend/src/components/SessionList.tsx` | Add chevron + Folder + count + Trash2 to project header; conditional render based on `useUIStore.collapsed` |
| `frontend/src/components/SessionCard.tsx` | Replace dot with lucide status icon; add 4th ERROR state; add 3 default + 3 folded metadata rows; add Copy button; add highlight class |
| `frontend/src/store/sessions.ts` | Update `Session` type (remove `AttentionLevel`, change `Status` to new union); subscribe to `highlight-session` event |
| `frontend/src/store/uistate.ts` | **New**. `collapsedProjects: Set<string>`, `highlightedKey: string \| null`, `expandedProjects: Set<string>`, plus mutators |
| `frontend/src/pages/SettingsPanel.tsx` | Replace 4 nav emojis with lucide |
| `frontend/src/pages/settings/AboutSettings.tsx` | Replace 📟 + › + ↗ with lucide |
| `frontend/src/index.css` | Add `--c-working`, `--c-waiting`, `--c-done`, `--c-error` and corresponding `--bg-*`, `--bd-*`, `--pill-*` tokens; add `pulse-highlight` keyframe |

### 7.2 New CSS keyframes

```css
@keyframes pulse-highlight {
  0%, 33%, 66%, 100% { background-color: var(--card-base-bg); }
  16%, 50%, 83%      { background-color: rgba(10, 132, 255, 0.18); }
}
.pulse-highlight {
  animation: pulse-highlight 1.5s ease-in-out;
}

@keyframes pulse-attn {
  0%, 100% { transform: scale(1); opacity: 1; }
  50%      { transform: scale(1.15); opacity: 0.7; }
}
.anim-pulse { animation: pulse-attn 1.6s ease-in-out infinite; }

@keyframes spin-loader {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
.anim-spin { animation: spin-loader 1.4s linear infinite; }
```

---

## 8. Persistence / Settings Schema

`~/.config/pager/settings.json` gains:

```json
{
  "collapsed_projects": ["my-other-app", "archived-thing"]
}
```

Existing keys (`language`, `theme`, `opacity`, `hotkey_toggle`,
`notification_level`, `popup_width`, `popup_pinned`, `session_load_hours`,
`notification_events`) are unchanged.

SQLite schema is migrated by extending the existing `migrate(db)` function in
`internal/infra/store/sqlite.go`; full migration SQL and the new fresh-install
`schema.sql` are specified in §5.4. The migration is idempotent (uses the
existing "errors ignored" pattern with `_, _ = db.Exec(...)`), so re-running on
already-migrated databases is a no-op. No separate `meta` / `schema_version` table
is introduced.

---

## 9. Testing Strategy

### 9.1 Go unit tests

- `internal/domain/session/status_test.go` — table-driven test of `DeriveStatus`
  covering all event types × `bypassPermissions` × `hasPendingAskUser` combinations.
  Existing `tracker_test.go` is updated to assert on new `Session.Status` values.
- `internal/wails/hotkey_test.go` (new) — verifies `IsHotkeyHealthy` and
  re-registration triggers; uses a fake hotkey registrar interface.
- `internal/infra/store/sqlite_test.go` — adds `TestMigrationV1ToV2` proving an
  existing `pager.db` with `active/finished/attention_level` rows migrates without
  data loss.
- `internal/adapter/notify/notify_test.go` — adapted to mock the
  `notifications.NotificationService` interface; asserts `Data["session_id"]` is set
  correctly.

### 9.2 Manual / integration

- Start with empty SQLite, send a `PreToolUse` `AskUserQuestion` event via the bridge,
  verify card renders WAITING (red, HandHelping pulse) and tray dot is red.
- Send `Stop`, verify DONE (light blue, CheckCircle2).
- Send `StopFailure`, verify ERROR (orange, AlertTriangle).
- Toggle a project group; restart Pager; confirm collapsed state persisted.
- Click Clear; confirm all sessions in that project are removed from list and SQLite
  rows are soft-deleted.
- Switch to another macOS Space; press Alt+E; window appears on the active Space.
- Click on a system notification; popup appears with the right project expanded and
  the corresponding card pulse-highlights.
- Build unsigned (dev mode); confirm Pager shows "通知服务不可用" banner instead of
  crashing.

---

## 10. Migration & Backward Compat

Pager is currently v1.0.0; no production users with persisted state from prior schema
beyond local development databases. We accept a one-shot lossy column drop
(`attention_level`) and value rewrite (`active→working`, `finished→done`).

The `notification_events` config keys (CamelCase event types) are unchanged.

---

## 11. Risks & Open Items

| Risk | Mitigation |
|------|------------|
| Wails NotificationService requires signed app on macOS — `make dev` won't work | Surface clear "通知服务不可用" banner in Settings → Notifications; document in README that signed builds are required for notification click-through. |
| `MacWindowCollectionBehaviorMoveToActiveSpace` may interact unexpectedly with `AlwaysOnTop=true` (when pinned) | QA matrix: pinned×Space-switch, pinned×Show, pinned×Hide. Document any anomaly. |
| `golang.design/x/hotkey` has no public health-probe API; we infer health from goroutine state | Acceptable — health check on app-activate is best-effort, and re-register is idempotent. |
| Lucide icons add ~30 KB to bundle | Tree-shaking via named imports only; verified by Vite production build. |
| `notifications.NotificationService` API surface is in alpha and may change before v3 stable | We pin to alpha.96 in `go.mod` until upgrade is intentional. |

---

## 12. Implementation Order Hint

Suggested for `writing-plans` consumption:

1. §C status model (foundation; everything else depends on the new SessionStatus values)
2. §F multi-Space (one-line wails option, eliminates the most user-visible bug fast)
3. §A project grouping + §B expanded card (UI work, no backend coupling beyond §C)
4. §D lucide migration (mechanical, can land alongside §A/§B)
5. §E hotkey resilience (logging-driven, low risk)
6. §G Wails-native notifications (largest API surface; depends on §F because click handler does `Show()`)
