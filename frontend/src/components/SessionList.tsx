import { useSessionStore } from '../store/sessions'
import SessionCard from './SessionCard'

export default function SessionList() {
  const filteredSessions = useSessionStore((s) => s.filteredSessions())

  if (filteredSessions.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-[--pager-text-faint] gap-3">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z" />
          <path d="M12 6v6l4 2" />
        </svg>
        <span className="text-[13px]">No sessions in this view</span>
      </div>
    )
  }

  return (
    <div className="p-[10px] space-y-[5px]">
      {filteredSessions.map((session) => (
        <SessionCard key={session.Key} session={session} />
      ))}
    </div>
  )
}
