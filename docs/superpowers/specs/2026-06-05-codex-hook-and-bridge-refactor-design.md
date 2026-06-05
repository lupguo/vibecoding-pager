# Codex Hook 接入 & Bridge 重构设计

> 日期：2026-06-05
> 范围：(1) 接入 Codex hook 体系实现完整对等观测；(2) Bridge 架构重构为 Agent 接口 + 纯 DSL 抽取；(3) 二进制改名 + install-hooks 改造为 Go 程序
> 预估工作量：~6-8h（含测试）

---

## 1. 背景与目标

### 1.1 触发动机

Codex CLI 0.137.0 已支持 hook 体系，事件名与 stdin JSON 格式与 Claude Code 高度兼容（同名 `session_id` / `transcript_path` / `hook_event_name` / `tool_name` / `tool_input` / `tool_response` / `prompt`），但有两点关键差异：

1. **工具模型不同**：CC 是命名工具集（`Bash`/`Edit`/`Write`/`Read`/...），Codex 是 shell-centric（`exec_command` 承载所有执行；文件编辑通过 `apply_patch` 子命令；`tool_response` 是单 string 而非 object）
2. **少数字段语义**：Codex SessionStart 用 `trigger`（startup/resume/clear/compact），CC 用 `source`；Codex 多一个 `turn_id`；Codex 不发 SessionEnd/StopFailure/Notification/Elicitation 等 13 个事件

CR 现有 bridge 代码时还发现一组**根因性问题**：

- `cmd/bridge/main.go` 把 `Agent` 字段**硬编码**为 `entity.AgentClaudeCode`——CodeBuddy 事件实际上一直被打成 `agent=claude-code`，只是 `AgentLabel=CodeBuddy` 区分。`AgentCodex` 常量定义了从未被写入。
- `ExtractEventContent` 是 13+ case 的硬编码 Go switch，全部按 `CCHookInput` 字段读，加 agent 必然要在这里加 case
- `extract_rules.yaml` 只覆盖工具事件，非工具事件全在 Go 里
- `install-hooks.sh` 用覆盖式写入（`settings['hooks'][event] = [pager_group]`），会清掉用户自定义的 hooks
- `scripts/install-hooks.sh` 里硬编码 23 件事件清单，与 Go 侧 `entity.EventXxx` 常量两份真理

### 1.2 目标

一次性解决：

1. **Codex 完整对等观测**：所有 10 个 Codex 事件能进入 Pager，UI 上 Codex 会话有正确的状态（working/waiting/done/error）
2. **Bridge 架构清晰化**：建立 Agent 接口抽象，每个 agent 拥有独立的输入解析逻辑，加新 agent = 一个 Go 文件 + 一段 YAML
3. **DSL 统一所有内容抽取**：废除 `ExtractEventContent` 的硬编码 switch，工具事件与非工具事件统一走 `extract_rules.yaml` v3
4. **install-hooks 升级为 Go 程序**：append 式幂等写入、保护用户自定义 hooks、读 `--print-hooks` 协议作为唯一真理来源
5. **二进制改名**：`vibecoding-pager-cc-bridge` → `pager-bridge`，迁移所有 settings 文件中的旧路径

### 1.3 非目标

- 不实现 hook 返回值影响 agent 行为（v1.0 边界，与 CLAUDE.md 一致）
- 不引入 WebSocket/Linux/Windows 支持
- 不改 `internal/domain/session/status.go` 的 `DeriveStatus` 状态机逻辑——只新增 2 个事件常量走 default 分支
- 不迁移已有数据：用户重装 pager-bridge 后老 hook entry 会被自动清理替换

---

## 2. 决策清单（已锁定）

| # | 议题 | 决定 |
|---|---|---|
| 1 | 接入 Codex 的目标层级 | 完整对等观测（事件常量、状态机、UI 一致） |
| 2 | Codex hook 配置文件 | `~/.codex/hooks.json`（独立文件，Codex 原生支持） |
| 3 | Codex hook trust | 首次启动让用户走 "Trust all and continue" 交互流（不用 `--dangerously-bypass`） |
| 4 | bridge 二进制策略 | 单一二进制 + `--agent` 区分，**不**为每家 agent 单独建二进制 |
| 5 | install-hooks 调度 | 统一在 `make install-bridge` 一次写入三家 settings |
| 6 | 架构抽象层级 | Agent 接口抽象（介于"struct 拆分"与"包级隔离"之间） |
| 7 | 内容抽取统一性 | 纯 DSL：`extract_rules.yaml` v3 覆盖所有事件，含 `lookup`/`pluck`/`join`/`prefix` 四个新 filter + 链式 filter |
| 8 | YAML 版本兼容 | 直接 break v1（embed 进 binary，无外部用户） |
| 9 | hook 文件路径分布 | Codex `~/.codex/hooks.json`；CC `~/.claude/settings.local.json`；CodeBuddy `~/.codebuddy/settings.local.json` |
| 10 | 多入口共存 | append + 幂等：用户自定义 hook 与 pager hook 共存 |
| 11 | `--print-hooks` 协议 | bridge 子命令吐 JSON，installer 消费；Go 是唯一真理来源 |
| 12 | install-hooks 实现语言 | Go (`cmd/pager-installhooks/main.go`)，不再用 bash |
| 13 | 二进制改名 | `vibecoding-pager-cc-bridge` → `pager-bridge`；installer 兼容旧名做迁移清理 |
| 14 | Go cmd 参数解析 | 统一使用 `pflag`（`spf13/pflag`） |

---

## 3. 总体架构

### 3.1 数据流

```
Coding Agent (CC / CodeBuddy / Codex)
   │  stdin = hook JSON (agent-specific schema)
   │  argv  = ["--agent", "Codex", "--event", "PreToolUse"]
   ▼
cmd/pager-bridge/main.go
   │
   ├──[1] parseFlags(pflag)  →  agentLabel, eventType
   ├──[2] agents.SelectByLabel(agentLabel)  →  Agent 实例
   ├──[3] io.ReadAll(stdin)  →  raw bytes
   ├──[4] agent.ParseEnvelope(raw)  →  *Envelope
   ├──[5] agent.Accept(eventType, env) ?  →  否则 silent return
   ├──[6] bridge.Render(agent.ID(), eventType, env.ToolName, raw)
   │       │
   │       └─→ rules.Lookup(agentID, eventType)  →  yaml.Node
   │           rules.PickTemplate(toolName)       →  string
   │           Evaluate(template, raw, toolName)  →  contentRaw
   │           truncateRunes(contentRaw, 60)      →  content
   │
   ├──[7] 组装 entity.AgentEvent
   └──[8] bridge.PostEvent(e)  →  HTTP POST 127.0.0.1:7421/event
            │
            ▼
internal/adapter/httpapi/server.go: HandleEvent
   │  tracker.TrackEvent (DeriveStatus 走原有逻辑)
   │  store.Record (SQLite)
   ▼
UI / 系统通知 / 终端跳回
```

### 3.2 关注点分离

```
┌────────────────────────────────────────────────────────────────┐
│  内部分层                                                          │
│                                                                  │
│  cmd/pager-bridge        ←  唯一主流程（~30 行）                    │
│       │                                                          │
│       │ uses                                                     │
│       ▼                                                          │
│  internal/adapter/bridge/agents/    ←  Agent 抽象（每家一个 .go）   │
│       Agent interface                                            │
│       ClaudeFamily / Codex                                       │
│       registry                                                   │
│       │                                                          │
│       │ produces                                                 │
│       ▼                                                          │
│  internal/adapter/bridge/envelope.go ←  通用输入结构（11 字段）     │
│       │                                                          │
│       │ feeds into                                               │
│       ▼                                                          │
│  internal/adapter/bridge/render.go   ←  内容抽取统一入口           │
│       │                                                          │
│       │ delegates to                                             │
│       ▼                                                          │
│  internal/adapter/bridge/rules.go    ←  DSL 引擎（Evaluate + filter）│
│       extract_rules.yaml v3          ←  规则数据                   │
└────────────────────────────────────────────────────────────────┘
```

四个职责边界清晰：

- **Agent**：身份 + JSON schema → Envelope；hook 注册清单；早丢弃规则
- **Envelope**：路由所需的核心字段 + 完整 raw payload
- **Render**：根据 (agentID, eventType) 选规则、应用模板
- **Rules + Evaluate**：DSL 引擎，agent 无关、event 无关，只接 (template, payload) 出 string

---

## 4. 详细设计

### § 4.1 Agent 接口

#### 4.1.1 接口定义

```go
// internal/adapter/bridge/agents/agent.go

// Agent 描述一个被 pager 支持的 coding agent（CC / CodeBuddy / Codex / ...）。
// 一个 Agent 实现可以服务多个 label——例如 ClaudeFamily{} 同时映射 "CC"、
// "CC-Internal"、"CodeBuddy" 三个 label，因为它们 schema 完全相同。
type Agent interface {
    // ID 返回写入 entity.AgentEvent.Agent 的稳定标识，
    // 同时是 extract_rules.yaml 的 namespace key。
    // 必须返回 entity.AgentXxx 中的常量。
    ID() string

    // ParseEnvelope 把 hook stdin 原始字节解为通用 Envelope。
    // 各 agent 在这里实现自己的 JSON schema 映射（CC 读 source；
    // Codex 读 turn_id/trigger）。返回 ErrInvalidPayload 表示 JSON 解码失败。
    ParseEnvelope(raw []byte) (*Envelope, error)

    // Accept 决定一个事件是否应该上报给 pager 服务器。
    // 返回 false 时事件被静默丢弃（例：CodeBuddy 的 auth_success Notification
    // 没有 session_id，落到下游会污染 session 列表）。
    Accept(eventType string, env *Envelope) bool

    // Hooks 返回该 agent 应该注册的全部 hook 事件清单。
    // pager-installhooks 通过 --print-hooks 子命令读取这个清单，
    // 据此向各 settings 文件写入条目。
    Hooks() []HookSpec
}

// HookSpec 描述一个待注册的 hook 事件。
// Matcher 仅 PreToolUse / PostToolUse 等工具事件需要（"*" 表示匹配所有工具）；
// 非工具事件留空，installer 写入时不输出 matcher 字段。
type HookSpec struct {
    Event   string
    Matcher string
}
```

#### 4.1.2 ClaudeFamily 实现

```go
// internal/adapter/bridge/agents/claude.go

type ClaudeFamily struct{}

func (ClaudeFamily) ID() string { return entity.AgentClaudeCode }

func (ClaudeFamily) ParseEnvelope(raw []byte) (*Envelope, error) {
    var w struct {
        SessionID      string          `json:"session_id"`
        TranscriptPath string          `json:"transcript_path"`
        CWD            string          `json:"cwd"`
        HookEventName  string          `json:"hook_event_name"`
        PermissionMode string          `json:"permission_mode"`
        ToolName       string          `json:"tool_name"`
        ToolInput      json.RawMessage `json:"tool_input"`
        ToolUseID      string          `json:"tool_use_id"`
        ToolResponse   json.RawMessage `json:"tool_response"`
    }
    if err := json.Unmarshal(raw, &w); err != nil {
        return nil, ErrInvalidPayload
    }
    return &Envelope{
        SessionID:      w.SessionID,
        TranscriptPath: w.TranscriptPath,
        CWD:            w.CWD,
        EventName:      w.HookEventName,
        PermissionMode: w.PermissionMode,
        ToolName:       w.ToolName,
        ToolUseID:      w.ToolUseID,
        ToolInput:      w.ToolInput,
        ToolResponse:   w.ToolResponse,
        RawPayload:     raw,
    }, nil
}

func (ClaudeFamily) Accept(eventType string, env *Envelope) bool {
    // CodeBuddy 的 auth_success Notification 没 session_id，丢
    if eventType == "Notification" && env.SessionID == "" {
        return false
    }
    return true
}

func (ClaudeFamily) Hooks() []HookSpec {
    return []HookSpec{
        {Event: "PreToolUse", Matcher: "*"},
        {Event: "PostToolUse", Matcher: "*"},
        {Event: "PostToolUseFailure", Matcher: "*"},
        {Event: "PermissionRequest", Matcher: "*"},
        {Event: "PermissionDenied", Matcher: "*"},
        {Event: "PostToolBatch"},
        {Event: "SessionStart"},
        {Event: "SessionEnd"},
        {Event: "UserPromptSubmit"},
        {Event: "UserPromptExpansion"},
        {Event: "Stop"},
        {Event: "StopFailure"},
        {Event: "SubagentStart"},
        {Event: "SubagentStop"},
        {Event: "TaskCreated"},
        {Event: "TaskCompleted"},
        {Event: "Notification"},
        {Event: "PreCompact"},
        {Event: "PostCompact"},
        {Event: "InstructionsLoaded"},
        {Event: "Elicitation"},
        {Event: "MessageDisplay"},
        {Event: "ElicitationResult"},
    }
}
```

#### 4.1.3 Codex 实现

```go
// internal/adapter/bridge/agents/codex.go

type Codex struct{}

func (Codex) ID() string { return entity.AgentCodex }

func (Codex) ParseEnvelope(raw []byte) (*Envelope, error) {
    var w struct {
        SessionID      string          `json:"session_id"`
        TurnID         string          `json:"turn_id"`
        TranscriptPath string          `json:"transcript_path"`
        CWD            string          `json:"cwd"`
        HookEventName  string          `json:"hook_event_name"`
        PermissionMode string          `json:"permission_mode"`
        ToolName       string          `json:"tool_name"`
        ToolInput      json.RawMessage `json:"tool_input"`
        ToolUseID      string          `json:"tool_use_id"`
        ToolResponse   json.RawMessage `json:"tool_response"`
    }
    if err := json.Unmarshal(raw, &w); err != nil {
        return nil, ErrInvalidPayload
    }
    return &Envelope{
        SessionID:      w.SessionID,
        TurnID:         w.TurnID,
        TranscriptPath: w.TranscriptPath,
        CWD:            w.CWD,
        EventName:      w.HookEventName,
        PermissionMode: w.PermissionMode,
        ToolName:       w.ToolName,
        ToolUseID:      w.ToolUseID,
        ToolInput:      w.ToolInput,
        ToolResponse:   w.ToolResponse,
        RawPayload:     raw,
    }, nil
}

func (Codex) Accept(string, *Envelope) bool { return true }

func (Codex) Hooks() []HookSpec {
    return []HookSpec{
        {Event: "PreToolUse", Matcher: "*"},
        {Event: "PostToolUse", Matcher: "*"},
        {Event: "PermissionRequest", Matcher: "*"},
        {Event: "PreCompact"},
        {Event: "PostCompact"},
        {Event: "SessionStart"},
        {Event: "UserPromptSubmit"},
        {Event: "SubagentStart"},
        {Event: "SubagentStop"},
        {Event: "Stop"},
    }
}
```

#### 4.1.4 注册表

```go
// internal/adapter/bridge/agents/registry.go

var registry = map[string]Agent{
    "CC":          ClaudeFamily{},
    "CC-Internal": ClaudeFamily{},
    "CodeBuddy":   ClaudeFamily{},
    "Codex":       Codex{},
}

// SelectByLabel 根据 --agent 命令行参数返回对应 Agent 实例。
// 未知 label 返回 (nil, false)，主流程据此 silent return。
func SelectByLabel(label string) (Agent, bool) {
    a, ok := registry[label]
    return a, ok
}

// LabeledAgent 是 (label, agent) 的配对，All() 用这个返回。
type LabeledAgent struct {
    Label string
    Agent Agent
}

// All 返回所有已注册的 Agent，按 label 字典序。
// pager-installhooks 用这个枚举所有 settings 文件，避免在 installer 里重复维护
// label 列表。
func All() []LabeledAgent {
    labels := make([]string, 0, len(registry))
    for l := range registry {
        labels = append(labels, l)
    }
    sort.Strings(labels)
    out := make([]LabeledAgent, 0, len(labels))
    for _, l := range labels {
        out = append(out, LabeledAgent{Label: l, Agent: registry[l]})
    }
    return out
}
```

### § 4.2 Envelope

```go
// internal/adapter/bridge/envelope.go

// Envelope 是 hook stdin 经 Agent 解析后的通用形态，承担两个角色：
//   1. 给主流程提供路由 / 状态机所需的核心字段
//   2. 通过 RawPayload 字段把完整 JSON 透传给 DSL 规则引擎
//
// 注意：本结构体只放"主流程使用"的字段。事件特定字段（source / reason /
// last_assistant_message / notification_type / agent_type / ...）一律不进，
// 它们由 DSL 直接从 RawPayload 通过 gjson 路径读取。
type Envelope struct {
    SessionID      string          // Registry / SessionKey
    TranscriptPath string          // 日志展示
    CWD            string          // SessionKey fallback
    EventName      string          // hook_event_name
    PermissionMode string          // DeriveStatus 输入
    ToolName       string          // PreToolUse/PostToolUse 模板路由
    ToolUseID      string          // PostToolUse 配对
    ToolInput      json.RawMessage // 工具规则 pre 输入（仅向后兼容保留；实际 DSL 直接读 RawPayload）
    ToolResponse   json.RawMessage // 同上
    TurnID         string          // Codex 才填
    RawPayload     json.RawMessage // 完整 stdin，DSL 读这个
}
```

字段总数从 `CCHookInput` 的 33 个降到 11 个；纯路由必需，不留事件特定 union 字段。

`CCHookInput` struct 整体删除（含其下挂的 `BashInput` / `EditInput` / `GlobInput` 等子 struct，DSL 不需要 typed 子结构）。

### § 4.3 DSL v3：YAML + filter + Render

#### 4.3.1 YAML 结构（事件名做一级 key）

`extract_rules.yaml`：

```yaml
version: 3

claude:
  PreToolUse: &claude_pre
    Bash:            "{$.tool_input.command}"
    Edit:            "编辑 {$.tool_input.file_path}"
    Write:           "写入 {$.tool_input.file_path}"
    Read:            "读取 {$.tool_input.file_path}"
    Glob:            "查找 {$.tool_input.pattern}"
    Grep:            "搜索 {$.tool_input.pattern}"
    WebFetch:        "抓取 {$.tool_input.url}"
    WebSearch:       "搜索 {$.tool_input.query}"
    AskUserQuestion: "{$.tool_input.questions.0.question}"
    Agent:           "子任务: {$.tool_input.description // $.tool_input.prompt}"
    Task:            "子任务: {$.tool_input.description}"
    TaskCreate:      "{$.tool_input.subject}"
    TaskUpdate:      "{$.tool_input.subject} →{$.tool_input.status}"
    NotebookEdit:    "{$.tool_input.notebook_path}"
    LSP:             "LSP {$.tool_input.operation}: {$.tool_input.filePath}"
    default:         "{$.tool_name}"

  PostToolUse:
    Bash:            "{$.tool_response.stdout|firstline} (exit={$.tool_response.exitCode|default:0})"
    Read:            "已读 {$.tool_response.file.numLines|default:?} 行"
    Edit:            "{$.tool_response.success|bool:✓写入,✗失败} {$.tool_response.filePath}"
    Write:           "{$.tool_response.success|bool:✓写入,✗失败} {$.tool_response.filePath}"
    AskUserQuestion: "{$.tool_response.answers.0 // $.tool_response}"
    Agent:           "{$.tool_response.0.text|firstpara}"
    default:         "{$.tool_response.stdout // $.tool_response.text // $.tool_response.message // $.tool_response.result // $.tool_response.summary // $.tool_response}"

  PermissionRequest: *claude_pre                         # YAML anchor 复用 PreToolUse
  PermissionDenied:  "{$.tool_name}: {$.denial_reason}"

  SessionStart:        "{$.source}"
  SessionEnd:          "{$.reason}"
  UserPromptSubmit:    "{$.prompt}"
  UserPromptExpansion: "/{$.command_name}{$.command_args|prefix: }"
  Stop:                "{$.last_assistant_message} // {$.stop_reason}"
  StopFailure:         "{$.error_type}: {$.error_message}"
  Error:               "{$.error_type}: {$.error_message}"
  SubagentStart:       "{$.agent_type} // {$.agent_id}"
  SubagentStop:        "{$.last_assistant_message} // {$.agent_type}"
  TaskCreated:         "{$.task_subject}"
  TaskCompleted:       "{$.task_subject}"
  PostToolUseFailure:  "{$.tool_name}: {$.error}"
  PostToolBatch:       "{$.tool_calls|pluck:tool_name|join:, }"
  Notification:        "{$.notification_type|lookup:permission_prompt=[等授权] ,idle_prompt=[等输入] ,default=}{$.message // $.notification_type}"
  Elicitation:         "{$.server_name}: {$.message}"
  ElicitationResult:   "{$.server_name}"
  PreCompact:          "{$.trigger}"
  PostCompact:         ""
  InstructionsLoaded:  "{$.file_path}"
  MessageDisplay:      "{$.delta}"

codex:
  PreToolUse: &codex_pre
    exec_command: "{$.tool_input.cmd}"
    write_stdin:  "stdin→session {$.tool_input.session_id}"
    default:      "{$.tool_name}"

  PostToolUse:
    exec_command: "{$.tool_response|firstline}"
    write_stdin:  "{$.tool_response|firstline}"
    default:      "{$.tool_response|firstline}"

  PermissionRequest: *codex_pre

  SessionStart:        "{$.trigger}"
  UserPromptSubmit:    "{$.prompt}"
  Stop:                "{$.stop_reason}"
  SubagentStart:       "{$.agent_type}"
  SubagentStop:        "{$.agent_type}"
  PreCompact:          "{$.trigger}"
  PostCompact:         ""
```

**关键设计点**：

1. **事件名做一级 key**——废除 `pre`/`post`/`events` 中间层
2. **DSL payload 永远是完整 stdin JSON**——`{$.tool_input.command}` 而非 `{$.command}`；所有事件 DSL 输入统一
3. **per-tool 分桶事件用 yaml.MappingNode**（map[string]string），单 string 模板事件用 yaml.ScalarNode
4. **`PermissionRequest: *claude_pre`** 通过 YAML anchor 复用 PreToolUse 整组规则，零 Go 代码处理

#### 4.3.2 Rules struct v3

```go
// internal/adapter/bridge/rules.go

type Rules struct {
    Version int                   `yaml:"version"`
    Claude  map[string]yaml.Node  `yaml:"claude"`
    Codex   map[string]yaml.Node  `yaml:"codex"`
}

// Lookup 返回 (agentID, eventType) 对应的 yaml.Node。
// 节点 Kind 为 ScalarNode（单 string 模板）或 MappingNode（per-tool 分桶）。
// 未命中返回 (zero, false)。
func (r *Rules) Lookup(agentID, eventType string) (yaml.Node, bool) {
    var bucket map[string]yaml.Node
    switch agentID {
    case entity.AgentClaudeCode:
        bucket = r.Claude
    case entity.AgentCodex:
        bucket = r.Codex
    default:
        return yaml.Node{}, false
    }
    n, ok := bucket[eventType]
    return n, ok
}

// PickTemplate 在 MappingNode 中按 toolName 查模板，找不到回落 "default"。
// ScalarNode 直接返回 .Value，忽略 toolName。
func PickTemplate(n yaml.Node, toolName string) string { ... }
```

#### 4.3.3 Evaluate 升级——链式 filter

当前 `resolveToken` 只支持单个 filter（`{$.x|firstline}`）。重构后支持链式：

```go
// 解析 token 体的伪代码：
//   "$.tool_calls|pluck:tool_name|join:, "
//   →
//   path = "$.tool_calls"
//   filters = [
//     {name: "pluck", arg: "tool_name"},
//     {name: "join",  arg: ", "},
//   ]
//   → 顺序应用每个 filter

func resolveToken(token string, payload []byte, toolName string) string {
    parts := strings.Split(token, "|")
    head := strings.TrimSpace(parts[0])
    value := resolvePath(head, payload, toolName)  // 取初始值
    for _, f := range parts[1:] {
        name, arg := splitFilter(strings.TrimSpace(f))
        value = applyFilter(name, arg, value)
    }
    return value
}
```

#### 4.3.4 新增 4 个 filter

| Filter | 形态 | 实现 | LOC |
|---|---|---|---|
| `prefix:<lit>` | `{$.x\|prefix: }` | `if value=="" { return "" }; return literal+value` | ~8 |
| `lookup:k1=v1,k2=v2,default=dv` | `{$.x\|lookup:permission_prompt=[等授权] ,idle_prompt=[等输入] ,default=}` | parse args 成 map，按 value 查表，未命中走 default | ~25 |
| `pluck:<field>` | `{$.tool_calls\|pluck:tool_name}` | `gjson.Get(value, "#.<field>").Raw` | ~10 |
| `join:<sep>` | `{$.x\|pluck:tool_name\|join:, }` | 解码 JSON 数组（必须是 string 数组）→ `strings.Join` | ~15 |

#### 4.3.5 Render 单一入口

```go
// internal/adapter/bridge/render.go

const contentMaxRunes = 60

// Render 把 (agent, event, tool, payload) 解析为展示字符串。
// payload 必须是 hook stdin 的完整原文。
// 非工具事件传 toolName="" 即可，YAML 该事件下若是 ScalarNode 不走 toolName 分支。
func Render(agentID, eventType, toolName string, payload []byte) (raw, content string) {
    node, ok := loadedRules.Lookup(agentID, eventType)
    if !ok {
        return "", ""
    }
    template := PickTemplate(node, toolName)
    raw = strings.TrimSpace(Evaluate(template, payload, toolName))
    return raw, truncateRunes(raw, contentMaxRunes)
}
```

主流程里只有一处调用：

```go
contentRaw, content := bridge.Render(agent.ID(), eventType, env.ToolName, raw)
```

无 switch、无 pre/post 分流、无 RenderContent 中转——`PreToolUse` 和 `Stop` 走完全相同的代码，差异完全在 YAML。

### § 4.4 主流程

```go
// cmd/pager-bridge/main.go

func main() {
    defer os.Exit(0)

    var (
        agentLabel string
        eventType  string
        printHooks bool
        debug      bool
    )
    pflag.StringVar(&agentLabel, "agent", "", "agent label (CC/CC-Internal/CodeBuddy/Codex)")
    pflag.StringVar(&eventType, "event", "", "hook event name")
    pflag.BoolVar(&printHooks, "print-hooks", false, "print hook spec JSON for given --agent and exit")
    pflag.BoolVar(&debug, "debug", false, "dump raw stdin to /tmp/pager-raw.json")
    pflag.Parse()

    if printHooks {
        printHooksJSON(agentLabel)
        return
    }

    agent, ok := agents.SelectByLabel(agentLabel)
    if !ok {
        return  // 未知 agent，silent
    }

    raw, err := io.ReadAll(os.Stdin)
    if err != nil {
        return
    }
    if debug || os.Getenv("PAGER_DEBUG") == "1" {
        _ = os.WriteFile("/tmp/pager-raw.json", raw, 0644)
    }

    env, err := agent.ParseEnvelope(raw)
    if err != nil || !agent.Accept(eventType, env) {
        return
    }

    contentRaw, content := bridge.Render(agent.ID(), eventType, env.ToolName, raw)

    bridge.PostEvent(entity.AgentEvent{
        Agent:          agent.ID(),
        AgentLabel:     agentLabel,
        EventType:      eventType,
        SessionID:      env.SessionID,
        CWD:            firstNonEmpty(env.CWD, os.Getenv("PWD")),
        TTY:            detectTTY(),
        Host:           "local",
        TermProgram:    os.Getenv("TERM_PROGRAM"),
        ITermSessionID: os.Getenv("ITERM_SESSION_ID"),
        ToolName:       env.ToolName,
        ToolUseID:      env.ToolUseID,
        PermissionMode: env.PermissionMode,
        Content:        content,
        ContentRaw:     contentRaw,
        RawPayload:     raw,
        Timestamp:      time.Now(),
    })
}
```

主流程主体 ~30 行（含 pflag 设置）。每行一个职责，无 switch eventType，无硬编码 agent 字符串，无事件特定字段读取。

### § 4.5 `--print-hooks` 协议

```bash
$ pager-bridge --print-hooks --agent Codex
[
  {"event": "PreToolUse", "matcher": "*"},
  {"event": "PostToolUse", "matcher": "*"},
  {"event": "PermissionRequest", "matcher": "*"},
  {"event": "PreCompact"},
  {"event": "PostCompact"},
  {"event": "SessionStart"},
  {"event": "UserPromptSubmit"},
  {"event": "SubagentStart"},
  {"event": "SubagentStop"},
  {"event": "Stop"}
]
```

实现：

```go
func printHooksJSON(agentLabel string) {
    agent, ok := agents.SelectByLabel(agentLabel)
    if !ok {
        fmt.Fprintf(os.Stderr, "unknown agent: %s\n", agentLabel)
        os.Exit(1)
    }
    out := agent.Hooks()
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    _ = enc.Encode(out)
}
```

未来加新 agent 不需要修改 installer——installer 通过这个协议自动发现。

### § 4.6 pager-installhooks

#### 4.6.1 责任

替代 `scripts/install-hooks.sh`。一次执行：

1. 枚举所有已注册 agent
2. 每个 agent：
   - 调用 `pager-bridge --print-hooks --agent <label>` 取事件清单
   - 决定目标 settings 文件路径
   - 读取 / 创建该文件
   - **append + 幂等**写入：先剔除所有 `command` 包含 `vibecoding-pager-cc-bridge` 或 `pager-bridge` 的旧条目，再 append 当前版本
   - 写回文件

#### 4.6.2 Agent → 文件路径映射

```go
// cmd/pager-installhooks/targets.go

type Target struct {
    AgentLabel   string
    SettingsFile string
    Format       string  // "settings" | "hooks-only"
}

var targets = []Target{
    {"CC",          "~/.claude/settings.local.json",          "settings"},
    {"CC-Internal", "~/.claude-internal/settings.local.json", "settings"},
    {"CodeBuddy",   "~/.codebuddy/settings.local.json",       "settings"},
    {"Codex",       "~/.codex/hooks.json",                    "hooks-only"},
}
```

> **CC-Internal 验证项**：当前 Makefile 的 `install-hooks-cc-internal` target 写的是 `~/.claude-internal/settings.json`（非 `.local.json`）。本 spec 把它改成 `.local.json` 是为了与 CC / CodeBuddy 一致——但要在 PR 1 实施时验证 CC-Internal 的 loader 确实识别 `settings.local.json`。如果不识别，回退到 `settings.json` 但保留 append+幂等写入策略（用户主 settings 不被覆盖式破坏）。

两种 format：

- `settings`：文件顶层是完整 settings 对象，hooks 在 `.hooks` 字段下
- `hooks-only`：文件顶层就是 hooks 对象（Codex `~/.codex/hooks.json` 的格式约定）

#### 4.6.3 写入算法

```go
const (
    legacyBinaryName  = "vibecoding-pager-cc-bridge"  // 旧名，迁移用
    currentBinaryName = "pager-bridge"
)

// isPagerHookGroup 判断 settings.json 里的某个 hook group 是否由 pager 自己写入。
// 用 basename 严格匹配（避免误判用户自己写的 /custom/pager-bridge-helper 之类）。
func isPagerHookGroup(group HookGroup) bool {
    for _, h := range group.Hooks {
        // 取命令的第一个 token 作为可执行文件路径
        cmdParts := strings.Fields(h.Command)
        if len(cmdParts) == 0 {
            continue
        }
        base := filepath.Base(cmdParts[0])
        if base == currentBinaryName || base == legacyBinaryName {
            return true
        }
    }
    return false
}

func upsertHooks(settings map[string]any, agentLabel string, specs []HookSpec, bridgePath string) {
    hooks, _ := settings["hooks"].(map[string]any)
    if hooks == nil {
        hooks = map[string]any{}
    }
    for _, spec := range specs {
        existing, _ := hooks[spec.Event].([]any)
        // 1. 剔除所有 pager 自己的条目（兼容旧名）
        filtered := make([]any, 0, len(existing))
        for _, g := range existing {
            if group, ok := g.(map[string]any); ok && !isPagerHookGroup(group) {
                filtered = append(filtered, g)
            }
        }
        // 2. append 当前版本的 pager 条目
        pagerGroup := buildHookGroup(spec, agentLabel, bridgePath)
        filtered = append(filtered, pagerGroup)
        hooks[spec.Event] = filtered
    }
    settings["hooks"] = hooks
}
```

幂等保证：重复执行 `make install-bridge` 不会重复 entry，且会自动迁移老二进制路径。用户自定义的非 pager hook 在 `filtered` 阶段被完整保留。

#### 4.6.4 命令行参数（pflag）

```bash
pager-installhooks                        # 默认行为：枚举所有 agent，全部安装
pager-installhooks --agent CC             # 仅装一家
pager-installhooks --agent CC --dry-run   # 只打印 diff，不写文件
pager-installhooks --uninstall            # 反向操作：剔除所有 pager hook
pager-installhooks --bridge /custom/path  # 覆盖默认 bridge 路径
```

```go
pflag.StringVar(&agentFilter, "agent", "", "only install for this agent label (default: all)")
pflag.BoolVar(&dryRun, "dry-run", false, "show what would change, do not write")
pflag.BoolVar(&uninstall, "uninstall", false, "remove pager hook entries from all settings files")
pflag.StringVar(&bridgePath, "bridge", defaultBridgePath(), "path to pager-bridge binary")
```

`defaultBridgePath()` 实现：
```go
// 优先用 installer 自己所在目录里的 pager-bridge —— make install-bridge 的标准布局
// 是 bin/pager-bridge 与 bin/pager-installhooks 同目录。
func defaultBridgePath() string {
    self, err := os.Executable()
    if err != nil {
        return "pager-bridge"  // 回落到 PATH 查找
    }
    return filepath.Join(filepath.Dir(self), "pager-bridge")
}
```

### § 4.7 状态机扩展

`internal/domain/entity/event.go` 新增两个常量：

```go
EventPostCompact   = "PostCompact"
EventSubagentStart = "SubagentStart"
```

`internal/domain/session/status.go` 的 `DeriveStatus` **不需要新 case**——这俩事件是"运行中"语义，落到 default 分支返回 `StatusWorking`。

`status_test.go` 新增覆盖：

```go
{name: "PostCompact → working", eventType: "PostCompact", want: StatusWorking},
{name: "SubagentStart → working", eventType: "SubagentStart", want: StatusWorking},
{name: "Codex SessionStart → working", eventType: "SessionStart", permissionMode: "default", want: StatusWorking},
```

### § 4.8 二进制改名 + 迁移

**改名映射**：

| 旧 | 新 |
|---|---|
| `bin/vibecoding-pager-cc-bridge` | `bin/pager-bridge` |
| `cmd/bridge/main.go` | `cmd/pager-bridge/main.go` |

**Makefile 调整**：

```makefile
BRIDGE_BIN := $(PWD)/bin/pager-bridge
INSTALLER_BIN := $(PWD)/bin/pager-installhooks

.PHONY: bridge installer install-bridge

bridge:
	go build -o $(BRIDGE_BIN) ./cmd/pager-bridge

installer:
	go build -o $(INSTALLER_BIN) ./cmd/pager-installhooks

install-bridge: bridge installer
	$(INSTALLER_BIN) --bridge $(BRIDGE_BIN)
	@echo "✓ pager-bridge installed at $(BRIDGE_BIN)"
	@stat -f "  mtime: %Sm  sha: $$(shasum -a 256 $(BRIDGE_BIN) | cut -d' ' -f1)" $(BRIDGE_BIN)
```

废除以下 target：
- `install-hooks`（被 installer 自带功能取代）
- `install-hooks-codebuddy`
- `install-hooks-cc-internal`

废除文件：
- `scripts/install-hooks.sh`

**迁移行为**：

第一次 `make install-bridge` 后：
- `bin/vibecoding-pager-cc-bridge`：用户可手动 `rm` 或留着，installer 不删
- `~/.claude/settings.json`、`~/.codebuddy/settings.json` 中**留有 `vibecoding-pager-cc-bridge` 路径的 hook entry**：会被 installer 检测并清理（pager-installhooks 默认扫所有 4 个目标 settings 文件）

为兼容老用户，installer 加一段额外清理逻辑：扫 `~/.claude/settings.json`（不是 `.local.json`）和 `~/.codebuddy/settings.json`，剔除其中的 pager 条目（防止双份）。新 entry 只写入 `.local.json`。

### § 4.9 settings 文件分布

写入后的最终状态：

```
~/.claude/
  settings.json          ← 用户主配置，pager 不动
  settings.local.json    ← pager 写这里（hooks）

~/.claude-internal/
  settings.json          ← 用户主配置，pager 不动
  settings.local.json    ← pager 写这里（hooks）

~/.codebuddy/
  settings.json          ← 用户主配置，pager 不动
  settings.local.json    ← pager 写这里（hooks）

~/.codex/
  config.toml            ← 用户主配置，pager 不动
  hooks.json             ← pager 写这里（顶层 = hooks 对象）
```

CC/CodeBuddy 没有 include 机制，但 `settings.local.json` 是它们 loader 内置识别的 overlay 文件，pager 写这里既不污染主 settings 也不需要用户做任何 hack。

---

## 5. 工程改造清单

### 5.1 新文件

| 路径 | 用途 |
|---|---|
| `cmd/pager-bridge/main.go` | 主流程（替代 `cmd/bridge/main.go`） |
| `cmd/pager-installhooks/main.go` | installer 入口 |
| `cmd/pager-installhooks/targets.go` | agent → 文件路径映射 |
| `cmd/pager-installhooks/upsert.go` | append + 幂等 写入逻辑 |
| `internal/adapter/bridge/agents/agent.go` | Agent 接口 + HookSpec |
| `internal/adapter/bridge/agents/claude.go` | ClaudeFamily 实现 |
| `internal/adapter/bridge/agents/codex.go` | Codex 实现 |
| `internal/adapter/bridge/agents/registry.go` | label → Agent 映射 |
| `internal/adapter/bridge/envelope.go` | Envelope struct |
| `internal/adapter/bridge/render.go` | Render 单一入口 |

### 5.2 删除文件

| 路径 | 删除原因 |
|---|---|
| `cmd/bridge/main.go` | 改为 `cmd/pager-bridge/main.go` |
| `cmd/bridge/main_test.go` | 同上（测试随之搬家） |
| `scripts/install-hooks.sh` | 改为 Go 实现 |
| `internal/adapter/bridge/extractor.go` | `ExtractContent` / `ExtractEventContent` 整体被 Render 取代 |
| `internal/adapter/bridge/extractor_test.go` | 测试随之搬家 |

### 5.3 重大改动文件

| 路径 | 改动概要 |
|---|---|
| `internal/adapter/bridge/types.go` | 删除 `CCHookInput` 与所有 typed sub-input struct（约 -120 行） |
| `internal/adapter/bridge/rules.go` | `Rules` 结构升 v3；`Evaluate` 支持链式 filter；新增 4 个 filter |
| `internal/adapter/bridge/rules_test.go` | 全面重写：覆盖 v3 schema、链式 filter、4 个新 filter |
| `internal/adapter/bridge/extract_rules.yaml` | v3 重组，事件名做一级 key，包含 claude/codex 两段 |
| `internal/adapter/bridge/poster.go` | 不变（HTTP POST 逻辑无关 agent） |
| `internal/domain/entity/event.go` | 新增 `EventPostCompact` / `EventSubagentStart` |
| `internal/domain/session/status_test.go` | 增加 Codex 对应用例 |
| `Makefile` | `BRIDGE_BIN` 改名；废弃 install-hooks 系列 target；新增 installer target |
| `go.mod` / `go.sum` | 引入 `github.com/spf13/pflag` |

### 5.4 文档更新

`CLAUDE.md`：
- "Bridge Binary Lifecycle" 节：`vibecoding-pager-cc-bridge` → `pager-bridge`
- "Key Files" 表：所有 `cmd/bridge/main.go` 路径改 `cmd/pager-bridge/main.go`
- 新增"Codex Hook Integration"小节，说明 `~/.codex/hooks.json` 与 first-time trust 流程
- "Configuration Paths" 节：补充 `~/.claude/settings.local.json` 等四个目标文件的角色

`docs/PRD.md`：
- 第二代 agent 接入：从"Codex 字段保留未接通"改为"Codex 完整接通，事件清单见 v3 YAML"

### 5.5 改动量估算

| 类别 | 净 LOC |
|---|---|
| 新增 Go 代码 | +650 |
| 删除 Go 代码 | -480 |
| 新增 YAML | +60 |
| 删除 shell | -130 |
| 新增测试 | +250 |
| 删除测试 | -180 |
| **净增** | **+170** |

---

## 6. 实施顺序（PR 切分建议）

按依赖收敛，三段式落地：

### PR 1：Bridge 重构（不接 Codex）

**目标**：架构搬到位，行为零变化。CC + CodeBuddy 现网完全等效。

- 新增 Agent 接口 + ClaudeFamily 实现 + registry
- Envelope 替代 CCHookInput
- DSL v3：YAML 升 v3、Evaluate 链式、4 个新 filter
- `cmd/pager-bridge/main.go` 取代 `cmd/bridge/main.go`
- `cmd/pager-installhooks/` 取代 `scripts/install-hooks.sh`
- 二进制改名 + 迁移逻辑
- Makefile 调整
- 运行 `make install-bridge`，CC/CodeBuddy 重新触发若干事件，**对比新旧 ContentRaw 完全一致**

**验收**：`make test` 全绿；CC + CodeBuddy 实操一遍，session 列表/UI 行为无差异。

### PR 2：Codex 接入

**目标**：Codex 0.137 完整可观测。

- 新增 `internal/adapter/bridge/agents/codex.go`
- registry 加 `"Codex": Codex{}`
- `entity.EventPostCompact` / `EventSubagentStart` 新常量 + status_test 用例
- `extract_rules.yaml` 增加 `codex:` 段
- `pager-installhooks` 的 `targets.go` 加 Codex target
- 真实场景验证：在 `project_grace` 启动 codex，触发 10 个事件，UI 全显示

**验收**：手动测试 + Codex first-run trust 流程文档；CLAUDE.md 增补 Codex 节。

### PR 3：文档与边角

- CLAUDE.md / PRD.md 更新
- 旧 `bin/vibecoding-pager-cc-bridge` 清理（installer log 提示用户手动 rm）
- 新增 `pager-installhooks --uninstall` 命令的文档

---

## 7. 测试策略

### 7.1 单测

| 模块 | 用例要点 |
|---|---|
| `agents/claude_test.go` | ParseEnvelope 解 22 个 fixture（每事件类型一份）；Accept 对 auth_success 返回 false；Hooks 数量正确 |
| `agents/codex_test.go` | ParseEnvelope 解真实 codex stdin（带 turn_id）；SessionStart 的 trigger 进 RawPayload；Hooks 数量正确 |
| `agents/registry_test.go` | 4 个 label 都能映射；未知 label 返 false |
| `bridge/rules_test.go` | v3 YAML 加载；Lookup 落 ScalarNode/MappingNode 两路；PickTemplate 的 default 回落；4 个新 filter 各自单测；链式 filter 顺序正确 |
| `bridge/render_test.go` | (claude, PreToolUse, Bash) 出字符串；(codex, PostToolUse, exec_command) 出 firstline；Notification 走 lookup filter；PostToolBatch 走 pluck+join |
| `cmd/pager-bridge/main_test.go` | argv 解析；--print-hooks 输出 JSON；agent 未知 silent return |
| `cmd/pager-installhooks/upsert_test.go` | 空 settings 写入；已存在 pager 旧条目（含 legacy 名）→ 替换；用户自定义条目 → 保留；--dry-run 不写文件；--uninstall 全清 |
| `domain/session/status_test.go` | 新增 PostCompact / SubagentStart / Codex SessionStart 用例 |

### 7.2 集成测试

- 启动 `bin/pager-bridge --print-hooks --agent Codex`，断言 stdout JSON 与 fixture 比对
- `cmd/pager-installhooks` 在 tmpdir 模拟 `~/.codex/hooks.json` 写入，断言文件结构正确

### 7.3 手动验收

按 PR 切分各自的"验收"部分执行。

---

## 8. 风险 & 缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| Codex hook trust 拒绝弹出导致 hooks 不触发 | Codex 事件零接收 | CLAUDE.md 写明首次启动 Codex 后必看 "Trust all and continue" 弹窗；installer 写入完成后 stdout 提示 |
| Codex 实际 stdin schema 与 binary 字符串推断不符 | Envelope.ParseEnvelope 拿不到字段 | PR 2 阶段先用 `--debug` 把 raw stdin 落到 `/tmp/pager-raw.json`，对照真实样本调整 fixture 与解析 |
| YAML anchor 在 yaml.v3 处理顺序问题 | `PermissionRequest: *claude_pre` 解析失败 | 在 `rules_test.go` 显式覆盖 PermissionRequest 与 PreToolUse 的 yaml.Node 在内存上的指针/值一致性 |
| 二进制改名后老用户重启 pager.app 时 hook 失效 | 用户体验断裂 | installer 自动清理老路径条目；Makefile `install-bridge` 在末尾打印迁移摘要 |
| `pflag` 引入对其他 cmd（icongen / testserver）传染压力 | 代码风格混乱 | 本 spec 范围内仅迁移 pager-bridge 和 pager-installhooks；其他 cmd 保留 stdlib `flag`，未来有改动再改 |
| pluck filter 的 gjson 路径行为依赖版本 | 数组投影出错 | rules_test.go 显式锁定 gjson `#.field` 行为；go.sum 锁版本 |
| installer 误删用户自定义 PreToolUse hook | 用户工作流被破坏 | `isPagerHookGroup` 严格匹配 binary 名（vibecoding-pager-cc-bridge / pager-bridge）；非 pager 条目零触碰；新增 `--dry-run` 让用户先看 diff |

---

## 9. 不在范围内

- 不引入 Aider / Cursor / 其他 agent 的具体接入（Agent 接口已经预留扩展点，但本 spec 不做实际连线）
- 不重写 `extract_rules.yaml` 之外的工具规则（如新增 MCP 工具的 pre/post 模板）
- 不动 `internal/adapter/notify/` 通知逻辑、`internal/wails/` 框架层
- 不改 SQLite schema（`Agent` 字段值范围扩展不需要 DDL 调整）
- icongen / testserver 不迁移到 pflag

---

## 10. 关键文件索引（实施时直接命中）

| 文件 | 角色 |
|---|---|
| `cmd/pager-bridge/main.go` | 主流程入口 |
| `cmd/pager-installhooks/main.go` | installer 入口 |
| `internal/adapter/bridge/agents/agent.go` | Agent 接口契约 |
| `internal/adapter/bridge/agents/claude.go` | ClaudeFamily 实现 |
| `internal/adapter/bridge/agents/codex.go` | Codex 实现 |
| `internal/adapter/bridge/agents/registry.go` | 单一注册点（新 agent 加这里） |
| `internal/adapter/bridge/envelope.go` | 通用输入结构 |
| `internal/adapter/bridge/render.go` | DSL 唯一调用入口 |
| `internal/adapter/bridge/rules.go` | v3 schema + Evaluate + filter 实现 |
| `internal/adapter/bridge/extract_rules.yaml` | 模板规则数据 |
| `internal/domain/entity/event.go` | 事件常量（新 agent 加事件加这里） |
| `internal/domain/session/status.go` | 状态机（不变，但需复核新事件落 default 分支正确） |

---

## 11. 参考

- 上一版 hook 增强 spec：`docs/superpowers/specs/2026-06-01-hook-enrichment-design.md`
- 事件解析 bug 修复 spec：`docs/superpowers/specs/2026-06-03-pager-event-parsing-bugfix-design.md`
- 整体架构：`ARCHITECTURE.md`
- 项目主指令：`CLAUDE.md`（Codex Hook Integration 节将在 PR 2 后补充）
