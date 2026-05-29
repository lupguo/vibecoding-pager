# Pager v1.1 — UI Restructure & Settings Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the Popup UI from tab-filtered to project-grouped flat list, fix settings cross-window sync, and add window management (pin/drag/resize).

**Architecture:** Go backend adds new config fields + WindowBinding for pin/resize control + settings event broadcast. Frontend replaces the filter-based App with a project-grouped list using Lucide icons, receives cross-window settings events.

**Tech Stack:** Go 1.25, Wails v3 (alpha.96), React 18, TypeScript, Tailwind, zustand, lucide-react, i18next

---

## File Structure

| File | Role |
|------|------|
| `internal/infra/config/config.go` | Add `PopupWidth` + `PopupPinned` fields |
| `internal/infra/config/config_test.go` | Update tests for new fields |
| `internal/wails/window_svc.go` | **New** — `WindowBinding` with `SetPinned`, `SetPopupWidth`, `OpenSettings` |
| `internal/wails/app.go` | Register WindowBinding, adjust popup window options, remove accelerator |
| `internal/wails/settings_svc.go` | Emit `settings-changed` event after save |
| `frontend/package.json` | Add `lucide-react` |
| `frontend/src/store/sessions.ts` | Remove filter state, add project grouping + done-age filter |
| `frontend/src/store/settings.ts` | Add cross-window `settings-changed` event listener |
| `frontend/src/App.tsx` | Complete rewrite — navbar (pin/title/gear) + grouped session list |
| `frontend/src/components/SessionList.tsx` | Rewrite — render project groups |
| `frontend/src/components/SessionCard.tsx` | Rewrite — new row layout |
| `frontend/src/components/EmptyState.tsx` | **New** — empty state component |
| `frontend/src/main.tsx` | Register settings-changed listener on popup window |

---

### Task 1: Go Backend — Config, WindowBinding, Settings Event

**Files:**
- Modify: `internal/infra/config/config.go`
- Modify: `internal/infra/config/config_test.go`
- Create: `internal/wails/window_svc.go`
- Modify: `internal/wails/settings_svc.go`
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Add new config fields and update defaults**

```go
// internal/infra/config/config.go
type Settings struct {
	Language          string `json:"language"`
	Theme             string `json:"theme"`
	Opacity           int    `json:"opacity"`
	HotkeyToggle      string `json:"hotkey_toggle"`
	NotificationLevel string `json:"notification_level"`
	PopupWidth        int    `json:"popup_width"`
	PopupPinned       bool   `json:"popup_pinned"`
}

func Defaults() Settings {
	return Settings{
		Language:          "zh",
		Theme:             "system",
		Opacity:           75,
		HotkeyToggle:     "Alt+E",
		NotificationLevel: "attention_only",
		PopupWidth:        380,
		PopupPinned:       false,
	}
}
```

- [ ] **Step 2: Update config tests for new fields**

```go
// internal/infra/config/config_test.go — add to TestLoadReturnsDefaultsWhenFileNotExists
if cfg.PopupWidth != 380 {
    t.Errorf("expected default popup_width 380, got %d", cfg.PopupWidth)
}
if cfg.PopupPinned != false {
    t.Errorf("expected default popup_pinned false, got %v", cfg.PopupPinned)
}

// Update TestSaveAndLoad to include new fields in the test struct
cfg := Settings{
    Language:          "en",
    Theme:             "dark",
    Opacity:           50,
    HotkeyToggle:     "Ctrl+Shift+P",
    NotificationLevel: "all",
    PopupWidth:        450,
    PopupPinned:       true,
}
```

- [ ] **Step 3: Run tests**

Run: `cd /private/data/projects/github.com/sapaude/pager && go test ./internal/infra/config/ -v`
Expected: All PASS

- [ ] **Step 4: Create WindowBinding**

```go
// internal/wails/window_svc.go
package wails

import (
	"pager/internal/infra/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowBinding exposes popup window controls to the frontend.
type WindowBinding struct {
	popup          *application.WebviewWindow
	settings       *application.WebviewWindow
	configPath     string
}

func NewWindowBinding(popup, settings *application.WebviewWindow) *WindowBinding {
	return &WindowBinding{
		popup:      popup,
		settings:   settings,
		configPath: config.DefaultPath(),
	}
}

// SetPinned toggles always-on-top and HideOnFocusLost.
func (w *WindowBinding) SetPinned(pinned bool) error {
	w.popup.SetAlwaysOnTop(pinned)
	// When pinned, don't hide on focus lost
	// When unpinned, re-enable hide on focus lost
	// Note: Wails v3 may not expose HideOnFocusLost toggle at runtime;
	// if not available, we handle via frontend focus event instead.

	cfg, _ := config.LoadFrom(w.configPath)
	cfg.PopupPinned = pinned
	return config.SaveTo(w.configPath, cfg)
}

// SetPopupWidth persists the user-adjusted width.
func (w *WindowBinding) SetPopupWidth(width int) error {
	if width < 300 {
		width = 300
	}
	if width > 600 {
		width = 600
	}
	cfg, _ := config.LoadFrom(w.configPath)
	cfg.PopupWidth = width
	return config.SaveTo(w.configPath, cfg)
}

// OpenSettings shows the settings window.
func (w *WindowBinding) OpenSettings() {
	w.settings.Show()
	w.settings.Focus()
}
```

- [ ] **Step 5: Update SettingsBinding to emit event**

```go
// internal/wails/settings_svc.go — UpdateSettings method
func (s *SettingsBinding) UpdateSettings(cfg config.Settings) error {
	if err := config.SaveTo(s.path, cfg); err != nil {
		return err
	}
	// Broadcast to all windows (popup + settings) for cross-window sync
	if app := application.Get(); app != nil {
		app.Event.Emit("settings-changed", cfg)
	}
	if s.onChange != nil {
		s.onChange(cfg)
	}
	return nil
}
```

- [ ] **Step 6: Update app.go — register WindowBinding, adjust popup options, remove accelerator**

Key changes to `internal/wails/app.go`:

1. Popup window options:
```go
popupWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
    Title:            "Pager",
    Name:             "pager-panel",
    Width:            initialCfg.PopupWidth, // from config
    Height:           520,
    MinWidth:         300,
    MaxWidth:         600,
    MinHeight:        520,
    MaxHeight:        520,
    Hidden:           true,
    Frameless:        true,
    AlwaysOnTop:      initialCfg.PopupPinned, // from config
    DisableResize:    false,
    HideOnFocusLost:  !initialCfg.PopupPinned, // respect pin state
    BackgroundColour: application.NewRGBA(0, 0, 0, 0),
})
```

2. Register WindowBinding:
```go
windowBinding := NewWindowBinding(popupWindow, settingsWindow)

// In application.Options.Services:
Services: []application.Service{
    application.NewService(p),
    application.NewService(sessionBinding),
    application.NewService(settingsBinding),
    application.NewService(windowBinding),
},
```

3. Remove accelerator from tray menu:
```go
trayMenu.Add("偏好设置...").
    OnClick(func(_ *application.Context) {
        settingsWindow.Show()
        settingsWindow.Focus()
    })
```

4. Load initial config before window creation:
```go
initialCfg, _ := config.LoadFrom(config.DefaultPath())
```

- [ ] **Step 7: Run full Go test suite and verify build**

Run: `cd /private/data/projects/github.com/sapaude/pager && go test ./... && go build .`
Expected: All tests PASS, binary compiles

- [ ] **Step 8: Commit**

```bash
git add internal/infra/config/ internal/wails/
git commit -m "feat(backend): add WindowBinding, config fields, settings event broadcast"
```

---

### Task 2: Frontend Infrastructure — Lucide, Store Refactoring, Cross-window Sync

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/src/store/sessions.ts`
- Modify: `frontend/src/store/settings.ts`
- Modify: `frontend/src/main.tsx`

- [ ] **Step 1: Install lucide-react**

Run: `cd /private/data/projects/github.com/sapaude/pager/frontend && npm install lucide-react`

- [ ] **Step 2: Rewrite sessions store — remove filter, add grouping + done filter**

```typescript
// frontend/src/store/sessions.ts
import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { ListSessions } from '../../bindings/pager/internal/wails/sessionbinding.js'

export type AttentionLevel = 'attention' | 'running' | 'done'

export interface AgentEvent {
  agent: string
  host: string
  cwd: string
  tty: string
  session_id: string
  term_program: string
  iterm_session_id?: string
  event_type: string
  tool_name: string
  tool_use_id: string
  content: string
  content_raw: string
  attention_level: AttentionLevel
  agent_label: string
  permission_mode?: string
  timestamp: string
}

export interface Session {
  Key: string
  Agent: string
  Host: string
  CWD: string
  TTY: string
  TermProgram: string
  ITermSessionID: string
  Status: string
  AttentionLevel: AttentionLevel
  AgentLabel: string
  SessionID: string
  LastEvent: AgentEvent | null
  PendingTools: Record<string, AgentEvent | null>
  UpdatedAt: string
}

export interface ProjectGroup {
  project: string
  sessions: Session[]
}

interface SessionStore {
  sessions: Session[]
  setSessions: (sessions: Session[]) => void
}

const DONE_TIMEOUT_MS = 30 * 60 * 1000 // 30 minutes

export const useSessionStore = create<SessionStore>((set) => ({
  sessions: [],
  setSessions: (sessions) =>
    set({
      sessions: [...sessions].sort(
        (a, b) => new Date(b.UpdatedAt).getTime() - new Date(a.UpdatedAt).getTime()
      ),
    }),
}))

/** Filter out done sessions older than 30 minutes */
function filterExpiredDone(sessions: Session[]): Session[] {
  const now = Date.now()
  return sessions.filter((s) => {
    if (s.AttentionLevel !== 'done') return true
    const updatedAt = new Date(s.UpdatedAt).getTime()
    return now - updatedAt < DONE_TIMEOUT_MS
  })
}

/** Extract project name from CWD (last path segment) */
function projectFromCWD(cwd: string): string {
  const segments = cwd.split('/').filter(Boolean)
  return segments[segments.length - 1] || cwd
}

/** Group sessions by project, ordered by most recent activity */
export function useProjectGroups(): ProjectGroup[] {
  const sessions = useSessionStore((s) => s.sessions)
  const visible = filterExpiredDone(sessions)

  const groupMap = new Map<string, Session[]>()
  for (const s of visible) {
    const project = projectFromCWD(s.CWD)
    const group = groupMap.get(project) || []
    group.push(s)
    groupMap.set(project, group)
  }

  // Sort groups by most recent session in each group
  const groups: ProjectGroup[] = Array.from(groupMap.entries()).map(([project, sessions]) => ({
    project,
    sessions, // already sorted by UpdatedAt from store
  }))

  groups.sort((a, b) => {
    const aTime = new Date(a.sessions[0]?.UpdatedAt || 0).getTime()
    const bTime = new Date(b.sessions[0]?.UpdatedAt || 0).getTime()
    return bTime - aTime
  })

  return groups
}

export function initSessionSync() {
  ListSessions()
    .then((sessions: any) => {
      const valid = (sessions ?? []).filter((s: any) => s !== null)
      if (valid.length > 0) {
        useSessionStore.getState().setSessions(valid)
      }
    })
    .catch((err: unknown) => {
      console.warn('[pager] ListSessions failed:', err)
    })

  Events.On('sessions-updated', (ev: any) => {
    const sessions = ev?.data ?? ev ?? []
    if (!Array.isArray(sessions)) return
    queueMicrotask(() => {
      useSessionStore.getState().setSessions(sessions)
    })
  })
}
```

- [ ] **Step 3: Update settings store — add cross-window event listener**

```typescript
// frontend/src/store/settings.ts
import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import i18n from '../i18n'

export interface Settings {
  language: string
  theme: string
  opacity: number
  hotkey_toggle: string
  notification_level: string
  popup_width: number
  popup_pinned: boolean
}

const DEFAULT_SETTINGS: Settings = {
  language: 'zh',
  theme: 'system',
  opacity: 75,
  hotkey_toggle: 'Alt+E',
  notification_level: 'attention_only',
  popup_width: 380,
  popup_pinned: false,
}

interface SettingsStore {
  settings: Settings
  loaded: boolean
  loadSettings: () => Promise<void>
  updateSettings: (partial: Partial<Settings>) => Promise<void>
}

export const useSettingsStore = create<SettingsStore>((set, get) => ({
  settings: DEFAULT_SETTINGS,
  loaded: false,

  loadSettings: async () => {
    try {
      const { GetSettings } = await import('../../bindings/pager/internal/wails/settingsbinding.js')
      const cfg = await GetSettings()
      set({ settings: cfg, loaded: true })
      i18n.changeLanguage(cfg.language)
      applyTheme(cfg.theme)
      applyOpacity(cfg.opacity)
    } catch (err) {
      console.warn('[pager] GetSettings failed, using defaults:', err)
      set({ loaded: true })
    }
  },

  updateSettings: async (partial) => {
    const current = get().settings
    const updated = { ...current, ...partial }
    set({ settings: updated })

    // Apply locally for immediate feedback
    if (partial.language) i18n.changeLanguage(partial.language)
    if (partial.theme) applyTheme(partial.theme)
    if (partial.opacity !== undefined) applyOpacity(partial.opacity)

    try {
      const { UpdateSettings } = await import('../../bindings/pager/internal/wails/settingsbinding.js')
      await UpdateSettings(updated)
    } catch (err) {
      console.error('[pager] UpdateSettings failed:', err)
    }
  },
}))

export function applyTheme(theme: string) {
  const root = document.documentElement
  root.classList.remove('light', 'dark')
  if (theme === 'light' || theme === 'dark') {
    root.classList.add(theme)
  }
}

export function applyOpacity(opacity: number) {
  document.documentElement.style.setProperty('--pager-opacity', String(opacity / 100))
}

export function initSettings() {
  useSettingsStore.getState().loadSettings()

  // Cross-window sync: when settings change in another window, apply here too
  Events.On('settings-changed', (ev: any) => {
    const cfg = ev?.data ?? ev
    if (!cfg) return
    useSettingsStore.setState({ settings: cfg })
    i18n.changeLanguage(cfg.language)
    applyTheme(cfg.theme)
    applyOpacity(cfg.opacity)
  })
}
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /private/data/projects/github.com/sapaude/pager/frontend && npx tsc --noEmit`
Expected: No errors (bindings may need regeneration — run `make bindings` first if needed)

- [ ] **Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/store/
git commit -m "feat(frontend): add lucide-react, refactor stores for project grouping and cross-window sync"
```

---

### Task 3: Popup UI Components Rewrite

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/SessionList.tsx`
- Modify: `frontend/src/components/SessionCard.tsx`
- Create: `frontend/src/components/EmptyState.tsx`

- [ ] **Step 1: Create EmptyState component**

```typescript
// frontend/src/components/EmptyState.tsx
import { Clock } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function EmptyState() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col items-center justify-center h-64 text-[--pager-text-faint] gap-3">
      <Clock size={32} strokeWidth={1.5} />
      <span className="text-[13px]">{t('session.empty', 'No active sessions')}</span>
    </div>
  )
}
```

- [ ] **Step 2: Rewrite SessionCard with new layout**

```typescript
// frontend/src/components/SessionCard.tsx
import { useState } from 'react'
import { ArrowRight } from 'lucide-react'
import type { Session } from '../store/sessions'
import { JumpToTerminal } from '../../bindings/pager/internal/wails/sessionbinding.js'

interface Props {
  session: Session
}

export default function SessionCard({ session }: Props) {
  const [jumping, setJumping] = useState(false)

  const content = session.LastEvent?.content ?? session.Status
  const contentRaw = session.LastEvent?.content_raw ?? ''
  const toolName = session.LastEvent?.tool_name ?? ''
  const sessionPrefix = (session.SessionID || session.Key || '').slice(0, 8)
  const agentLabel = session.AgentLabel || 'CC'
  const isAttention = session.AttentionLevel === 'attention'
  const isRunning = session.AttentionLevel === 'running'
  const [expanded, setExpanded] = useState(false)

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

  const cardStyles = isAttention
    ? 'bg-[--pager-card-attention-bg] border-[--pager-card-attention-border] shadow-sm'
    : isRunning
    ? 'bg-[--pager-card-running-bg] border-[--pager-card-running-border]'
    : 'bg-[--pager-card-done-bg] border-[--pager-card-done-border] opacity-65'

  const statusDot = isAttention
    ? 'bg-[--pager-red] animate-pulse-status'
    : isRunning
    ? 'bg-[--pager-green]'
    : 'bg-[--pager-gray-dot]'

  const statusTag = isAttention
    ? { text: 'WAITING', cls: 'bg-[rgba(255,69,58,0.15)] text-[--pager-red]' }
    : isRunning
    ? { text: 'ACTIVE', cls: 'bg-[rgba(48,209,88,0.12)] text-[--pager-green]' }
    : { text: 'DONE', cls: 'bg-[rgba(255,255,255,0.06)] text-[--pager-text-muted]' }

  return (
    <div
      className={`rounded-lg border cursor-pointer transition-all duration-150 hover:shadow-sm ${cardStyles}`}
      onClick={() => setExpanded(!expanded)}
    >
      <div className="px-[10px] py-[8px]">
        {/* Row 1: dot → status tag → agent badge ... session_id → time */}
        <div className="flex items-center gap-[6px] mb-[4px]">
          <div className="flex items-center gap-[6px]">
            <span className={`w-[6px] h-[6px] rounded-full shrink-0 ${statusDot}`} />
            <span className={`text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] ${statusTag.cls} tracking-wide`}>
              {statusTag.text}
            </span>
            <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide uppercase">
              {agentLabel}
            </span>
          </div>
          <span className="flex-1" />
          <span className="text-[9px] text-[--pager-text-faint] font-mono">{sessionPrefix}</span>
          <span className="text-[9px] text-[--pager-text-faint]">{relativeTime}</span>
        </div>

        {/* Row 2: tool tag + content + jump button */}
        <div className="flex items-center justify-between pl-[12px]">
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
            className="ml-[6px] p-[3px] rounded text-[--pager-text-faint] hover:text-[--pager-blue] hover:bg-[--pager-filter-bg] disabled:opacity-30 shrink-0 transition-colors"
          >
            {jumping ? (
              <svg className="w-[13px] h-[13px] animate-spin" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="8" />
              </svg>
            ) : (
              <ArrowRight size={13} />
            )}
          </button>
        </div>

        {/* Expanded content */}
        {expanded && contentRaw && (
          <div className="mt-[6px] pl-[12px]">
            <pre className="p-[6px] bg-[--pager-detail-bg] rounded-md text-[10px] leading-relaxed text-[--pager-detail-text] font-mono whitespace-pre-wrap break-all max-h-32 overflow-y-auto border border-[--pager-detail-border]">
              {contentRaw}
            </pre>
          </div>
        )}
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

- [ ] **Step 3: Rewrite SessionList with project grouping**

```typescript
// frontend/src/components/SessionList.tsx
import { Folder } from 'lucide-react'
import { useProjectGroups } from '../store/sessions'
import SessionCard from './SessionCard'
import EmptyState from './EmptyState'

export default function SessionList() {
  const groups = useProjectGroups()

  if (groups.length === 0) {
    return <EmptyState />
  }

  return (
    <div className="p-[10px] space-y-[12px]">
      {groups.map((group) => (
        <div key={group.project}>
          {/* Project header */}
          <div className="flex items-center gap-[6px] px-[4px] mb-[6px]">
            <Folder size={14} className="text-[--pager-text-muted]" strokeWidth={2} />
            <span className="text-[11px] font-semibold text-[--pager-text-muted] tracking-[0.3px]">
              {group.project}
            </span>
            <span className="flex-1 h-px bg-[--pager-border] opacity-50" />
          </div>
          {/* Sessions in this project */}
          <div className="space-y-[5px]">
            {group.sessions.map((session) => (
              <SessionCard key={session.Key} session={session} />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Rewrite App.tsx — navbar with pin/title/gear**

```typescript
// frontend/src/App.tsx
import { useState, useEffect } from 'react'
import { Pin, Settings } from 'lucide-react'
import SessionList from './components/SessionList'
import { useSettingsStore } from './store/settings'

function App() {
  const settings = useSettingsStore((s) => s.settings)
  const [pinned, setPinned] = useState(settings.popup_pinned ?? false)

  useEffect(() => {
    setPinned(settings.popup_pinned ?? false)
  }, [settings.popup_pinned])

  const handleTogglePin = async () => {
    const newPinned = !pinned
    setPinned(newPinned)
    try {
      const { SetPinned } = await import('../bindings/pager/internal/wails/windowbinding.js')
      await SetPinned(newPinned)
    } catch (err) {
      console.error('SetPinned failed:', err)
      setPinned(!newPinned) // revert on error
    }
  }

  const handleOpenSettings = async () => {
    try {
      const { OpenSettings } = await import('../bindings/pager/internal/wails/windowbinding.js')
      await OpenSettings()
    } catch (err) {
      console.error('OpenSettings failed:', err)
    }
  }

  return (
    <div className="w-full h-screen bg-[--pager-bg] backdrop-blur-2xl text-[--pager-text] overflow-hidden rounded-xl border border-[--pager-border] flex flex-col">
      {/* Navbar — drag region */}
      <header
        className="sticky top-0 z-10 bg-[--pager-header-bg] backdrop-blur-xl px-3 py-2.5 border-b border-[--pager-border] flex items-center justify-between"
        style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
      >
        {/* Left: Pin button */}
        <button
          onClick={handleTogglePin}
          className={`w-6 h-6 flex items-center justify-center rounded-[5px] transition-all ${
            pinned ? 'bg-[rgba(10,132,255,0.12)]' : 'hover:bg-[--pager-filter-bg]'
          }`}
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          title={pinned ? 'Unpin window' : 'Pin window on top'}
        >
          <Pin
            size={14}
            className={`transition-all ${pinned ? 'text-[--pager-blue] rotate-0' : 'text-[--pager-text-muted] -rotate-45'}`}
            strokeWidth={pinned ? 2.5 : 2}
          />
        </button>

        {/* Center: Title */}
        <span className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">
          Pager
        </span>

        {/* Right: Settings gear */}
        <button
          onClick={handleOpenSettings}
          className="w-6 h-6 flex items-center justify-center rounded-[5px] hover:bg-[--pager-filter-bg] transition-all"
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          title="Settings"
        >
          <Settings size={14} className="text-[--pager-text-muted]" strokeWidth={2} />
        </button>
      </header>

      {/* Session list — scrollable */}
      <div className="flex-1 overflow-y-auto">
        <SessionList />
      </div>
    </div>
  )
}

export default App
```

- [ ] **Step 5: Regenerate Wails bindings**

Run: `cd /private/data/projects/github.com/sapaude/pager && make bindings`
Expected: New `windowbinding.js` generated in `frontend/bindings/`

- [ ] **Step 6: Verify full build compiles**

Run: `cd /private/data/projects/github.com/sapaude/pager && make lint`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/
git commit -m "feat(ui): rewrite popup with project-grouped list, new card layout, lucide icons"
```

---

### Task 4: Window Behavior & Final Integration

**Files:**
- Modify: `frontend/src/App.tsx` (add resize handler)
- Modify: `frontend/src/index.css` (minor cleanup)
- Modify: `frontend/src/main.tsx` (no changes needed — already handles settings init)

- [ ] **Step 1: Add resize width persistence to App.tsx**

Add this `useEffect` inside `App()` after the pin state:

```typescript
// Save width on window resize
useEffect(() => {
  let resizeTimeout: ReturnType<typeof setTimeout>

  const handleResize = () => {
    clearTimeout(resizeTimeout)
    resizeTimeout = setTimeout(async () => {
      const width = window.innerWidth
      if (width >= 300 && width <= 600) {
        try {
          const { SetPopupWidth } = await import('../bindings/pager/internal/wails/windowbinding.js')
          await SetPopupWidth(width)
        } catch (err) {
          console.error('SetPopupWidth failed:', err)
        }
      }
    }, 500) // debounce 500ms
  }

  window.addEventListener('resize', handleResize)
  return () => {
    clearTimeout(resizeTimeout)
    window.removeEventListener('resize', handleResize)
  }
}, [])
```

- [ ] **Step 2: Remove old FilterDot-related CSS if any, verify index.css is clean**

No changes expected in `index.css` — the existing CSS variables and scrollbar styles remain valid. The `animate-pulse-status` keyframes are still used. Remove the body `overflow: hidden` so the session list can scroll:

```css
body {
  margin: 0;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', Roboto, sans-serif;
}
```

(Remove `overflow: hidden` from body — the flex container in App.tsx handles overflow.)

- [ ] **Step 3: Run dev mode and verify visually**

Run: `cd /private/data/projects/github.com/sapaude/pager && make dev`

Manual checks:
1. Popup opens via Alt+E — shows project-grouped list
2. Pin button toggles (icon rotates, window stays on top when pinned)
3. Gear icon opens settings window
4. Navbar drag moves window
5. Horizontal resize works (left + right edges), width persisted across reopens
6. Opacity slider in settings immediately affects popup
7. Done sessions older than 30min disappear from list
8. Empty state shows when no sessions

- [ ] **Step 4: Commit**

```bash
git add frontend/src/ internal/
git commit -m "feat: window behavior (pin/resize/drag) and final integration"
```

---

## Verification Checklist

After all tasks complete, run through acceptance criteria:

- [ ] `make test` — all Go tests pass
- [ ] `make lint` — no errors
- [ ] `make dev` — app launches, popup renders correctly
- [ ] Pin toggle works and persists across app restart
- [ ] Horizontal resize saves to config
- [ ] Settings opacity slider affects popup in real-time
- [ ] Cmd+, no longer in tray menu accelerator
- [ ] Sessions grouped by project correctly
- [ ] Card layout: dot → status → agent ... id → time
