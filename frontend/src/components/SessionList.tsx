import { useSessionStore } from '../store/sessions'
import SessionCard from './SessionCard'

export default function SessionList() {
  const sessions = useSessionStore((s) => s.sessions)

  if (sessions.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500 text-sm">
        暂无活跃会话
      </div>
    )
  }

  return (
    <div className="p-2 space-y-2">
      {sessions.map((session) => (
        <SessionCard key={session.Key} session={session} />
      ))}
    </div>
  )
}
