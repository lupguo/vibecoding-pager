# Pager v1.1 Core Experience Design

> Date: 2026-05-28
> Scope: Session 唯一性 + 通知内容丰富度 + 跳转精准度 + 窗口行为 + UI 交互优化
> Status: Approved

---

## Context

Pager v1.0 实现了基础功能链路（hook → server → UI），但日常使用中暴露出以下核心体验问题：

1. 同一项目下多个 CC 实例无法区分（TTY 检测为空时 SessionKey 冲突）
2. 通知内容过于精简（仅显示工具名，缺少具体上下文）
3. 缺乏注意力分级（所有通知视觉权重相同，用户不知哪些需要操作）
4. 窗口行为不符合 macOS 惯例（点击外部不收起）
5. 跳转到终端不够精准

本 spec 解决以上问题，将 Pager 从「事件列表」升级为「注意力分级系统」。

---

## 1. 三级注意力模型

### 1.1 状态定义

| 级别 | 状态名 | 触发条件 | 含义 |
|------|--------|---------|------|
| 🔴 Attention | `attention` | `pre_tool_use` 且 CC 处于需用户确认模式 | 需要用户操作（approve/回答问题） |
| 🟢 Running | `running` | `pre_tool_use`/`post_tool_use` 在 bypass 模式下 | 后台自动执行中，仅通知 |
| ⚫ Done | `done` | `stop` 事件 | 会话已结束 |

### 1.2 判断逻辑（bridge 端）

```
IF event_type == "stop" → done
IF event_type == "pre_tool_use":
    IF permission_mode == "bypassPermissions" → running
    ELSE → attention
IF event_type == "post_tool_use" → running
```

CC hook stdin 中的 `permission_mode` 字段用于判断。Bridge 提取后写入 AgentEvent 新字段 `attention_level`。

### 1.3 顶部筛选栏

三色圆点筛选器，每个显示对应数量：
- 🔴 红点（默认选中）：仅展示 attention 级别
- 🟢 绿点：展示 running 级别
- ⚫ 灰点：展示 done 级别

默认展示红色（用户最需关注的）。

---

## 2. Session 模型重设计

### 2.1 SessionKey 变更

```
旧: host:cwd:tty（TTY 可能为空导致冲突）
新: session_id（CC 原生唯一 ID，保证每个 CC 实例独立）
```

### 2.2 显示规则

**折叠卡片第 1 行：**
```
[Agent]  项目名                session_id前8位 · 相对时间
```

- Agent badge: Hook 安装时配置的产品标识（CC / CC-Int / Codex / 自定义）
- 项目名: CWD 的最后一段目录名
- session_id 前 8 位: 靠右对齐，与时间戳同样式（10px, 低透明度）
- 相对时间: now / 3s / 1m / 5m

**展开后额外显示：**
```
session: b2e1f04c-8a9d-4f2e-a3c1-7d6e5f8a9b0c
path: /Users/example/projects/api-server
```

### 2.3 同一 Session 行为

- 每个 session_id 永远只有一张卡片
- 新事件更新现有卡片内容（状态 + content），不产生新卡片
- pre/post tool_use 是状态转换，不是两条独立通知

---

## 3. 通知内容丰富度

### 3.1 Bridge 端提取增强

| 工具 | 当前提取 | 改进后 |
|------|---------|--------|
| AskUserQuestion | `"AskUserQuestion"` | 提取 `tool_input.questions[0].question`（第一个问题文本） |
| Bash | `command` ✅ | 不变 |
| Agent/Task | `"Task"` 或 `description` | 提取 `tool_input.prompt` 前 60 字符 |
| Edit | 文件路径 | 文件路径（不变，已足够） |
| Read | 文件路径 ✅ | 不变 |
| Write | 文件路径 ✅ | 不变 |
| WebSearch | `query` ✅ | 不变 |
| WebFetch | `url` ✅ | 不变 |

### 3.2 ContentRaw 保留完整 tool_input

bridge 端将 `tool_input` JSON 序列化后作为 `content_raw`，供展开详情时渲染。

### 3.3 UI 端展示

- **折叠**: Tool tag + content（60 字符截断）
- **展开**: 灰底代码块显示 content_raw（格式化后的完整内容）

---

## 4. 卡片 UI 设计（v8 确认版）

### 4.1 卡片结构

**折叠状态（默认）：**
```
┌────────────────────────────────────────────────┐
│ [CC]  pager              f7a3b2e1 · now        │  Row 1
│ ASKUSER  你偏好哪种 UI 风格？             →    │  Row 2
└────────────────────────────────────────────────┘
```

**展开状态（点击卡片）：**
```
┌────────────────────────────────────────────────┐
│ [CC-Int]  api-server     b2e1f04c · 3s         │  Row 1
│ BASH  rm -rf node_modules && npm install  →    │  Row 2
│ ┌────────────────────────────────────────┐     │
│ │ $ rm -rf node_modules && npm install   │     │  Detail
│ │ # 删除依赖后重新安装                    │     │
│ └────────────────────────────────────────┘     │
│ session: b2e1f04c-8a9d-4f2e-a3c1-7d6e5f8a9b0c │  Meta
│ path: /Users/example/projects/api-server     │
└────────────────────────────────────────────────┘
```

### 4.2 视觉规则

- **Attention 卡片**: 红色背景色（rgba红 0.06）+ 红色边框（rgba红 0.12）
- **Running 卡片**: 绿色背景色（rgba绿 0.04）+ 绿色边框（rgba绿 0.08）
- **Done 卡片**: 无背景 + 低透明度整体淡化（opacity 0.5）
- **无左侧色条**
- **跳转箭头**: 固定在第 2 行末尾，收起/展开位置一致

### 4.3 macOS 主题适配

面板跟随系统主题自动切换：
- **Light**: 白色毛玻璃背景 + macOS 系统色（红#ff3b30 / 绿#34c759 / 蓝#007aff）
- **Dark**: 深灰毛玻璃背景 + macOS Dark 系统色（红#ff453a / 绿#30d158 / 蓝#0a84ff）

检测方式: CSS `@media (prefers-color-scheme: dark)` + Wails 系统主题 API。

### 4.4 Agent Badge

- Hook 安装时通过环境变量或命令行参数配置: `--agent-name CC`
- 默认值: `CC`（Claude Code）
- 存储在 bridge 二进制旁的 `.pager-bridge.json` 配置文件
- 传递路径: bridge 读取配置 → 写入 AgentEvent.agent_label 字段 → UI 显示

---

## 5. 跳转精准度

### 5.1 iTerm2 精确跳转

利用 `ITERM_SESSION_ID` 环境变量（格式: `w0t1p0:GUID`）：

```applescript
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (unique id of s) contains "GUID" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell
```

### 5.2 Fallback 策略

1. **优先**: ITERM_SESSION_ID（最精准）
2. **次选**: TTY 匹配（`tty of session`）
3. **兜底**: 仅 activate 终端 app

### 5.3 SessionKey 用于跳转

由于 SessionKey 改为 `session_id`，跳转信息（TTY、ITERM_SESSION_ID、TermProgram）作为 Session 的辅助字段存储，从最新的 AgentEvent 中获取。

---

## 6. 窗口行为

### 6.1 点击外部收起

Wails v3 的 `AttachWindow` 已支持此行为（tray-attached window 在失焦时自动隐藏）。需确认当前版本是否生效，若不生效：

- 监听 `WindowDidResignKey` 事件
- 调用 `window.Hide()`

### 6.2 面板行为规范

| 操作 | 行为 |
|------|------|
| 点击 tray icon | 显示/隐藏面板 |
| 点击面板外区域 | 隐藏面板 |
| 点击跳转箭头 | 跳转终端 + 隐藏面板 |
| Escape 键 | 隐藏面板 |

---

## 7. 数据结构变更

### 7.1 AgentEvent 新增字段

```go
type AgentEvent struct {
    // ... 现有字段 ...

    // 新增
    AttentionLevel string `json:"attention_level"` // "attention" | "running" | "done"
    AgentLabel     string `json:"agent_label"`     // "CC" | "CC-Int" | "Codex" | 自定义
    PermissionMode string `json:"permission_mode"` // "bypassPermissions" | "default" | ...
}
```

### 7.2 Registry SessionKey 变更

```go
// 旧
func (e *AgentEvent) SessionKey() string {
    return e.Host + ":" + e.CWD + ":" + e.TTY
}

// 新
func (e *AgentEvent) SessionKey() string {
    if e.SessionID != "" {
        return e.SessionID
    }
    // Fallback for agents that don't provide session_id
    return e.Host + ":" + e.CWD + ":" + e.TTY
}
```

### 7.3 Agent 标识传递（命令行参数方案）

**无配置文件**。Agent 身份在 hook 安装时就通过命令行参数固化：

```bash
# install-hooks.sh 用法
./install-hooks.sh /path/to/pager-cc-bridge --agent CC-Int
```

生成的 hook 配置：
```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "/path/to/pager-cc-bridge pre_tool_use --agent CC-Int",
        "async": true
      }]
    }]
  }
}
```

Bridge `main()` 解析 `--agent` 参数，写入 `AgentEvent.AgentLabel`：
```go
// 参数解析（os.Args 示例）
// pager-cc-bridge pre_tool_use --agent CC-Int
eventType := os.Args[1]  // "pre_tool_use"
agentLabel := "CC"       // 默认值
for i, arg := range os.Args {
    if arg == "--agent" && i+1 < len(os.Args) {
        agentLabel = os.Args[i+1]
    }
}
```

**多 Agent 共存示例：**
| Agent 产品 | Hook command | Badge 显示 |
|-----------|-------------|-----------|
| Claude Code | `pager-cc-bridge pre_tool_use --agent CC` | `[CC]` |
| Claude Code Internal | `pager-cc-bridge pre_tool_use --agent CC-Int` | `[CC-Int]` |
| Codex | `pager-cc-bridge pre_tool_use --agent Codex` | `[Codex]` |
| 自定义 Agent | `pager-cc-bridge pre_tool_use --agent MyAgent` | `[MyAgent]` |

同一个二进制，不同 hook 传不同参数，零配置文件。

---

## 8. 实现优先级

| 优先级 | 任务 | 影响面 |
|--------|------|--------|
| P0 | SessionKey 改为 session_id | Registry + Bridge |
| P0 | 三级注意力判断 + 顶部筛选 | Bridge + Registry + Frontend |
| P0 | 卡片 UI 重写（v8 设计） | Frontend |
| P1 | Content 提取增强（AskUserQuestion 等） | Bridge extractor |
| P1 | 窗口点击外部收起 | Wails main.go |
| P1 | Agent badge 配置 | Bridge + install script |
| P2 | macOS Light/Dark 主题适配 | Frontend CSS |
| P2 | 跳转精准度优化 | terminal/jump.go |

---

## 9. 验证方案

1. **Session 唯一性**: 同 CWD 开 2 个 CC 实例，确认面板显示 2 张独立卡片（不同 session 前缀）
2. **注意力分级**: 在 bypass 模式和默认模式分别触发事件，确认红/绿分类正确
3. **内容丰富度**: 触发 AskUserQuestion hook，确认面板显示具体问题文本而非工具名
4. **跳转**: 从面板点击箭头，确认 iTerm2 精确切换到对应 tab
5. **窗口行为**: 打开面板后点击桌面，确认面板自动收起

---

## 10. 不在本次范围

以下推迟到后续迭代：
- SQLite 存储 + 统计面板
- 系统配置页（主题/透明度/快捷键）
- 右键菜单配置面板
- 项目结构整理 + README
- 日志/配置模块化
