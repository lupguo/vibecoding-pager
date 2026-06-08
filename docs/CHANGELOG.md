# Changelog

All notable changes to VibeCoding Pager are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Removed (2026-06-08, after merge of codex-hook-bridge-refactor)

- **`legacyCleanupTargets`** — dropped. The "legacy cleanup" phase that swept
  `~/.<agent>/settings.local.json` files was based on an incorrect premise
  (those paths were never a valid user-level hook location per Anthropic CC
  v2 docs). Keeping it in the code suggested `.local.json` had once been
  supported. Removed: the variable in `targets.go`, both cleanup loops in
  `cmd/pager-installhooks/main.go`, and the corresponding assertions in
  `TestTargets_Paths`. `removeAllPagerHooks()` itself stays — `--uninstall`
  still uses it.

### Changed (2026-06-08)

- **All log output goes through `infra/log` slog**. `cmd/pager-installhooks`,
  `cmd/pager-bridge` (printHooksJSON error path), and `cmd/testserver` no
  longer use `fmt.Fprintf`/`fmt.Printf` for diagnostic output. `make
  install-bridge` output changed from `[CC] installed 23 events at ...` to
  `time=... level=INFO msg="hooks installed" module=installhooks agent=CC
  events=23 path=...` — same information density, structured for grep/jq,
  consistent with the existing `bridge.poster` / `bridge.cli` modules.
  - `--verbose` flag on installer now bumps slog level to DEBUG.
  - Two intentional `fmt.*` survivors: dry-run JSON output (the user
    pipes/inspects this artifact directly) and HTTP response body in
    `httpapi/server.go` (writes to ResponseWriter, not a log sink).

### Added — Codex agent integration & bridge refactor (2026-06-08)

Branch `feat/codex-hook-bridge-refactor`, 25 commits, merged to `main` on
2026-06-08. Spec: `docs/superpowers/specs/2026-06-05-codex-hook-and-bridge-refactor-design.md`.
Plan: `docs/superpowers/plans/2026-06-05-codex-hook-and-bridge-refactor.md`.

#### New: Codex CLI hook integration

- **`agents.Codex`** (`internal/adapter/bridge/agents/codex.go`) — new `Agent`
  implementation for OpenAI Codex 0.137+. Handles its 10-event lifecycle
  (`PreToolUse / PostToolUse / PermissionRequest / PreCompact / PostCompact /
  SessionStart / UserPromptSubmit / SubagentStart / SubagentStop / Stop`) with
  Codex-specific schema differences from CC: `trigger` field replaces `source`
  on `SessionStart`, extra `turn_id` field, `exec_command` tool replaces named
  shell tools.
- **`extract_rules.yaml` codex section** — DSL render rules for all 10 Codex
  events. `tool_response` is a string for `exec_command` (vs CC's object) so
  `firstline` filter is used instead of `$.tool_response.stdout|firstline`.
- **`agents.Registry`** now serves CC, CC-Internal, CodeBuddy, **Codex**.
- **`PostCompact` + `SubagentStart` event constants** added to
  `internal/domain/entity/event.go`. Status mapping defaults to `working`
  via the existing fallthrough in `session.DeriveStatus`.

#### Changed: bridge architecture

- **Agent abstraction**: replaced flat `CCHookInput`-only path with an `Agent`
  interface (`ID() / ParseEnvelope() / Accept() / Hooks()`) and a
  `bridge.Envelope` struct as the single cross-agent payload type. The
  `internal/adapter/bridge/agents/` package holds per-agent implementations.
- **DSL v3 (`extract_rules.yaml`)**:
  - YAML schema is now `version: 3` with `claude:` / `codex:` namespaces,
    keyed by event name, with `&claude_pre`/`*claude_pre` anchors handled by
    an explicit `AliasNode` resolver in `Rules.LoadRules`.
  - Render pipeline supports chained filters: `{$.path|filter1:arg1|filter2:arg2}`.
  - 4 new filters: `prefix`, `lookup` (k1=v1,k2=v2,default=dv switch table),
    `pluck` (gjson `#.field` array projection), `join`.
  - Single entry point: `bridge.Render(agentID, eventType, toolName, payload)`
    replaces the prior `ExtractContent` + `ExtractEventContent` pair.
- **Binary rename**: `vibecoding-pager-cc-bridge` → `pager-bridge`. Old
  `cmd/bridge/` deleted; new `cmd/pager-bridge/` uses `pflag` for
  `--agent / --event / --print-hooks / --debug`.
- **Go-based installer** at `cmd/pager-installhooks/` replaces
  `scripts/install-hooks.sh`. Append + idempotent: never duplicates entries,
  matches old `vibecoding-pager-cc-bridge` by basename and replaces, preserves
  user-defined hooks under the same event. Runs `pager-bridge --print-hooks`
  to learn the per-agent hook list.
- **Structured logging via `infra/log` slog** (`module=bridge.poster /
  bridge.cli`) replaces ad-hoc `fmt.Fprintf(os.Stderr, …)`. Bridge now also
  surfaces `ParseEnvelope` failures as `WARN` (was completely silent).

#### Build & docs

- `Makefile`: new `installer` target; `install-bridge` builds both binaries
  and runs the installer (`$(INSTALLER_BIN) --bridge $(BRIDGE_BIN) --verbose`)
  for all 4 agents in one shot. Tail hint prints if a stale
  `bin/vibecoding-pager-cc-bridge` is detected.
- `BRIDGE_BIN` / `INSTALLER_BIN` use `$(PWD)/...` absolute paths because the
  paths get embedded into agent hook configs and resolve from arbitrary
  working dirs at hook-fire time.
- Removed: `make install-hooks`, `make install-hooks-cc-internal`,
  `make install-hooks-codebuddy`, `scripts/install-hooks.sh`.
- `CLAUDE.md`, `README.md`, `docs/PRD.md` updated; "Codex/other agent bridge
  reserved" removed from v1.0 boundary list.

#### Fixed (bugs caught during e2e verification)

The 23-task plan landed cleanly through static tests. Five additional root
causes surfaced when actually running real Codex / CC / CodeBuddy sessions
against the new pipeline:

1. **`6516ebe`** — Makefile `BRIDGE_BIN` was relative; embedding
   `bin/pager-bridge` into `~/.codex/hooks.json` caused agents to spawn
   the hook from arbitrary cwds → silent `exit 127`. Fix: `$(PWD)/...`
   prefixes; `cmd/pager-installhooks` also defensively `filepath.Abs`s
   the bridge path before writing.
2. **`12d8ef7`** — Codex `~/.codex/hooks.json` schema misread as bare-events
   at top level; correct shape per `codex-rs/config/src/hook_config.rs::HooksFile`
   is `{"hooks": {<events>}}` (same wrapper as CC's settings.json). Codex 0.137
   also rejects `"async": true` (`codex_hooks::engine::discovery` skips with
   "async hooks are not supported yet"). Fix: drop the synthetic
   "hooks-only" format; per-agent async = `agentLabel != "Codex"`.
3. **`c11694b`** — CC-Internal (Tencent fork v1.1.9) silently ignores
   `~/.claude-internal/settings.local.json`. Hooks looked installed but
   never fired. Fix: write to `settings.json` instead.
4. **`ecd1061`** — Same root cause applies to **all** CC-family agents per
   Anthropic CC v2 docs (https://code.claude.com/docs/en/hooks):
   `settings.local.json` is **per-project only** (in repo `.claude/`,
   gitignored), NOT in the user-level loader chain. CC, CC-Internal,
   CodeBuddy all write to `~/.<agent>/settings.json` now.
   `legacyCleanupTargets` sweeps any orphan `.local.json` entries.
5. **`2248757`** — Bridge silently dropped events on `ParseEnvelope` error
   (e.g. payload with literal newline inside JSON string). Fix: `WARN`-log
   parse failures via slog so the failure mode is visible in agent stderr.

#### Test coverage

Added ≈ 50 unit tests across:

- `internal/adapter/bridge/agents/` — `ClaudeFamily` and `Codex` ID, hooks
  table, ParseEnvelope, Accept, registry lookup
- `internal/adapter/bridge/` — `Rules.Lookup`, `PickTemplate`, `Evaluate`,
  chained filters, 4 new filters, `Render` per agent × event matrix
- `cmd/pager-installhooks/` — fresh / replace-legacy / preserve-user /
  idempotent / basename matching / legacy cleanup / target-paths regression
  guard

End-to-end verified:

- All 4 agents’ hooks land in their respective config files with the correct
  schema (verified for Codex via `codex app-server` JSON-RPC `hooks/list` —
  Codex itself reports 10 hooks loaded, 0 warnings).
- Bridge → HTTP → tracker → SQLite → Wails `sessions-updated` event → React
  store live for CC, CC-Internal, CodeBuddy, Codex.
- Status state machine (`working → waiting → done` plus `error`) walks through
  for each agent's lifecycle.

### Migration

- The legacy binary `bin/vibecoding-pager-cc-bridge` is no longer produced.
  `make install-bridge` will detect a stale copy on disk and print a hint to
  remove it. Existing hook entries pointing at the old binary basename are
  auto-replaced by the installer.
- `~/.<agent>/settings.local.json` no longer holds pager hooks; the installer
  cleans them on next run. User-defined non-pager entries in those files are
  preserved.
- Anyone running `make install-hooks` / `make install-hooks-codebuddy` /
  `make install-hooks-cc-internal` from muscle memory should switch to
  `make install-bridge` (same-shot for all 4 agents).

---

## Earlier history

Earlier changes are tracked in git history; see `git log --since='2026-06-01'`
prior to this branch for the v1.5 SQLite + UI redesign work
(`feature/ui-state-redesign-2026-06-01`).
