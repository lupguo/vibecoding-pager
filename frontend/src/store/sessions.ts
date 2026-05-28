import { create } from 'zustand'

export type SessionStatus = 'waiting' | 'active' | 'finished' | 'error'

export interface Session {
  Key: string
  Agent: string
  Host: string
  CWD: string
  TTY: string
  TermProgram: string
  ITermSessionID: string
  Status: SessionStatus
  LastEvent: {
    Content: string
    ContentRaw: string
    ToolName: string
    EventType: string
  } | null
  UpdatedAt: string
}

interface SessionStore {
  sessions: Session[]
  setSessions: (sessions: Session[]) => void
}

export const useSessionStore = create<SessionStore>((set) => ({
  sessions: [],
  setSessions: (sessions) =>
    set({
      sessions: [...sessions].sort(
        (a, b) => new Date(b.UpdatedAt).getTime() - new Date(a.UpdatedAt).getTime()
      ),
    }),
}))

// Initialize session sync with Wails events.
export function initSessionSync() {
  // Wails runtime injects globals at runtime, try to use them
  try {
    if (typeof window !== 'undefined' && (window as any).runtime) {
      const runtime = (window as any).runtime
      runtime.EventsOn('sessions-updated', (sessions: Session[]) => {
        useSessionStore.getState().setSessions(sessions ?? [])
      })
    }
  } catch {
    console.warn('[pager] Wails runtime not available')
  }
}
