import { useState } from 'react'
import {
  ArrowRight, AlertTriangle, CheckCircle2, HandHelping, Loader2,
} from 'lucide-react'
import type { Session, SessionStatus } from '../store/sessions'
import { JumpToTerminal } from '../../bindings/pager/internal/wails/sessionbinding.js'

interface Props {
  session: Session
  highlighted?: boolean
}

const STATUS_TAG: Record<SessionStatus, { text: string; bgClass: string; textClass: string }> = {
  working: { text: 'WORKING', bgClass: 'bg-[--pill-working]', textClass: 'text-[--c-working]' },
  waiting: { text: 'WAITING', bgClass: 'bg-[--pill-waiting]', textClass: 'text-[--c-waiting]' },
  done:    { text: 'DONE',    bgClass: 'bg-[--pill-done]',    textClass: 'text-[--c-done]' },
  error:   { text: 'ERROR',   bgClass: 'bg-[--pill-error]',   textClass: 'text-[--c-error]' },
}

const CARD_BG: Record<SessionStatus, string> = {
  working: 'bg-[--bg-working] border-[--bd-working]',
  waiting: 'bg-[--bg-waiting] border-[--bd-waiting] shadow-sm',
  done:    'bg-[--bg-done] border-[--bd-done] opacity-90',
  error:   'bg-[--bg-error] border-[--bd-error]',
}

function StatusIcon({ status }: { status: SessionStatus }) {
  const cls = `text-[--c-${status}]`
  switch (status) {
    case 'waiting': return <HandHelping size={14} strokeWidth={2} className={`${cls} anim-pulse-attn`} />
    case 'working': return <Loader2 size={14} strokeWidth={2.5} className={`${cls} anim-spin-loader`} />
    case 'done':    return <CheckCircle2 size={14} strokeWidth={2.5} className={cls} />
    case 'error':   return <AlertTriangle size={14} strokeWidth={2.5} className={cls} />
  }
}

export default function SessionCard({ session, highlighted }: Props) {
  const [jumping, setJumping] = useState(false)
  const [expanded, setExpanded] = useState(false)

  const status: SessionStatus = (session.Status as SessionStatus) || 'working'
  const tag = STATUS_TAG[status]
  const bg = CARD_BG[status]

  const content = session.LastEvent?.content ?? ''
  const toolName = session.LastEvent?.tool_name ?? ''
  const sessionPrefix = (session.SessionID || session.Key || '').slice(0, 8)
  const agentLabel = session.AgentLabel || 'CC'

  const handleJump = async (e: React.MouseEvent) => {
    e.stopPropagation()
    setJumping(true)
    try {
      await JumpToTerminal(session.Key)
    } catch (err) {
      console.error('Jump failed:', err)
    } finally {
      setJumping(false)
    }
  }

  const relativeTime = formatRelativeTime(session.UpdatedAt)

  return (
    <div
      className={`rounded-lg border cursor-pointer transition-all duration-300 hover:shadow-sm ${bg} ${highlighted ? 'pulse-highlight' : ''}`}
      onClick={() => setExpanded(!expanded)}>
      <div className="px-[10px] py-[8px]">
        <div className="flex items-center gap-[6px] mb-[4px]">
          <StatusIcon status={status} />
          <span className={`text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] tracking-wide ${tag.bgClass} ${tag.textClass}`}>
            {tag.text}
          </span>
          <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide uppercase">
            {agentLabel}
          </span>
          <span className="flex-1" />
          <span className="text-[9px] text-[--pager-text-faint] font-mono">{sessionPrefix}</span>
          <span className="text-[9px] text-[--pager-text-faint]">{relativeTime}</span>
        </div>
        <div className="flex items-center justify-between pl-[22px]">
          <div className="flex items-center gap-[6px] flex-1 min-w-0">
            {toolName && (
              <span className="text-[9px] font-mono font-medium px-[4px] py-[1px] bg-[--pager-tool-bg] rounded-[3px] text-[--pager-tool-text] shrink-0">
                {toolName.length > 12 ? toolName.slice(0, 12) : toolName}
              </span>
            )}
            <span className="text-[11px] text-[--pager-text-primary] whitespace-nowrap overflow-hidden text-ellipsis">
              {content}
            </span>
          </div>
          <button
            onClick={handleJump}
            disabled={jumping}
            title="Jump to terminal"
            className="ml-[6px] p-[3px] rounded text-[--pager-text-faint] hover:text-[--pager-blue] hover:bg-[--pager-filter-bg] disabled:opacity-30 shrink-0 transition-colors">
            {jumping ? (
              <Loader2 size={13} className="animate-spin" />
            ) : (
              <ArrowRight size={13} />
            )}
          </button>
        </div>
        {/* Expanded section is added in Task 18 */}
      </div>
    </div>
  )
}

function formatRelativeTime(iso: string): string {
  if (!iso) return ''
  const now = Date.now()
  const then = new Date(iso).getTime()
  if (isNaN(then)) return ''
  const diffSec = Math.floor((now - then) / 1000)
  if (diffSec < 5) return 'now'
  if (diffSec < 60) return `${diffSec}s`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}
