import { useState } from 'react'
import type { Session } from '../store/sessions'
import { JumpToTerminal } from '../../bindings/pager/sessionservice.js'

interface Props {
  session: Session
}

export default function SessionCard({ session }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [jumping, setJumping] = useState(false)

  const content = session.LastEvent?.content ?? session.Status
  const contentRaw = session.LastEvent?.content_raw ?? ''
  const toolName = session.LastEvent?.tool_name ?? ''
  const cwdLast = session.CWD.split('/').filter(Boolean).pop() ?? session.CWD
  const sessionPrefix = (session.SessionID || session.Key || '').slice(0, 8)
  const agentLabel = session.AgentLabel || 'CC'
  const isAttention = session.AttentionLevel === 'attention'

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

  const cardBg = isAttention
    ? 'bg-[--pager-card-attention-bg] border-[--pager-card-attention-border]'
    : session.AttentionLevel === 'running'
    ? 'bg-[--pager-card-running-bg] border-[--pager-card-running-border]'
    : 'bg-[--pager-card-done-bg] border-[--pager-card-done-border] opacity-60'

  return (
    <div className={`rounded-lg border cursor-pointer transition-all duration-150 ${cardBg}`} onClick={() => setExpanded(!expanded)}>
      <div className="px-[10px] py-[9px]">
        {/* Row 1: Agent badge + project + session_id + time */}
        <div className="flex items-center gap-[5px] mb-[5px]">
          <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide">{agentLabel}</span>
          <span className="text-[10px] text-[--pager-text-secondary]">{cwdLast}</span>
          <span className="flex-1" />
          <span className="text-[10px] text-[--pager-text-faint]">{sessionPrefix}</span>
          <span className="text-[10px] text-[--pager-text-faint]">·</span>
          <span className="text-[10px] text-[--pager-text-faint]">{relativeTime}</span>
        </div>

        {/* Row 2: Tool tag + content + arrow */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-[6px] flex-1 min-w-0">
            {toolName && (
              <span className="text-[10px] font-mono px-[5px] py-[1px] bg-[--pager-tool-bg] rounded-[3px] text-[--pager-tool-text] uppercase shrink-0 tracking-wide">
                {toolName.length > 10 ? toolName.slice(0, 10) : toolName}
              </span>
            )}
            <span className="text-[12px] text-[--pager-text-primary] whitespace-nowrap overflow-hidden text-ellipsis">{content}</span>
          </div>
          <button onClick={handleJump} disabled={jumping} title="Jump to terminal" className="ml-[6px] p-[3px] rounded text-[--pager-text-faint] hover:text-[--pager-text-secondary] disabled:opacity-30 shrink-0 transition-colors">
            {jumping ? (
              <svg className="w-[14px] h-[14px] animate-spin" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="8" /></svg>
            ) : (
              <svg className="w-[14px] h-[14px]" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M3 8h10M9 4l4 4-4 4" /></svg>
            )}
          </button>
        </div>

        {/* Expanded content */}
        {expanded && (
          <div className="mt-2">
            {contentRaw && (
              <pre className="p-[8px] bg-[--pager-detail-bg] rounded-md text-[11px] leading-relaxed text-[--pager-detail-text] font-mono whitespace-pre-wrap break-all max-h-36 overflow-y-auto border border-[--pager-detail-border] mb-2">{contentRaw}</pre>
            )}
            <div className="text-[9px] font-mono text-[--pager-text-faint] leading-[1.7]">
              <div><span className="text-[--pager-text-muted]">session:</span> {session.SessionID || session.Key}</div>
              <div><span className="text-[--pager-text-muted]">path:</span> {session.CWD}</div>
            </div>
          </div>
        )}
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
