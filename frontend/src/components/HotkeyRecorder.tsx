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

    // Use e.code instead of e.key to avoid macOS dead key issue
    // (e.g., Alt+E produces "Dead" via e.key, but e.code gives "KeyE")
    const key = codeToKey(e.code)
    if (!key) return // modifier-only press
    if (parts.length === 0) return // no modifier held

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

/** Convert KeyboardEvent.code to a clean key name for the hotkey string */
function codeToKey(code: string): string | null {
  // Ignore modifier-only codes
  if (['ControlLeft', 'ControlRight', 'AltLeft', 'AltRight',
       'ShiftLeft', 'ShiftRight', 'MetaLeft', 'MetaRight'].includes(code)) {
    return null
  }
  // Letter keys: "KeyA" → "A"
  if (code.startsWith('Key')) return code.slice(3)
  // Digit keys: "Digit0" → "0"
  if (code.startsWith('Digit')) return code.slice(5)
  // Function keys: "F1" → "F1"
  if (/^F\d+$/.test(code)) return code
  // Special keys
  const specialMap: Record<string, string> = {
    Space: 'Space', Enter: 'Return', Escape: 'Escape',
    Backspace: 'Delete', Tab: 'Tab', Delete: 'Delete',
    ArrowLeft: 'Left', ArrowRight: 'Right',
    ArrowUp: 'Up', ArrowDown: 'Down',
  }
  return specialMap[code] || null
}

function formatHotkeyDisplay(hotkey: string): string {
  return hotkey
    .replace('Alt', '⌥')
    .replace('Ctrl', '⌃')
    .replace('Shift', '⇧')
    .replace('Cmd', '⌘')
    .replace(/\+/g, ' ')
}
