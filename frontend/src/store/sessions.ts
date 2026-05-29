import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { ListSessions } from '../../bindings/pager/internal/wails/sessionbinding.js'

export type AttentionLevel = 'attention' | 'running' | 'done'
export type FilterLevel = AttentionLevel

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

interface SessionStore {
  sessions: Session[]
  filter: FilterLevel
  setSessions: (sessions: Session[]) => void
  setFilter: (filter: FilterLevel) => void
}

export const useSessionStore = create<SessionStore>((set) => ({
  sessions: [],
  filter: 'attention' as FilterLevel,

  setSessions: (sessions) =>
    set({
      sessions: [...sessions].sort(
        (a, b) => new Date(b.UpdatedAt).getTime() - new Date(a.UpdatedAt).getTime()
      ),
    }),

  setFilter: (filter) => set({ filter }),
}))

export function useSessionCounts() {
  const sessions = useSessionStore((s) => s.sessions)
  return {
    attention: sessions.filter((s) => s.AttentionLevel === 'attention').length,
    running: sessions.filter((s) => s.AttentionLevel === 'running').length,
    done: sessions.filter((s) => s.AttentionLevel === 'done').length,
  }
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

    // Defer state update to next microtask to escape WKWebView's evaluateJavaScript
    // synchronous execution context. Without this, React's useSyncExternalStore
    // cannot properly schedule re-renders from zustand state changes.
    queueMicrotask(() => {
      useSessionStore.getState().setSessions(sessions)
    })
  })
}
