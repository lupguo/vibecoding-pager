# UI / Window / Notification Regression Checklist

> Run before shipping any change that touches `internal/wails`, `internal/adapter/notify`,
> `internal/domain/session`, or any file under `frontend/src/`.

**Build:** `make build`
**Launch:** `open bin/Pager.app`

Each row = one click-through. Check ✅ when verified, ❌ if regressed.

## A. Status Visual (4-state)

Send each event via `pager-cc-bridge` (or test bridge harness) and verify card render:

| Step | Event | Expected card |
|---|---|---|
| 1 | `--event PreToolUse --agent CC` with stdin `{"tool_name":"AskUserQuestion","cwd":"/p/x","session_id":"r1"}` | WAITING tag (warm orange #ff9500), HandHelping pulsing icon |
| 2 | `--event PreToolUse` with `{"tool_name":"Edit","permission_mode":"bypassPermissions",...}` | WORKING tag (green), Loader2 spinning |
| 3 | `--event Stop` (same session) | DONE tag (yellow-brown #cc9a00), CheckCircle2 |
| 4 | `--event StopFailure` (new session) | ERROR tag (red #ff453a), AlertTriangle |
| 5 | All four events above | header shows event_type as outline pill (no fill, secondary text color) right after the agent pill — values: `PreToolUse`, `Stop`, `StopFailure` |

## B. Project Grouping

| Step | Action | Expected |
|---|---|---|
| 1 | Click project header | Group collapses / chevron rotates from down to right |
| 2 | Click again | Group expands |
| 3 | Quit + relaunch | Previously collapsed projects still collapsed |
| 4 | Hover project header | BrushCleaning button fades in on the right |
| 5 | Click BrushCleaning (扫帚 icon) | All cards in that project vanish; SQLite `t_sessions.deleted_at` populated |

## C. Expanded Card

| Step | Action | Expected |
|---|---|---|
| 1 | Click any card | Card expands; `<pre>` with content_raw appears as the first block |
| 2 | Look below `<pre>` | A single-row footer shows PATH (FolderOpen icon + value) and SESSION (Hash icon + value), separated from `<pre>` by a dashed top border |
| 3 | Hover the footer | Copy buttons fade in next to PATH and SESSION values |
| 4 | Click Copy on PATH | Icon swaps to Check for ~800ms; clipboard contains the CWD |
| 5 | Verify removed UI | NO "更多 / 收起" button anywhere; NO TIME row; NO TOOL ID / TTY / PERM rows reachable |
| 6 | Click the card again | Card collapses back to the summary row |

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

## I. Headless Data-Path Regression

Run the integration test (no UI involvement):

```bash
go test -tags=integration ./internal/... -run 'TestE2E_' -v
```

Expected: PASS for all 9 sub-tests in `TestE2E_StatusModelDataPath` plus `TestE2E_DismissByProjectFlow`.

After running, manually verify the new columns are populated:

```bash
sqlite3 ~/.config/pager/pager.db "SELECT event_type, cwd, project_name FROM t_events ORDER BY id DESC LIMIT 5"
```

Expected: every row has non-empty `cwd` and `project_name` (project_name = last segment of cwd).

---

If any row fails, file an issue and reference this checklist. Do NOT ship until all
rows pass on a fresh `make build && open` from the target branch.
