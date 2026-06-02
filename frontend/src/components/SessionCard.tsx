import { useState } from 'react'
import {
  ArrowRight, AlertTriangle, CheckCircle2,
  Copy, Check, FolderOpen, HandHelping, Hash, Loader2,
} from 'lucide-react'
import type { Session, SessionStatus } from '../store/sessions'
import { JumpToTerminal } from '../../bindings/pager/internal/wails/sessionbinding.js'

const COPY_FEEDBACK_MS = 800

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

function MetaRow({
  Icon, label, value, copyable, inline,
}: {
  Icon: typeof FolderOpen
  label: string
  value: string
  copyable?: boolean
  /**
   * When true, render in a horizontal "footer" style: no uppercase label
   * text (the icon carries the meaning), tighter padding, Copy button only
   * appears when the parent row is hovered. Used in the compact expanded
   * card footer; existing default-block callers leave this unset.
   */
  inline?: boolean
}) {
  const [copied, setCopied] = useState(false)
  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!value) return
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), COPY_FEEDBACK_MS)
    } catch (err) {
      console.error('[pager] copy failed:', err)
    }
  }

  if (inline) {
    return (
      <div
        className="group/metarow flex items-center gap-[5px] text-[10px] text-[--pager-text-muted] min-w-0"
        title={label}>
        <Icon size={11} className="opacity-65 shrink-0" strokeWidth={2} />
        <span
          className="font-mono text-[--pager-text-secondary] truncate min-w-0"
          title={value}>
          {value || '—'}
        </span>
        <button
          onClick={handleCopy}
          className={`w-[16px] h-[16px] flex items-center justify-center rounded-[3px] text-[--pager-text-faint] hover:bg-[rgba(255,255,255,0.06)] hover:text-[--pager-text-secondary] shrink-0 transition-opacity opacity-0 group-hover/metarow:opacity-100 ${
            !copyable || !value ? 'invisible' : ''
          }`}
          title="复制">
          {copied ? <Check size={10} strokeWidth={2.5} /> : <Copy size={10} strokeWidth={2} />}
        </button>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-[6px] px-[8px] py-[3px] text-[10px] leading-[1.6] text-[--pager-text-secondary]">
      <Icon size={11} className="opacity-65 shrink-0 text-[--pager-text-muted]" strokeWidth={2} />
      <span className="text-[9px] uppercase tracking-[0.3px] text-[--pager-text-muted] w-[60px] shrink-0">
        {label}
      </span>
      <span className="font-mono text-[--pager-text-primary] truncate flex-1" title={value}>
        {value || '—'}
      </span>
      <button
        onClick={handleCopy}
        className={`w-[18px] h-[18px] flex items-center justify-center rounded-[3px] text-[--pager-text-faint] hover:bg-[rgba(255,255,255,0.06)] hover:text-[--pager-text-secondary] shrink-0 transition-colors ${
          !copyable || !value ? 'invisible' : ''
        }`}
        title="复制">
        {copied ? <Check size={11} strokeWidth={2.5} /> : <Copy size={11} strokeWidth={2} />}
      </button>
    </div>
  )
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
  const eventType = session.LastEvent?.event_type ?? ''

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
          {eventType && (
            <span className="text-[9px] font-medium px-[5px] py-[1px] rounded-[3px] tracking-wide border border-[--pager-border] text-[--pager-text-secondary]">
              {eventType}
            </span>
          )}
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
        {expanded && (
          <div className="mt-[6px] pl-[18px]">
            {session.LastEvent?.content_raw && (
              <pre className="px-[6px] py-[5px] bg-[--pager-detail-bg] border border-[--pager-detail-border] rounded-[6px] text-[10px] leading-relaxed text-[--pager-detail-text] font-mono whitespace-pre-wrap break-all max-h-32 overflow-y-auto">
                {session.LastEvent.content_raw}
              </pre>
            )}
            <div className="flex items-center gap-[14px] mt-[5px] pt-[5px] border-t border-dashed border-[rgba(255,255,255,0.08)] min-w-0">
              <MetaRow Icon={FolderOpen} label="PATH" value={session.CWD || ''} copyable inline />
              <MetaRow Icon={Hash} label="SESSION" value={session.SessionID || session.Key || ''} copyable inline />
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
