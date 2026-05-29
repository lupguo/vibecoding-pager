import { useState, useEffect } from 'react'
import { Pin, Settings } from 'lucide-react'
import SessionList from './components/SessionList'
import { useSettingsStore } from './store/settings'

function App() {
  const settings = useSettingsStore((s) => s.settings)
  const [pinned, setPinned] = useState(settings.popup_pinned ?? false)

  useEffect(() => {
    setPinned(settings.popup_pinned ?? false)
  }, [settings.popup_pinned])

  const handleTogglePin = async () => {
    const newPinned = !pinned
    setPinned(newPinned)
    try {
      const { SetPinned } = await import('../bindings/pager/internal/wails/windowbinding.js')
      await SetPinned(newPinned)
    } catch (err) {
      console.error('SetPinned failed:', err)
      setPinned(!newPinned)
    }
  }

  const handleOpenSettings = async () => {
    try {
      const { OpenSettings } = await import('../bindings/pager/internal/wails/windowbinding.js')
      await OpenSettings()
    } catch (err) {
      console.error('OpenSettings failed:', err)
    }
  }

  return (
    <div className="w-full h-screen bg-[--pager-bg] backdrop-blur-2xl text-[--pager-text] overflow-hidden rounded-xl border border-[--pager-border] flex flex-col">
      {/* Navbar — drag region */}
      <header
        className="sticky top-0 z-10 bg-[--pager-header-bg] backdrop-blur-xl px-3 py-2.5 border-b border-[--pager-border] flex items-center justify-between"
        style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
      >
        {/* Left: Pin button */}
        <button
          onClick={handleTogglePin}
          className={`w-6 h-6 flex items-center justify-center rounded-[5px] transition-all ${
            pinned ? 'bg-[rgba(10,132,255,0.12)]' : 'hover:bg-[--pager-filter-bg]'
          }`}
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          title={pinned ? 'Unpin window' : 'Pin window on top'}
        >
          <Pin
            size={14}
            className={`transition-all ${pinned ? 'text-[--pager-blue] rotate-0' : 'text-[--pager-text-muted] -rotate-45'}`}
            strokeWidth={pinned ? 2.5 : 2}
          />
        </button>

        {/* Center: Title */}
        <span className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">
          Pager
        </span>

        {/* Right: Settings gear */}
        <button
          onClick={handleOpenSettings}
          className="w-6 h-6 flex items-center justify-center rounded-[5px] hover:bg-[--pager-filter-bg] transition-all"
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          title="Settings"
        >
          <Settings size={14} className="text-[--pager-text-muted]" strokeWidth={2} />
        </button>
      </header>

      {/* Session list — scrollable */}
      <div className="flex-1 overflow-y-auto">
        <SessionList />
      </div>
    </div>
  )
}

export default App
