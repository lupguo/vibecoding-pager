# Pager 2026-06-01 UI 重设计 — 抛光续作 设计文档

**Date:** 2026-06-02
**Scope:** 5 个独立的小议题（前端 + 后端混合），紧跟 2026-06-01 UI/state 重设计之后
**Issue source:** 用户跑了一段 dogfood 后反馈的问题集
**Related specs:** `2026-06-01-pager-ui-and-state-redesign-design.md` · `2026-06-01-hook-enrichment-design.md` · `2026-06-02-session-card-compact-layout-design.md`

---

## 0. 议题清单与本 spec 覆盖范围

| # | 议题 | 本 spec 处理？ |
|---|---|---|
| 1 | `event_type=Notification` 当前映射成 WAITING，是否应该是 DONE | ❌ 推迟（详见 §1）|
| 2 | header 里把 `event_type` 加到 `agent_label` 后面 | ✅ §2 |
| 3 | 4 状态配色调整：Working 绿 / Waiting+Done 橙 / Error 红（独占）| ✅ §3 |
| 4 | `t_events` 表冗余 `cwd` + `project_name` | ✅ §4 |
| 5 | 多个事件 `content` 为空或过精简（PostToolBatch / MessageDisplay / TaskCompleted / SubagentStop / Error）| ✅ §5 |

不在本 spec 范围内：会话状态机重构、新事件类型支持、SQLite Schema 大改、跨平台支持。

---

## 1. §1（推迟）Notification 状态语义

**议题：** 当前 `DeriveStatus` 把 `Notification` 事件归到 `StatusWaiting`（同 PermissionRequest / Elicitation 一组）；用户怀疑应该归到 `StatusDone`。

**结论：先观察一段时间，本次不动。**

**理由：**

- `Notification` 事件覆盖范围比单纯"提示用户"更广 — CC 内部用它表达"系统侧条件型暂停"，例如 idle 超时提醒、上下文压缩通知等
- 真实样本不足以判断这些子类全部都属于 Done 还是部分仍属于 Waiting；贸然改 mapping 风险大
- 等用户多用一段时间，攒出 5～10 条真实 `Notification` payload 后再决定
- 短期影响可接受（错把"该提示"当成"等用户响应"更安全 — 不会漏，只会假阳性）

**追踪方式：** 后续遇到具体的 Notification payload 时，新建议题再讨论。

---

## 2. §2 header 添加 event_type 标签（前端，单文件）

### 2.1 现状

`SessionCard` header 当前结构（左→右）：

```
[StatusIcon] [STATUS pill] [agent pill]  ......  [sessionPrefix] [time]
```

`event_type`（PreToolUse / Stop / PermissionRequest / ...）当前**完全不展示**。用户需要点击展开卡片才能在 `<pre>` 里看到部分线索（且不是显式的事件类型，是 content 内容）。

### 2.2 问题

- 同一 session 在不同时刻可能处于相同状态（比如都是 WAITING）但事件来源不同（PermissionRequest vs Elicitation vs Notification）— 用户分不清
- 状态 pill 是「派生信息」，event_type 是「原始信息」— 现状只展示派生层，原始信息要点开才能查看
- 对 dogfood 调试也不友好（看一眼卡片就知道 hook 推了什么事件）

### 2.3 方案：在 agent pill 后追加 outline 样式 event_type pill

**视觉位置：** `[StatusIcon] [STATUS pill] [agent pill] [event_type pill] ...`

**样式：** outline-only（无填充背景），和已有两个 pill 形成对比但不抢主视觉。从 brainstorm 可视化对照（A=同款 badge / B=outline / C=纯 mono 文本）中选定方案 **B**。

**实现：**

- 文件：`frontend/src/components/SessionCard.tsx`
- 在现有 `agentLabel` pill 后面加一个新 span，class 类似：
  ```
  text-[10px] font-medium px-1.5 py-px rounded
  border border-[--pager-border-faint]
  text-[--pager-text-secondary]
  ```
  （颜色 token 引用现有 design system，避免硬编码）
- 取值：`event.event_type`（如 `PreToolUse` / `Stop` / `PermissionRequest`），原样展示，不做大小写转换
- 处理空值：如果 `event_type` 为空字符串，整个 span 不渲染（避免空 pill 占位）
- 不显示 PostToolUse 之类太频繁的事件？— **不过滤**。简单一致，后续如果太吵再 follow up

### 2.4 影响范围

- 单文件改动（`SessionCard.tsx`）
- 无 schema 改动（`event_type` 已经在 `AgentEvent` 上）
- 无后端改动

### 2.5 测试

- 视觉手测：触发 PreToolUse / Stop / PermissionRequest 各 1 次，确认卡片头部出现对应 event_type outline pill
- 空字符串边界：构造 `event_type=""` 的事件（人工 SQLite 插入），确认 pill 不渲染
- 列入 `docs/regression/2026-06-01-ui-state-regression.md` Section A 的"Visual"小节

---

## 3. §3 配色调整：Waiting+Done 橙系，Error 独占红色

### 3.1 现状（v1.5 已上线的 4 状态）

| 状态 | `--c-*` token (light) | `--c-*` token (dark) | 含义 |
|---|---|---|---|
| WORKING | `#28a745` | `#30d158` | 绿（运行中）|
| WAITING | `#ff3b30` | `#ff453a` | 红（等用户响应）|
| DONE | `#5ac8fa` | `#5ac8fa` | 浅蓝（完成）|
| ERROR | `#ff9f0a` | `#ff9f0a` | 橙（错误）|

### 3.2 问题

用户认为：
- Done 用浅蓝（"传统的成功色"）传达不出"也需要看一眼"的紧迫感
- Error 用橙色比红色弱，但 Error 才是真出问题（StopFailure / auth / billing）— 应该是最强的视觉权重
- 整体颜色语义需要重排：**红 = 真出错（独占）、橙系 = 需要看一眼（Waiting OR Done）、绿 = 正常运行**

### 3.3 方案

从 brainstorm 可视化对照（D1 / D2 / D3）中选定方案 **D2**：Waiting 偏鲜橙、Done 偏黄褐，二者同色系但深浅可分。

**最终配色：**

| 状态 | 新 `--c-*` 值 | 新 `--pill-*` 值 (rgba 透明) | 视觉直觉 |
|---|---|---|---|
| WORKING | `#30d158`（保持）| `rgba(48,209,88,0.15)`（保持）| 绿，正常运行 |
| WAITING | `#ff9500` ← 改为暖橙 | `rgba(255,149,0,0.18)` | 橙（鲜亮，"催促感"）|
| DONE | `#cc9a00` ← 改为黄褐 | `rgba(255,204,0,0.18)` | 黄褐（沉稳，"完成的安静"）|
| ERROR | `#ff453a` ← 改为红 | `rgba(255,69,58,0.18)` | 红（严重，仅此一处）|

### 3.4 实现

**单文件改动：** `frontend/src/index.css`

需要更新 **3 个 CSS 块**（第 36-55 行 light、第 91-110 行 dark、第 144-163 行 `@media (prefers-color-scheme: dark)`），每块 8 个 token（4×`--c-*` + 4×`--pill-*`）。

**dark/light 同值（默认）：** spec 默认 light 与 dark 都用上面表里那一套 hex（即 dark 也是 `#cc9a00`）。如果实施完手测发现 dark 模式 `#cc9a00` 在深色背景下对比度不够、看起来发灰，再单独把 dark 那一块的 `--c-done` 调亮一档（参考值 `#d4a000`）。但 plan 里默认按统一 hex 写，**不预先分叉**。

### 3.5 影响范围

- 仅 `frontend/src/index.css`
- 没有动 `SessionCard.tsx` 的 token 引用（仍然是 `text-[--c-waiting]` 之类）
- `SessionList.tsx:60` 一处硬编码 `rgba(255,69,58,0.15)` + `text-[--c-waiting]` 是 hover 扫帚（项目清理）按钮的样式，**语义是"危险/破坏性操作"，应该跟新色板里的 ERROR 红绑定**。改为：bg 用 `var(--pill-error)`，text 用 `var(--c-error)`。这样配色是红色（与 destructive 操作语义一致），同时不再硬编码
- StatusIcon 颜色自动跟随（已经用的 `text-[--c-${status}]`）

### 3.6 测试

- 列入回归 `docs/regression/2026-06-01-ui-state-regression.md` Section A：把表格里"Expected card"列的颜色描述更新（DONE 从 light blue 改为 yellow-brown，ERROR 从 orange 改为 red）
- 视觉手测：4 个状态的卡片各看 1 次，确认颜色符合 D2

---

## 4. §4 t_events 表冗余 cwd + project_name

### 4.1 现状

`t_events` schema（`internal/infra/store/schema.sql:26-44`）：

```sql
CREATE TABLE IF NOT EXISTS t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key     TEXT    NOT NULL,
    agent_label     TEXT    NOT NULL DEFAULT '',
    event_type      TEXT    NOT NULL,
    tool_name       TEXT    NOT NULL DEFAULT '',
    ...
);
```

**问题场景：** 调试时打开 SQLite 看 `t_events`，要知道某条事件属于哪个项目，必须 JOIN `t_sessions` 查 `cwd` / `project_name`。日常 ad-hoc 排查很笨。

### 4.2 方案：ADD COLUMN 冗余两列（不强求列序）

**为什么不重建表插到 `id` 后面？**
SQLite ALTER TABLE 不支持插入列到指定位置；要求列序就得"建新表 + INSERT SELECT + DROP + RENAME"，复杂度大、风险高（v1.5 已经在用户机上跑），收益小（列序不影响查询）。**所以列加在末尾**。

**Schema 改动（`schema.sql`）：**

```sql
CREATE TABLE IF NOT EXISTS t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key     TEXT    NOT NULL,
    agent_label     TEXT    NOT NULL DEFAULT '',
    event_type      TEXT    NOT NULL,
    tool_name       TEXT    NOT NULL DEFAULT '',
    tool_use_id     TEXT    NOT NULL DEFAULT '',
    content         TEXT    NOT NULL DEFAULT '',
    content_raw     TEXT    NOT NULL DEFAULT '',
    permission_mode TEXT    NOT NULL DEFAULT '',
    raw_payload     BLOB             DEFAULT NULL,
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),
    cwd             TEXT    NOT NULL DEFAULT '',          -- v2.1: 冗余自 t_sessions
    project_name    TEXT    NOT NULL DEFAULT '',          -- v2.1: 冗余自 t_sessions

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);

CREATE INDEX IF NOT EXISTS idx_events_project ON t_events(project_name, timestamp DESC);
```

**migrate() 改动（`sqlite.go:400-411`）：**

```go
// v2.1: redundant cwd / project_name on t_events for ad-hoc debugging
_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN cwd          TEXT NOT NULL DEFAULT ''`)
_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN project_name TEXT NOT NULL DEFAULT ''`)
_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_project ON t_events(project_name, timestamp DESC)`)
```

**INSERT 路径改动（`sqlite.go:373-379`）：**

```go
_, err = tx.Exec(`
    INSERT INTO t_events (session_key, agent_label, event_type, tool_name, tool_use_id,
                          content, content_raw, permission_mode, raw_payload, timestamp,
                          cwd, project_name)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
    sessionKey, e.AgentLabel, e.EventType, e.ToolName, e.ToolUseID,
    e.Content, e.ContentRaw, e.PermissionMode,
    []byte(e.RawPayload), ts,
    e.CWD, projectNameOf(e.CWD),  // projectNameOf 已存在于 sqlite.go
)
```

**回填存量数据（migrate 内一次性）：**

```go
// v2.1: backfill cwd / project_name for existing events
_, _ = db.Exec(`
    UPDATE t_events
    SET cwd = (SELECT s.cwd FROM t_sessions s WHERE s.session_key = t_events.session_key),
        project_name = (SELECT s.project_name FROM t_sessions s WHERE s.session_key = t_events.session_key)
    WHERE cwd = '' OR project_name = ''
`)
```

回填使用 `WHERE cwd = '' OR project_name = ''` 保证幂等；多次执行不会重复 UPDATE 已经填好的行。

### 4.3 影响范围

| 文件 | 改动 |
|---|---|
| `internal/infra/store/schema.sql` | 加 2 列 + 1 索引 |
| `internal/infra/store/sqlite.go` | `migrate()` 加 ALTER + UPDATE backfill；INSERT 加 2 列绑定 |
| `internal/infra/store/sqlite_test.go` | 新增 1 个 test 验证插入路径写入 cwd / project_name；1 个 test 验证 migrate 回填 |

无前端改动（前端不直接读这两列）。

### 4.4 测试

- **新增 unit test:** `TestSQLite_InsertEvent_PopulatesCwdAndProjectName` — 写入一条事件，查询同条数据回来，断言 `cwd != ""` 且 `project_name == filepath.Base(cwd)`
- **新增 unit test:** `TestSQLite_Migrate_BackfillsCwdAndProjectName` — 用 raw SQL 在没有这俩列的旧 schema 下插入数据，再调 `migrate()`，断言数据被填上
- **现有测试** 不应破坏；已有 `sqlite_test.go` 跑通即视为兼容

---

## 5. §5 多个事件 content 为空或过精简

### 5.1 现状（基于真实 SQLite raw_payload 样本）

| event_type | 现状 content | 真实 raw_payload 关键字段 | 修复后 content |
|---|---|---|---|
| `Error` | `""`（无 case 匹配，落到 default）| `error_type`, `error_message` | `<type>: <message>` |
| `MessageDisplay` | `in.MessageText`（**字段名错** — CC 实际发的是 `delta`）| `delta`, `final` | `delta` 文本 |
| `PostToolBatch` | `""`（硬编码 empty）| `tool_calls` 数组 | 拼出 `Bash, Edit, Read` 之类工具名列表 |
| `TaskCompleted` | `in.TaskTitle`（**字段名错** — CC 实际发的是 `task_subject`）| `task_subject` | `task_subject` 文本 |
| `SubagentStop` | `in.AgentType`（"claude" 这种太短）| `last_assistant_message` | 子 agent 最后一条消息（截断到 60 rune）|

### 5.2 根因

两类 bug 共存：

**类 A：CCHookInput struct 的 json tag 错对齐**

文件：`internal/adapter/bridge/types.go`

```go
TaskTitle    string `json:"task_title"`    // ← CC 实际发 "task_subject"
MessageText  string `json:"message_text"`  // ← CC 实际发 "delta"
```

**类 B：extractor.go switch case 实现不到位**

文件：`internal/adapter/bridge/extractor.go`

- `case "Error"` 整个不存在 → `default: return "", ""`
- `case "PostToolBatch": return "", ""` — 硬编码空
- `case "SubagentStop": return in.AgentType, in.AgentType` — 字段错（应优先用 `last_assistant_message`）
- `case "TaskCompleted": return in.TaskTitle, ...` — 类 A 的 bug 让它读到空串
- `case "MessageDisplay": return in.MessageText, ...` — 类 A 的 bug 让它读到空串

### 5.3 修复方案

#### 5.3.1 修 CCHookInput 字段（types.go）

```go
TaskSubject string `json:"task_subject"`  // ← 重命名（原 TaskTitle）
Delta       string `json:"delta"`          // ← 重命名（原 MessageText）
// 同时保留 final bool? — 不需要；当前代码不消费 final
```

由于这两个字段当前已经在用（错的），**重命名而非新增**，避免遗留死字段。

#### 5.3.2 修 extractor switch（extractor.go）

```go
case "SubagentStop":
    // 优先用子 agent 最后一句话；fallback 到 agent type
    if in.LastAssistantMessage != "" {
        return in.LastAssistantMessage, truncateRunes(in.LastAssistantMessage, contentMaxRunes)
    }
    return in.AgentType, in.AgentType

case "TaskCreated":
    return in.TaskSubject, truncateRunes(in.TaskSubject, contentMaxRunes)

case "TaskCompleted":
    return in.TaskSubject, truncateRunes(in.TaskSubject, contentMaxRunes)

case "MessageDisplay":
    return in.Delta, truncateRunes(in.Delta, contentMaxRunes)

case "PostToolBatch":
    // tool_calls 是 []{tool_name: ...}; 解析出 tool_name 列表，逗号拼接
    names := extractBatchToolNames(in.ToolCalls)  // 新增辅助函数
    if len(names) == 0 {
        return "", ""
    }
    raw := strings.Join(names, ", ")
    return raw, truncateRunes(raw, contentMaxRunes)

case "Error":
    // 新增 — Hook Error 事件
    raw := in.ErrorType + ": " + in.ErrorMessage
    if raw == ": " {  // 两个字段都空时不显示无意义的 ": "
        return "", ""
    }
    return raw, truncateRunes(raw, contentMaxRunes)
```

#### 5.3.3 新增辅助函数 extractBatchToolNames（extractor.go）

```go
// extractBatchToolNames 从 PostToolBatch 的 tool_calls JSON 数组里
// 解析出每个 element 的 tool_name 字段，按顺序返回。
// 容错：解析失败返回空切片。
func extractBatchToolNames(raw json.RawMessage) []string {
    if len(raw) == 0 {
        return nil
    }
    var calls []struct {
        ToolName string `json:"tool_name"`
    }
    if err := json.Unmarshal(raw, &calls); err != nil {
        return nil
    }
    names := make([]string, 0, len(calls))
    for _, c := range calls {
        if c.ToolName != "" {
            names = append(names, c.ToolName)
        }
    }
    return names
}
```

### 5.4 影响范围

| 文件 | 改动 |
|---|---|
| `internal/adapter/bridge/types.go` | `TaskTitle` → `TaskSubject`；`MessageText` → `Delta` |
| `internal/adapter/bridge/extractor.go` | 5 个 case 调整（含新增 Error）+ 1 个辅助函数 |
| `internal/adapter/bridge/extractor_test.go` | 现有测试用例同步重命名；新增 5 个测试 |
| `internal/adapter/bridge/types_test.go` | 同步字段名 |

### 5.5 测试

#### 现有测试更新

- `extractor_test.go` 中所有 `MessageText` 字面量改 `Delta`、`TaskTitle` 改 `TaskSubject`
- `types_test.go` 中 JSON fixture 字面量同步：`"task_title"` → `"task_subject"`，`"message_text"` → `"delta"`

#### 新增 5 个 sub-test（在 `extractor_test.go`）

```go
func TestExtractEventContent_Error_UsesErrorTypeAndMessage(t *testing.T) {
    in := bridge.CCHookInput{
        ErrorType:    "rate_limit",
        ErrorMessage: "60 req/min exceeded",
    }
    raw, content := bridge.ExtractEventContent("Error", in)
    if raw != "rate_limit: 60 req/min exceeded" {
        t.Errorf("raw = %q", raw)
    }
    if content != "rate_limit: 60 req/min exceeded" {
        t.Errorf("content = %q", content)
    }
}

func TestExtractEventContent_Error_BothEmpty_ReturnsEmpty(t *testing.T) {
    in := bridge.CCHookInput{}
    raw, content := bridge.ExtractEventContent("Error", in)
    if raw != "" || content != "" {
        t.Errorf("expected empty, got raw=%q content=%q", raw, content)
    }
}

func TestExtractEventContent_PostToolBatch_JoinsToolNames(t *testing.T) {
    in := bridge.CCHookInput{
        ToolCalls: json.RawMessage(`[{"tool_name":"Bash"},{"tool_name":"Edit"},{"tool_name":"Read"}]`),
    }
    raw, content := bridge.ExtractEventContent("PostToolBatch", in)
    if raw != "Bash, Edit, Read" {
        t.Errorf("raw = %q", raw)
    }
    _ = content
}

func TestExtractEventContent_MessageDisplay_UsesDelta(t *testing.T) {
    in := bridge.CCHookInput{Delta: "这一节没问题吗？"}
    raw, _ := bridge.ExtractEventContent("MessageDisplay", in)
    if raw != "这一节没问题吗？" {
        t.Errorf("raw = %q", raw)
    }
}

func TestExtractEventContent_SubagentStop_PrefersLastAssistantMessage(t *testing.T) {
    in := bridge.CCHookInput{
        AgentType:            "claude",
        LastAssistantMessage: "All changes have been applied.",
    }
    raw, _ := bridge.ExtractEventContent("SubagentStop", in)
    if raw != "All changes have been applied." {
        t.Errorf("raw = %q", raw)
    }
}

func TestExtractEventContent_SubagentStop_FallsBackToAgentType(t *testing.T) {
    in := bridge.CCHookInput{AgentType: "claude"}
    raw, _ := bridge.ExtractEventContent("SubagentStop", in)
    if raw != "claude" {
        t.Errorf("raw = %q", raw)
    }
}

func TestExtractEventContent_TaskCompleted_UsesTaskSubject(t *testing.T) {
    in := bridge.CCHookInput{TaskSubject: "Task 3: Migrate schema"}
    raw, _ := bridge.ExtractEventContent("TaskCompleted", in)
    if raw != "Task 3: Migrate schema" {
        t.Errorf("raw = %q", raw)
    }
}
```

---

## 6. 端到端验证

实施完成后，必须通过：

1. **`make test`** 全绿（含新增 5 个 extractor sub-test 和 2 个 SQLite sub-test）
2. **`make build`** 成功生成 `.app`
3. **手动回归** `docs/regression/2026-06-01-ui-state-regression.md` Section A（Status Visual）：
   - 新颜色映射符合 §3 表格（DONE 黄褐、ERROR 红、WAITING 暖橙、WORKING 绿不变）
   - 每张卡片头部出现 outline event_type pill（§2）
4. **SQLite 现场检查：** 触发 5 类事件后跑 `sqlite3 ~/.config/pager/pager.db "SELECT event_type, content, cwd, project_name FROM t_events ORDER BY id DESC LIMIT 10"`：
   - 5 类事件 content 都不为空（除 Error 双字段都空的边界）
   - cwd / project_name 自动填充
5. **存量数据兼容：** 用旧版本（v1.5）建过的 db 启动新版本，确认 migrate 后 t_events 既有列保留 + 新两列回填，所有功能正常

---

## 7. 不在本 spec 范围

- 议题 #1（Notification → Done 映射）：推迟（§1）
- 颜色 dark mode 微调：实施期视觉确认决定，写在 plan 备注里
- ARCHITECTURE.md 更新：跟随 schema 变化的部分由实施者自行更新
- pager-cc-bridge 的额外 stdin 字段支持：当前 5 类事件需要的字段都已经在 CCHookInput 里，无需扩展

---

## 8. 文件总清单（实施 reference）

| 文件 | 改动类型 | 议题 |
|---|---|---|
| `frontend/src/components/SessionCard.tsx` | 加 event_type pill | §2 |
| `frontend/src/index.css` | 12 个 token 改值（4 状态 × 3 块）| §3 |
| `frontend/src/components/SessionList.tsx` | 替换 1 处硬编码颜色 | §3 |
| `internal/infra/store/schema.sql` | t_events 加 2 列 + 1 索引 | §4 |
| `internal/infra/store/sqlite.go` | migrate() 加 ALTER+backfill；INSERT 加 2 列 | §4 |
| `internal/infra/store/sqlite_test.go` | +2 个测试 | §4 |
| `internal/adapter/bridge/types.go` | 2 个字段重命名 | §5 |
| `internal/adapter/bridge/extractor.go` | 5 个 case 调整 + 1 辅助函数 | §5 |
| `internal/adapter/bridge/extractor_test.go` | 重命名 + 新增 5 个测试 | §5 |
| `internal/adapter/bridge/types_test.go` | JSON fixture 字段名同步 | §5 |
| `docs/regression/2026-06-01-ui-state-regression.md` | Section A 颜色描述更新 | §3 §2 |

总计 11 个文件。所有改动都是聚焦修复，不引入新架构。
