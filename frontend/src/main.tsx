import './i18n'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { HashRouter, Routes, Route } from 'react-router-dom'
import App from './App'
import SettingsPanel from './pages/SettingsPanel'
import './index.css'
import { initSessionSync } from './store/sessions'

initSessionSync()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HashRouter>
      <Routes>
        <Route path="/" element={<App />} />
        <Route path="/settings" element={<SettingsPanel />} />
      </Routes>
    </HashRouter>
  </React.StrictMode>,
)
