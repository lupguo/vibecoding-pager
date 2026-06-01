# Pager

macOS MenuBar app for AI coding agent status awareness. CC hooks push events -> Pager shows notifications + session list -> user jumps back to terminal tab.

## Commands

```bash
make dev              # Run in dev mode (Wails + Vite hot reload)
make build            # Build production .app
make run              # Build and run
make bridge           # Build pager-cc-bridge binary
make frontend-deps    # Install frontend deps
make bindings         # Regenerate Wails bindings (after changing Go services)
make test             # Run all Go tests
make lint             # Go vet + TypeScript check
make help             # Show all available targets
```

## Architecture

> Full directory convention: see `ARCHITECTURE.md`

**Data flow:**
```
CC hook (stdin) -> pager-cc-bridge -> HTTP POST :7421 -> Wails app (Registry) -> UI + Notification
```

**Four-layer structure (`internal/`):**

| Layer | Path | Responsibility |
|-------|------|----------------|
| Domain | `internal/domain/entity/` | AgentEvent struct + shared constants |
| Domain | `internal/domain/session/` | Registry state machine |
| Adapter | `internal/adapter/httpapi/` | HTTP server receiving bridge events |
| Adapter | `internal/adapter/notify/` | macOS system notifications |
| Adapter | `internal/adapter/terminal/` | Terminal tab jump (iTerm2, Terminal.app) |
| Adapter | `internal/adapter/bridge/` | CC hook event parsing (shared with cmd/bridge) |
| Infra | `internal/infra/log/` | slog structured logging |
| Infra | `internal/infra/config/` | JSON settings persistence |
| Infra | `internal/infra/store/` | Storage interface (v1.5: SQLite) |
| Wails | `internal/wails/` | Framework bindings, lifecycle, hotkey, tray |

**Dependency rule:** domain ← adapter ← wails; infra used by all.

## Tech Stack

- Go 1.25, Wails v3 (alpha.96), log/slog
- React 18, TypeScript, Tailwind CSS, zustand, i18next
- macOS only (osascript, SystemTray, LaunchAgent)
- `golang.design/x/hotkey` for global shortcuts

## Non-Negotiable Decisions

These are final. Do not suggest alternatives:

1. Hook is fire-and-forget -- no response, no blocking the agent
2. No approve/deny decisions inside Pager
3. Single-process architecture (Daemon + UI in one Wails process)
4. AgentEvent is the only cross-layer data structure
5. Four-layer architecture (domain/adapter/infra/wails) -- see ARCHITECTURE.md

## Session State Machine

`Session.Status` is one of four mutually-exclusive `entity.SessionStatus` values:
`StatusWorking`, `StatusWaiting`, `StatusDone`, `StatusError`. The mapping from
incoming `(EventType, ToolName, PermissionMode, hasPendingAskUser)` to a status
lives in **one place only**: `session.DeriveStatus` in
`internal/domain/session/status.go`. Do not duplicate this logic anywhere else
— the tracker, SQLite replay, notify predicate, and tray icon all delegate to
the same function.

Summary of the canonical mapping (see `DeriveStatus` for the full table):

| Event                                              | Status                       |
|----------------------------------------------------|------------------------------|
| `PreToolUse` (AskUserQuestion or non-bypass perms) | `waiting`                    |
| `PreToolUse` (other tool + `bypassPermissions`)    | `working`                    |
| `PostToolUse`, `SessionStart`, default             | `working`                    |
| `Stop`/`SessionEnd`/`SubagentStop`                 | `done` (or `waiting` if AskUser still pending) |
| `PermissionRequest`/`PermissionDenied`/`Notification`/`Elicitation`/`PostToolUseFailure` | `waiting` |
| `StopFailure`/`Error`                              | `error`                      |

Session key: CC `session_id` (preferred) or `host:cwd:tty` triple (fallback).

## Code Conventions

- Go: standard library preferred; errors in bridge must be silent (exit 0)
- Logging: use `log/slog` with module attribute (via `infra/log.Module("name")`)
- Naming: Wails bindings = `XxxBinding`, managers = `XxxManager`, files = `*_svc.go`
- Frontend: Tailwind utility classes only; zustand for state; i18next for i18n
- Wails bindings in `frontend/bindings/` are auto-generated -- never edit manually
- Content strings truncated to 60 runes for display; full version in ContentRaw

## Design Workflow

- UI layout design defaults to browser-based mockup for visual confirmation before implementation
- Use Chrome browser to present design mockups for user review; proceed to coding only after approval

## v1.0 Scope Boundaries

Do NOT implement these:

- Hook return values affecting CC behavior (approve/deny/defer)
- Text replies to CC from within Pager
- Multi-machine unified view
- Codex/other agent bridge (fields reserved, not wired)
- SQLite persistence (in-memory Registry is sufficient for v1)
- WebSocket (Wails Events handles Go -> UI push)
- Linux/Windows support

## Key Files

| Path | Purpose |
|------|---------|
| `main.go` | Entry point (~20 lines: init log + NewPagerApp + Run) |
| `internal/wails/app.go` | PagerApp assembly + lifecycle |
| `internal/wails/session_svc.go` | SessionBinding (frontend ops) |
| `internal/wails/settings_svc.go` | SettingsBinding (frontend settings) |
| `internal/wails/hotkey.go` | Global hotkey registration |
| `internal/domain/entity/event.go` | AgentEvent struct + constants |
| `internal/domain/session/registry.go` | Session state machine |
| `internal/adapter/httpapi/server.go` | HTTP server (:7421) |
| `internal/adapter/notify/notify.go` | macOS notifications |
| `internal/adapter/terminal/jump.go` | Terminal tab jump |
| `internal/infra/config/config.go` | Settings persistence |
| `internal/infra/log/log.go` | slog module logger |
| `cmd/bridge/main.go` | pager-cc-bridge CLI |
| `frontend/src/store/sessions.ts` | zustand session store |
| `frontend/src/pages/SettingsPanel.tsx` | Settings panel UI |

## References

- Directory convention: `ARCHITECTURE.md`
- Full technical PRD: `docs/PRD.md`
- Wails v3 docs: `docs/wails/`
