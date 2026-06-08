# Pager — Technical PRD

> Version: 1.0
> MVP v1.0 (Claude Code hook only, macOS only)
> Last updated: 2026-05-28

---

## 0. Reading Guide

This document is the **complete technical specification** for Pager.

Conventions:
- `// [IMPL]` marks key logic that needs implementation
- `// [TODO-v1.5]` marks features deferred to v1.5
- `// [PoC-VERIFIED]` marks code verified in PoC phase

---

## 1. Core Data Structures

### 1.1 AgentEvent (project-wide event structure)

File: `internal/event/types.go`

```go
package event

import (
    "encoding/json"
    "time"
)

// EventType enum
const (
    EventPreToolUse  = "pre_tool_use"
    EventPostToolUse = "post_tool_use"
    EventStop        = "stop"
    EventError       = "error"
    EventNotification = "notification" // CC Notification hook (v1.5)
)

// Agent enum
const (
    AgentClaudeCode = "claude-code"
    AgentCodex      = "codex" // wired in v1.5 via cmd/pager-bridge --agent Codex
)

// SessionStatus enum
const (
    StatusWaiting  = "waiting"   // pre_tool_use fired, no post yet
    StatusActive   = "active"    // post_tool_use received
    StatusFinished = "finished"  // stop received
    StatusError    = "error"
)

// AgentEvent is the only transport structure: bridge -> server -> UI
// bridge fills all fields; server and UI are read-only consumers
type AgentEvent struct {
    // === Source identification ===
    Agent          string `json:"agent"`
    Host           string `json:"host"`

    // === Session ID (triple uniquely identifies a session) ===
    CWD            string `json:"cwd"`
    TTY            string `json:"tty"`
    SessionID      string `json:"session_id"`

    // === Terminal location (for jump) ===
    TermProgram    string `json:"term_program"`
    ITermSessionID string `json:"iterm_session_id,omitempty"`

    // === Event content ===
    EventType      string `json:"event_type"`
    ToolName       string `json:"tool_name"`
    ToolUseID      string `json:"tool_use_id"`
    Content        string `json:"content"`
    ContentRaw     string `json:"content_raw"`

    // === Metadata ===
    RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
    Timestamp      time.Time       `json:"timestamp"`
}

func (e *AgentEvent) SessionKey() string {
    return e.Host + ":" + e.CWD + ":" + e.TTY
}
```

### 1.2 Session (Registry internal state)

File: `internal/registry/registry.go`

```go
package registry

import (
    "sync"
    "time"
    "pager/internal/event"
)

type Session struct {
    Key        string
    Agent      string
    Host       string
    CWD        string
    TTY        string
    TermProgram string
    ITermSessionID string
    Status     string
    LastEvent  *event.AgentEvent
    PendingTools map[string]*event.AgentEvent
    UpdatedAt  time.Time
}

type Registry struct {
    mu       sync.RWMutex
    sessions map[string]*Session
    onChange func(sessions []*Session)
}

func New(onChange func([]*Session)) *Registry {
    return &Registry{
        sessions: make(map[string]*Session),
        onChange: onChange,
    }
}

// [IMPL] Apply processes an AgentEvent, updates Registry state
// Rules:
//   pre_tool_use  -> upsert session, Status=waiting, add to PendingTools[tool_use_id]
//   post_tool_use -> remove from PendingTools; if PendingTools empty -> Status=active
//   stop          -> Status=finished
//   error         -> Status=error
// Call onChange after every change
func (r *Registry) Apply(e *event.AgentEvent) {}

func (r *Registry) ListSorted() []*Session { return nil }

func (r *Registry) GetByTTY(tty string) (*Session, bool) { return nil, false }
```

---

## 2. Bridge (vibecoding-pager-cc-bridge)

### 2.1 Constraints

- Called by CC hook, receives CC hook JSON via stdin
- Extracts env info + Content -> constructs AgentEvent -> POST 127.0.0.1:7421/event
- **Any error (network/parse/timeout) must be silent, exit 0** - never affect CC
- HTTP timeout: hard 1 second
- Entire bridge process must exit within 2 seconds

### 2.2 CC Hook stdin schema

File: `internal/bridge/types.go`

```go
package bridge

import "encoding/json"

type CCHookInput struct {
    SessionID string          `json:"session_id"`
    CWD       string          `json:"cwd"`
    ToolName  string          `json:"tool_name"`
    ToolInput json.RawMessage `json:"tool_input"`
    ToolUseID string          `json:"tool_use_id"`
}

type BashInput struct { Command string `json:"command"` }
type FileInput struct { FilePath string `json:"file_path"` }
type GlobInput struct { Pattern string `json:"pattern"` }
type GrepInput struct { Pattern string `json:"pattern"` }
type WebFetchInput struct { URL string `json:"url"` }
type WebSearchInput struct { Query string `json:"query"` }
type TaskInput struct { Description string `json:"description"` }
```

### 2.3 Content Extraction Rules

File: `internal/bridge/extractor.go`

Content extraction by tool:
- Bash -> command string
- Edit -> "编辑 " + file_path
- Write -> "写入 " + file_path
- Read -> "读取 " + file_path
- Glob -> "查找 " + pattern
- Grep -> "搜索 " + pattern
- WebFetch -> "抓取 " + url
- WebSearch -> "搜索 " + query
- Task -> "子任务: " + description
- mcp__* -> "MCP: " + last segment

All content truncated to 60 runes for `Content` field; full version in `ContentRaw`.

### 2.4 Bridge main

File: `cmd/bridge/main.go`

- `defer os.Exit(0)` at top
- Event type from `os.Args[1]` (pre_tool_use | post_tool_use | stop)
- Read stdin, unmarshal CCHookInput
- Extract Content via ExtractContent()
- Detect TTY via `tty` command
- Read env: TERM_PROGRAM, ITERM_SESSION_ID, PWD
- Post to server via bridge.PostEvent()
- Debug mode: `PAGER_DEBUG=1` writes raw payload to `/tmp/pager-raw.json`

### 2.5 HTTP Poster

File: `internal/bridge/poster.go`

- POST to `http://127.0.0.1:7421/event`
- 1 second timeout
- Silent on failure (stderr warning only)

---

## 3. HTTP Server

File: `internal/server/server.go`

- Listen on `127.0.0.1:7421`
- `POST /event` -> decode AgentEvent -> Registry.Apply()
- `GET /sessions` -> debug endpoint, returns JSON

---

## 4. System Notifications

File: `internal/notify/notify.go`

- MVP: osascript implementation
- [TODO-v1.5]: UNUserNotificationCenter (CGO) after app signing
- Notification triggers:
  - pre_tool_use -> show agent + cwd + content
  - stop -> "任务完成"
  - error -> show error content
  - post_tool_use -> no notification

---

## 5. Terminal Jump

File: `internal/terminal/jump.go`

- JumpRequest: TTY, TermProgram, ITermSessionID
- iTerm2: priority ITermSessionID (unique id), fallback tty matching via AppleScript
- Terminal.app: tty matching via AppleScript
- WezTerm: [TODO-v1.5] fallback to activateApp
- Default: just activate the app

---

## 6. Wails Main Program

### 6.1 app.go

- OnStartup: create Registry with onChange callback, start HTTP server
- onChange: EmitEvent("sessions-updated") to frontend + trigger notify.Show

### 6.2 main.go (Wails v3 entry)

- embed frontend/dist
- SystemTray with icon states
- WebviewWindow: 400x600, frameless, always-on-top, hidden by default
- tray.AttachWindow(window)

### 6.3 service.go (Frontend Bindings)

- SessionService exposed to React frontend
- ListSessions() -> sorted sessions
- JumpToTerminal(sessionKey) -> terminal.Jump
- DismissSession(sessionKey) -> remove/dismiss from registry

---

## 7. Frontend Spec

### 7.1 Tech Stack

- React 18 + TypeScript
- Tailwind CSS (core utilities only)
- zustand (session state)
- Wails v3 bindings (auto-generated)

### 7.2 SessionCard Layout

```
┌─────────────────────────────────────────────────────┐
│ [icon] Agent · cwd-last-segment      Status  Time   │
│ Content (60 chars)                         [Jump]   │
│ [Expand] ContentRaw (click to expand)               │
└─────────────────────────────────────────────────────┘
```

Agent icon colors:
- claude-code -> blue #5B9BD5
- codex -> green #4CAF50
- unknown -> gray #9E9E9E

Status badge colors:
- waiting -> red bg, "等待 Ns" (live counter)
- active -> blue bg, "执行中"
- finished -> gray bg, "已完成"
- error -> orange bg, "错误"

### 7.3 MenuBar Icon States

- Idle (no active sessions) -> gray icon
- Active sessions (non-waiting) -> blue icon
- Waiting sessions -> red icon + badge (count)
- Icon switching done in Go side via tray.SetIcon

---

## 8. CC Hook Configuration

File: `scripts/install-hooks.sh`

All hooks use `async: true`, matcher `*`:
- PreToolUse: `$BRIDGE_PATH pre_tool_use`
- PostToolUse: `$BRIDGE_PATH post_tool_use`
- Stop: `$BRIDGE_PATH stop`

---

## 9. LaunchAgent

File: `scripts/install-launchd.sh`

- Label: com.sapaude.pager
- RunAtLoad + KeepAlive
- Logs: ~/.pager/pager.log, ~/.pager/pager-error.log

---

## 10. Development Milestones

| Milestone | Task | Done When |
|-----------|------|-----------|
| M1 | PoC link verification | All items verified |
| M2 | Wails v3 base framework | Tray icon shows popup panel |
| M3 | Session list UI | Events update list in real-time |
| M4 | Terminal jump | Jump button activates correct terminal tab |
| M5 | System notifications | PreToolUse triggers notification |
| M6 | Auto-dismiss | PostToolUse clears waiting state |
| M7 | Packaging & signing | .app installable on other Macs |
| M8 | LaunchAgent | Auto-start after reboot |

---

## 11. PoC Verification Status

### Verified

- bridge/server compilation (Go 1.22, no deps)
- bridge stdin -> Content extraction
- bridge -> server HTTP POST end-to-end
- /sessions API
- osascript notifications on macOS
- tty detection
- Fault tolerance (server down -> bridge silent exit)

### Needs Mac Verification

- iTerm2 `tty of session` / `unique id of session` AppleScript
- Terminal.app `tty of tab`
- CC real hook stdin field names (use PAGER_DEBUG=1)
- `async: true` support in current CC version
- App Store sandbox: port listening + osascript
