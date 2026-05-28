import SessionList from './components/SessionList'
import { useSessionStore, useSessionCounts, type FilterLevel } from './store/sessions'

function App() {
  const filter = useSessionStore((s) => s.filter)
  const setFilter = useSessionStore((s) => s.setFilter)
  const counts = useSessionCounts()

  return (
    <div className="w-full h-screen bg-[--pager-bg] backdrop-blur-2xl text-[--pager-text] overflow-y-auto rounded-xl border border-[--pager-border]">
      <header className="sticky top-0 z-10 bg-[--pager-header-bg] backdrop-blur-lg px-4 py-2.5 border-b border-[--pager-border] flex items-center justify-between">
        <h1 className="text-[13px] font-semibold text-[--pager-text-primary]">Pager</h1>
        <div className="flex gap-0.5 bg-[--pager-filter-bg] rounded-[7px] p-[2px] items-center">
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
  const textColors = { red: 'text-[--pager-red]', green: 'text-[--pager-text-muted]', gray: 'text-[--pager-text-faint]' }

  return (
    <button onClick={onClick} className={`flex items-center gap-1 px-[7px] py-[3px] rounded-[5px] transition-colors ${active ? 'bg-[--pager-filter-active]' : ''}`}>
      <span className={`w-[7px] h-[7px] rounded-full ${dotColors[color]} ${!active ? 'opacity-60' : ''}`} />
      <span className={`text-[10px] font-medium ${active ? textColors[color] : 'text-[--pager-text-faint]'}`}>{count}</span>
    </button>
  )
}

export default App
