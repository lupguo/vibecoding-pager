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
    // Wails v3 event data may be wrapped in different structures
    const cfg = ev?.data?.[0] ?? ev?.data ?? ev
    if (!cfg || typeof cfg !== 'object') return
    useSettingsStore.setState({ settings: cfg })
    if (cfg.language) i18n.changeLanguage(cfg.language)
    if (cfg.theme) applyTheme(cfg.theme)
    if (cfg.opacity !== undefined) applyOpacity(cfg.opacity)
  })
}
