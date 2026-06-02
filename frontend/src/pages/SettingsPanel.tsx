import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { X, Settings as SettingsIcon, Bell, Database, Info } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useSettingsStore } from '../store/settings'
import GeneralSettings from './settings/GeneralSettings'
import NotificationSettings from './settings/NotificationSettings'
import DataSettings from './settings/DataSettings'
import AboutSettings from './settings/AboutSettings'

type Page = 'general' | 'notifications' | 'data' | 'about'

export default function SettingsPanel() {
  const { t, i18n } = useTranslation()
  const [page, setPage] = useState<Page>('general')
  const settings = useSettingsStore((s) => s.settings)
  const isZh = i18n.language === 'zh'

  const navItems: { id: Page; Icon: LucideIcon; label: string }[] = [
    { id: 'general',       Icon: SettingsIcon, label: t('nav.general') },
    { id: 'notifications', Icon: Bell,         label: t('nav.notifications') },
    { id: 'data',          Icon: Database,     label: isZh ? '数据' : 'Data' },
    { id: 'about',         Icon: Info,         label: t('nav.about') },
  ]

  const handleClose = async () => {
    try {
      const { Hide } = await import('../../bindings/pager/internal/wails/windowbinding.js')
      await Hide()
    } catch {
      // Fallback: use window.close or just ignore
    }
  }

  return (
    <div className="w-full h-screen flex flex-col bg-[#f5f5f7] dark:bg-[#1e1e20] text-[--pager-text]"
         style={{ fontFamily: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif" }}>
      {/* Custom titlebar — draggable */}
      <div
        className="h-[38px] flex items-center justify-between px-3 border-b border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.06)] bg-[rgba(246,246,248,0.98)] dark:bg-[rgba(30,30,32,0.98)] shrink-0 select-none cursor-default"
        style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
      >
        <span className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">
          {isZh ? '设置' : 'Settings'}
        </span>
        <button
          onClick={handleClose}
          className="w-5 h-5 flex items-center justify-center rounded hover:bg-[rgba(0,0,0,0.06)] dark:hover:bg-[rgba(255,255,255,0.08)] transition-colors"
          style={{ '--wails-draggable': 'none' } as React.CSSProperties}
          title="Close"
        >
          <X size={12} className="text-[--pager-text-muted]" />
        </button>
      </div>

      {/* Content area */}
      <div className="flex flex-1 overflow-hidden">
        <div className="w-[180px] min-w-[180px] bg-[rgba(246,246,248,0.98)] dark:bg-[rgba(30,30,32,0.98)] border-r border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.06)] p-3 pt-2">
          {navItems.map((item) => (
            <button
              key={item.id}
              onClick={() => setPage(item.id)}
              className={`w-full text-left px-3 py-1.5 my-0.5 rounded-md text-[13px] flex items-center gap-2 transition-colors
                ${page === item.id
                  ? 'bg-[#007aff] text-white font-medium'
                  : 'text-[rgba(0,0,0,0.75)] dark:text-[rgba(255,255,255,0.6)] hover:bg-[rgba(0,0,0,0.04)] dark:hover:bg-[rgba(255,255,255,0.05)]'
                }`}
            >
              <item.Icon size={14} strokeWidth={2} className="shrink-0" />
              {item.label}
            </button>
          ))}
        </div>

        <div className="flex-1 p-5 overflow-y-auto bg-[#f5f5f7] dark:bg-[#2a2a2c]">
          {page === 'general' && <GeneralSettings />}
          {page === 'notifications' && <NotificationSettings />}
          {page === 'data' && <DataSettings />}
          {page === 'about' && <AboutSettings />}
        </div>
      </div>
    </div>
  )
}
