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
