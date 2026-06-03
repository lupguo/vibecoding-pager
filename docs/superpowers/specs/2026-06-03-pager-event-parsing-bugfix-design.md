# Pager 事件解析 4-Bug 修复设计

> 日期：2026-06-03
> 范围：bridge 事件提取 + Pager 主进程行为 + dev tooling
> 工作量：~6.8h

---

## 1. 背景

近期 Pager 收集的事件（CC / CC-INT / CodeBuddy 三个 Agent）在数据库 `t_events` 表中暴露 4 类问题，影响 UI 卡片可读性与会话识别准确性。本设计一次性修复 4 个 bug，并引入 dev tooling 防回归。

### 1.1 4 个 Bug 现象与诊断

| # | 现象 | 根因 | 修复层 |
|---|---|---|---|
| 1 | session_id 看起来是路径（id=7565） | CodeBuddy `auth_success` Notification 无 `session_id`，触发 `host:cwd:tty` fallback | bridge 加事件丢弃白名单 |
| 2 | PostToolUse(AskUserQuestion).content 是问题不是答案 | `ExtractContent` 不区分 Pre/Post 阶段，没看 `tool_response` | bridge 加 PostToolUse 分支 + 通用 tool_response 解析 |
| 3 | Notification 内容同质化、缺乏区分度 | `notification_type` 未参与 content 拼装 | bridge 在 message 前置类型 label |
| 4a | TaskCreated / PostToolBatch / TaskCompleted / MessageDisplay / SubagentStart 全空 | bridge 二进制过期（6/01 build 不含 6/02 commit `1ba099f` 字段对齐 refactor） | dev tooling：rebuild + Makefile + 文档 |
| 4b | SessionEnd 始终空 | `extractor.go` 中故意 `return "", ""` | bridge 改用 `reason` 字段 |
| 4c | SubagentStart 空（CodeBuddy） | code 取 `agent_type`，CodeBuddy raw_payload 只传 `agent_id` | bridge 加 `agent_id` fallback |

### 1.2 实证数据（id > 7000 抽样）

```
event_type        total  empty  empty_rate
PostToolBatch       21    21     100%
TaskCreated         16    16     100%
SessionEnd           8     8     100%
SubagentStart        8     8     100%
TaskCompleted        3     3     100%
MessageDisplay       1     1     100%
SubagentStop        10     1      10%
```

Notification 子类型分布：

```
notification_type    count   占比
idle_prompt          115     73%
permission_prompt     24     15%
auth_success           3      2%
(空 / 未知)           12      8%
```

### 1.3 跨 Agent Schema 差异（影响设计取舍）

- **CodeBuddy auth_success Notification** 没有 `session_id` 字段
- **AskUserQuestion** 在 CC-INT 与 CodeBuddy 的 `tool_response` schema 完全不同：
  - CC-INT：`{questions:[...], answers:["问：答"]}`（object）
  - CodeBuddy：`" · 问 → 答"`（裸 string）
- **SubagentStart** CodeBuddy 不传 `agent_type` 仅 `agent_id`

→ 决定 § 3 走"配置驱动 + 多 schema 兼容"骨架。

---

## 2. 总体架构

修复横跨三层，每层独立可测：

```
┌────────────────────────────────────────┐
│  bridge (cmd/bridge + adapter/bridge)  │ ← Bug 1 / 2 / 3 / 4b / 4c
│   • 事件丢弃白名单                       │
│   • Pre/Post 阶段化提取（YAML 规则驱动） │
│   • Notification 类型 label 拼装        │
│   • 缺失字段 fallback                   │
└────────────────────────────────────────┘
           │ HTTP POST :7421
           ▼
┌────────────────────────────────────────┐
│  Pager 主进程 (domain/session)          │ ← 本 spec 不改
└────────────────────────────────────────┘

┌────────────────────────────────────────┐
│  dev tooling (Makefile / CLAUDE.md)    │ ← Bug 4a
│   • install-bridge 显式部署目标         │
│   • dev/run/build 前置依赖              │
└────────────────────────────────────────┘
```

**依赖原则**：domain ← adapter ← cmd；新增 `github.com/tidwall/gjson` 仅在 `internal/adapter/bridge` 引入；其他模块保持 stdlib-only。

---

## 3. 详细设计

### § 3.1 Bug 1 — 无 session_id 系统通知丢弃

**位置**：`cmd/bridge/main.go`

**变更**：
```go
// 在 json.Unmarshal 之后、ExtractContent 之前
if shouldDrop(eventType, &in) { return }

func shouldDrop(eventType string, in *bridge.CCHookInput) bool {
    // 仅对系统级 Notification 且无 session_id 时丢弃
    if eventType == "Notification" && in.SessionID == "" {
        return true
    }
    return false
}
```

**为什么不全局丢弃 SessionID 空事件**：保留 fallback (`host:cwd:tty`) 以容错未来 CC 版本字段缺失场景，仅针对已知噪音类（`auth_success` / 早期生命周期）做局部丢弃。

**测试**：
- 输入：raw_payload 无 `session_id`，`hook_event_name=Notification` → 期望 `PostEvent` 未被调用
- 输入：raw_payload 无 `session_id`，`hook_event_name=PreToolUse` → 期望仍走 fallback 入库（不丢弃）

---

### § 3.2 Bug 2 — Pre/Post 阶段化 + 配置驱动 tool_response 提取

#### § 3.2.1 引入 gjson

`go.mod` 增加：
```
require github.com/tidwall/gjson v1.18.0
```

仅 `internal/adapter/bridge/` 包引用。其他模块保持 stdlib-only 软约束。

#### § 3.2.2 配置文件

**新文件 `internal/adapter/bridge/extract_rules.yaml`**（go:embed 编译进二进制）：

```yaml
version: 1

# Pre 阶段：从 tool_input 提取动作描述
pre:
  Bash:            "{$.command}"
  Edit:            "编辑 {$.file_path}"
  Write:           "写入 {$.file_path}"
  Read:            "读取 {$.file_path}"
  Glob:            "查找 {$.pattern}"
  Grep:            "搜索 {$.pattern}"
  WebFetch:        "抓取 {$.url}"
  WebSearch:       "搜索 {$.query}"
  AskUserQuestion: "{$.questions.0.question}"
  Agent:           "子任务: {$.description // $.prompt}"
  Task:            "子任务: {$.description}"
  TaskCreate:      "{$.subject}"
  TaskUpdate:      "{$.subject} →{$.status}"
  NotebookEdit:    "{$.notebook_path}"
  LSP:             "LSP {$.operation}: {$.filePath}"
  default:         "{tool_name}"

# Post 阶段：从 tool_response 提取结果摘要
post:
  Bash:            "{$.stdout|firstline} (exit={$.exitCode|default:0})"
  Read:            "已读 {$.file.numLines|default:?} 行"
  Edit:            "{$.success|bool:✓写入,✗失败} {$.filePath}"
  Write:           "{$.success|bool:✓写入,✗失败} {$.filePath}"
  AskUserQuestion:
    string:        "{value}"                # CodeBuddy 形态
    object:        "{$.answers.0}"          # CC-INT 形态
  Agent:           "{$.0.text|firstpara}"
  default:
    string:        "{value}"
    object_paths:                           # 按序探测首个非空
      - "$.answers.0"
      - "$.stdout"
      - "$.text"
      - "$.message"
      - "$.result"
      - "$.summary"
    fallback:      "<json:120>"             # JSON.stringify 后截 120 字符
```

#### § 3.2.3 Mini-DSL 规约（自实现 ~80 行）

| 语法 | 语义 |
|---|---|
| `{$.path}` | gjson 路径求值 |
| `{$.a // $.b}` | 第一个非空值 |
| `{$.x\|firstline}` | 取首行 |
| `{$.x\|firstpara}` | 取首段（首个空行前） |
| `{$.x\|default:N}` | 缺值用 N |
| `{$.x\|bool:T,F}` | 布尔三元 |
| `{value}` | 当 input 整体是 string 时引用整体 |
| `<json:N>` | JSON.stringify 后取首 N 字符；超长追加 `…`（U+2026） |
| 字面文本 | 直接拼接（包括 emoji ✓ ✗） |
| `{tool_name}` | 引用工具名（用于 default 规则） |

#### § 3.2.4 Code 变更清单

**新文件**：
- `internal/adapter/bridge/extract_rules.yaml` — 配置（embed）
- `internal/adapter/bridge/rules.go` — yaml 加载 + DSL 解析 + 求值器
- `internal/adapter/bridge/rules_test.go` — DSL 单元测试

**修改文件**：
- `internal/adapter/bridge/extractor.go`：
  - `ExtractContent` 签名改为 `ExtractContent(phase, toolName string, payload json.RawMessage) (raw, content string)`
  - `phase` 取值 `"pre"` 或 `"post"`，决定查 yaml 的哪张表
  - **payload 语义**：`phase="pre"` 时传 `tool_input`；`phase="post"` 时传 `tool_response`（即 raw_payload 中的 `tool_response` 字段，对应 `CCHookInput.ToolResult`）。Post 阶段**不读 tool_input**；如需混合（如 Bash 失败时同时显示命令和错误），由后续 spec 处理，本次不做。
- `cmd/bridge/main.go`：
  - PreToolUse → 调 `ExtractContent("pre", in.ToolName, in.ToolInput)`
  - PostToolUse → 调 `ExtractContent("post", in.ToolName, in.ToolResult)`
  - PermissionRequest / PermissionDenied 仍走 `"pre"`（沿用现有语义，提示用户即将执行的工具）

#### § 3.2.5 Failure-mode

任何 DSL 求值失败（路径不存在 / 类型不匹配 / yaml 缺工具）→ 回退到 default 规则 → 仍失败回退到 toolName 字面量。**永不返回 error，永不阻塞 hook**。

---

### § 3.3 Bug 3 — Notification 类型化丰富（简化版）

**位置**：`internal/adapter/bridge/extractor.go::ExtractEventContent`，单事件内完成，无跨事件依赖。

```go
case "Notification":
    msg := in.Message
    if msg == "" {
        msg = in.NotificationType
    }
    label := notificationTypeLabel(in.NotificationType)
    raw := label + msg
    return raw, truncateRunes(raw, contentMaxRunes)

func notificationTypeLabel(ntype string) string {
    switch ntype {
    case "permission_prompt":
        return "[等授权] "
    case "idle_prompt":
        return "[等输入] "
    case "auth_success":
        return "[认证] "  // § 3.1 已丢弃，防御性保留
    default:
        return ""
    }
}
```

**实证样本预览**（基于现 DB 数据）：

| 原 message | notification_type | 修复后 content_raw |
|---|---|---|
| `CodeBuddy is waiting for your input` | idle_prompt | `[等输入] CodeBuddy is waiting for your input` |
| `Claude is waiting for your input` | idle_prompt | `[等输入] Claude is waiting for your input` |
| `Claude needs your permission` | permission_prompt | `[等授权] Claude needs your permission` |
| `needs your permission to use Bash` | permission_prompt | `[等授权] needs your permission to use Bash` |
| `needs your permission to use AskUserQuestion` | permission_prompt | `[等授权] needs your permission to use AskUserQuestion` |
| `auth_success: example-user (Tencent)` | auth_success | (§ 3.1 丢弃，不入库) |

**为什么不做跨事件聚合**（明确放弃路径）：
- 一致性优于完美性：90% 场景 PreToolUse 卡片随后到达，UI 上下文已足够
- 避免 Tracker / Registry / ring buffer 改动，限制爆炸半径
- 顺序敏感性问题（9% 边界 case）自然消失

---

### § 3.4 Bug 4 — 杂项修复

#### § 3.4.1 SessionEnd 用 reason 字段

**`internal/adapter/bridge/types.go`** 增加：
```go
Reason string `json:"reason"` // SessionEnd: other/clear/compact
```

**`internal/adapter/bridge/extractor.go`** 修改：
```go
case "SessionEnd":
    return in.Reason, in.Reason
```

raw_payload 实测 `reason="other"` / `"clear"` / `"compact"`。

#### § 3.4.2 SubagentStart agent_id fallback

```go
case "SubagentStart":
    raw := firstNonEmpty(in.AgentType, in.AgentID)
    return raw, raw
```

#### § 3.4.3 dev tooling 防 bridge stale

**Makefile 改动**：
```makefile
.PHONY: install-bridge
install-bridge: bridge ## Build bridge and verify deployment
	@echo "✓ bridge built at bin/pager-cc-bridge"
	@echo "  mtime: $$(date -r bin/pager-cc-bridge '+%F %T')"
	@echo "  sha:   $$(shasum -a 256 bin/pager-cc-bridge | cut -c1-12)"

dev: bridge
run: install-bridge
build: install-bridge
```

> 注：当前 hook settings.json 中 command 指向 `/data/projects/github.com/sapaude/pager/bin/pager-cc-bridge`，与 `make bridge` 输出路径一致，无需 `cp` 拷贝步骤。

**CLAUDE.md** 新增章节：
```markdown
## Bridge Binary Lifecycle

修改 `internal/adapter/bridge/*` 或 `cmd/bridge/*` 后必须 `make install-bridge`，
否则 hook 仍调用旧二进制，新加的字段映射不会生效（这是 2026-06-03 Bug 4a 的根因）。

`make dev` / `make run` / `make build` 已自动前置依赖 bridge target。
```

#### § 3.4.4 历史数据 backfill（可选 / 不阻塞）

写 `cmd/backfill-content/main.go`：扫描 `t_events`，用新 extractor 从 `raw_payload` 反推填充 `content` / `content_raw`。**独立 PR**，不阻塞主修复。

---

## 4. 测试计划

| 测试项 | 方法 | 期望 |
|---|---|---|
| Bug 1 Notification 丢弃 | bridge unit test：raw_payload 无 session_id + Notification → assert PostEvent 未调用 | drop |
| Bug 1 非 Notification 不丢弃 | raw_payload 无 session_id + PreToolUse → assert 走 fallback | host:cwd:tty session_key |
| Bug 2 通用 DSL | rules_test.go 覆盖 Bash/Edit/Read/Write/AskUserQuestion 各 Pre+Post，CC-INT 与 CodeBuddy 双 schema | content_raw 符合 yaml 模板 |
| Bug 2 default 兜底 | 未在 yaml 注册的工具 + 各形态 tool_response | 命中 default.object_paths 或 fallback |
| Bug 3 类型 label | extractor_test.go：3 种 notification_type 各 1 case | content_raw 含 `[等授权]` / `[等输入]` / `[认证]` 前缀 |
| Bug 4b SessionEnd | raw_payload `reason="clear"` | content="clear" |
| Bug 4c SubagentStart | CodeBuddy 形态（无 agent_type 仅 agent_id） | content=agent_id |
| 端到端 | 手动跑 CC + CodeBuddy 真实任务，DB 抽查 | content 非空率 ≥ 95% |
| 防回归 | install-bridge 后比对 mtime | mtime > 最新 commit time |

---

## 5. 工作量估算

| 模块 | 估时 |
|---|---|
| § 3.1 Bug 1 (10 行 + test) | 0.5h |
| § 3.2 Bug 2 (DSL + rules.yaml + 引入 gjson + 重写 ExtractContent + 单测) | 4h |
| § 3.3 Bug 3 (notificationTypeLabel + 3 case 单测) | 0.3h |
| § 3.4.1-3.4.2 字段映射 | 0.5h |
| § 3.4.3 Makefile + CLAUDE.md | 0.5h |
| 集成自测 + DB 验证 | 1h |
| **合计** | **~6.8h** |

---

## 6. 不在本 spec 范围（明确边界）

- ❌ Token Usage 卡片（用户决定单独 spec）
- ❌ Notification 跨事件聚合（用户明确放弃）
- ❌ Pager 主进程 Tracker / Registry 任何改动
- ❌ 历史数据 backfill（独立 PR，§ 3.4.4）
- ❌ 修复"CodeBuddy 某些 session 缺 UserPromptSubmit"（待具体 case 定位，独立 follow-up）
- ❌ Hook settings.json 自动同步部署（保持手动 install-bridge）

---

## 7. 关键文件改动清单（实施 checklist）

### 新增
- [ ] `internal/adapter/bridge/extract_rules.yaml`
- [ ] `internal/adapter/bridge/rules.go`
- [ ] `internal/adapter/bridge/rules_test.go`

### 修改
- [ ] `internal/adapter/bridge/extractor.go` — 重写 ExtractContent + 加 Notification label + SessionEnd / SubagentStart 修复
- [ ] `internal/adapter/bridge/extractor_test.go` — 增 Pre/Post 双路径 + 双 agent schema 测试
- [ ] `internal/adapter/bridge/types.go` — 新增 `Reason` 字段
- [ ] `cmd/bridge/main.go` — 加 shouldDrop + 改 ExtractContent 调用签名
- [ ] `cmd/bridge/main_test.go` — 加 shouldDrop 测试
- [ ] `go.mod` / `go.sum` — 引入 gjson
- [ ] `Makefile` — install-bridge target + dev/run/build 前置依赖
- [ ] `CLAUDE.md` — 新增 "Bridge Binary Lifecycle" 章节

### 不动
- `internal/domain/**` / `internal/wails/**` / `internal/infra/**` / `frontend/**`

---

## 8. 风险与回滚

| 风险 | 缓解 |
|---|---|
| gjson 引入让 bridge 二进制增大 | gjson 仅 ~50KB，bridge 当前 ~8.6MB，影响 < 1% |
| DSL 解析 bug 导致部分工具 content 空 | failure-mode 永远回退 default → toolName，永不阻塞；single rollback = 删 yaml 中该工具行 |
| 配置文件被 embed 后无法热更 | 设计如此（v1.0 范围），需要重 build；后续可考虑磁盘加载 override |
| 部署忘 install-bridge | dev/run/build 已前置依赖；CLAUDE.md 显式提示 |

回滚策略：单文件 git revert 即可，不涉及 schema migration。
