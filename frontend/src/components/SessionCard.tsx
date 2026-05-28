import { useState } from 'react'
import type { Session } from '../store/sessions'
import StatusIcon from './StatusIcon'

interface Props {
  session: Session
}

export default function SessionCard({ session }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [jumping, setJumping] = useState(false)

  const content = session.LastEvent?.Content ?? session.Status
  const contentRaw = session.LastEvent?.ContentRaw ?? ''
  const cwdLast = session.CWD.split('/').filter(Boolean).pop() ?? session.CWD

  const handleJump = async () => {
    setJumping(true)
    try {
      // Wails v3 bindings are available on window at runtime
      const runtime = (window as any).runtime
      if (runtime && runtime.Call) {
        await runtime.Call('main.SessionService.JumpToTerminal', session.Key)
      }
    } catch {
      console.error('Jump failed')
    } finally {
      setJumping(false)
    }
  }

  const relativeTime = formatRelativeTime(session.UpdatedAt)

  return (
    <div className="bg-gray-800/80 rounded-lg p-3 border border-gray-700/50 hover:border-gray-600/50 transition-colors">
      {/* Header row */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2 min-w-0">
          <StatusIcon agent={session.Agent} />
          <span className="text-xs text-gray-400 truncate">
            {agentLabel(session.Agent)} · {cwdLast}
          </span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <StatusBadge status={session.Status} />
          <span className="text-xs text-gray-500">{relativeTime}</span>
        </div>
      </div>

      {/* Content row */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => setExpanded(!expanded)}
          className="text-sm text-gray-200 truncate text-left flex-1 hover:text-white transition-colors"
        >
          {content}
        </button>
        <button
          onClick={handleJump}
          disabled={jumping}
          className="ml-2 px-2 py-1 text-xs bg-blue-600/80 hover:bg-blue-500 rounded text-white disabled:opacity-50 shrink-0 transition-colors"
        >
          {jumping ? '...' : '跳转'}
        </button>
      </div>

      {/* Expanded raw content */}
      {expanded && contentRaw && (
        <pre className="mt-2 p-2 bg-gray-900/80 rounded text-xs text-gray-300 font-mono whitespace-pre-wrap break-all max-h-40 overflow-y-auto">
          {contentRaw}
        </pre>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    waiting: 'bg-red-500/80 text-white',
    active: 'bg-blue-500/80 text-white',
    finished: 'bg-gray-600/80 text-gray-300',
    error: 'bg-orange-500/80 text-white',
  }
  const labels: Record<string, string> = {
    waiting: '等待中',
    active: '执行中',
    finished: '已完成',
    error: '错误',
  }

  return (
    <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${colors[status] ?? colors.finished}`}>
      {labels[status] ?? status}
    </span>
  )
}

function agentLabel(agent: string): string {
  switch (agent) {
    case 'claude-code': return 'Claude Code'
    case 'codex': return 'Codex'
    default: return agent
  }
}

function formatRelativeTime(iso: string): string {
  const now = Date.now()
  const then = new Date(iso).getTime()
  const diffSec = Math.floor((now - then) / 1000)

  if (diffSec < 10) return '刚刚'
  if (diffSec < 60) return `${diffSec}秒前`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}分钟前`
  const d = new Date(iso)
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}
