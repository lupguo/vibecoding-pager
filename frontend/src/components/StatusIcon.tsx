interface Props {
  agent: string
}

export default function StatusIcon({ agent }: Props) {
  const colors: Record<string, string> = {
    'claude-code': 'bg-[#5B9BD5]',
    'codex': 'bg-[#4CAF50]',
  }
  const color = colors[agent] ?? 'bg-gray-400'

  return (
    <span className={`inline-block w-2 h-2 rounded-full ${color}`} />
  )
}
