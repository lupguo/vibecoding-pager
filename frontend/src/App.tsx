import SessionList from './components/SessionList'
import { useSessionStore, useSessionCounts, type FilterLevel } from './store/sessions'
import { useTranslation } from 'react-i18next'

function App() {
  const filter = useSessionStore((s) => s.filter)
  const setFilter = useSessionStore((s) => s.setFilter)
  const counts = useSessionCounts()
  const { t } = useTranslation()

  return (
    <div className="w-full h-screen bg-[--pager-bg] backdrop-blur-2xl text-[--pager-text] overflow-y-auto rounded-xl border border-[--pager-border]">
      <header className="sticky top-0 z-10 bg-[--pager-header-bg] backdrop-blur-xl px-3 py-2 border-b border-[--pager-border] flex items-center justify-between">
        <h1 className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">{t('session.title')}</h1>
        <div className="flex gap-0.5 bg-[--pager-filter-bg] rounded-[6px] p-[2px] items-center">
          <FilterDot color="red" count={counts.attention} active={filter === 'attention'} onClick={() => setFilter('attention')} />
          <FilterDot color="green" count={counts.running} active={filter === 'running'} onClick={() => setFilter('running')} />
          <FilterDot color="gray" count={counts.done} active={filter === 'done'} onClick={() => setFilter('done')} />
        </div>
      </header>
      <SessionList />
    </div>
  )
}

function FilterDot({ color, count, active, onClick }: {
  color: 'red' | 'green' | 'gray'
  count: number
  active: boolean
  onClick: () => void
}) {
  const dotColors = { red: 'bg-[--pager-red]', green: 'bg-[--pager-green]', gray: 'bg-[--pager-gray-dot]' }
  const textColors = { red: 'text-[--pager-red]', green: 'text-[--pager-text-secondary]', gray: 'text-[--pager-text-muted]' }

  return (
    <button onClick={onClick} className={`flex items-center gap-[3px] px-[6px] py-[2px] rounded-[4px] transition-all ${active ? 'bg-[--pager-filter-active] shadow-sm' : 'hover:bg-[--pager-filter-active]'}`}>
      <span className={`w-[6px] h-[6px] rounded-full ${dotColors[color]} ${!active ? 'opacity-50' : ''}`} />
      <span className={`text-[10px] font-medium tabular-nums ${active ? textColors[color] : 'text-[--pager-text-faint]'}`}>{count}</span>
    </button>
  )
}

export default App
