# VibeCoding Pager

macOS MenuBar app for AI coding agent status awareness. CC hooks push events -> Pager shows notifications + session list -> user jumps back to terminal tab.

## Commands

```bash
make dev              # Run in dev mode (Wails + Vite hot reload)
make build            # Build production .app
make run              # Build and run
make bridge           # Build pager-bridge binary
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
CC / CC-Internal / CodeBuddy / Codex hook (stdin JSON)
        │
        ▼
  pager-bridge --agent <Label> --event <X>   (CLI binary, must exit 0 silently)
        │
        ▼  HTTP POST :7421
   VibeCoding Pager.app
   ├── Registry              (in-memory state machine)
   ├── SQLite EventStore     (~/Library/Application Support/VibeCoding Pager/pager.db)
   ├── macOS Notification    (osascript)
   └── React UI              (Wails v3 + zustand)
```

**Four-layer structure (`internal/`):**

| Layer | Path | Responsibility |
|-------|------|----------------|
| Domain | `internal/domain/entity/` | AgentEvent struct + shared constants |
| Domain | `internal/domain/session/` | Registry state machine |
| Adapter | `internal/adapter/httpapi/` | HTTP server receiving bridge events |
| Adapter | `internal/adapter/notify/` | macOS system notifications |
| Adapter | `internal/adapter/terminal/` | Terminal tab jump (iTerm2, Terminal.app) |
| Adapter | `internal/adapter/bridge/` | Hook event parsing + DSL render (shared by all 4 agents via `agents/` registry) |
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

## Bridge Binary Lifecycle

After modifying `internal/adapter/bridge/*`, `cmd/pager-bridge/*`, or
`cmd/pager-installhooks/*`, run:

```bash
make install-bridge
```

This rebuilds both binaries and runs the Go-based installer
(`bin/pager-installhooks`) which writes hooks into:

- `~/.claude/settings.json`           (CC, Anthropic Claude Code)
- `~/.claude-internal/settings.json`  (CC-Internal, Tencent fork)
- `~/.codebuddy/settings.json`        (CodeBuddy)
- `~/.codex/hooks.json`               (Codex CLI v0.137+)

The installer is **append + idempotent**:
- Re-running never duplicates entries.
- Stale entries pointing at the old `vibecoding-pager-cc-bridge` binary are
  detected by **basename** match and replaced.
- User-defined hooks under the same event name are preserved.

**Why hooks land in `settings.json` and not `settings.local.json`:**
Anthropic CC v2 docs (https://code.claude.com/docs/en/hooks) only define
`settings.local.json` at the **per-project** layer (inside `<repo>/.claude/`,
gitignored). User-level config resolves to `~/.claude/settings.json` —
`~/.claude/settings.local.json` is NOT in the loader chain. CC-Internal v1.1.9
and CodeBuddy forks inherit this. Writing to `settings.json` is the union
path all CC-family agents recognize. The installer's `legacyCleanupTargets`
slice sweeps any orphan `.local.json` entries from earlier installer
versions.

**Bridge binary path is absolute:** the Makefile uses `$(PWD)/bin/pager-bridge`
on purpose. Hooks are spawned by agents from arbitrary working directories;
a relative `command` resolves to non-existent file → silent `exit 127`.

**Why this matters historically:** the 2026-06-03 root-cause investigation of
"TaskCreated / PostToolBatch / SessionEnd / SubagentStart all empty content"
found events were being recorded against a stale bridge binary that predated
commit `1ba099f` (CC field alignment refactor). New rules are inert until the
binary is rebuilt — this is by design (hooks are external processes), and
`make install-bridge` is the discipline that closes the gap.

`make dev` / `make run` / `make build` already depend on `install-bridge`, so
in practice you only need to run it explicitly when iterating on the bridge
without restarting the Pager app.

## Codex Hook Integration

Codex CLI 0.137+ exposes the same `(session, tool, event)` lifecycle as
Claude Code. Pager wires it through `~/.codex/hooks.json`. Two schema
differences from CC are handled by the `agents.Codex` implementation
(`internal/adapter/bridge/agents/codex.go`):

- **`SessionStart` field**: Codex sends `trigger` (`startup` / `resume` /
  `clear` / `compact`) instead of CC's `source`.
- **`exec_command` tool**: all shell-equivalent ops (Bash, file edits via
  `apply_patch`) come through `exec_command` with `tool_input.cmd`. CC uses
  named tools with `tool_input.command`.

**Codex schema gotchas baked into the installer:**

- `~/.codex/hooks.json` uses the **same wrapper** as CC's settings:
  `{"hooks": {<event>: [...]}}` per
  `codex-rs/config/src/hook_config.rs::HooksFile`. (Initial spec
  misread treated it as bare-events at the top level — fixed in `12d8ef7`.)
- Codex 0.137 does **not** support `"async": true` on hook entries
  (`codex-rs/hooks/src/engine/discovery.rs` skips them silently). Pager's
  installer writes sync hooks to Codex while keeping async for the CC family
  (which DO support it). The 5s timeout × ≤50ms HTTP POST means sync hooks
  don't meaningfully block a Codex turn.

**First-run trust prompt:** after `make install-bridge`, the next time you
start a TUI codex session it will prompt:

```
1 hook is new or changed.
Hooks need review
Hooks can run outside the sandbox after you trust them.
[Trust all and continue]  [Continue without trusting (hooks won't run)]
```

Pick **Trust all and continue**. Codex stores a hash of `hooks.json` and
won't prompt again until the file changes. Re-running `make install-bridge`
rewrites the file → hash changes → next session re-prompts.

**Codex `exec` mode does NOT dispatch lifecycle hooks** (only thread / turn /
item events emit). End-to-end hook verification requires an interactive TUI
session. Use `codex app-server` JSON-RPC `hooks/list` to verify Codex sees
the installer's output without needing a full session.

**Codex events covered (10):** PreToolUse, PostToolUse, PermissionRequest,
PreCompact, PostCompact, SessionStart, UserPromptSubmit, SubagentStart,
SubagentStop, Stop. CC family covers 23 events (a superset).

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
| Production (.app)     | Double-click VibeCoding Pager.app               | `~/Library/Application Support/VibeCoding Pager/`     |
| Test / ad-hoc         | `PAGER_CONFIG_DIR=/tmp/X ./bin/vibecoding-pager` | `/tmp/X/`                                  |

Resolution order in `config.BaseDir()`:
1. `PAGER_CONFIG_DIR` env var (highest priority)
2. `~/Library/Application Support/VibeCoding Pager/` (macOS-native default)

`make dev` / `make run` set `PAGER_CONFIG_DIR=$(PWD)/.config/pager` so dev
work never pollutes your installed Pager.app's data. The dev directory
`<repo>/.config/` is gitignored.

`make build` does NOT set the env var — the resulting `.app` ships to end
users who get the macOS-native default automatically.

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
- WebSocket (Wails Events handles Go -> UI push)
- Linux/Windows support

> **Note on SQLite:** Earlier drafts of this section deferred SQLite to a
> later release. As of the 2026-06-01 redesign, SQLite persistence is
> shipped (see `internal/infra/store/sqlite.go` — sessions and events are
> persisted, replayed on startup, soft-deleted on Dismiss). It is part
> of v1.5 — no longer a v1.0 boundary.

> **Note on Codex:** Earlier drafts listed "Codex/other agent bridge (fields
> reserved, not wired)" as a non-goal. As of the 2026-06-08 PR2 (commits
> `9a2f01e..ecd1061`), Codex is fully wired: dedicated `agents.Codex` with
> its own envelope parsing, 10 hook events covered in `extract_rules.yaml`,
> installer writes `~/.codex/hooks.json` with the correct schema. Part of
> v1.5 alongside SQLite — no longer a v1.0 boundary.

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
| `cmd/pager-bridge/main.go` | `pager-bridge` CLI — receives hook stdin, POSTs to :7421 |
| `cmd/pager-installhooks/main.go` | Go-based hook installer (replaces `scripts/install-hooks.sh`) |
| `internal/adapter/bridge/agents/` | Per-agent hook payload parsing (`ClaudeFamily`, `Codex`) |
| `internal/adapter/bridge/extract_rules.yaml` | DSL v3 rules: agent → event → tool → render template |
| `frontend/src/store/sessions.ts` | zustand session store |
| `frontend/src/pages/SettingsPanel.tsx` | Settings panel UI |

## References

- Directory convention: `ARCHITECTURE.md`
- Full technical PRD: `docs/PRD.md`
- Wails v3 docs: `docs/wails/`
