import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { ListSessions } from '../../bindings/pager/internal/wails/sessionbinding.js'

export type SessionStatus = 'working' | 'waiting' | 'done' | 'error'

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
  Status: SessionStatus
  AgentLabel: string
  SessionID: string
  LastEvent: AgentEvent | null
  // PendingTools is intentionally omitted — backend marks it `json:"-"`
  // because it is internal state-machine bookkeeping, not consumed by UI.
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
    if (s.Status !== 'done') return true
    const updatedAt = new Date(s.UpdatedAt).getTime()
    return now - updatedAt < DONE_TIMEOUT_MS
  })
}

/** Extract project name from CWD (last path segment) */
export function projectFromCWD(cwd: string): string {
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

  const groups: ProjectGroup[] = Array.from(groupMap.entries()).map(([project, sessions]) => ({
    project,
    sessions,
  }))

  // Sort projects alphabetically by name
  groups.sort((a, b) => a.project.localeCompare(b.project))

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
