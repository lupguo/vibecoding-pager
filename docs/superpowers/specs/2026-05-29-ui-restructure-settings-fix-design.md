# Pager v1.1 — UI Restructure & Settings Fix

**Date:** 2026-05-29
**Scope:** Popup UI overhaul, settings bug fixes, window behavior improvements

---

## Summary

Restructure the Popup window from a tab-filtered view (Attention/Running/Done) to a flat project-grouped list with inline status tags. Fix settings panel issues (Cmd+, hotkey, opacity not applying). Add window management features (pin-to-top, drag-to-move, horizontal resize).

---

## 1. Popup UI Restructure

### 1.1 Navigation Bar (Header)

| Position | Element | Behavior |
|----------|---------|----------|
| Left | Pin button (Lucide `Pin` icon) | Toggle always-on-top. Unpinned: -45° rotation, gray. Pinned: 0° rotation, blue highlight with subtle background |
| Center | "Pager" title text | Static, 12px semibold |
| Right | Settings gear (Lucide `Settings` icon) | Click → open Settings window |

- Entire navbar is a drag region (`-webkit-app-region: drag`)
- Pin button and gear icon are `no-drag` (clickable)
- Remove the existing `FilterDot` component and filter state entirely

### 1.2 Session List

**Grouping:**
- Sessions grouped by **project** (last path segment of `CWD`)
- Each group has a header: folder icon + project name + divider line
- Groups ordered by most recent activity (newest group first)
- Within each group, sessions sorted by `UpdatedAt` descending

**Filtering:**
- No tab-based filtering
- Done sessions older than 30 minutes are hidden from the list (frontend filter in zustand store)
- Empty groups (all sessions filtered out) are not rendered

**Empty state:**
- When no sessions are visible, show centered empty state: clock icon + "No active sessions" text

### 1.3 Session Card Layout

**Row 1 (status bar):**
```
[●dot] [STATUS tag] [Agent badge]          [session_id] [time]
 ←————— left-aligned ——————→                ←— right-aligned —→
```

- `●dot`: 6px circle, color by attention level (red=attention, green=active, gray=done). Attention dot pulses.
- `STATUS tag`: 9px bold label — `WAITING` / `ACTIVE` / `DONE`. Color-coded background pill.
- `Agent badge`: 9px bold — `CC` / `CC-INT` / etc. Purple background pill.
- `session_id`: 8-char prefix, monospace, 9px, faint.
- `time`: relative time string (5s / 2m / 1h), 9px, faint.

**Row 2 (content):**
```
    [tool_tag] [content_text ...]           [→ jump button]
```

- Indented 12px from left (align under status tag)
- `tool_tag`: monospace blue pill showing tool name (max 12 chars)
- `content_text`: 11px, ellipsis overflow
- `jump button`: arrow icon, hover → blue, click → `JumpToTerminal(session.Key)`

**Card styling by state:**
- Attention: red-tinted background, red border
- Active: green-tinted background, green border
- Done: neutral background, faint border, 65% opacity

### 1.4 Window Behavior

| Feature | Implementation |
|---------|---------------|
| Pin (always-on-top) | Toggle `window.SetAlwaysOnTop(bool)` via Wails binding. When pinned: disable `HideOnFocusLost`. Persist state to `popup_pinned` config. |
| Drag | Navbar region with `-webkit-app-region: drag`. Wails frameless window handles native drag. |
| Horizontal resize | `DisableResize: false`. Constrain via `MinWidth: 300, MaxWidth: 600, MinHeight: 520, MaxHeight: 520` (locking vertical). Save width to `popup_width` config on resize end. |
| Height | Locked at 520px via equal MinHeight/MaxHeight. Internal scroll on `.session-list`. |
| Position (pinned) | When pinned + user drags, remember position. When unpinned, revert to tray-attached position. |

### 1.5 Icon Library

- Add `lucide-react` dependency
- Replace all hand-written inline SVGs with Lucide components
- Icons used: `Pin`, `Settings`, `Folder`, `ArrowRight`, `Clock` (empty state)

---

## 2. Settings Fixes

### 2.1 Remove Cmd+, Accelerator

**Problem:** `ActivationPolicyAccessory` prevents the app from becoming frontmost, so menu accelerators never fire.

**Fix:**
- Remove `.SetAccelerator("CmdOrCtrl+,")` from the tray menu "偏好设置..." item
- Settings accessible via: tray right-click menu item / Popup gear icon
- No global hotkey for settings (KISS principle)

### 2.2 Fix Opacity Not Applying to Popup

**Problem:** `applyOpacity()` modifies CSS variable on the Settings window's `document`, but Popup is a separate window with its own `document`.

**Fix — Wails Event broadcast:**

1. `SettingsBinding.UpdateSettings()` (Go) → after saving config, emit:
   ```go
   application.Get().Event.Emit("settings-changed", cfg)
   ```

2. Popup `main.tsx` → on init, listen for the event:
   ```typescript
   Events.On("settings-changed", (ev) => {
     const cfg = ev?.data ?? ev
     applyOpacity(cfg.opacity)
     applyTheme(cfg.theme)
     i18n.changeLanguage(cfg.language)
   })
   ```

3. Settings window continues to apply changes locally (immediate feedback).

### 2.3 Real-time Settings Effect Matrix

| Setting | Effect | Mechanism |
|---------|--------|-----------|
| Language | Instant | `i18n.changeLanguage()` — already works in both windows via event |
| Theme | Instant | CSS class toggle — broadcast via `settings-changed` event |
| Opacity | Instant | CSS variable `--pager-opacity` — broadcast via `settings-changed` event |
| Hotkey | Instant | `onChange` callback → `RegisterHotkey()` re-registration (already works) |
| Notification level | Instant | Read from config on each notification (already works) |
| Popup width | Instant | Applied on drag; saved to config on mouseup |
| Popup pinned | Instant | Window property toggle; saved to config |

**No settings require app restart.**

---

## 3. New Config Fields

```go
type Settings struct {
    Language          string `json:"language"`
    Theme             string `json:"theme"`
    Opacity           int    `json:"opacity"`            // 30-100, default 75
    HotkeyToggle      string `json:"hotkey_toggle"`     // default "Alt+E"
    NotificationLevel string `json:"notification_level"` // "all" | "attention_only"
    PopupWidth        int    `json:"popup_width"`       // 300-600, default 380
    PopupPinned       bool   `json:"popup_pinned"`      // default false
}
```

---

## 4. Files Changed

| File | Change |
|------|--------|
| `frontend/package.json` | Add `lucide-react` dependency |
| `frontend/src/App.tsx` | Complete rewrite — remove FilterDot, new navbar with pin/gear, grouped list |
| `frontend/src/components/SessionList.tsx` | Rewrite — project grouping logic, 30min done filter |
| `frontend/src/components/SessionCard.tsx` | Rewrite — new row layout (dot → status → agent ... id → time) |
| `frontend/src/store/sessions.ts` | Add grouping selector, remove `filter` state, add done-age filter |
| `frontend/src/store/settings.ts` | Add `settings-changed` event listener for cross-window sync |
| `frontend/src/main.tsx` | Register `settings-changed` event listener on popup init |
| `internal/infra/config/config.go` | Add `PopupWidth`, `PopupPinned` fields with defaults |
| `internal/wails/app.go` | Remove `SetAccelerator`; adjust window options (MinWidth/MaxWidth, DisableResize:false); add pin binding |
| `internal/wails/settings_svc.go` | Emit `settings-changed` event after save |
| `internal/wails/window_svc.go` | **New file** — `WindowBinding` with `SetPinned(bool)`, `SetPopupWidth(int)`, `OpenSettings()` bindings. Holds `*application.WebviewWindow` ref. |

---

## 5. Future Considerations (Out of Scope)

### SQLite Persistence (v1.5)

When SQLite is introduced:
- **Write strategy:** Memory-first → async flush to SQLite (WAL mode)
- **Read strategy:** Startup loads SQLite → Registry; runtime reads from memory only
- **Cleanup:** Done sessions > N days purged from SQLite via periodic cleanup
- **Migration:** Current in-memory Registry interface unchanged; SQLite adapter implements same interface behind it

### Other Deferred Items
- Popup vertical resize (height remains fixed for now)
- Keyboard navigation within session list
- Session detail expansion panel redesign

---

## 6. Acceptance Criteria

1. Popup shows all sessions in a single list grouped by project, no tab filtering
2. Card row 1 layout: dot → status tag → agent badge ... session_id → time
3. Pin button toggles always-on-top and persists state
4. Navbar drag moves window, gear opens settings
5. Horizontal resize works (300-600px) with persistence
6. Opacity slider in settings immediately affects popup window appearance
7. Cmd+, removed from tray menu accelerator; settings accessible via gear + right-click
8. Done sessions >30min hidden from list
9. All inline SVGs replaced with Lucide React components
10. All settings changes take effect immediately without restart
