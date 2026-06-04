import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { useSessionStore, projectFromCWD } from './sessions'

export const HIGHLIGHT_DURATION_MS = 1500
export const COLLAPSED_DEBOUNCE_MS = 250

interface UIState {
  collapsedProjects: Set<string>
  highlightedKey: string | null

  initCollapsed: (list: string[]) => void
  toggleCollapsed: (project: string) => void
  isCollapsed: (project: string) => boolean

  setHighlighted: (key: string) => void
  clearHighlighted: () => void
}

export const useUIStore = create<UIState>((set, get) => ({
  collapsedProjects: new Set(),
  highlightedKey: null,

  initCollapsed: (list) => set({ collapsedProjects: new Set(list) }),

  toggleCollapsed: (project) => {
    const next = new Set(get().collapsedProjects)
    if (next.has(project)) next.delete(project)
    else next.add(project)
    set({ collapsedProjects: next })
    persistCollapsed(Array.from(next))
  },

  isCollapsed: (project) => get().collapsedProjects.has(project),

  setHighlighted: (key) => {
    set({ highlightedKey: key })
    setTimeout(() => {
      if (get().highlightedKey === key) {
        set({ highlightedKey: null })
      }
    }, HIGHLIGHT_DURATION_MS)
  },

  clearHighlighted: () => set({ highlightedKey: null }),
}))

let persistTimer: ReturnType<typeof setTimeout> | null = null

function persistCollapsed(list: string[]) {
  if (persistTimer) clearTimeout(persistTimer)
  persistTimer = setTimeout(async () => {
    try {
      const { SetCollapsedProjects } = await import(
        '../../bindings/github.com/lupguo/vibecoding-pager/internal/wails/windowbinding.js'
      )
      await SetCollapsedProjects(list)
    } catch (err) {
      console.error('[pager] SetCollapsedProjects failed:', err)
    }
  }, COLLAPSED_DEBOUNCE_MS)
}

/** Call once at app boot — wires the highlight-session event from Go. */
export function initUIState() {
  Events.On('highlight-session', (ev: any) => {
    const sessionID =
      ev?.data?.session_id ?? ev?.data?.[0]?.session_id ?? ev?.session_id
    if (!sessionID) return
    const sessions = useSessionStore.getState().sessions
    const session = sessions.find(
      (s) => s.Key === sessionID || s.SessionID === sessionID
    )
    if (!session) return

    const project = projectFromCWD(session.CWD)
    const ui = useUIStore.getState()
    if (ui.isCollapsed(project)) {
      ui.toggleCollapsed(project)
    }
    ui.setHighlighted(session.Key)
  })
}
