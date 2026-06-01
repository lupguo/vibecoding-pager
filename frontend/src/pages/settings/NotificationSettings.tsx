import React from 'react'
import { useTranslation } from 'react-i18next'
import { useSettingsStore } from '../../store/settings'

const EVENT_GROUPS = [
  {
    domain: 'SESSION',
    events: [
      { key: 'SessionStart', label: '会话启动', labelEn: 'Session Start' },
      { key: 'SessionEnd', label: '会话结束', labelEn: 'Session End' },
    ],
  },
  {
    domain: 'TURN',
    events: [
      { key: 'UserPromptSubmit', label: '用户输入', labelEn: 'User Input' },
      { key: 'Stop', label: 'Agent 回复完成', labelEn: 'Agent Reply' },
      { key: 'StopFailure', label: 'Agent 错误', labelEn: 'Agent Error', desc: 'rate_limit / auth / billing' },
      { key: 'UserPromptExpansion', label: '命令展开', labelEn: 'Command Expansion' },
    ],
  },
  {
    domain: 'TOOL',
    events: [
      { key: 'PreToolUse', label: '工具等待执行', labelEn: 'Tool Pending' },
      { key: 'PostToolUse', label: '工具执行完成', labelEn: 'Tool Done' },
      { key: 'PostToolUseFailure', label: '工具执行失败', labelEn: 'Tool Failed' },
      { key: 'PermissionRequest', label: '权限请求', labelEn: 'Permission Request' },
      { key: 'PermissionDenied', label: '权限被拒', labelEn: 'Permission Denied' },
      { key: 'PostToolBatch', label: '批次完成', labelEn: 'Batch Done' },
    ],
  },
  {
    domain: 'AGENT & TASK',
    events: [
      { key: 'SubagentStart', label: '子Agent 启动', labelEn: 'Subagent Start' },
      { key: 'SubagentStop', label: '子Agent 结束', labelEn: 'Subagent Stop' },
      { key: 'TaskCreated', label: '任务创建', labelEn: 'Task Created' },
      { key: 'TaskCompleted', label: '任务完成', labelEn: 'Task Completed' },
    ],
  },
  {
    domain: 'SYSTEM & MCP',
    events: [
      { key: 'Notification', label: '系统通知', labelEn: 'Notification', desc: 'permission_prompt / idle_prompt' },
      { key: 'Elicitation', label: 'MCP 表单请求', labelEn: 'MCP Elicitation' },
      { key: 'InstructionsLoaded', label: '指令加载', labelEn: 'Instructions Loaded' },
      { key: 'PreCompact', label: '上下文压缩', labelEn: 'Context Compact' },
      { key: 'MessageDisplay', label: '消息输出', labelEn: 'Message Display' },
    ],
  },
]

const ALL_EVENT_KEYS = EVENT_GROUPS.flatMap((g) => g.events.map((e) => e.key))

const PRESETS: Record<string, string[]> = {
  recommend: ['StopFailure', 'Notification', 'PermissionRequest', 'PostToolUseFailure', 'Elicitation'],
  critical: ['StopFailure', 'PermissionRequest'],
  all: ALL_EVENT_KEYS,
  none: [],
}

const KNOWN_AGENT_COLORS: Record<string, string> = {
  CC: '#007aff',
  'CC-INT': '#bf5af2',
  CodeBuddy: '#ff9f0a',
  Codex: '#ff9f0a',
  Gemini: '#30d158',
}

const DEFAULT_AGENT_COLOR = '#86868b'

export default function NotificationSettings() {
  const { t, i18n } = useTranslation()
  const isZh = i18n.language === 'zh'
  const settings = useSettingsStore((s) => s.settings)
  const updateSettings = useSettingsStore((s) => s.updateSettings)

  // For now, active agent is always CC. Multi-agent tab switching is v2.
  const [activeAgent, setActiveAgent] = React.useState('CC')
  const notifEvents = settings.notification_events || {}
  const agentEvents = notifEvents[activeAgent] || []

  const isEnabled = (eventKey: string) => agentEvents.includes(eventKey)
  const enabledCount = agentEvents.length
  const totalCount = ALL_EVENT_KEYS.length

  const toggleEvent = (eventKey: string) => {
    const current = [...agentEvents]
    const idx = current.indexOf(eventKey)
    if (idx >= 0) {
      current.splice(idx, 1)
    } else {
      current.push(eventKey)
    }
    updateSettings({
      notification_events: { ...notifEvents, [activeAgent]: current },
    })
  }

  const applyPreset = (presetKey: string) => {
    const events = PRESETS[presetKey] || []
    updateSettings({
      notification_events: { ...notifEvents, [activeAgent]: [...events] },
    })
  }

  // Derive agents from notification_events config keys
  const connectedAgents = Object.keys(notifEvents)
  const allAgents = [...new Set([...connectedAgents, ...Object.keys(KNOWN_AGENT_COLORS)])]
    .sort((a, b) => {
      // Connected agents first, then alphabetical
      const aConnected = connectedAgents.includes(a) ? 0 : 1
      const bConnected = connectedAgents.includes(b) ? 0 : 1
      if (aConnected !== bConnected) return aConnected - bConnected
      return a.localeCompare(b)
    })

  return (
    <div>
      <h3 className="text-[17px] font-semibold text-[--pager-text-primary] mb-4">
        {t('notifications.title')}
      </h3>

      {/* Agent Select */}
      <div className="flex items-center gap-2 mb-3">
        <span
          className="w-[7px] h-[7px] rounded-full flex-shrink-0"
          style={{ background: KNOWN_AGENT_COLORS[activeAgent] || DEFAULT_AGENT_COLOR }}
        />
        <select
          value={activeAgent}
          onChange={(e) => setActiveAgent(e.target.value)}
          className="flex-1 text-[13px] px-2 py-1.5 rounded-md border border-[rgba(0,0,0,0.1)] dark:border-[rgba(255,255,255,0.1)] bg-white dark:bg-[rgba(255,255,255,0.05)] text-[--pager-text-primary]"
        >
          {allAgents.map((agentId) => {
            const connected = connectedAgents.includes(agentId)
            return (
              <option key={agentId} value={agentId}>
                {agentId} {connected ? `✓` : ''}
              </option>
            )
          })}
        </select>
        <span className={`text-[10px] px-[5px] py-[2px] rounded-[4px] font-semibold ${
          connectedAgents.includes(activeAgent)
            ? 'bg-[rgba(48,209,88,0.12)] text-[#30d158]'
            : 'bg-[rgba(0,0,0,0.04)] dark:bg-[rgba(255,255,255,0.04)] text-[--pager-text-muted]'
        }`}>
          {connectedAgents.includes(activeAgent) ? t('notifications.connected') : t('notifications.notConnected')}
        </span>
      </div>

      {/* Description */}
      <p className="text-[11px] text-[--pager-text-muted] mb-3 leading-[1.4]">
        {t('notifications.description')}
      </p>

      {/* Presets */}
      <div className="flex gap-[6px] mb-3">
        {(['recommend', 'critical', 'all', 'none'] as const).map((key) => {
          const label = t(`notifications.preset${key.charAt(0).toUpperCase() + key.slice(1)}`)
          const isActive = JSON.stringify([...agentEvents].sort()) === JSON.stringify([...(PRESETS[key] || [])].sort())
          return (
            <button
              key={key}
              onClick={() => applyPreset(key)}
              className={`text-[10px] font-medium px-[8px] py-[3px] rounded-[5px] border transition-colors
                ${isActive
                  ? 'bg-[rgba(0,122,255,0.08)] border-[#007aff] text-[#007aff]'
                  : 'border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] text-[--pager-text-secondary] hover:border-[#007aff] hover:text-[#007aff]'
                }`}
            >
              {label}
            </button>
          )
        })}
      </div>

      {/* Event List */}
      <div className="bg-white dark:bg-[rgba(255,255,255,0.04)] rounded-[10px] border border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.08)] overflow-hidden">
        {EVENT_GROUPS.map((group) => (
          <div key={group.domain}>
            <div className="text-[10px] font-semibold text-[--pager-text-muted] uppercase tracking-wider px-[14px] py-[6px] bg-[rgba(0,0,0,0.02)] dark:bg-[rgba(255,255,255,0.02)] border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)]">
              {group.domain}
            </div>
            {group.events.map((event) => (
              <div
                key={event.key}
                className="flex items-center justify-between px-[14px] py-[7px] border-b border-[rgba(0,0,0,0.06)] dark:border-[rgba(255,255,255,0.06)] last:border-b-0"
              >
                <div>
                  <div className="text-[13px] text-[--pager-text-primary]">
                    {isZh ? event.label : event.labelEn}
                  </div>
                  <div className="text-[10px] text-[--pager-text-muted] mt-[1px]">
                    {event.key}{event.desc ? ` — ${event.desc}` : ''}
                  </div>
                </div>
                <button
                  onClick={() => toggleEvent(event.key)}
                  className={`relative w-[34px] h-[19px] rounded-[10px] transition-colors flex-shrink-0 ${
                    isEnabled(event.key) ? 'bg-[#007aff]' : 'bg-[#e0e0e0] dark:bg-[rgba(255,255,255,0.15)]'
                  }`}
                >
                  <span
                    className={`absolute top-[2px] left-[2px] w-[15px] h-[15px] rounded-full bg-white shadow-sm transition-transform ${
                      isEnabled(event.key) ? 'translate-x-[15px]' : ''
                    }`}
                  />
                </button>
              </div>
            ))}
          </div>
        ))}
      </div>

      {/* Footer */}
      <p className="text-[11px] text-[--pager-text-muted] text-center mt-[10px]">
        {isZh
          ? `当前已开启 ${enabledCount} 项通知 · 共 ${totalCount} 项事件`
          : `${enabledCount} notifications enabled · ${totalCount} total events`}
      </p>
    </div>
  )
}
