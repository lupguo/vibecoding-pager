import { useEffect, useRef } from 'react'
import { BrushCleaning, ChevronDown, ChevronRight, Folder } from 'lucide-react'
import { useProjectGroups } from '../store/sessions'
import { useUIStore } from '../store/uistate'
import { useSettingsStore } from '../store/settings'
import SessionCard from './SessionCard'
import EmptyState from './EmptyState'

export default function SessionList() {
  const groups = useProjectGroups()
  const collapsedProjects = useUIStore((s) => s.collapsedProjects)
  const toggleCollapsed = useUIStore((s) => s.toggleCollapsed)
  const highlightedKey = useUIStore((s) => s.highlightedKey)
  const initCollapsed = useUIStore((s) => s.initCollapsed)
  const settings = useSettingsStore((s) => s.settings)
  const settingsLoaded = useSettingsStore((s) => s.loaded)

  const hydrated = useRef(false)
  useEffect(() => {
    if (!hydrated.current && settingsLoaded) {
      initCollapsed(settings.collapsed_projects ?? [])
      hydrated.current = true
    }
  }, [settingsLoaded, settings.collapsed_projects, initCollapsed])

  const cardRefs = useRef(new Map<string, HTMLDivElement>())
  useEffect(() => {
    if (!highlightedKey) return
    const el = cardRefs.current.get(highlightedKey)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }, [highlightedKey])

  if (groups.length === 0) {
    return <EmptyState />
  }

  return (
    <div className="p-[10px] space-y-[12px]">
      {groups.map((group) => {
        const collapsed = collapsedProjects.has(group.project)
        return (
          <div key={group.project}>
            <div
              className="group flex items-center gap-[6px] px-[4px] mb-[6px] cursor-pointer select-none"
              onClick={() => toggleCollapsed(group.project)}>
              {collapsed ? (
                <ChevronRight size={10} className="text-[--pager-text-muted]" strokeWidth={2.5} />
              ) : (
                <ChevronDown size={10} className="text-[--pager-text-muted]" strokeWidth={2.5} />
              )}
              <Folder size={11} className="text-[--pager-text-muted] opacity-60" strokeWidth={2} />
              <span className="text-[11px] font-semibold text-[--pager-text-muted] tracking-[0.3px]">
                {group.project}
              </span>
              <span className="text-[9px] bg-[rgba(255,255,255,0.06)] px-[5px] py-[1px] rounded-[3px] text-[--pager-text-faint]">
                {group.sessions.length}
              </span>
              <span className="flex-1 h-px bg-[--pager-border] opacity-50" />
              <button
                className="opacity-0 group-hover:opacity-100 w-[22px] h-[22px] flex items-center justify-center rounded-[4px] text-[--pager-text-faint] hover:bg-[--pill-error] hover:text-[--c-error] transition-colors"
                title="清理该项目历史会话"
                onClick={async (e) => {
                  e.stopPropagation()
                  try {
                    const { DismissSessionsByProject } = await import(
                      '../../bindings/github.com/lupguo/vibecoding-pager/internal/wails/sessionbinding.js'
                    )
                    await DismissSessionsByProject(group.project)
                  } catch (err) {
                    console.error('[pager] DismissSessionsByProject failed:', err)
                  }
                }}>
                <BrushCleaning size={11} strokeWidth={2} />
              </button>
            </div>
            {!collapsed && (
              <div className="space-y-[5px]">
                {group.sessions.map((session) => (
                  <div
                    key={session.Key}
                    ref={(el) => {
                      if (el) cardRefs.current.set(session.Key, el)
                      else cardRefs.current.delete(session.Key)
                    }}>
                    <SessionCard session={session} highlighted={highlightedKey === session.Key} />
                  </div>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
