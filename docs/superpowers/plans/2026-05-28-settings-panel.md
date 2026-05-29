# Settings Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a settings panel window with tray menu, i18n, global hotkey, notification preferences, and app icon to Pager.

**Architecture:** Go `internal/settings` package handles JSON file persistence. A `SettingsService` Wails binding exposes load/save to the React frontend. The frontend uses hash router to serve both the popup (session list) and settings
panel from a single embedded build. i18next handles multilingual UI. `golang.design/x/hotkey` provides global hotkey registration.

**Tech Stack:** Go 1.25, Wails v3 alpha.96, React 18, zustand, i18next, react-i18next, golang.design/x/hotkey, Tailwind CSS

---

## File Structure

### Go (new files)

| File                                 | Responsibility                                                  |
|--------------------------------------|-----------------------------------------------------------------|
| `internal/settings/settings.go`      | Config struct, Load/Save with defaults, atomic file write       |
| `internal/settings/settings_test.go` | Unit tests for load/save/defaults                               |
| `settings_service.go`                | Wails binding: GetSettings, UpdateSettings, exposed to frontend |

### Go (modified files)

| File                        | Change                                                                      |
|-----------------------------|-----------------------------------------------------------------------------|
| `main.go`                   | Add settings window, tray menu rework, hotkey registration, embed tray icon |
| `app.go`                    | Pass settings to notify, wire settings onChange for hotkey/theme updates    |
| `internal/notify/notify.go` | Accept notification_level + language params, filter events accordingly      |
| `go.mod` / `go.sum`         | Add `golang.design/x/hotkey` dependency                                     |

### Frontend (new files)

| File                                              | Responsibility                                                |
|---------------------------------------------------|---------------------------------------------------------------|
| `frontend/src/i18n/index.ts`                      | i18next init + language detection                             |
| `frontend/src/i18n/locales/zh.json`               | Chinese translations                                          |
| `frontend/src/i18n/locales/en.json`               | English translations                                          |
| `frontend/src/store/settings.ts`                  | zustand settings store                                        |
| `frontend/src/pages/SettingsPanel.tsx`            | Settings panel layout (sidebar + content)                     |
| `frontend/src/pages/settings/GeneralSettings.tsx` | General settings content (appearance, hotkeys, notifications) |
| `frontend/src/pages/settings/AboutSettings.tsx`   | About page content                                            |
| `frontend/src/components/HotkeyRecorder.tsx`      | Hotkey capture input component                                |

### Frontend (modified files)

| File                                      | Change                                                |
|-------------------------------------------|-------------------------------------------------------|
| `frontend/src/main.tsx`                   | Add hash router, init i18n, conditional render        |
| `frontend/src/App.tsx`                    | Wrap with i18n useTranslation                         |
| `frontend/src/index.css`                  | Add class-based theme switching (replace media query) |
| `frontend/src/components/SessionCard.tsx` | Use i18n for hardcoded strings                        |
| `frontend/src/components/SessionList.tsx` | Use i18n for hardcoded strings                        |
| `frontend/package.json`                   | Add i18next, react-i18next deps                       |

### Assets (new files)

| File                      | Responsibility                 |
|---------------------------|--------------------------------|
| `assets/icon.png`         | 1024px app icon master         |
| `assets/tray-icon.png`    | 22×22 monochrome tray template |
| `assets/tray-icon@2x.png` | 44×44 retina tray template     |
| `build/appicon.png`       | Wails build icon reference     |

---

## Task 1: Go Settings Package

**Files:**

- Create: `internal/settings/settings.go`
- Create: `internal/settings/settings_test.go`

- [ ] **Step 1: Write failing test for Load with no file (returns defaults)**

```go
// internal/settings/settings_test.go
package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenFileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Language != "zh" {
		t.Errorf("expected default language 'zh', got '%s'", cfg.Language)
	}
	if cfg.Theme != "system" {
		t.Errorf("expected default theme 'system', got '%s'", cfg.Theme)
	}
	if cfg.Opacity != 75 {
		t.Errorf("expected default opacity 75, got %d", cfg.Opacity)
	}
	if cfg.HotkeyToggle != "Alt+E" {
		t.Errorf("expected default hotkey 'Alt+E', got '%s'", cfg.HotkeyToggle)
	}
	if cfg.NotificationLevel != "attention_only" {
		t.Errorf("expected default notification_level 'attention_only', got '%s'", cfg.NotificationLevel)
	}
	// File should not exist — no auto-create on Load
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to not be created on Load")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/settings/ -run TestLoadReturnsDefaultsWhenFileNotExists -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Write Settings struct and LoadFrom**

```go
// internal/settings/settings.go
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds all user-configurable preferences.
type Settings struct {
	Language          string `json:"language"`
	Theme             string `json:"theme"`
	Opacity           int    `json:"opacity"`
	HotkeyToggle      string `json:"hotkey_toggle"`
	NotificationLevel string `json:"notification_level"`
}

// Defaults returns the default settings.
func Defaults() Settings {
	return Settings{
		Language:          "zh",
		Theme:             "system",
		Opacity:           75,
		HotkeyToggle:      "Alt+E",
		NotificationLevel: "attention_only",
	}
}

// DefaultPath returns ~/.config/pager/settings.json
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "settings.json")
}

// LoadFrom reads settings from the given path.
// Returns defaults if file does not exist.
func LoadFrom(path string) (Settings, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Defaults(), err
	}
	return cfg, nil
}

// SaveTo writes settings to the given path atomically (tmp + rename).
func SaveTo(path string, cfg Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/settings/ -run TestLoadReturnsDefaultsWhenFileNotExists -v`
Expected: PASS

- [ ] **Step 5: Write test for SaveTo + LoadFrom round-trip**

```go
// Append to internal/settings/settings_test.go
func TestSaveAndLoad(t *testing.T) {
tmpDir := t.TempDir()
path := filepath.Join(tmpDir, "sub", "settings.json")

cfg := Settings{
Language:          "en",
Theme:             "dark",
Opacity:           50,
HotkeyToggle:     "Ctrl+Shift+P",
NotificationLevel: "all",
}

if err := SaveTo(path, cfg); err != nil {
t.Fatalf("SaveTo failed: %v", err)
}

loaded, err := LoadFrom(path)
if err != nil {
t.Fatalf("LoadFrom failed: %v", err)
}

if loaded != cfg {
t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", loaded, cfg)
}
}

func TestLoadFromCorruptedFileReturnsDefaults(t *testing.T) {
tmpDir := t.TempDir()
path := filepath.Join(tmpDir, "settings.json")

os.WriteFile(path, []byte("{invalid json"), 0o644)

cfg, err := LoadFrom(path)
if err == nil {
t.Fatal("expected error for corrupted JSON")
}
// Should still return defaults
if cfg != Defaults() {
t.Errorf("expected defaults on corruption, got %+v", cfg)
}
}
```

- [ ] **Step 6: Run all settings tests**

Run: `go test ./internal/settings/ -v`
Expected: 3 tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/settings/
git commit -m "feat(settings): add settings package with JSON persistence"
```

---

## Task 2: Wails SettingsService Binding

**Files:**

- Create: `settings_service.go`
- Modify: `main.go` (register service)

- [ ] **Step 1: Create SettingsService with GetSettings and UpdateSettings**

```go
// settings_service.go
package main

import (
	"pager/internal/settings"
)

// SettingsService exposes user settings to the React frontend via Wails bindings.
type SettingsService struct {
	path     string
	onChange func(settings.Settings)
}

// NewSettingsService creates a SettingsService.
// onChange is called after every successful UpdateSettings — use it to apply changes
// (hotkey re-register, notification filter update, etc.)
func NewSettingsService(onChange func(settings.Settings)) *SettingsService {
	return &SettingsService{
		path:     settings.DefaultPath(),
		onChange: onChange,
	}
}

// GetSettings returns the current settings (reads from file on each call).
func (s *SettingsService) GetSettings() settings.Settings {
	cfg, _ := settings.LoadFrom(s.path)
	return cfg
}

// UpdateSettings saves new settings and triggers onChange.
func (s *SettingsService) UpdateSettings(cfg settings.Settings) error {
	if err := settings.SaveTo(s.path, cfg); err != nil {
		return err
	}
	if s.onChange != nil {
		s.onChange(cfg)
	}
	return nil
}
```

- [ ] **Step 2: Register SettingsService in main.go**

In `main.go`, after `svc := &SessionService{...}`, add:

```go
settingsSvc := NewSettingsService(func (cfg settings.Settings) {
// TODO: wire up hotkey re-registration and notify filter in later tasks
})
```

Add to the Services slice:

```go
Services: []application.Service{
application.NewService(myApp),
application.NewService(svc),
application.NewService(settingsSvc),
},
```

Add import for `"pager/internal/settings"`.

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Success (no errors)

- [ ] **Step 4: Commit**

```bash
git add settings_service.go main.go
git commit -m "feat: add SettingsService Wails binding for frontend settings access"
```

---

## Task 3: Tray Menu + Settings Window

**Files:**

- Modify: `main.go`

- [ ] **Step 1: Add settings window definition**

After the existing popup window definition in `main.go`, add:

```go
// ── Settings window ─────────────────────────────────────────────────────
// Standard macOS window for preferences. Hidden by default, shown via tray menu.
settingsWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
Title:         "Pager 设置",
Name:          "pager-settings",
Width:         720,
Height:        520,
Hidden:        true,
DisableResize: true,
URL:           "#/settings",
})
```

- [ ] **Step 2: Replace the current quit menu with full tray menu**

Replace the existing tray menu block (`quitMenu := ...` through `tray.SetMenu(quitMenu)`) with:

```go
// ── Tray menu ───────────────────────────────────────────────────────────
trayMenu := wailsApp.NewMenu()
trayMenu.Add("偏好设置...").
SetAccelerator("CmdOrCtrl+,").
OnClick(func (_ *application.Context) {
settingsWindow.Show()
settingsWindow.Focus()
})
trayMenu.AddSeparator()
trayMenu.Add("退出 Pager").
SetAccelerator("CmdOrCtrl+Q").
OnClick(func (_ *application.Context) {
wailsApp.Quit()
})
tray.SetMenu(trayMenu)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add settings window and tray context menu (Preferences + Quit)"
```

---

## Task 4: Frontend Hash Router Setup

**Files:**

- Modify: `frontend/package.json` (add react-router-dom)
- Modify: `frontend/src/main.tsx`
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/pages/SettingsPanel.tsx` (placeholder)

- [ ] **Step 1: Install react-router-dom**

Run: `cd frontend && npm install react-router-dom`

- [ ] **Step 2: Create placeholder SettingsPanel**

```tsx
// frontend/src/pages/SettingsPanel.tsx
export default function SettingsPanel() {
    return (
        <div className="w-full h-screen bg-[--pager-bg] text-[--pager-text]">
            <p className="p-4">Settings (TODO)</p>
        </div>
    )
}
```

- [ ] **Step 3: Rewrite main.tsx with HashRouter**

```tsx
// frontend/src/main.tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import {HashRouter, Routes, Route} from 'react-router-dom'
import App from './App'
import SettingsPanel from './pages/SettingsPanel'
import './index.css'
import {initSessionSync} from './store/sessions'

initSessionSync()

ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <HashRouter>
            <Routes>
                <Route path="/" element={<App/>}/>
                <Route path="/settings" element={<SettingsPanel/>}/>
            </Routes>
        </HashRouter>
    </React.StrictMode>,
)
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/main.tsx frontend/src/pages/
git commit -m "feat(frontend): add hash router for multi-window routing"
```

---

## Task 5: i18n Setup

**Files:**

- Create: `frontend/src/i18n/index.ts`
- Create: `frontend/src/i18n/locales/zh.json`
- Create: `frontend/src/i18n/locales/en.json`
- Modify: `frontend/src/main.tsx`
- Modify: `frontend/package.json`

- [ ] **Step 1: Install i18next dependencies**

Run: `cd frontend && npm install i18next react-i18next`

- [ ] **Step 2: Create Chinese locale file**

```json
{
    "nav": {
        "general": "基本设置",
        "about": "关于"
    },
    "general": {
        "title": "基本设置",
        "appearance": "外观",
        "language": "语言",
        "theme": "主题",
        "themeLight": "浅色",
        "themeDark": "深色",
        "themeSystem": "系统",
        "opacity": "透明度",
        "hotkeys": "快捷键",
        "hotkeyToggle": "唤起窗口",
        "hotkeyToggleDesc": "全局快捷键打开 Pager 面板",
        "hotkeyRecording": "按下新快捷键...",
        "notifications": "通知",
        "notificationLevel": "通知级别",
        "notificationLevelDesc": "选择何时发送系统通知",
        "notifyAll": "所有消息",
        "notifyAttentionOnly": "仅需确认时"
    },
    "about": {
        "title": "关于",
        "description": "AI coding agents 状态感知层",
        "version": "版本",
        "author": "作者",
        "checkUpdate": "检查更新",
        "github": "GitHub",
        "copyright": "© 2026 sapaude · Built with Wails v3"
    },
    "session": {
        "title": "Pager",
        "attention": "需确认",
        "running": "执行中",
        "done": "已完成",
        "jumpToTerminal": "跳转到终端",
        "session": "会话",
        "path": "路径"
    },
    "tray": {
        "preferences": "偏好设置...",
        "quit": "退出 Pager"
    }
}
```

- [ ] **Step 3: Create English locale file**

```json
{
    "nav": {
        "general": "General",
        "about": "About"
    },
    "general": {
        "title": "General",
        "appearance": "Appearance",
        "language": "Language",
        "theme": "Theme",
        "themeLight": "Light",
        "themeDark": "Dark",
        "themeSystem": "System",
        "opacity": "Opacity",
        "hotkeys": "Shortcuts",
        "hotkeyToggle": "Toggle Window",
        "hotkeyToggleDesc": "Global shortcut to open Pager panel",
        "hotkeyRecording": "Press new shortcut...",
        "notifications": "Notifications",
        "notificationLevel": "Notification Level",
        "notificationLevelDesc": "Choose when to send system notifications",
        "notifyAll": "All messages",
        "notifyAttentionOnly": "Only when confirmation needed"
    },
    "about": {
        "title": "About",
        "description": "Status awareness layer for AI coding agents",
        "version": "Version",
        "author": "Author",
        "checkUpdate": "Check for Updates",
        "github": "GitHub",
        "copyright": "© 2026 sapaude · Built with Wails v3"
    },
    "session": {
        "title": "Pager",
        "attention": "Attention",
        "running": "Running",
        "done": "Done",
        "jumpToTerminal": "Jump to terminal",
        "session": "Session",
        "path": "Path"
    },
    "tray": {
        "preferences": "Preferences...",
        "quit": "Quit Pager"
    }
}
```

- [ ] **Step 4: Create i18n init**

```ts
// frontend/src/i18n/index.ts
import i18n from 'i18next'
import {initReactI18next} from 'react-i18next'
import zh from './locales/zh.json'
import en from './locales/en.json'

i18n.use(initReactI18next).init({
    resources: {
        zh: {translation: zh},
        en: {translation: en},
    },
    lng: 'zh', // default, will be overridden by settings
    fallbackLng: 'zh',
    interpolation: {escapeValue: false},
})

export default i18n
```

- [ ] **Step 5: Import i18n in main.tsx (before ReactDOM.createRoot)**

Add at the top of `frontend/src/main.tsx`:

```ts
import './i18n'
```

- [ ] **Step 6: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add frontend/src/i18n/ frontend/package.json frontend/package-lock.json frontend/src/main.tsx
git commit -m "feat(i18n): add i18next with zh/en locale files"
```

---

## Task 6: Settings Zustand Store

**Files:**

- Create: `frontend/src/store/settings.ts`

- [ ] **Step 1: Create the settings store**

```ts
// frontend/src/store/settings.ts
import {create} from 'zustand'
import i18n from '../i18n'

export interface Settings {
    language: string
    theme: string
    opacity: number
    hotkey_toggle: string
    notification_level: string
}

const DEFAULT_SETTINGS: Settings = {
    language: 'zh',
    theme: 'system',
    opacity: 75,
    hotkey_toggle: 'Alt+E',
    notification_level: 'attention_only',
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
            // Dynamic import to avoid issues when bindings aren't generated yet
            const {GetSettings} = await import('../../bindings/pager/settingsservice.js')
            const cfg = await GetSettings()
            set({settings: cfg, loaded: true})
            // Apply language
            i18n.changeLanguage(cfg.language)
            // Apply theme
            applyTheme(cfg.theme)
            applyOpacity(cfg.opacity)
        } catch (err) {
            console.warn('[pager] GetSettings failed, using defaults:', err)
            set({loaded: true})
        }
    },

    updateSettings: async (partial) => {
        const current = get().settings
        const updated = {...current, ...partial}
        set({settings: updated})

        // Apply immediately
        if (partial.language) i18n.changeLanguage(partial.language)
        if (partial.theme) applyTheme(partial.theme)
        if (partial.opacity !== undefined) applyOpacity(partial.opacity)

        try {
            const {UpdateSettings} = await import('../../bindings/pager/settingsservice.js')
            await UpdateSettings(updated)
        } catch (err) {
            console.error('[pager] UpdateSettings failed:', err)
        }
    },
}))

function applyTheme(theme: string) {
    const root = document.documentElement
    root.classList.remove('light', 'dark')
    if (theme === 'light' || theme === 'dark') {
        root.classList.add(theme)
    }
    // 'system' → no class, CSS media query handles it
}

function applyOpacity(opacity: number) {
    document.documentElement.style.setProperty('--pager-opacity', String(opacity / 100))
}

export function initSettings() {
    useSettingsStore.getState().loadSettings()
}
```

- [ ] **Step 2: Call initSettings in main.tsx**

Add after `initSessionSync()` in `frontend/src/main.tsx`:

```ts
import {initSettings} from './store/settings'

initSettings()
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success (binding import will be lazy/dynamic)

- [ ] **Step 4: Commit**

```bash
git add frontend/src/store/settings.ts frontend/src/main.tsx
git commit -m "feat(frontend): add settings zustand store with theme/opacity/language application"
```

---

## Task 7: CSS Theme Class-Based Switching

**Files:**

- Modify: `frontend/src/index.css`

- [ ] **Step 1: Convert media-query dark mode to class-based**

Replace the `@media (prefers-color-scheme: dark)` block in `index.css` with class-based + system fallback:

```css
/* Class-based theme override */
:root.dark {
    --pager-bg: rgba(30, 30, 32, calc(var(--pager-opacity, 0.94)));
    --pager-header-bg: rgba(38, 38, 40, 0.96);
    --pager-border: rgba(255, 255, 255, 0.1);
    --pager-text: #f5f5f7;
    --pager-text-primary: rgba(255, 255, 255, 0.92);
    --pager-text-secondary: rgba(255, 255, 255, 0.55);
    --pager-text-muted: rgba(255, 255, 255, 0.4);
    --pager-text-faint: rgba(255, 255, 255, 0.22);
    --pager-red: #ff453a;
    --pager-green: #30d158;
    --pager-blue: #0a84ff;
    --pager-gray-dot: rgba(255, 255, 255, 0.35);
    --pager-filter-bg: rgba(255, 255, 255, 0.06);
    --pager-filter-active: rgba(255, 255, 255, 0.1);
    --pager-badge-bg: rgba(130, 120, 255, 0.15);
    --pager-badge-text: #b4a5ff;
    --pager-tool-bg: rgba(10, 132, 255, 0.12);
    --pager-tool-text: #64b5f6;
    --pager-card-attention-bg: rgba(255, 69, 58, 0.08);
    --pager-card-attention-border: rgba(255, 69, 58, 0.2);
    --pager-card-running-bg: rgba(48, 209, 88, 0.06);
    --pager-card-running-border: rgba(48, 209, 88, 0.15);
    --pager-card-done-bg: rgba(255, 255, 255, 0.02);
    --pager-card-done-border: rgba(255, 255, 255, 0.06);
    --pager-detail-bg: rgba(0, 0, 0, 0.3);
    --pager-detail-border: rgba(255, 255, 255, 0.06);
    --pager-detail-text: rgba(255, 255, 255, 0.55);
}

:root.light {
    --pager-bg: rgba(246, 246, 248, calc(var(--pager-opacity, 0.92)));
}

/* System preference fallback (when no class is set = "system" theme) */
@media (prefers-color-scheme: dark) {
    :root:not(.light) {
        --pager-bg: rgba(30, 30, 32, calc(var(--pager-opacity, 0.94)));
        --pager-header-bg: rgba(38, 38, 40, 0.96);
        --pager-border: rgba(255, 255, 255, 0.1);
        --pager-text: #f5f5f7;
        --pager-text-primary: rgba(255, 255, 255, 0.92);
        --pager-text-secondary: rgba(255, 255, 255, 0.55);
        --pager-text-muted: rgba(255, 255, 255, 0.4);
        --pager-text-faint: rgba(255, 255, 255, 0.22);
        --pager-red: #ff453a;
        --pager-green: #30d158;
        --pager-blue: #0a84ff;
        --pager-gray-dot: rgba(255, 255, 255, 0.35);
        --pager-filter-bg: rgba(255, 255, 255, 0.06);
        --pager-filter-active: rgba(255, 255, 255, 0.1);
        --pager-badge-bg: rgba(130, 120, 255, 0.15);
        --pager-badge-text: #b4a5ff;
        --pager-tool-bg: rgba(10, 132, 255, 0.12);
        --pager-tool-text: #64b5f6;
        --pager-card-attention-bg: rgba(255, 69, 58, 0.08);
        --pager-card-attention-border: rgba(255, 69, 58, 0.2);
        --pager-card-running-bg: rgba(48, 209, 88, 0.06);
        --pager-card-running-border: rgba(48, 209, 88, 0.15);
        --pager-card-done-bg: rgba(255, 255, 255, 0.02);
        --pager-card-done-border: rgba(255, 255, 255, 0.06);
        --pager-detail-bg: rgba(0, 0, 0, 0.3);
        --pager-detail-border: rgba(255, 255, 255, 0.06);
        --pager-detail-text: rgba(255, 255, 255, 0.55);
    }
}
```

Also update the light `:root` to use `--pager-opacity`:

```css
:root {
    --pager-bg: rgba(246, 246, 248, calc(var(--pager-opacity, 0.92)));
    /* ... rest unchanged ... */
}
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add frontend/src/index.css
git commit -m "feat(frontend): switch to class-based theme with opacity CSS variable"
```

---

## Task 8: Settings Panel UI — Layout + General Settings

**Files:**

- Rewrite: `frontend/src/pages/SettingsPanel.tsx`
- Create: `frontend/src/pages/settings/GeneralSettings.tsx`
- Create: `frontend/src/components/HotkeyRecorder.tsx`

- [ ] **Step 1: Create HotkeyRecorder component**

```tsx
// frontend/src/components/HotkeyRecorder.tsx
import {useState, useCallback} from 'react'
import {useTranslation} from 'react-i18next'

interface Props {
    value: string
    onChange: (hotkey: string) => void
}

export default function HotkeyRecorder({value, onChange}: Props) {
    const {t} = useTranslation()
    const [recording, setRecording] = useState(false)

    const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
        e.preventDefault()
        e.stopPropagation()

        if (e.key === 'Escape') {
            setRecording(false)
            return
        }

        // Build hotkey string from modifiers + key
        const parts: string[] = []
        if (e.ctrlKey) parts.push('Ctrl')
        if (e.altKey) parts.push('Alt')
        if (e.shiftKey) parts.push('Shift')
        if (e.metaKey) parts.push('Cmd')

        const key = e.key.length === 1 ? e.key.toUpperCase() : e.key
        // Only accept combos with at least one modifier
        if (parts.length === 0) return
        if (['Control', 'Alt', 'Shift', 'Meta'].includes(e.key)) return

        parts.push(key)
        const hotkey = parts.join('+')
        onChange(hotkey)
        setRecording(false)
    }, [onChange])

    return (
        <div
            onClick={() => setRecording(true)}
            onKeyDown={recording ? handleKeyDown : undefined}
            onBlur={() => setRecording(false)}
            tabIndex={0}
            className={`px-3 py-1 rounded-md text-xs font-mono cursor-pointer select-none transition-all outline-none
        ${recording
                ? 'bg-blue-50 border-2 border-blue-400 text-blue-600 dark:bg-blue-950 dark:border-blue-500 dark:text-blue-300'
                : 'bg-[rgba(0,0,0,0.04)] border border-[rgba(0,0,0,0.1)] text-[rgba(0,0,0,0.7)] dark:bg-[rgba(255,255,255,0.06)] dark:border-[rgba(255,255,255,0.1)] dark:text-[rgba(255,255,255,0.7)]'
            }`}
        >
            {recording ? t('general.hotkeyRecording') : formatHotkeyDisplay(value)}
        </div>
    )
}

function formatHotkeyDisplay(hotkey: string): string {
    return hotkey
        .replace('Alt', '⌥')
        .replace('Ctrl', '⌃')
        .replace('Shift', '⇧')
        .replace('Cmd', '⌘')
        .replace(/\+/g, ' ')
}
```

- [ ] **Step 2: Create GeneralSettings page**

```tsx
// frontend/src/pages/settings/GeneralSettings.tsx
import {useTranslation} from 'react-i18next'
import {useSettingsStore} from '../../store/settings'
import HotkeyRecorder from '../../components/HotkeyRecorder'

export default function GeneralSettings() {
    const {t} = useTranslation()
    const settings = useSettingsStore((s) => s.settings)
    const updateSettings = useSettingsStore((s) => s.updateSettings)

    return (
        <div>
            <h3 className="text-[17px] font-semibold text-[--pager-text-primary] mb-4">{t('general.title')}</h3>

            {/* Appearance section */}
            <Section label={t('general.appearance')}>
                {/* Language */}
                <Row label={t('general.language')}>
                    <select
                        value={settings.language}
                        onChange={(e) => updateSettings({language: e.target.value})}
                        className="settings-dropdown"
                    >
                        <option value="zh">中文</option>
                        <option value="en">English</option>
                    </select>
                </Row>

                {/* Theme */}
                <Row label={t('general.theme')} border>
                    <div className="settings-segmented">
                        {(['light', 'dark', 'system'] as const).map((theme) => (
                            <button
                                key={theme}
                                onClick={() => updateSettings({theme})}
                                className={`settings-segmented-btn ${settings.theme === theme ? 'active' : ''}`}
                            >
                                {t(`general.theme${theme.charAt(0).toUpperCase() + theme.slice(1)}`)}
                            </button>
                        ))}
                    </div>
                </Row>

                {/* Opacity */}
                <Row label={t('general.opacity')} last>
                    <div className="flex items-center gap-2">
                        <input
                            type="range"
                            min={30}
                            max={100}
                            value={settings.opacity}
                            onChange={(e) => updateSettings({opacity: Number(e.target.value)})}
                            className="w-24 h-1 accent-[#007aff]"
                        />
                        <span className="text-[11px] text-[--pager-text-muted] w-7 text-right">{settings.opacity}%</span>
                    </div>
                </Row>
            </Section>

            {/* Hotkeys section */}
            <Section label={t('general.hotkeys')}>
                <Row label={t('general.hotkeyToggle')} desc={t('general.hotkeyToggleDesc')} last>
                    <HotkeyRecorder
                        value={settings.hotkey_toggle}
                        onChange={(hotkey) => updateSettings({hotkey_toggle: hotkey})}
                    />
                </Row>
            </Section>

            {/* Notifications section */}
            <Section label={t('general.notifications')}>
                <Row label={t('general.notificationLevel')} desc={t('general.notificationLevelDesc')} last>
                    <select
                        value={settings.notification_level}
                        onChange={(e) => updateSettings({notification_level: e.target.value})}
                        className="settings-dropdown"
                    >
                        <option value="all">{t('general.notifyAll')}</option>
                        <option value="attention_only">{t('general.notifyAttentionOnly')}</option>
                    </select>
                </Row>
            </Section>
        </div>
    )
}

function Section({label, children}: { label: string; children: React.ReactNode }) {
    return (
        <div className="mb-4">
            <div className="text-[12px] font-semibold text-[--pager-text-muted] mb-1.5 pl-0.5">{label}</div>
            <div className="bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden">
                {children}
            </div>
        </div>
    )
}

function Row({label, desc, children, border, last}: {
    label: string
    desc?: string
    children: React.ReactNode
    border?: boolean
    last?: boolean
}) {
    return (
        <div className={`flex items-center justify-between px-3.5 py-2.5 ${!last ? 'border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)]' : ''}`}>
            <div>
                <div className="text-[13px] text-[--pager-text-primary]">{label}</div>
                {desc && <div className="text-[11px] text-[--pager-text-muted] mt-0.5">{desc}</div>}
            </div>
            {children}
        </div>
    )
}
```

- [ ] **Step 3: Rewrite SettingsPanel with sidebar navigation**

```tsx
// frontend/src/pages/SettingsPanel.tsx
import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import GeneralSettings from './settings/GeneralSettings'
import AboutSettings from './settings/AboutSettings'

type Page = 'general' | 'about'

export default function SettingsPanel() {
    const {t} = useTranslation()
    const [page, setPage] = useState<Page>('general')

    const navItems: { id: Page; icon: string; label: string }[] = [
        {id: 'general', icon: '⚙️', label: t('nav.general')},
        {id: 'about', icon: 'ℹ️', label: t('nav.about')},
    ]

    return (
        <div className="w-full h-screen flex bg-[#f5f5f7] dark:bg-[#1e1e20] text-[--pager-text]"
             style={{fontFamily: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif"}}>
            {/* Sidebar */}
            <div className="w-[180px] min-w-[180px] bg-[rgba(246,246,248,0.98)] dark:bg-[rgba(30,30,32,0.98)] border-r border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.06)] p-3 pt-2">
                <div className="text-[11px] font-semibold text-[rgba(0,0,0,0.4)] dark:text-[rgba(255,255,255,0.35)] tracking-wide mb-1 px-3">
                    {t('general.title').includes('基本') ? '通用' : 'GENERAL'}
                </div>
                {navItems.map((item) => (
                    <button
                        key={item.id}
                        onClick={() => setPage(item.id)}
                        className={`w-full text-left px-3 py-1.5 my-0.5 rounded-md text-[13px] flex items-center gap-2 transition-colors
              ${page === item.id
                            ? 'bg-[#007aff] text-white font-medium'
                            : 'text-[rgba(0,0,0,0.75)] dark:text-[rgba(255,255,255,0.6)] hover:bg-[rgba(0,0,0,0.04)] dark:hover:bg-[rgba(255,255,255,0.05)]'
                        }`}
                    >
                        <span className="text-[14px]">{item.icon}</span>
                        {item.label}
                    </button>
                ))}
            </div>

            {/* Content */}
            <div className="flex-1 p-5 overflow-y-auto bg-[#f5f5f7] dark:bg-[#2a2a2c]">
                {page === 'general' && <GeneralSettings/>}
                {page === 'about' && <AboutSettings/>}
            </div>
        </div>
    )
}
```

- [ ] **Step 4: Create placeholder AboutSettings**

```tsx
// frontend/src/pages/settings/AboutSettings.tsx
import {useTranslation} from 'react-i18next'

export default function AboutSettings() {
    const {t} = useTranslation()

    return (
        <div className="flex flex-col items-center justify-center h-full">
            <p className="text-[--pager-text-muted]">{t('about.title')} (TODO)</p>
        </div>
    )
}
```

- [ ] **Step 5: Add settings-specific utility classes to index.css**

Append to the bottom of `frontend/src/index.css`:

```css
/* Settings panel utility classes */
.settings-dropdown {
    padding: 3px 10px;
    background: rgba(0, 0, 0, 0.04);
    border-radius: 5px;
    font-size: 12px;
    color: rgba(0, 0, 0, 0.6);
    border: 1px solid rgba(0, 0, 0, 0.08);
    outline: none;
    cursor: pointer;
    -webkit-appearance: none;
    appearance: none;
    padding-right: 20px;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='rgba(0,0,0,0.3)'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 6px center;
}

.dark .settings-dropdown {
    background: rgba(255, 255, 255, 0.06);
    color: rgba(255, 255, 255, 0.7);
    border-color: rgba(255, 255, 255, 0.1);
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='rgba(255,255,255,0.4)'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 6px center;
}

.settings-segmented {
    display: flex;
    gap: 1px;
    background: rgba(0, 0, 0, 0.06);
    border-radius: 6px;
    padding: 2px;
}

.dark .settings-segmented {
    background: rgba(255, 255, 255, 0.06);
}

.settings-segmented-btn {
    padding: 3px 10px;
    font-size: 11px;
    border-radius: 4px;
    color: rgba(0, 0, 0, 0.5);
    transition: all 0.15s;
}

.dark .settings-segmented-btn {
    color: rgba(255, 255, 255, 0.5);
}

.settings-segmented-btn.active {
    background: #fff;
    color: rgba(0, 0, 0, 0.85);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

.dark .settings-segmented-btn.active {
    background: rgba(255, 255, 255, 0.12);
    color: #fff;
}
```

- [ ] **Step 6: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/ frontend/src/components/HotkeyRecorder.tsx frontend/src/index.css
git commit -m "feat(frontend): implement settings panel UI with general settings page"
```

---

## Task 9: About Page UI

**Files:**

- Rewrite: `frontend/src/pages/settings/AboutSettings.tsx`

- [ ] **Step 1: Implement full About page**

```tsx
// frontend/src/pages/settings/AboutSettings.tsx
import {useTranslation} from 'react-i18next'

export default function AboutSettings() {
    const {t} = useTranslation()

    const openURL = (url: string) => {
        window.open(url, '_blank')
    }

    return (
        <div className="flex flex-col items-center justify-center h-full">
            {/* App icon */}
            <div className="w-[72px] h-[72px] mb-3 rounded-[16px] flex items-center justify-center shadow-lg"
                 style={{background: 'linear-gradient(135deg, #007aff, #5856d6)'}}>
                <span className="text-[36px]">📟</span>
            </div>
            <div className="text-[18px] font-semibold text-[--pager-text-primary] mb-0.5">Pager</div>
            <div className="text-[12px] text-[--pager-text-muted] mb-6">{t('about.description')}</div>

            {/* Info card */}
            <div className="w-full max-w-[320px] bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden mb-4">
                <div className="flex justify-between items-center px-3.5 py-2.5 border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)]">
                    <span className="text-[13px] text-[--pager-text-primary]">{t('about.version')}</span>
                    <span className="text-[12px] text-[--pager-text-muted]">1.0.0 (build 1)</span>
                </div>
                <div className="flex justify-between items-center px-3.5 py-2.5">
                    <span className="text-[13px] text-[--pager-text-primary]">{t('about.author')}</span>
                    <a className="text-[12px] text-[#007aff] cursor-pointer hover:underline"
                       onClick={() => openURL('https://github.com/sapaude')}>sapaude</a>
                </div>
            </div>

            {/* Actions card */}
            <div className="w-full max-w-[320px] bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden mb-4">
                <div className="flex justify-between items-center px-3.5 py-2.5 border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)] cursor-pointer hover:bg-[rgba(0,0,0,0.02)] dark:hover:bg-[rgba(255,255,255,0.02)]"
                     onClick={() => openURL('https://github.com/sapaude/pager/releases')}>
                    <span className="text-[13px] text-[--pager-text-primary]">{t('about.checkUpdate')}</span>
                    <span className="text-[12px] text-[--pager-text-faint]">›</span>
                </div>
                <div className="flex justify-between items-center px-3.5 py-2.5 cursor-pointer hover:bg-[rgba(0,0,0,0.02)] dark:hover:bg-[rgba(255,255,255,0.02)]"
                     onClick={() => openURL('https://github.com/sapaude/pager')}>
                    <span className="text-[13px] text-[--pager-text-primary]">{t('about.github')}</span>
                    <span className="text-[12px] text-[--pager-text-faint]">↗</span>
                </div>
            </div>

            {/* Copyright */}
            <div className="text-[11px] text-[--pager-text-faint]">{t('about.copyright')}</div>
        </div>
    )
}
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/settings/AboutSettings.tsx
git commit -m "feat(frontend): implement About page with version info and links"
```

---

## Task 10: Notification Level Integration

**Files:**

- Modify: `internal/notify/notify.go`
- Modify: `app.go`

- [ ] **Step 1: Add ShouldNotify function to notify package**

```go
// Add to internal/notify/notify.go, above Show():

// ShouldNotify determines whether an event should trigger a system notification
// based on the configured notification level.
func ShouldNotify(e *event.AgentEvent, level string) bool {
switch level {
case "all":
// Same as current behavior: pre_tool_use, stop, error
switch e.EventType {
case event.EventPreToolUse, event.EventStop, event.EventError:
return true
}
return false
case "attention_only":
// Only attention-level events + stop + error
if e.EventType == event.EventStop || e.EventType == event.EventError {
return true
}
if e.EventType == event.EventPreToolUse && e.AttentionLevel == event.AttentionAttention {
return true
}
return false
default:
return false
}
}
```

- [ ] **Step 2: Modify Show to accept level parameter**

Replace the current `Show` function signature and add filtering:

```go
// ShowWithLevel triggers a notification only if the event passes the level filter.
func ShowWithLevel(e *event.AgentEvent, level string) {
if !ShouldNotify(e, level) {
return
}
Show(e)
}
```

- [ ] **Step 3: Wire notification level in app.go onChange callback**

In `app.go`, modify the `NewApp()` function's onChange callback. The registry onChange currently calls `notify.Show(sessions[0].LastEvent)`. Change it to:

```go
// 2. Show a macOS system notification based on notification level setting.
if len(sessions) > 0 && sessions[0].LastEvent != nil {
cfg, _ := settings.LoadFrom(settings.DefaultPath())
notify.ShowWithLevel(sessions[0].LastEvent, cfg.NotificationLevel)
}
```

Add import for `"pager/internal/settings"`.

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/notify/notify.go app.go
git commit -m "feat(notify): add notification level filtering (all vs attention_only)"
```

---

## Task 11: Global Hotkey Registration

**Files:**

- Modify: `go.mod` (add golang.design/x/hotkey)
- Modify: `main.go`

- [ ] **Step 1: Add hotkey dependency**

Run: `go get golang.design/x/hotkey`

- [ ] **Step 2: Add hotkey registration logic in main.go**

Add a hotkey manager after the settings service setup in `main.go`:

```go
import "golang.design/x/hotkey"

// ── Global hotkey ────────────────────────────────────────────────────────
// Register global hotkey to toggle popup window visibility.
var currentHotkey *hotkey.Hotkey

func registerHotkey(window *application.WebviewWindow, hotkeyStr string) {
// Unregister previous
if currentHotkey != nil {
currentHotkey.Unregister()
currentHotkey = nil
}

mods, key, ok := parseHotkey(hotkeyStr)
if !ok {
log.Printf("[pager] invalid hotkey string: %s", hotkeyStr)
return
}

hk := hotkey.New(mods, key)
if err := hk.Register(); err != nil {
log.Printf("[pager] hotkey register failed: %v", err)
return
}
currentHotkey = hk

go func () {
for range hk.Keydown() {
if window.IsVisible() {
window.Hide()
} else {
window.Show()
window.Focus()
}
}
}()
}

// parseHotkey converts "Alt+E" format to hotkey modifiers and key.
func parseHotkey(s string) ([]hotkey.Modifier, hotkey.Key, bool) {
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
if len(keyStr) == 1 && keyStr[0] >= 'A' && keyStr[0] <= 'Z' {
key := hotkey.Key(keyStr[0])
return mods, key, true
}

return nil, 0, false
}
```

Add `"strings"` to imports.

- [ ] **Step 3: Call registerHotkey after app starts**

In the `main()` function, after `tray.AttachWindow(window).WindowOffset(5)`, add:

```go
// Register initial hotkey from settings
initialCfg, _ := settings.LoadFrom(settings.DefaultPath())
registerHotkey(window, initialCfg.HotkeyToggle)
```

- [ ] **Step 4: Wire hotkey re-registration in SettingsService onChange**

Update the `NewSettingsService` callback in `main.go`:

```go
settingsSvc := NewSettingsService(func (cfg settings.Settings) {
registerHotkey(window, cfg.HotkeyToggle)
})
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum main.go
git commit -m "feat: add global hotkey registration with Alt+E default"
```

---

## Task 12: i18n Integration in Existing Components

**Files:**

- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/SessionCard.tsx`

- [ ] **Step 1: Add useTranslation to App.tsx header**

In `App.tsx`, add `useTranslation` import and use translated labels:

```tsx
import {useTranslation} from 'react-i18next'
import SessionList from './components/SessionList'
import {useSessionStore, useSessionCounts, type FilterLevel} from './store/sessions'

function App() {
    const {t} = useTranslation()
    const filter = useSessionStore((s) => s.filter)
    const setFilter = useSessionStore((s) => s.setFilter)
    const counts = useSessionCounts()

    return (
        <div className="w-full h-screen bg-[--pager-bg] backdrop-blur-2xl text-[--pager-text] overflow-y-auto rounded-xl border border-[--pager-border]">
            <header className="sticky top-0 z-10 bg-[--pager-header-bg] backdrop-blur-xl px-3 py-2 border-b border-[--pager-border] flex items-center justify-between">
                <h1 className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">{t('session.title')}</h1>
                <div className="flex gap-0.5 bg-[--pager-filter-bg] rounded-[6px] p-[2px] items-center">
                    <FilterDot color="red" count={counts.attention} active={filter === 'attention'} onClick={() => setFilter('attention')}/>
                    <FilterDot color="green" count={counts.running} active={filter === 'running'} onClick={() => setFilter('running')}/>
                    <FilterDot color="gray" count={counts.done} active={filter === 'done'} onClick={() => setFilter('done')}/>
                </div>
            </header>
            <SessionList/>
        </div>
    )
}
```

- [ ] **Step 2: Add i18n to SessionCard expanded section**

In `SessionCard.tsx`, add `useTranslation` and replace hardcoded strings:

```tsx
import {useTranslation} from 'react-i18next'

// Inside the component:
const {t} = useTranslation()

// Replace in the expanded section:
< div > < span
className = "text-[--pager-text-muted]" > {t('session.session'
)
}:</span>
{
    session.SessionID || session.Key
}
</div>
<div><span className="text-[--pager-text-muted]">{t('session.path')}:</span> {session.CWD}</div>
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/SessionCard.tsx
git commit -m "feat(i18n): integrate translations in session popup components"
```

---

## Task 13: Go Notification Language Support

**Files:**

- Modify: `internal/notify/notify.go`

- [ ] **Step 1: Add bilingual notification text**

Add a language-aware text map and modify `Show`:

```go
// Notification text by language
var notifyText = map[string]map[string]string{
"zh": {
"waiting":  "等待确认: ",
"finished": "任务完成",
"error":    " [错误]",
},
"en": {
"waiting":  "Waiting: ",
"finished": "Task completed",
"error":    " [Error]",
},
}

func getText(lang, key string) string {
if m, ok := notifyText[lang]; ok {
if v, ok := m[key]; ok {
return v
}
}
return notifyText["zh"][key]
}
```

- [ ] **Step 2: Add ShowWithLevelAndLang function**

```go
// ShowFull is the main notification entrypoint with all configuration.
func ShowFull(e *event.AgentEvent, level string, lang string) {
if !ShouldNotify(e, level) {
return
}

var title, body string
switch e.EventType {
case event.EventPreToolUse:
title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
body = e.Content
if body == "" {
body = getText(lang, "waiting") + e.ToolName
}
case event.EventStop:
title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
body = getText(lang, "finished")
case event.EventError:
title = fmt.Sprintf("%s · %s%s", agentLabel(e.Agent), lastPath(e.CWD), getText(lang, "error"))
body = e.Content
default:
return
}
showOsascript(title, body)
}
```

- [ ] **Step 3: Update app.go to pass language**

In `app.go`, update the notification call:

```go
if len(sessions) > 0 && sessions[0].LastEvent != nil {
cfg, _ := settings.LoadFrom(settings.DefaultPath())
notify.ShowFull(sessions[0].LastEvent, cfg.NotificationLevel, cfg.Language)
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/notify/notify.go app.go
git commit -m "feat(notify): add bilingual notification text (zh/en)"
```

---

## Task 14: App Icon Generation

**Files:**

- Create: `assets/icon.png`
- Create: `assets/tray-icon.png`
- Create: `assets/tray-icon@2x.png`
- Create: `build/appicon.png`
- Modify: `main.go` (embed and use tray icon)

- [ ] **Step 1: Generate app icon using sips/ImageMagick**

Create a simple SVG and convert to PNG. If ImageMagick is not available, use a Go script:

```bash
mkdir -p assets build
```

Create a Go program to generate the icon (`cmd/icongen/main.go`):

```go
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	size := 1024
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Blue-purple gradient background with squircle shape
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Squircle mask (superellipse n=5)
			nx := float64(x-size/2) / float64(size/2)
			ny := float64(y-size/2) / float64(size/2)
			d := math.Pow(math.Abs(nx), 5) + math.Pow(math.Abs(ny), 5)
			if d > 0.85 {
				img.Set(x, y, color.Transparent)
				continue
			}

			// Gradient: #007aff -> #5856d6 (135 degrees)
			t := (float64(x)/float64(size) + float64(y)/float64(size)) / 2.0
			r := uint8(0 + t*float64(0x58-0x00))
			g := uint8(0x7a + t*float64(0x56-0x7a))
			b := uint8(0xff + t*float64(0xd6-0xff))
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// Draw white "P" letter in center (simplified)
	// For production, use a proper font renderer
	f, _ := os.Create("assets/icon.png")
	defer f.Close()
	png.Encode(f, img)

	// Copy as build icon
	f2, _ := os.Create("build/appicon.png")
	defer f2.Close()
	png.Encode(f2, img)
}
```

Run: `go run cmd/icongen/main.go`

- [ ] **Step 2: Generate tray icons (22×22 and 44×44 monochrome)**

Use sips to resize or create a simple monochrome version:

```bash
# Create a 44x44 monochrome tray icon (black circle with P)
# For now use a placeholder — replace with designed asset later
sips -z 44 44 assets/icon.png --out assets/tray-icon@2x.png 2>/dev/null || cp assets/icon.png assets/tray-icon@2x.png
sips -z 22 22 assets/icon.png --out assets/tray-icon.png 2>/dev/null || cp assets/icon.png assets/tray-icon.png
```

- [ ] **Step 3: Embed tray icon in main.go**

Add at the top of `main.go`:

```go
//go:embed assets/tray-icon@2x.png
var trayIconData []byte
```

Replace `tray.SetTemplateIcon(icons.SystrayMacTemplate)` with:

```go
tray.SetTemplateIcon(trayIconData)
```

Remove the `"github.com/wailsapp/wails/v3/pkg/icons"` import if no longer used elsewhere.

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add assets/ build/appicon.png cmd/icongen/ main.go
git commit -m "feat: add app icon and custom tray icon"
```

---

## Task 15: Wails Bindings Regeneration + Integration Test

**Files:**

- Regenerated: `frontend/src/bindings/`

- [ ] **Step 1: Regenerate Wails bindings**

Run: `wails3 generate bindings`

This should produce `frontend/src/bindings/pager/settingsservice.js` (and `.d.ts`) alongside the existing `sessionservice.js`.

- [ ] **Step 2: Verify the generated bindings include SettingsService methods**

Run: `ls frontend/src/bindings/pager/` — should contain `settingsservice.js`

Run: `grep -l "GetSettings\|UpdateSettings" frontend/src/bindings/pager/settingsservice.*`
Expected: File found with both methods

- [ ] **Step 3: Full build verification**

Run: `cd frontend && npm run build && cd .. && go build ./...`
Expected: Both frontend and Go build succeed

- [ ] **Step 4: Commit**

```bash
git add frontend/src/bindings/
git commit -m "chore: regenerate Wails bindings with SettingsService"
```

---

## Task 16: End-to-End Smoke Test

- [ ] **Step 1: Run dev mode**

Run: `task dev`

Verify:

1. Tray icon appears in menubar
2. Right-click shows "偏好设置..." and "退出 Pager"
3. Clicking "偏好设置..." opens settings window (720×520, standard macOS title bar)
4. Settings window shows sidebar with "基本设置" and "关于"
5. General settings page shows 3 sections: 外观, 快捷键, 通知
6. Theme toggle works (light/dark/system)
7. Language toggle switches all text to English/Chinese
8. About page shows icon, version, links
9. `Alt+E` toggles popup window
10. Notification settings persist across app restart

- [ ] **Step 2: Commit any fixes**

```bash
git add -A
git commit -m "fix: smoke test adjustments"
```
