import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

interface Props {
  value: string
  onChange: (hotkey: string) => void
}

export default function HotkeyRecorder({ value, onChange }: Props) {
  const { t } = useTranslation()
  const [recording, setRecording] = useState(false)

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    e.preventDefault()
    e.stopPropagation()

    if (e.key === 'Escape') {
      setRecording(false)
      return
    }

    const parts: string[] = []
    if (e.ctrlKey) parts.push('Ctrl')
    if (e.altKey) parts.push('Alt')
    if (e.shiftKey) parts.push('Shift')
    if (e.metaKey) parts.push('Cmd')

    const key = e.key.length === 1 ? e.key.toUpperCase() : e.key
    if (parts.length === 0) return
    if (['Control', 'Alt', 'Shift', 'Meta'].includes(e.key)) return

    parts.push(key)
    onChange(parts.join('+'))
    setRecording(false)
  }, [onChange])

  return (
    <div
      onClick={() => setRecording(true)}
      onKeyDown={recording ? handleKeyDown : undefined}
      onBlur={() => setRecording(false)}
      tabIndex={0}
      className={`px-3 py-1 rounded-md text-xs font-mono cursor-pointer select-none transition-all outline-none
        ${recording
          ? 'bg-blue-50 border-2 border-blue-400 text-blue-600'
          : 'bg-[rgba(0,0,0,0.04)] border border-[rgba(0,0,0,0.1)] text-[rgba(0,0,0,0.7)]'
        }`}
    >
      {recording ? t('general.hotkeyRecording') : formatHotkeyDisplay(value)}
    </div>
  )
}

function formatHotkeyDisplay(hotkey: string): string {
  return hotkey
    .replace('Alt', '⌥')
    .replace('Ctrl', '⌃')
    .replace('Shift', '⇧')
    .replace('Cmd', '⌘')
    .replace(/\+/g, ' ')
}
