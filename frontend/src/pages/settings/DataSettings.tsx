import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useSettingsStore } from '../../store/settings'

interface DataStats {
  db_size_bytes: number
  event_count: number
}

export default function DataSettings() {
  const { t, i18n } = useTranslation()
  const settings = useSettingsStore((s) => s.settings)
  const updateSettings = useSettingsStore((s) => s.updateSettings)
  const [stats, setStats] = useState<DataStats>({ db_size_bytes: 0, event_count: 0 })
  const [purging, setPurging] = useState(false)
  const isZh = i18n.language === 'zh'

  const loadStats = async () => {
    try {
      const { GetDataStats } = await import('../../../bindings/github.com/lupguo/vibecoding-pager/internal/wails/settingsbinding.js')
      const s = await GetDataStats()
      setStats(s)
    } catch (err) {
      console.warn('[DataSettings] GetDataStats failed:', err)
    }
  }

  useEffect(() => {
    loadStats()
  }, [])

  const handlePurge = async (days: number) => {
    const msg = days === 0
      ? (isZh ? '确定清理全部数据？此操作不可撤销。' : 'Delete ALL data? This cannot be undone.')
      : (isZh ? `确定清理 ${days} 天前的数据？` : `Delete data older than ${days} days?`)

    if (!confirm(msg)) return

    setPurging(true)
    try {
      const { PurgeData } = await import('../../../bindings/github.com/lupguo/vibecoding-pager/internal/wails/settingsbinding.js')
      await PurgeData(days)
      await loadStats()
    } catch (err) {
      console.error('[DataSettings] PurgeData failed:', err)
    } finally {
      setPurging(false)
    }
  }

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
  }

  return (
    <div>
      <h3 className="text-[17px] font-semibold text-[--pager-text-primary] mb-4">
        {isZh ? '数据管理' : 'Data Management'}
      </h3>

      {/* Session Load Hours */}
      <Section label={isZh ? '启动加载' : 'Startup Loading'}>
        <Row label={isZh ? '加载范围' : 'Load Range'}
             desc={isZh ? '重启应用后，从数据库加载最近 N 小时内的会话' : 'Load sessions from the last N hours on startup'}
             last>
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={1}
              max={168}
              value={settings.session_load_hours || 24}
              onChange={(e) => updateSettings({ session_load_hours: Number(e.target.value) })}
              className="w-16 px-2 py-1 text-[13px] text-center rounded-md border border-[rgba(0,0,0,0.1)] dark:border-[rgba(255,255,255,0.1)] bg-white dark:bg-[rgba(255,255,255,0.05)] text-[--pager-text-primary]"
            />
            <span className="text-[12px] text-[--pager-text-muted]">
              {isZh ? '小时' : 'hours'}
            </span>
          </div>
        </Row>
      </Section>

      {/* Database Stats */}
      <Section label={isZh ? '数据库统计' : 'Database Stats'}>
        <Row label={isZh ? '数据库大小' : 'Database Size'} border>
          <span className="text-[13px] text-[--pager-text-secondary]">
            {formatBytes(stats.db_size_bytes)}
          </span>
        </Row>
        <Row label={isZh ? '事件总数' : 'Total Events'} last>
          <span className="text-[13px] text-[--pager-text-secondary]">
            {stats.event_count.toLocaleString()}
          </span>
        </Row>
      </Section>

      {/* Purge Actions */}
      <Section label={isZh ? '清理数据' : 'Purge Data'}>
        <div className="px-3.5 py-3 space-y-2">
          <p className="text-[11px] text-[--pager-text-muted] mb-2">
            {isZh ? '物理删除指定时间范围的事件和会话数据，操作不可撤销。' : 'Permanently delete events and sessions. This cannot be undone.'}
          </p>
          <div className="flex gap-2">
            <PurgeButton
              onClick={() => handlePurge(7)}
              disabled={purging}
              label={isZh ? '清理 7 天前' : '> 7 days'}
            />
            <PurgeButton
              onClick={() => handlePurge(30)}
              disabled={purging}
              label={isZh ? '清理 30 天前' : '> 30 days'}
            />
            <PurgeButton
              onClick={() => handlePurge(0)}
              disabled={purging}
              label={isZh ? '清理全部' : 'All'}
              danger
            />
          </div>
        </div>
      </Section>
    </div>
  )
}

function PurgeButton({ onClick, disabled, label, danger }: {
  onClick: () => void; disabled: boolean; label: string; danger?: boolean
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`px-3 py-1.5 text-[12px] font-medium rounded-md border transition-colors disabled:opacity-50
        ${danger
          ? 'border-red-300 dark:border-red-800 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950'
          : 'border-[rgba(0,0,0,0.1)] dark:border-[rgba(255,255,255,0.1)] text-[--pager-text-secondary] hover:bg-[rgba(0,0,0,0.04)] dark:hover:bg-[rgba(255,255,255,0.05)]'
        }`}
    >
      {label}
    </button>
  )
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-4">
      <div className="text-[12px] font-semibold text-[--pager-text-muted] mb-1.5 pl-0.5">{label}</div>
      <div className="bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden">
        {children}
      </div>
    </div>
  )
}

function Row({ label, desc, children, border, last }: {
  label: string; desc?: string; children?: React.ReactNode; border?: boolean; last?: boolean
}) {
  return (
    <div className={`flex items-center justify-between px-3.5 py-2.5 ${!last ? 'border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)]' : ''}`}>
      <div>
        <div className="text-[13px] text-[--pager-text-primary]">{label}</div>
        {desc && <div className="text-[11px] text-[--pager-text-muted] mt-0.5">{desc}</div>}
      </div>
      {children}
    </div>
  )
}
