import { useTranslation } from 'react-i18next'
import { useSettingsStore } from '../../store/settings'
import HotkeyRecorder from '../../components/HotkeyRecorder'

export default function GeneralSettings() {
  const { t } = useTranslation()
  const settings = useSettingsStore((s) => s.settings)
  const updateSettings = useSettingsStore((s) => s.updateSettings)

  return (
    <div>
      <h3 className="text-[17px] font-semibold text-[--pager-text-primary] mb-4">{t('general.title')}</h3>

      <Section label={t('general.appearance')}>
        <Row label={t('general.language')}>
          <select
            value={settings.language}
            onChange={(e) => updateSettings({ language: e.target.value })}
            className="settings-dropdown"
          >
            <option value="zh">中文</option>
            <option value="en">English</option>
          </select>
        </Row>
        <Row label={t('general.theme')} border>
          <div className="settings-segmented">
            {(['light', 'dark', 'system'] as const).map((theme) => (
              <button
                key={theme}
                onClick={() => updateSettings({ theme })}
                className={`settings-segmented-btn ${settings.theme === theme ? 'active' : ''}`}
              >
                {t(`general.theme${theme.charAt(0).toUpperCase() + theme.slice(1)}`)}
              </button>
            ))}
          </div>
        </Row>
        <Row label={t('general.opacity')} last>
          <div className="flex items-center gap-2">
            <input
              type="range"
              min={30}
              max={100}
              value={settings.opacity}
              onChange={(e) => updateSettings({ opacity: Number(e.target.value) })}
              className="w-24 h-1 accent-[#007aff]"
            />
            <span className="text-[11px] text-[--pager-text-muted] w-7 text-right">{settings.opacity}%</span>
          </div>
        </Row>
      </Section>

      <Section label={t('general.hotkeys')}>
        <Row label={t('general.hotkeyToggle')} desc={t('general.hotkeyToggleDesc')} last>
          <HotkeyRecorder
            value={settings.hotkey_toggle}
            onChange={(hotkey) => updateSettings({ hotkey_toggle: hotkey })}
          />
        </Row>
      </Section>


    </div>
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
  label: string; desc?: string; children: React.ReactNode; border?: boolean; last?: boolean
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
