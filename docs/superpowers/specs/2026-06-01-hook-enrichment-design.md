# Hook 体系丰富化设计

> 日期: 2026-06-01
> 状态: Draft
> 范围: Bridge 扩展 + 全量 Hook 注册 + 通知配置

## 1. 背景与问题

### 现状
- 仅注册 3 个 Hook: PreToolUse, PostToolUse, Stop
- Content 提取不完整：TaskCreate/TaskUpdate 仅显示工具名，无实际内容
- Stop 事件无 content：缺少 `last_assistant_message` 和 `stop_reason`
- 无 Notification/SessionStart 等事件感知

### 目标
- 对齐 Claude Code 官方 6 层 29 事件体系，按需注册高价值 Hook
- 丰富所有事件的 content 提取（解决内容缺失问题）
- 支持多 Agent（CC/CC-INT/Codex/Gemini）统一接入
- 用户可配置哪些事件触发系统通知

## 2. 官方 Hook 全景对齐

### 6 层 29 事件

| 层 | 事件 | 官方能力 |
|---|---|---|
| **Session** | SessionStart, Setup, SessionEnd | Inject / Observe |
| **Turn** | UserPromptSubmit, UserPromptExpansion, Stop, StopFailure | Block / Inject / Observe |
| **Tool Loop** | PreToolUse, PostToolUse, PostToolUseFailure, PostToolBatch, PermissionRequest, PermissionDenied | Block / Inject / Observe |
| **Agent & Task** | SubagentStart, SubagentStop, TaskCreated, TaskCompleted, TeammateIdle | Block / Observe |
| **Context & Config** | InstructionsLoaded, ConfigChange, CwdChanged, FileChanged, PreCompact, PostCompact | Block(部分) / Observe |
| **MCP & UI** | Notification, Elicitation, ElicitationResult, MessageDisplay, WorktreeCreate, WorktreeRemove | Observe / Replace |

### Pager 定位：纯 Observe

- 所有 Hook 均 `async: true`，不阻塞 CC 执行
- 不做 Block（不返回 exit 2 / deny 决策）
- 不做 Inject（不向 CC 注入上下文）
- 单向数据流：CC → bridge → Pager

## 3. Hook 注册方案

### 命令格式标准化

```bash
pager-cc-bridge --event <event_type> --agent <agent_label>
```

### 注册清单（22 个 Hook）

在 `~/.claude/settings.json` 的 `hooks` 中注册：

```json
{
  "SessionStart": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event session_start --agent CC", "timeout": 5, "async": true }] }],
  "SessionEnd": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event session_end --agent CC", "timeout": 5, "async": true }] }],
  "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event user_prompt_submit --agent CC", "timeout": 5, "async": true }] }],
  "UserPromptExpansion": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event user_prompt_expansion --agent CC", "timeout": 5, "async": true }] }],
  "Stop": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event stop --agent CC", "timeout": 5, "async": true }] }],
  "StopFailure": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event stop_failure --agent CC", "timeout": 5, "async": true }] }],
  "PreToolUse": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "pager-cc-bridge --event pre_tool_use --agent CC", "timeout": 5, "async": true }] }],
  "PostToolUse": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "pager-cc-bridge --event post_tool_use --agent CC", "timeout": 5, "async": true }] }],
  "PostToolUseFailure": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "pager-cc-bridge --event post_tool_use_failure --agent CC", "timeout": 5, "async": true }] }],
  "PostToolBatch": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event post_tool_batch --agent CC", "timeout": 5, "async": true }] }],
  "PermissionRequest": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "pager-cc-bridge --event permission_request --agent CC", "timeout": 5, "async": true }] }],
  "PermissionDenied": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "pager-cc-bridge --event permission_denied --agent CC", "timeout": 5, "async": true }] }],
  "SubagentStart": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event subagent_start --agent CC", "timeout": 5, "async": true }] }],
  "SubagentStop": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event subagent_stop --agent CC", "timeout": 5, "async": true }] }],
  "TaskCreated": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event task_created --agent CC", "timeout": 5, "async": true }] }],
  "TaskCompleted": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event task_completed --agent CC", "timeout": 5, "async": true }] }],
  "Notification": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event notification --agent CC", "timeout": 5, "async": true }] }],
  "PreCompact": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event pre_compact --agent CC", "timeout": 5, "async": true }] }],
  "PostCompact": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event post_compact --agent CC", "timeout": 5, "async": true }] }],
  "InstructionsLoaded": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event instructions_loaded --agent CC", "timeout": 5, "async": true }] }],
  "Elicitation": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event elicitation --agent CC", "timeout": 5, "async": true }] }],
  "MessageDisplay": [{ "hooks": [{ "type": "command", "command": "pager-cc-bridge --event message_display --agent CC", "timeout": 5, "async": true }] }]
}
```

### 排除的事件（7 个）

| 事件 | 排除理由 |
|------|---------|
| Setup | 仅 `--init-only`/`--maintenance` 时触发，无观测价值 |
| TeammateIdle | Agent Team 功能，当前不使用 |
| ConfigChange | 频率低且无关键信息 |
| CwdChanged | 频率极高，噪音大 |
| FileChanged | 频率极高，性能风险 |
| WorktreeCreate | Replace 类型，不适合 async observe |
| WorktreeRemove | Replace 类型，不适合 async observe |

## 4. CCHookInput 结构体扩展

扩展 `internal/adapter/bridge/types.go`，覆盖所有 22 个事件的 stdin 字段：

```go
type CCHookInput struct {
    // ═══ Common（所有事件） ═══
    SessionID      string          `json:"session_id"`
    TranscriptPath string          `json:"transcript_path"`
    CWD            string          `json:"cwd"`
    HookEventName  string          `json:"hook_event_name"`
    PermissionMode string          `json:"permission_mode"`
    Effort         *EffortLevel    `json:"effort,omitempty"`

    // ═══ Session 层 ═══
    Source       string `json:"source"`        // SessionStart: startup/resume/clear/compact
    Model        string `json:"model"`         // SessionStart
    SessionTitle string `json:"session_title"` // SessionStart

    // ═══ Turn 层 ═══
    Prompt               string `json:"prompt"`                  // UserPromptSubmit
    ExpansionType        string `json:"expansion_type"`          // UserPromptExpansion
    CommandName          string `json:"command_name"`            // UserPromptExpansion
    CommandArgs          string `json:"command_args"`            // UserPromptExpansion
    CommandSource        string `json:"command_source"`          // UserPromptExpansion
    StopReason           string `json:"stop_reason"`             // Stop: end_turn/max_tokens
    LastAssistantMessage string `json:"last_assistant_message"`  // Stop
    StopHookActive       bool   `json:"stop_hook_active"`        // Stop
    ErrorType            string `json:"error_type"`              // StopFailure
    ErrorMessage         string `json:"error_message"`           // StopFailure

    // ═══ Tool 层 ═══
    ToolName     string          `json:"tool_name"`
    ToolInput    json.RawMessage `json:"tool_input"`
    ToolUseID    string          `json:"tool_use_id"`
    ToolResult   json.RawMessage `json:"tool_result,omitempty"`   // PostToolUse
    ToolError    string          `json:"error"`                   // PostToolUseFailure
    DenialReason string          `json:"denial_reason"`           // PermissionDenied
    ToolCalls    json.RawMessage `json:"tool_calls,omitempty"`    // PostToolBatch

    // ═══ Agent & Task 层 ═══
    AgentID         string `json:"agent_id"`         // SubagentStart/Stop
    AgentType       string `json:"agent_type"`       // SubagentStart/Stop
    TaskID          string `json:"task_id"`          // TaskCreated/Completed
    TaskTitle       string `json:"task_title"`       // TaskCreated/Completed
    TaskDescription string `json:"task_description"` // TaskCreated

    // ═══ Context 层 ═══
    FilePath   string `json:"file_path"`   // InstructionsLoaded
    MemoryType string `json:"memory_type"` // InstructionsLoaded
    LoadReason string `json:"load_reason"` // InstructionsLoaded
    Trigger    string `json:"trigger"`     // PreCompact/PostCompact: manual/auto

    // ═══ MCP & UI 层 ═══
    NotificationType string          `json:"notification_type"` // Notification
    Message          string          `json:"message"`           // Notification/Elicitation
    ServerName       string          `json:"server_name"`       // Elicitation
    Request          json.RawMessage `json:"request,omitempty"` // Elicitation
    UserResponse     string          `json:"user_response"`     // ElicitationResult
    MessageText      string          `json:"message_text"`      // MessageDisplay
}

type EffortLevel struct {
    Level string `json:"level"` // low/medium/high/xhigh/max
}
```

## 5. Content 提取规则

### 5.1 Tool 层 — tool_input 提取（修复现有问题）

在 `extractor.go` 的 `extractRaw` 中新增 case：

```go
case "TaskCreate":
    var in struct {
        Subject     string `json:"subject"`
        Description string `json:"description"`
    }
    if unmarshal(&in) && in.Subject != "" {
        return "任务: " + in.Subject
    }

case "TaskUpdate":
    var in struct {
        TaskID  string `json:"taskId"`
        Status  string `json:"status"`
        Subject string `json:"subject"`
    }
    if unmarshal(&in) {
        parts := []string{}
        if in.Subject != "" {
            parts = append(parts, in.Subject)
        }
        if in.Status != "" {
            parts = append(parts, "→"+in.Status)
        }
        if len(parts) > 0 {
            return "任务: " + strings.Join(parts, " ")
        }
    }

case "TaskGet", "TaskList":
    return "查看任务"

case "TaskStop":
    var in struct { TaskID string `json:"task_id"` }
    if unmarshal(&in) && in.TaskID != "" {
        return "停止任务: " + in.TaskID
    }

case "NotebookEdit":
    var in struct { NotebookPath string `json:"notebook_path"` }
    if unmarshal(&in) && in.NotebookPath != "" {
        return "编辑 " + in.NotebookPath
    }

case "LSP":
    var in struct {
        Operation string `json:"operation"`
        FilePath  string `json:"filePath"`
    }
    if unmarshal(&in) {
        return "LSP " + in.Operation + ": " + in.FilePath
    }
```

### 5.2 非 Tool 事件 — content 提取

重构 `ExtractEventContent` 为完整路由：

```go
func ExtractEventContent(eventType string, in *CCHookInput) (contentRaw, content string) {
    switch eventType {
    // Session
    case "session_start":
        raw := "会话启动: " + in.Source
        return raw, truncateRunes(raw, contentMaxRunes)
    case "session_end":
        return "会话结束", "会话结束"

    // Turn
    case "user_prompt_submit":
        raw := in.Prompt
        if raw == "" { raw = "用户输入" }
        return raw, truncateRunes(raw, contentMaxRunes)
    case "user_prompt_expansion":
        raw := "/" + in.CommandName
        if in.CommandArgs != "" { raw += " " + in.CommandArgs }
        return raw, truncateRunes(raw, contentMaxRunes)
    case "stop":
        if in.LastAssistantMessage != "" {
            return in.LastAssistantMessage, truncateRunes(in.LastAssistantMessage, contentMaxRunes)
        }
        if in.StopReason != "" {
            raw := "回复完成: " + in.StopReason
            return raw, raw
        }
        return "回复完成", "回复完成"
    case "stop_failure":
        raw := in.ErrorType + ": " + in.ErrorMessage
        return raw, truncateRunes(raw, contentMaxRunes)

    // Agent & Task
    case "subagent_start":
        raw := "子Agent启动: " + in.AgentType
        return raw, truncateRunes(raw, contentMaxRunes)
    case "subagent_stop":
        raw := "子Agent完成: " + in.AgentType
        return raw, truncateRunes(raw, contentMaxRunes)
    case "task_created":
        raw := "任务: " + in.TaskTitle
        return raw, truncateRunes(raw, contentMaxRunes)
    case "task_completed":
        raw := "任务完成: " + in.TaskTitle
        return raw, truncateRunes(raw, contentMaxRunes)

    // Tool (non-standard)
    case "post_tool_use_failure":
        raw := in.ToolName + " 失败: " + in.ToolError
        return raw, truncateRunes(raw, contentMaxRunes)
    case "permission_request":
        return ExtractContent(in.ToolName, in.ToolInput)
    case "permission_denied":
        raw := in.ToolName + " 被拒: " + in.DenialReason
        return raw, truncateRunes(raw, contentMaxRunes)
    case "post_tool_batch":
        return "批次完成", "批次完成"

    // Context
    case "pre_compact":
        raw := "上下文压缩: " + in.Trigger
        return raw, raw
    case "post_compact":
        return "压缩完成", "压缩完成"
    case "instructions_loaded":
        raw := "加载: " + in.FilePath
        return raw, truncateRunes(raw, contentMaxRunes)

    // MCP & UI
    case "notification":
        raw := in.Message
        if raw == "" { raw = in.NotificationType }
        return raw, truncateRunes(raw, contentMaxRunes)
    case "elicitation":
        raw := "MCP表单: " + in.ServerName + " - " + in.Message
        return raw, truncateRunes(raw, contentMaxRunes)
    case "elicitation_result":
        raw := "MCP响应: " + in.ServerName
        return raw, truncateRunes(raw, contentMaxRunes)
    case "message_display":
        raw := in.MessageText
        return raw, truncateRunes(raw, contentMaxRunes)

    default:
        return eventType, eventType
    }
}
```

## 6. Attention Level 逻辑

### 扩展 DetermineAttentionLevel

```go
var attentionTools = map[string]bool{
    "AskUserQuestion": true,
}

func DetermineAttentionLevel(eventType, permissionMode, toolName string) string {
    switch eventType {
    // Done 类
    case "stop", "session_end", "subagent_stop":
        return AttentionDone

    // Attention 类
    case "stop_failure":
        return AttentionAttention
    case "permission_request":
        return AttentionAttention
    case "notification":
        return AttentionAttention
    case "elicitation":
        return AttentionAttention

    // Tool 类 — 需要细分
    case "pre_tool_use":
        if attentionTools[toolName] {
            return AttentionAttention
        }
        if permissionMode == "bypassPermissions" {
            return AttentionRunning
        }
        return AttentionAttention
    case "post_tool_use", "post_tool_batch":
        return AttentionRunning
    case "post_tool_use_failure", "permission_denied":
        return AttentionAttention

    // Running 类（默认）
    default:
        return AttentionRunning
    }
}
```

### 假设
- 终端状态与 Pager 完全同步
- 后续事件覆盖前事件的 attention level
- 不引入 TTL/超时机制

## 7. Bridge 命令行参数重构

### 新的参数解析

```go
func parseArgs(args []string) (eventType, agentLabel string) {
    eventType = "unknown"
    agentLabel = "CC"

    for i := 0; i < len(args); i++ {
        switch {
        case args[i] == "--event" && i+1 < len(args):
            eventType = args[i+1]
            i++
        case args[i] == "--agent" && i+1 < len(args):
            agentLabel = args[i+1]
            i++
        // 向后兼容：无 flag 的第一个非 -- 参数作为 event
        case !strings.HasPrefix(args[i], "--") && eventType == "unknown":
            eventType = args[i]
        }
    }
    return
}
```

### isToolEvent 扩展

```go
func isToolEvent(eventType string) bool {
    switch eventType {
    case "pre_tool_use", "post_tool_use", "permission_request", "permission_denied":
        return true
    }
    return false
}
```

## 8. 通知配置

### 数据模型

Settings 仅控制**哪些事件触发 macOS 系统通知**。Panel 始终全量展示所有事件。

```go
// config.go 新增字段
type Settings struct {
    // ... 现有字段 ...

    // 通知事件配置 — agent_type → 触发通知的 event_type 列表
    NotificationEvents map[string][]string `json:"notification_events"`
}
```

### 默认配置

```json
{
  "notification_events": {
    "CC": [
      "stop_failure",
      "notification",
      "permission_request",
      "post_tool_use_failure",
      "elicitation"
    ],
    "CC-INT": [
      "stop_failure",
      "notification"
    ]
  }
}
```

### Settings UI

**位置变更**：从 General 面板中移除现有的 "通知级别" 配置块，独立为左侧 sidebar 的新 Tab。

**Sidebar 导航**（新增 🔔）：
```
⚙️ 通用 (General)
🔔 通知 (Notifications)  ← 新增
💾 数据 (Data)
ℹ️ 关于 (About)
```

**GeneralSettings.tsx 变更**：删除 `<Section label="通知">` 及其中的 `notification_level` 下拉框。

**新增 NotificationSettings.tsx**：
- 顶部：Agent Tabs（CC / CC-INT / Codex / Gemini），已接入的显示绿色标签
- 说明文字：开启的事件将触发 macOS 系统通知。所有事件均在会话面板中展示。
- 预设按钮：推荐 / 仅关键 / 全通知 / 全关闭
- 内容：按 5 个 domain 分组（SESSION / TURN / TOOL / AGENT & TASK / SYSTEM & MCP）
- 每行：事件中文名 + hook_event_name + iOS 风格 Toggle 开关
- 底部统计：当前已开启 N 项通知 · 共 22 项事件
- 未接入 Agent Tab 置灰，提示安装命令

**SettingsPanel.tsx 变更**：
```typescript
type Page = 'general' | 'notifications' | 'data' | 'about'

const navItems = [
  { id: 'general', icon: '⚙️', label: t('nav.general') },
  { id: 'notifications', icon: '🔔', label: isZh ? '通知' : 'Notifications' },
  { id: 'data', icon: '💾', label: isZh ? '数据' : 'Data' },
  { id: 'about', icon: 'ℹ️', label: t('nav.about') },
]
```

- 删除 sidebar 顶部的 "通用" / "GENERAL" section label（`<div>` at line 55-57）
- 导航项直接从顶部开始，无分组标题

## 9. 安装脚本更新

`install-hooks.sh` 需更新为注册全部 22 个 Hook：

```bash
#!/bin/bash
BRIDGE_PATH="${BRIDGE_PATH:-$(dirname "$0")/../bin/pager-cc-bridge}"
AGENT="${1:-CC}"

# 生成所有 hook 注册的 JSON patch
# 写入 ~/.claude/settings.json
```

### Makefile 联动

```makefile
# make bridge 时同步编译，避免版本不一致
bridge:
    go build -o $(BRIDGE_BIN) ./cmd/bridge
    @echo "Bridge compiled: $(BRIDGE_BIN)"
```

## 10. 不做的事项

- ❌ Block 能力（exit 2 / deny 决策）
- ❌ Inject 能力（stdout 注入上下文）
- ❌ Pager 统一事件中间层（直接使用各 Agent 原生 hook event name）
- ❌ PendingTools TTL 超时机制
- ❌ CwdChanged/FileChanged/WorktreeCreate/WorktreeRemove Hook
- ❌ 多 Agent bridge 实现（仅预留 agent_label 参数，Codex/Gemini 为 v2）

## 11. 实施顺序

1. **CCHookInput 扩展** — 新增全部字段（types.go）
2. **Content 提取修复** — TaskCreate/TaskUpdate/Stop 等（extractor.go）
3. **Attention 逻辑扩展** — 新事件类型（attention.go）
4. **Bridge 参数重构** — `--event`/`--agent` flag 解析（cmd/bridge/main.go）
5. **settings.json 注册** — 22 个 Hook（install-hooks.sh）
6. **Settings UI** — 通知配置面板（NotificationSettings.tsx）
7. **重新编译 bridge** — `make bridge`（修复 AskUserQuestion attention bug）
