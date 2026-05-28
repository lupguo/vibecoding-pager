# Pager

macOS MenuBar app for AI coding agent status awareness. CC hooks push events -> Pager shows notifications + session list -> user jumps back to terminal tab.

## Commands

```bash
task dev              # Run in dev mode (Wails + Vite hot reload)
task build            # Build production .app
task run              # Run production build
cd frontend && npm install  # Install frontend deps
go build ./cmd/bridge       # Build pager-cc-bridge binary
```

## Architecture

```
CC hook (stdin) -> pager-cc-bridge -> HTTP POST :7421 -> Wails app (Registry) -> UI + Notification
```

- **bridge** (`cmd/bridge/`): Stateless CLI, reads CC hook stdin, posts AgentEvent to server. Must exit 0 always, 1s HTTP timeout.
- **server** (`internal/server/`): HTTP on 127.0.0.1:7421, receives events, feeds Registry.
- **registry** (`internal/registry/`): In-memory session state machine. onChange callback pushes to frontend via Wails Events.
- **notify** (`internal/notify/`): macOS system notifications via osascript.
- **terminal** (`internal/terminal/`): AppleScript-based terminal tab jump (iTerm2, Terminal.app).
- **frontend** (`frontend/`): React + TypeScript + Tailwind + zustand. Wails v3 bindings auto-generated.
- **service** (`service.go`): Go bindings exposed to frontend (ListSessions, JumpToTerminal, DismissSession).

## Tech Stack

- Go 1.25, Wails v3 (alpha.96)
- React 18, TypeScript, Tailwind CSS, zustand
- macOS only (osascript, SystemTray, LaunchAgent)

## Non-Negotiable Decisions

These are final. Do not suggest alternatives:

1. Hook is fire-and-forget -- no response, no blocking the agent
2. No approve/deny decisions inside Pager
3. Single-process architecture (Daemon + UI in one Wails process)
4. AgentEvent is the only cross-layer data structure

## Session State Machine

```
pre_tool_use  -> Status=waiting (add to PendingTools[tool_use_id])
post_tool_use -> Remove from PendingTools; if empty -> Status=active
stop          -> Status=finished
error         -> Status=error
```

Session key: `host:cwd:tty` (triple uniquely identifies a session).

## Code Conventions

- Go: standard library preferred; errors in bridge must be silent (exit 0)
- Frontend: Tailwind utility classes only, no plugins; zustand for state
- Wails bindings in `frontend/src/bindings/` are auto-generated -- never edit manually
- Content strings truncated to 60 runes for display; full version in ContentRaw
- Chinese UI labels (e.g., "等待确认", "任务完成", "执行中")

## Design Workflow

- UI layout design defaults to browser-based mockup for visual confirmation before implementation
- Use Chrome browser to present design mockups for user review; proceed to coding only after approval

## v1.0 Scope Boundaries

Do NOT implement these:

- Hook return values affecting CC behavior (approve/deny/defer)
- Text replies to CC from within Pager
- Multi-machine unified view
- Codex/other agent bridge (fields reserved, not wired)
- SQLite persistence (in-memory Registry is sufficient)
- WebSocket (Wails Events handles Go -> UI push)
- Linux/Windows support

## Key Files

| Path                                      | Purpose                            |
|-------------------------------------------|------------------------------------|
| `cmd/bridge/main.go`                      | pager-cc-bridge entry point        |
| `internal/event/types.go`                 | AgentEvent struct (project-wide)   |
| `internal/registry/registry.go`           | Session state machine              |
| `internal/server/server.go`               | HTTP server (:7421)                |
| `internal/notify/notify.go`               | macOS notifications                |
| `internal/terminal/jump.go`               | Terminal tab jump                  |
| `app.go`                                  | Wails app lifecycle                |
| `main.go`                                 | Wails v3 entry + tray setup        |
| `service.go`                              | SessionService (frontend bindings) |
| `frontend/src/store/sessions.ts`          | zustand session store              |
| `frontend/src/components/SessionCard.tsx` | Session card UI                    |
| `scripts/install-hooks.sh`                | Install CC hooks                   |
| `scripts/install-launchd.sh`              | Register LaunchAgent               |

## References

- Full technical PRD with implementation details: `docs/PRD.md`
- Wails v3 docs: `docs/wails/`
