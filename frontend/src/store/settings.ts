import { create } from 'zustand'
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
      const { GetSettings } = await import('../../bindings/pager/settingsservice.js')
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

    if (partial.language) i18n.changeLanguage(partial.language)
    if (partial.theme) applyTheme(partial.theme)
    if (partial.opacity !== undefined) applyOpacity(partial.opacity)

    try {
      const { UpdateSettings } = await import('../../bindings/pager/settingsservice.js')
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
}

function applyOpacity(opacity: number) {
  document.documentElement.style.setProperty('--pager-opacity', String(opacity / 100))
}

export function initSettings() {
  useSettingsStore.getState().loadSettings()
}
