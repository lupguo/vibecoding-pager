import { useTranslation } from 'react-i18next'

export default function AboutSettings() {
  const { t } = useTranslation()

  const openURL = (url: string) => {
    window.open(url, '_blank')
  }

  return (
    <div className="flex flex-col items-center justify-center h-full">
      <div className="w-[72px] h-[72px] mb-3 rounded-[16px] flex items-center justify-center shadow-lg"
           style={{ background: 'linear-gradient(135deg, #007aff, #5856d6)' }}>
        <span className="text-[36px]">📟</span>
      </div>
      <div className="text-[18px] font-semibold text-[--pager-text-primary] mb-0.5">Pager</div>
      <div className="text-[12px] text-[--pager-text-muted] mb-6">{t('about.description')}</div>

      <div className="w-full max-w-[320px] bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden mb-4">
        <div className="flex justify-between items-center px-3.5 py-2.5 border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)]">
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.version')}</span>
          <span className="text-[12px] text-[--pager-text-muted]">1.0.0 (build 1)</span>
        </div>
        <div className="flex justify-between items-center px-3.5 py-2.5">
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.author')}</span>
          <a className="text-[12px] text-[#007aff] cursor-pointer hover:underline"
             onClick={() => openURL('https://github.com/sapaude')}>sapaude</a>
        </div>
      </div>

      <div className="w-full max-w-[320px] bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden mb-4">
        <div className="flex justify-between items-center px-3.5 py-2.5 border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)] cursor-pointer hover:bg-[rgba(0,0,0,0.02)] dark:hover:bg-[rgba(255,255,255,0.02)]"
             onClick={() => openURL('https://github.com/sapaude/pager/releases')}>
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.checkUpdate')}</span>
          <span className="text-[12px] text-[--pager-text-faint]">›</span>
        </div>
        <div className="flex justify-between items-center px-3.5 py-2.5 cursor-pointer hover:bg-[rgba(0,0,0,0.02)] dark:hover:bg-[rgba(255,255,255,0.02)]"
             onClick={() => openURL('https://github.com/sapaude/pager')}>
          <span className="text-[13px] text-[--pager-text-primary]">{t('about.github')}</span>
          <span className="text-[12px] text-[--pager-text-faint]">↗</span>
        </div>
      </div>

      <div className="text-[11px] text-[--pager-text-faint]">{t('about.copyright')}</div>
    </div>
  )
}
