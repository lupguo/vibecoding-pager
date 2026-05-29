import { Clock } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function EmptyState() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col items-center justify-center h-64 text-[--pager-text-faint] gap-3">
      <Clock size={32} strokeWidth={1.5} />
      <span className="text-[13px]">{t('session.empty', 'No active sessions')}</span>
    </div>
  )
}
