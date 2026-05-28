import SessionList from './components/SessionList'

function App() {
  return (
    <div className="w-full h-screen bg-gray-900/95 backdrop-blur-xl text-white overflow-y-auto rounded-xl border border-gray-700/50">
      <header className="sticky top-0 z-10 bg-gray-900/90 backdrop-blur px-4 py-3 border-b border-gray-700/50">
        <h1 className="text-sm font-semibold text-gray-300">Pager</h1>
      </header>
      <SessionList />
    </div>
  )
}

export default App
