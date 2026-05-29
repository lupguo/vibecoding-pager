import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { useSettingsStore } from '../store/settings'
import GeneralSettings from './settings/GeneralSettings'
import AboutSettings from './settings/AboutSettings'

type Page = 'general' | 'about'

export default function SettingsPanel() {
  const { t } = useTranslation()
  const [page, setPage] = useState<Page>('general')
  const settings = useSettingsStore((s) => s.settings)

  const navItems: { id: Page; icon: string; label: string }[] = [
    { id: 'general', icon: '⚙️', label: t('nav.general') },
    { id: 'about', icon: 'ℹ️', label: t('nav.about') },
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
        className="h-[38px] flex items-center justify-between px-3 border-b border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.06)] bg-[rgba(246,246,248,0.98)] dark:bg-[rgba(30,30,32,0.98)] shrink-0"
        style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
      >
        <span className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">
          {settings.language === 'zh' ? '设置' : 'Settings'}
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
          <div className="text-[11px] font-semibold text-[rgba(0,0,0,0.4)] dark:text-[rgba(255,255,255,0.35)] tracking-wide mb-1 px-3">
            {settings.language === 'zh' ? '通用' : 'GENERAL'}
          </div>
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
              <span className="text-[14px]">{item.icon}</span>
              {item.label}
            </button>
          ))}
        </div>

        <div className="flex-1 p-5 overflow-y-auto bg-[#f5f5f7] dark:bg-[#2a2a2c]">
          {page === 'general' && <GeneralSettings />}
          {page === 'about' && <AboutSettings />}
        </div>
      </div>
    </div>
  )
}
