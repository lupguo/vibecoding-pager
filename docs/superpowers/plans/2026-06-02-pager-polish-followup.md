# Pager 2026-06-01 UI 重设计抛光续作 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 spec `2026-06-02-pager-polish-followup-design.md` 的 5 节落地（§1 推迟）— 配色重排 + header 加 event_type pill + t_events 冗余两列 + 5 个事件 content 修复。

**Architecture:** 11 个文件、零新架构。前端是 CSS token 重值 + SessionCard pill 增加；后端是 SQLite migrate `ALTER TABLE` 加列 + bridge 包字段重命名 + extractor 5 个 case 调整。所有改动可以分 11 个独立 task 串行实施。

**Tech Stack:** Go 1.25 / Wails v3 / SQLite (modernc.org/sqlite) / React 18 / TypeScript / Tailwind CSS

---

## File Structure

文件总览（实施期间一一对应）：

| File | Responsibility | 改动幅度 |
|---|---|---|
| `frontend/src/index.css` | 4 个状态 token 的 hex/rgba 值 | 12 行替换（4 token × 3 块）|
| `frontend/src/components/SessionList.tsx` | broom hover 颜色 | 1 行替换 |
| `frontend/src/components/SessionCard.tsx` | header 加 event_type outline pill | +5 行 |
| `internal/adapter/bridge/types.go` | CCHookInput 字段重命名 2 处 | 2 行 |
| `internal/adapter/bridge/types_test.go` | JSON fixture key 同步 + 引用变量名 | 4 行 |
| `internal/adapter/bridge/extractor.go` | 5 个 case + 1 辅助函数 | ~30 行 |
| `internal/adapter/bridge/extractor_test.go` | 现有 fixture 同步 + 新增 7 个测试 | ~80 行 |
| `internal/infra/store/schema.sql` | t_events 加 2 列 + 1 索引 | 3 行 |
| `internal/infra/store/sqlite.go` | migrate 加 ALTER+backfill；INSERT 加 2 列 | ~10 行 |
| `internal/infra/store/sqlite_test.go` | 新增 2 个测试 | ~50 行 |
| `docs/regression/2026-06-01-ui-state-regression.md` | 颜色描述更新 + event_type pill 验收行 | ~5 行 |

**实施顺序原则：** §3 配色（无依赖）→ §2 pill（无依赖）→ §5 后端字段+extractor（同包内变更尽量原子）→ §4 schema → 文档 + 验证。

---

## Task 1: §3 — 更新 4 状态颜色 token（CSS）

**Files:**
- Modify: `frontend/src/index.css:36-55`（light 块）, `91-110`（dark 块）, `144-163`（prefers-color-scheme dark 块）

**目标值（D2 方案）：**

| token | 当前 | 新值 |
|---|---|---|
| `--c-working` | `#28a745` (light) / `#30d158` (dark) | **保持不变** |
| `--c-waiting` | `#ff3b30` / `#ff453a` | `#ff9500` 暖橙（light + dark 同值）|
| `--c-done` | `#5ac8fa` | `#cc9a00` 黄褐（light + dark 同值）|
| `--c-error` | `#ff9f0a` | `#ff453a` 红（light + dark 同值）|
| `--pill-working` | `rgba(40,167,69,0.12)` / `rgba(48,209,88,0.15)` | **保持不变** |
| `--pill-waiting` | `rgba(255,59,48,0.15)` / `rgba(255,69,58,0.18)` | `rgba(255,149,0,0.18)`（light + dark 同值）|
| `--pill-done` | `rgba(90,200,250,0.14)` | `rgba(255,204,0,0.18)`（light + dark 同值）|
| `--pill-error` | `rgba(255,159,10,0.16)` / `rgba(255,159,10,0.18)` | `rgba(255,69,58,0.18)`（light + dark 同值）|

- [ ] **Step 1: 更新 light 块（第 36-55 行）**

把这 4 行改成新值：

```css
  --c-working: #28a745;            /* 不变 */
  --c-waiting: #ff9500;            /* 改 */
  --c-done:    #cc9a00;            /* 改 */
  --c-error:   #ff453a;            /* 改 */
```

把这 4 行改成新值：

```css
  --pill-working: rgba(40, 167, 69, 0.12);      /* 不变 */
  --pill-waiting: rgba(255, 149, 0, 0.18);      /* 改 */
  --pill-done:    rgba(255, 204, 0, 0.18);      /* 改 */
  --pill-error:   rgba(255, 69, 58, 0.18);      /* 改 */
```

- [ ] **Step 2: 更新 dark 块（第 91-110 行）**

```css
  --c-working: #30d158;     /* 不变 */
  --c-waiting: #ff9500;     /* 改 */
  --c-done:    #cc9a00;     /* 改 */
  --c-error:   #ff453a;     /* 改 */
```

```css
  --pill-working: rgba(48, 209, 88, 0.15);     /* 不变 */
  --pill-waiting: rgba(255, 149, 0, 0.18);     /* 改 */
  --pill-done:    rgba(255, 204, 0, 0.18);     /* 改 */
  --pill-error:   rgba(255, 69, 58, 0.18);     /* 改 */
```

- [ ] **Step 3: 更新 prefers-color-scheme: dark 块（第 144-163 行）— 与 dark 块同值**

```css
    --c-working: #30d158;     /* 不变 */
    --c-waiting: #ff9500;     /* 改 */
    --c-done:    #cc9a00;     /* 改 */
    --c-error:   #ff453a;     /* 改 */
```

```css
    --pill-working: rgba(48, 209, 88, 0.15);     /* 不变 */
    --pill-waiting: rgba(255, 149, 0, 0.18);     /* 改 */
    --pill-done:    rgba(255, 204, 0, 0.18);     /* 改 */
    --pill-error:   rgba(255, 69, 58, 0.18);     /* 改 */
```

- [ ] **Step 4: 验证 grep 结果**

Run: `grep -n '\-\-c-waiting\|\-\-c-done\|\-\-c-error' frontend/src/index.css`
Expected: 9 行（3 块 × 3 token，working 已经保持不变没在 grep 范围内），每行右侧的 hex 值匹配上面的目标。

```bash
grep -nE '\-\-c-(waiting|done|error):' frontend/src/index.css
```

期望：
```
37:  --c-waiting: #ff9500;
38:  --c-done:    #cc9a00;
39:  --c-error:   #ff453a;
92:  --c-waiting: #ff9500;
93:  --c-done:    #cc9a00;
94:  --c-error:   #ff453a;
145:    --c-waiting: #ff9500;
146:    --c-done:    #cc9a00;
147:    --c-error:   #ff453a;
```

- [ ] **Step 5: 提交**

```bash
git add frontend/src/index.css
git commit -m "feat(ui): D2 color palette — Waiting+Done in orange family, Error owns red

- Waiting: #ff9500 warm orange (was red)
- Done: #cc9a00 yellow-brown (was sky blue)
- Error: #ff453a red (was orange) — now the only red, reserved for
  StopFailure/auth/billing
- Working stays green
Light + dark + prefers-color-scheme blocks all updated.
Implements §3 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 2: §3 — 修 SessionList broom hover 颜色

**Files:**
- Modify: `frontend/src/components/SessionList.tsx:60`

当前：

```tsx
className="opacity-0 group-hover:opacity-100 w-[22px] h-[22px] flex items-center justify-center rounded-[4px] text-[--pager-text-faint] hover:bg-[rgba(255,69,58,0.15)] hover:text-[--c-waiting] transition-colors"
```

问题：硬编码 `rgba(255,69,58,0.15)` + 引用 `--c-waiting`。新色板里 `--c-waiting` 已经是橙色，但 broom 是 destructive 操作（清理项目），语义上应该是红色（即新的 `--c-error`）。

- [ ] **Step 1: 改 className 的两处颜色**

把上面那行改成（替换 hover bg 和 hover text）：

```tsx
className="opacity-0 group-hover:opacity-100 w-[22px] h-[22px] flex items-center justify-center rounded-[4px] text-[--pager-text-faint] hover:bg-[--pill-error] hover:text-[--c-error] transition-colors"
```

变更点：
- `hover:bg-[rgba(255,69,58,0.15)]` → `hover:bg-[--pill-error]`
- `hover:text-[--c-waiting]` → `hover:text-[--c-error]`

- [ ] **Step 2: 验证没有遗留硬编码 rgba**

Run: `grep -n 'rgba(255,69,58' frontend/src/components/SessionList.tsx`
Expected: 没有任何匹配（输出为空，exit 1 也算 OK）。

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/SessionList.tsx
git commit -m "fix(ui): broom-icon hover uses --c-error (was hardcoded waiting red)

Project清理 button is a destructive action — should bind to the new
ERROR color token, not WAITING (which is now orange).
Removes the only hardcoded rgba in this file."
```

---

## Task 3: §2 — SessionCard header 添加 event_type outline pill

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx:144-156`

当前结构（第 145-156 行）：

```tsx
<div className="flex items-center gap-[6px] mb-[4px]">
  <StatusIcon status={status} />
  <span className={`text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] tracking-wide ${tag.bgClass} ${tag.textClass}`}>
    {tag.text}
  </span>
  <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide uppercase">
    {agentLabel}
  </span>
  <span className="flex-1" />
  <span className="text-[9px] text-[--pager-text-faint] font-mono">{sessionPrefix}</span>
  <span className="text-[9px] text-[--pager-text-faint]">{relativeTime}</span>
</div>
```

需要在 agentLabel pill 与 `flex-1` spacer 之间插入新 outline pill。

- [ ] **Step 1: 在 SessionCard.tsx 顶部 agentLabel 处添加 eventType 变量**

在第 124 行附近（其他变量定义处），添加：

```tsx
const eventType = session.LastEvent?.event_type ?? ''
```

完整上下文（第 121-124 行 → 第 121-125 行）：

```tsx
  const content = session.LastEvent?.content ?? ''
  const toolName = session.LastEvent?.tool_name ?? ''
  const sessionPrefix = (session.SessionID || session.Key || '').slice(0, 8)
  const agentLabel = session.AgentLabel || 'CC'
  const eventType = session.LastEvent?.event_type ?? ''
```

- [ ] **Step 2: 在 agentLabel pill 后插入 event_type outline pill**

在第 152 行后（`{agentLabel}</span>` 后）和第 153 行 `<span className="flex-1" />` 前，插入：

```tsx
          {eventType && (
            <span className="text-[9px] font-medium px-[5px] py-[1px] rounded-[3px] tracking-wide border border-[--pager-border] text-[--pager-text-secondary]">
              {eventType}
            </span>
          )}
```

完整上下文（第 145-157 行新版本）：

```tsx
        <div className="flex items-center gap-[6px] mb-[4px]">
          <StatusIcon status={status} />
          <span className={`text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] tracking-wide ${tag.bgClass} ${tag.textClass}`}>
            {tag.text}
          </span>
          <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide uppercase">
            {agentLabel}
          </span>
          {eventType && (
            <span className="text-[9px] font-medium px-[5px] py-[1px] rounded-[3px] tracking-wide border border-[--pager-border] text-[--pager-text-secondary]">
              {eventType}
            </span>
          )}
          <span className="flex-1" />
          <span className="text-[9px] text-[--pager-text-faint] font-mono">{sessionPrefix}</span>
          <span className="text-[9px] text-[--pager-text-faint]">{relativeTime}</span>
        </div>
```

- [ ] **Step 3: 检查 TypeScript 编译**

Run: `cd frontend && pnpm exec tsc --noEmit`
Expected: 无类型错误（如果之前 `session.LastEvent.event_type` 字段已经有类型定义；frontend/src/store/sessions.ts:15 确认了 event_type:string 字段存在）。

- [ ] **Step 4: 提交**

```bash
git add frontend/src/components/SessionCard.tsx
git commit -m "feat(ui): show event_type as outline pill in card header

Adds an outline-only badge after the agent pill displaying the raw
event_type (PreToolUse / Stop / PermissionRequest / ...). Hidden when
event_type is empty.
- Outline only (no fill) so it doesn't compete with status/agent pills
- text-[--pager-text-secondary] for subdued contrast
- Empty value → not rendered
Implements §2 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 4: §5.1 — 重命名 CCHookInput 字段（TaskTitle→TaskSubject, MessageText→Delta）

**Files:**
- Modify: `internal/adapter/bridge/types.go:47, 62`
- Modify: `internal/adapter/bridge/types_test.go:32, 39-40`
- Modify: `internal/adapter/bridge/extractor.go:181, 183, 218`（消费方同步）
- Modify: `internal/adapter/bridge/extractor_test.go:279, 281`（消费方同步）

**注意：** 这是一个原子重命名 task — 改 struct 字段就必须同步改所有 consumer，否则 `go build` 失败。本 task 不增加新行为，只重命名。

- [ ] **Step 1: 改 types.go 字段 + json tag**

在 `internal/adapter/bridge/types.go` 第 47 行：

```go
// 从：
TaskTitle       string `json:"task_title"`       // TaskCreated/Completed
// 改为：
TaskSubject     string `json:"task_subject"`     // TaskCreated/Completed
```

第 62 行：

```go
// 从：
MessageText      string          `json:"message_text"`      // MessageDisplay
// 改为：
Delta            string          `json:"delta"`             // MessageDisplay
```

- [ ] **Step 2: 改 extractor.go consumer**

在 `internal/adapter/bridge/extractor.go` 第 181-183 行：

```go
// 从：
case "TaskCreated":
    return in.TaskTitle, truncateRunes(in.TaskTitle, contentMaxRunes)
case "TaskCompleted":
    return in.TaskTitle, truncateRunes(in.TaskTitle, contentMaxRunes)
// 改为：
case "TaskCreated":
    return in.TaskSubject, truncateRunes(in.TaskSubject, contentMaxRunes)
case "TaskCompleted":
    return in.TaskSubject, truncateRunes(in.TaskSubject, contentMaxRunes)
```

第 217-218 行：

```go
// 从：
case "MessageDisplay":
    return in.MessageText, truncateRunes(in.MessageText, contentMaxRunes)
// 改为：
case "MessageDisplay":
    return in.Delta, truncateRunes(in.Delta, contentMaxRunes)
```

- [ ] **Step 3: 改 extractor_test.go**

在 `internal/adapter/bridge/extractor_test.go` 的 `TestExtractEventContent_TaskCreated`（第 278-284 行）：

```go
// 从：
in := &CCHookInput{TaskTitle: "Implement auth flow", TaskDescription: "Add JWT tokens"}
// 改为：
in := &CCHookInput{TaskSubject: "Implement auth flow", TaskDescription: "Add JWT tokens"}
```

- [ ] **Step 4: 改 types_test.go**

在 `internal/adapter/bridge/types_test.go` 的 `TestCCHookInput_TaskCreatedFields`（第 27-45 行）：

```go
// 第 32 行 JSON fixture key：
"task_title": "Fix login bug",
// 改为：
"task_subject": "Fix login bug",
```

```go
// 第 39-40 行字段引用：
if in.TaskTitle != "Fix login bug" {
    t.Errorf("TaskTitle: got %q", in.TaskTitle)
}
// 改为：
if in.TaskSubject != "Fix login bug" {
    t.Errorf("TaskSubject: got %q", in.TaskSubject)
}
```

- [ ] **Step 5: 跑 bridge 包 + types 测试**

Run: `go test ./internal/adapter/bridge/...`
Expected: PASS（所有测试都通过；如果有遗漏的 consumer 没改，go build 会失败并指明文件）

如果失败：根据编译错误定位还有哪个文件引用旧字段名，改掉再跑。

- [ ] **Step 6: 全包验证（cmd/bridge 也消费）**

Run: `grep -rn 'TaskTitle\|MessageText' --include='*.go' .`
Expected: 没有任何匹配（确认 cmd/bridge 没有遗漏）。

- [ ] **Step 7: 提交**

```bash
git add internal/adapter/bridge/types.go internal/adapter/bridge/types_test.go \
        internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "refactor(bridge): align CCHookInput fields with CC's actual JSON keys

CC sends 'task_subject' / 'delta' but our struct had 'task_title' /
'message_text' — the json tags didn't match, so these fields silently
unmarshalled to zero value. Renaming for accuracy:
  TaskTitle   -> TaskSubject  (json: task_title -> task_subject)
  MessageText -> Delta        (json: message_text -> delta)
All consumers in extractor.go + tests updated. No behavior change yet
— content fixes follow in subsequent commits.
Part of §5 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 5: §5.2 — 添加 Error 事件 case

**Files:**
- Modify: `internal/adapter/bridge/extractor.go:166-222`（switch 内新增 case）
- Modify: `internal/adapter/bridge/extractor_test.go`（新增 2 个测试）

- [ ] **Step 1: 写失败测试 — Error event 用 ErrorType + ErrorMessage**

在 `internal/adapter/bridge/extractor_test.go` 末尾追加：

```go
func TestExtractEventContent_Error_UsesErrorTypeAndMessage(t *testing.T) {
	in := &CCHookInput{
		ErrorType:    "rate_limit",
		ErrorMessage: "60 req/min exceeded",
	}
	raw, content := ExtractEventContent("Error", in)
	if raw != "rate_limit: 60 req/min exceeded" {
		t.Errorf("raw = %q, want %q", raw, "rate_limit: 60 req/min exceeded")
	}
	if content != "rate_limit: 60 req/min exceeded" {
		t.Errorf("content = %q", content)
	}
}

func TestExtractEventContent_Error_BothEmpty_ReturnsEmpty(t *testing.T) {
	in := &CCHookInput{}
	raw, content := ExtractEventContent("Error", in)
	if raw != "" || content != "" {
		t.Errorf("expected empty, got raw=%q content=%q", raw, content)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_Error -v`
Expected: FAIL — `Error` 事件落到 default case，返回 `""`，与 `"rate_limit: 60 req/min exceeded"` 不等。

- [ ] **Step 3: 在 extractor.go 添加 Error case**

在 `internal/adapter/bridge/extractor.go` 第 173 行后（StopFailure case 之后），插入：

```go
	case "Error":
		raw := in.ErrorType + ": " + in.ErrorMessage
		if raw == ": " {
			return "", ""
		}
		return raw, truncateRunes(raw, contentMaxRunes)
```

完整上下文（第 171-180 行新版本）：

```go
	case "StopFailure":
		raw := in.ErrorType + ": " + in.ErrorMessage
		return raw, truncateRunes(raw, contentMaxRunes)
	case "Error":
		raw := in.ErrorType + ": " + in.ErrorMessage
		if raw == ": " {
			return "", ""
		}
		return raw, truncateRunes(raw, contentMaxRunes)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_Error -v`
Expected: PASS（两个 sub-test 都过）。

- [ ] **Step 5: 提交**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): Error event uses ErrorType + ErrorMessage

Hook 'Error' events were falling through to default case and returning
empty content — users couldn't tell what error occurred from the card.
Now formatted as '<error_type>: <error_message>', mirroring StopFailure.
When both fields are empty, returns '' (avoids displaying ': ').
Part of §5 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 6: §5.3 — 修 PostToolBatch（解析 tool_calls 数组）

**Files:**
- Modify: `internal/adapter/bridge/extractor.go:194-195`（替换硬编码 `return "", ""`）
- Modify: `internal/adapter/bridge/extractor.go`（末尾新增 `extractBatchToolNames` 辅助函数）
- Modify: `internal/adapter/bridge/extractor_test.go`（新增 2 个测试）

- [ ] **Step 1: 写失败测试 — PostToolBatch 拼接 tool_name 列表**

在 `internal/adapter/bridge/extractor_test.go` 末尾追加：

```go
func TestExtractEventContent_PostToolBatch_JoinsToolNames(t *testing.T) {
	in := &CCHookInput{
		ToolCalls: json.RawMessage(`[{"tool_name":"Bash"},{"tool_name":"Edit"},{"tool_name":"Read"}]`),
	}
	raw, content := ExtractEventContent("PostToolBatch", in)
	if raw != "Bash, Edit, Read" {
		t.Errorf("raw = %q, want %q", raw, "Bash, Edit, Read")
	}
	if content != "Bash, Edit, Read" {
		t.Errorf("content = %q", content)
	}
}

func TestExtractEventContent_PostToolBatch_EmptyArray(t *testing.T) {
	in := &CCHookInput{ToolCalls: json.RawMessage(`[]`)}
	raw, content := ExtractEventContent("PostToolBatch", in)
	if raw != "" || content != "" {
		t.Errorf("expected empty, got raw=%q content=%q", raw, content)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_PostToolBatch -v`
Expected: FAIL — 当前实现 `return "", ""`，得不到 `"Bash, Edit, Read"`。

- [ ] **Step 3: 在 extractor.go 改 PostToolBatch case + 新增辅助函数**

第 194-195 行：

```go
// 从：
case "PostToolBatch":
    return "", ""
// 改为：
case "PostToolBatch":
    names := extractBatchToolNames(in.ToolCalls)
    if len(names) == 0 {
        return "", ""
    }
    raw := strings.Join(names, ", ")
    return raw, truncateRunes(raw, contentMaxRunes)
```

在 extractor.go 文件末尾（最后一个 `}` 之后）追加辅助函数：

```go
// extractBatchToolNames parses PostToolBatch's tool_calls JSON array
// (each element having a tool_name) and returns the names in order.
// On parse failure or nil input, returns nil.
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

- [ ] **Step 4: 检查 import — 确认 `strings` 和 `encoding/json` 都已 import**

Run: `head -20 internal/adapter/bridge/extractor.go`
Expected: import block 包含 `"encoding/json"` 和 `"strings"`。如果 `strings` 没 import（很可能 — 现有代码看起来没用到），把它加到 import 块：

```go
import (
	"encoding/json"
	"strings"
	// 其他已存在的 import 保留
)
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_PostToolBatch -v`
Expected: PASS。

- [ ] **Step 6: 跑整包确认未破坏其他测试**

Run: `go test ./internal/adapter/bridge/...`
Expected: PASS（所有 extractor + types 测试通过）。

- [ ] **Step 7: 提交**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): PostToolBatch content joins tool_calls' tool names

PostToolBatch was hardcoded to return empty content. Now parses the
tool_calls JSON array and joins the tool_name fields, e.g.:
  [{tool_name:Bash},{tool_name:Edit}] -> 'Bash, Edit'
Empty array / parse failure / missing field falls back to empty string
(unchanged behavior for malformed payloads).
Adds unexported helper extractBatchToolNames.
Part of §5 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 7: §5.4 — 修 SubagentStop（优先用 LastAssistantMessage）

**Files:**
- Modify: `internal/adapter/bridge/extractor.go:178-179`
- Modify: `internal/adapter/bridge/extractor_test.go`（新增 2 个测试）

- [ ] **Step 1: 写失败测试 — SubagentStop 优先用 LastAssistantMessage**

在 `internal/adapter/bridge/extractor_test.go` 末尾追加：

```go
func TestExtractEventContent_SubagentStop_PrefersLastAssistantMessage(t *testing.T) {
	in := &CCHookInput{
		AgentType:            "claude",
		LastAssistantMessage: "All changes have been applied.",
	}
	raw, content := ExtractEventContent("SubagentStop", in)
	if raw != "All changes have been applied." {
		t.Errorf("raw = %q", raw)
	}
	if content != "All changes have been applied." {
		t.Errorf("content = %q", content)
	}
}

func TestExtractEventContent_SubagentStop_FallsBackToAgentType(t *testing.T) {
	in := &CCHookInput{AgentType: "claude"}
	raw, _ := ExtractEventContent("SubagentStop", in)
	if raw != "claude" {
		t.Errorf("raw = %q, want %q", raw, "claude")
	}
}
```

- [ ] **Step 2: 跑测试确认第一个失败**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_SubagentStop -v`
Expected: 第一个 FAIL（当前实现总是返回 `AgentType` 即 `"claude"`），第二个 PASS。

- [ ] **Step 3: 改 SubagentStop case**

在 `internal/adapter/bridge/extractor.go` 第 178-179 行：

```go
// 从：
case "SubagentStop":
    return in.AgentType, in.AgentType
// 改为：
case "SubagentStop":
    if in.LastAssistantMessage != "" {
        return in.LastAssistantMessage, truncateRunes(in.LastAssistantMessage, contentMaxRunes)
    }
    return in.AgentType, in.AgentType
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_SubagentStop -v`
Expected: PASS（两个 sub-test 都过）。

- [ ] **Step 5: 提交**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): SubagentStop prefers last_assistant_message over agent_type

When CC ends a subagent, the cardgo summary should show what the
subagent actually said (its closing message), not just the boilerplate
agent_type ('claude'). Falls back to AgentType when LastAssistantMessage
is empty (preserving existing behavior for that edge case).
Part of §5 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 8: §5 — 验证 TaskCompleted + MessageDisplay 字段重命名后的行为

**Files:**
- Modify: `internal/adapter/bridge/extractor_test.go`（新增 2 个测试）

**注意：** Task 4 已经完成了字段重命名 + extractor.go 的 consumer 同步。本 task 只是为这两类事件加显式回归测试，确保行为正确。

- [ ] **Step 1: 写测试 — TaskCompleted 用 TaskSubject**

在 `internal/adapter/bridge/extractor_test.go` 末尾追加：

```go
func TestExtractEventContent_TaskCompleted_UsesTaskSubject(t *testing.T) {
	in := &CCHookInput{TaskSubject: "Task 3: Migrate schema"}
	raw, content := ExtractEventContent("TaskCompleted", in)
	if raw != "Task 3: Migrate schema" {
		t.Errorf("raw = %q", raw)
	}
	if content != "Task 3: Migrate schema" {
		t.Errorf("content = %q", content)
	}
}

func TestExtractEventContent_MessageDisplay_UsesDelta(t *testing.T) {
	in := &CCHookInput{Delta: "这一节没问题吗？"}
	raw, content := ExtractEventContent("MessageDisplay", in)
	if raw != "这一节没问题吗？" {
		t.Errorf("raw = %q", raw)
	}
	if content != "这一节没问题吗？" {
		t.Errorf("content = %q", content)
	}
}
```

- [ ] **Step 2: 跑测试确认通过（应当直接通过，因为 Task 4 已经修了）**

Run: `go test ./internal/adapter/bridge/ -run 'TestExtractEventContent_(TaskCompleted|MessageDisplay)' -v`
Expected: PASS。

如果 FAIL：说明 Task 4 的 consumer 同步漏了某处（重新检查 extractor.go 第 181-183, 217-218 行），改完再跑。

- [ ] **Step 3: 跑整包**

Run: `go test ./internal/adapter/bridge/...`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add internal/adapter/bridge/extractor_test.go
git commit -m "test(bridge): explicit regression for TaskCompleted + MessageDisplay

After the field rename in 'refactor(bridge): align CCHookInput fields',
add explicit tests proving:
  TaskCompleted reads in.TaskSubject (was empty due to wrong json tag)
  MessageDisplay reads in.Delta (was empty due to wrong json tag)
Part of §5 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 9: §4 — t_events schema 加 cwd + project_name 列 + 索引

**Files:**
- Modify: `internal/infra/store/schema.sql:26-44`
- Modify: `internal/infra/store/sqlite.go:398-411`（migrate 函数）

- [ ] **Step 1: 改 schema.sql — 在 t_events 表末尾增列**

在 `internal/infra/store/schema.sql` 第 36 行（`timestamp` 那行）后、第 38 行 `FOREIGN KEY` 前，插入两列：

```sql
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),
    cwd             TEXT    NOT NULL DEFAULT '',          -- v2.1: 冗余自 t_sessions（调试便利）
    project_name    TEXT    NOT NULL DEFAULT '',          -- v2.1: 冗余自 t_sessions

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
```

并在 `idx_events_timestamp` 索引后追加新索引：

```sql
CREATE INDEX IF NOT EXISTS idx_events_timestamp  ON t_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_project    ON t_events(project_name, timestamp DESC);
```

- [ ] **Step 2: 改 sqlite.go migrate() — 加 ALTER + backfill**

在 `internal/infra/store/sqlite.go` 第 410 行 `_, _ = db.Exec(\`ALTER TABLE t_events DROP COLUMN attention_level\`)` 之后，第 411 行 `}` 之前，追加：

```go
	// v2.1: redundant cwd / project_name on t_events for ad-hoc debugging
	_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN cwd          TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN project_name TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_project ON t_events(project_name, timestamp DESC)`)

	// v2.1: backfill cwd / project_name for existing events that joined late
	_, _ = db.Exec(`
		UPDATE t_events
		SET cwd = (SELECT s.cwd FROM t_sessions s WHERE s.session_key = t_events.session_key),
		    project_name = (SELECT s.project_name FROM t_sessions s WHERE s.session_key = t_events.session_key)
		WHERE cwd = '' OR project_name = ''
	`)
```

完整 migrate 函数末段示意：

```go
func migrate(db *sqlx.DB) {
	// v1.1: agent_label on t_events (existing)
	_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN agent_label TEXT NOT NULL DEFAULT ''`)

	// v2.0: rewrite legacy status values
	_, _ = db.Exec(`UPDATE t_sessions SET status = 'working' WHERE status = 'active'`)
	_, _ = db.Exec(`UPDATE t_sessions SET status = 'done'    WHERE status = 'finished'`)

	// v2.0: drop obsolete attention_level columns
	_, _ = db.Exec(`ALTER TABLE t_sessions DROP COLUMN attention_level`)
	_, _ = db.Exec(`ALTER TABLE t_events DROP COLUMN attention_level`)

	// v2.1: redundant cwd / project_name on t_events for ad-hoc debugging
	_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN cwd          TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE t_events ADD COLUMN project_name TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_project ON t_events(project_name, timestamp DESC)`)

	// v2.1: backfill cwd / project_name for existing events
	_, _ = db.Exec(`
		UPDATE t_events
		SET cwd = (SELECT s.cwd FROM t_sessions s WHERE s.session_key = t_events.session_key),
		    project_name = (SELECT s.project_name FROM t_sessions s WHERE s.session_key = t_events.session_key)
		WHERE cwd = '' OR project_name = ''
	`)
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./internal/infra/store/...`
Expected: 无错误。

- [ ] **Step 4: 跑现有 store 测试**

Run: `go test ./internal/infra/store/...`
Expected: PASS（INSERT 还没改，但因为新列 NOT NULL DEFAULT ''，旧 INSERT 写入时会自动填 ''，不破坏测试）。

- [ ] **Step 5: 提交**

```bash
git add internal/infra/store/schema.sql internal/infra/store/sqlite.go
git commit -m "feat(store): t_events redundant cwd + project_name columns

Adds two columns to t_events (and idx_events_project index) so ad-hoc
SQLite debugging doesn't need a JOIN with t_sessions:
  ALTER TABLE t_events ADD COLUMN cwd          TEXT NOT NULL DEFAULT '';
  ALTER TABLE t_events ADD COLUMN project_name TEXT NOT NULL DEFAULT '';
  CREATE INDEX idx_events_project ON t_events(project_name, timestamp DESC);
Migrate path also backfills these from t_sessions for existing rows
(idempotent — only updates rows where the columns are still '').
Columns are appended at the end (SQLite ALTER TABLE limitation —
column reordering would require table rebuild, not worth the risk
since column order doesn't affect queries).
INSERT path will start writing these in the next commit.
Implements §4 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 10: §4 — INSERT 路径填充 cwd + project_name

**Files:**
- Modify: `internal/infra/store/sqlite.go:373-380`（INSERT INTO t_events 语句）
- Modify: `internal/infra/store/sqlite_test.go`（新增 2 个测试）

- [ ] **Step 1: 写失败测试 — Record 后 cwd / project_name 落库**

在 `internal/infra/store/sqlite_test.go` 末尾追加：

```go
func TestSQLiteStore_InsertEvent_PopulatesCwdAndProjectName(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	e := makeTestEvent("sess-cwd", entity.EventPreToolUse, "Bash")
	e.CWD = "/Users/dev/projects/myapp"
	s.Record(e)

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen with raw SQL to inspect the new columns
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer rawDB.Close()

	var cwd, project string
	err = rawDB.QueryRow(
		`SELECT cwd, project_name FROM t_events WHERE session_key = ? ORDER BY id DESC LIMIT 1`,
		"sess-cwd",
	).Scan(&cwd, &project)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if cwd != "/Users/dev/projects/myapp" {
		t.Errorf("cwd = %q, want %q", cwd, "/Users/dev/projects/myapp")
	}
	if project != "myapp" {
		t.Errorf("project_name = %q, want %q", project, "myapp")
	}
}

func TestSQLiteStore_Migrate_BackfillsCwdAndProjectName(t *testing.T) {
	dbPath := tempDBPath(t)

	// First open: NewSQLiteStore creates schema + runs migrate (no-op for new DB)
	// + insert a session and an event with the new columns auto-populated.
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	e := makeTestEvent("sess-bf", entity.EventPreToolUse, "Bash")
	e.CWD = "/projects/legacy"
	s.Record(e)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate "old data": clear the new columns to '' on the existing row,
	// then re-open (which runs migrate -> backfill).
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := rawDB.Exec(`UPDATE t_events SET cwd='', project_name='' WHERE session_key='sess-bf'`); err != nil {
		t.Fatalf("clear cols: %v", err)
	}
	rawDB.Close()

	// Reopen — migrate() should backfill from t_sessions.
	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	rawDB2, _ := sql.Open("sqlite", dbPath)
	defer rawDB2.Close()
	var cwd, project string
	if err := rawDB2.QueryRow(
		`SELECT cwd, project_name FROM t_events WHERE session_key='sess-bf' LIMIT 1`,
	).Scan(&cwd, &project); err != nil {
		t.Fatalf("query: %v", err)
	}
	if cwd == "" || project == "" {
		t.Errorf("expected backfill, got cwd=%q project=%q", cwd, project)
	}
	if project != "legacy" {
		t.Errorf("project_name = %q, want %q", project, "legacy")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/infra/store/ -run 'TestSQLiteStore_(InsertEvent_Populates|Migrate_Backfills)' -v`
Expected: 第一个 FAIL —— INSERT 还没绑这两列，所以查回来都是空字符串；第二个 PASS（backfill 已经在 Task 9 装好了）。

- [ ] **Step 3: 改 INSERT 语句加两列绑定**

在 `internal/infra/store/sqlite.go` 第 373-380 行：

```go
// 从：
_, err = tx.Exec(`
    INSERT INTO t_events (session_key, agent_label, event_type, tool_name, tool_use_id, content, content_raw, permission_mode, raw_payload, timestamp)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
    sessionKey, e.AgentLabel, e.EventType, e.ToolName, e.ToolUseID,
    e.Content, e.ContentRaw, e.PermissionMode,
    []byte(e.RawPayload), ts,
)

// 改为：
_, err = tx.Exec(`
    INSERT INTO t_events (session_key, agent_label, event_type, tool_name, tool_use_id, content, content_raw, permission_mode, raw_payload, timestamp, cwd, project_name)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
    sessionKey, e.AgentLabel, e.EventType, e.ToolName, e.ToolUseID,
    e.Content, e.ContentRaw, e.PermissionMode,
    []byte(e.RawPayload), ts,
    e.CWD, session.ProjectFromCWD(e.CWD),
)
```

注意：`session.ProjectFromCWD` 已经在文件顶部 import（被第 349 行 `projectName := session.ProjectFromCWD(e.CWD)` 使用），无需新加 import。

- [ ] **Step 4: 跑测试确认两个都通过**

Run: `go test ./internal/infra/store/ -run 'TestSQLiteStore_(InsertEvent_Populates|Migrate_Backfills)' -v`
Expected: PASS（两个都过）。

- [ ] **Step 5: 跑整个 store 包**

Run: `go test ./internal/infra/store/...`
Expected: PASS（所有现有测试 + 2 个新测试）。

- [ ] **Step 6: 提交**

```bash
git add internal/infra/store/sqlite.go internal/infra/store/sqlite_test.go
git commit -m "feat(store): INSERT into t_events writes cwd + project_name

Now that the columns + backfill exist (previous commit), the live
INSERT path also writes them. project_name is computed via
session.ProjectFromCWD (the same helper t_sessions.project_name uses,
keeping the two consistent).
Adds 2 unit tests:
  - InsertEvent_PopulatesCwdAndProjectName: live INSERT writes both
  - Migrate_BackfillsCwdAndProjectName: existing rows get filled on reopen
Implements §4 of 2026-06-02-pager-polish-followup-design.md"
```

---

## Task 11: 更新回归测试文档

**Files:**
- Modify: `docs/regression/2026-06-01-ui-state-regression.md:13-21`（Section A 颜色描述）
- Modify: `docs/regression/2026-06-01-ui-state-regression.md`（新增 event_type pill + cwd/project_name 验收行）

- [ ] **Step 1: 改 Section A 表格**

在 `docs/regression/2026-06-01-ui-state-regression.md` 第 17-20 行：

```markdown
| 1 | `--event PreToolUse --agent CC` with stdin `{"tool_name":"AskUserQuestion","cwd":"/p/x","session_id":"r1"}` | WAITING tag (warm orange #ff9500), HandHelping pulsing icon |
| 2 | `--event PreToolUse` with `{"tool_name":"Edit","permission_mode":"bypassPermissions",...}` | WORKING tag (green), Loader2 spinning |
| 3 | `--event Stop` (same session) | DONE tag (yellow-brown #cc9a00), CheckCircle2 |
| 4 | `--event StopFailure` (new session) | ERROR tag (red #ff453a), AlertTriangle |
```

变更点：
- 行 1：`(red)` → `(warm orange #ff9500)`
- 行 3：`(light blue)` → `(yellow-brown #cc9a00)`
- 行 4：`(orange)` → `(red #ff453a)`

- [ ] **Step 2: 在 Section A 表格末尾追加新验收行**

在第 4 行后（`StopFailure` 那行之后），追加：

```markdown
| 5 | All four events above | header shows event_type as outline pill (no fill, secondary text color) right after the agent pill — values: `PreToolUse`, `Stop`, `StopFailure` |
```

- [ ] **Step 3: 在 Section I（Headless Data-Path Regression）末尾追加 SQLite cwd 验收行**

在 Section I 末尾（`Expected: PASS for all 9 sub-tests...` 那段后），追加：

```markdown

After running, manually verify the new columns are populated:

```bash
sqlite3 ~/.config/pager/pager.db "SELECT event_type, cwd, project_name FROM t_events ORDER BY id DESC LIMIT 5"
```

Expected: every row has non-empty `cwd` and `project_name` (project_name = last segment of cwd).
```

- [ ] **Step 4: 提交**

```bash
git add docs/regression/2026-06-01-ui-state-regression.md
git commit -m "docs(regression): update for D2 colors + event_type pill + cwd column

- Section A: card color descriptions (Waiting orange / Done yellow-brown
  / Error red / Working unchanged)
- Section A: new row for event_type outline pill in card header
- Section I: SQLite manual check that t_events.cwd/project_name are
  populated end-to-end"
```

---

## Task 12: 端到端验证 — make build + 全量测试 + 手动回归

**Files:** 无代码改动；只跑命令 + 视觉确认。

- [ ] **Step 1: 跑全量 Go 测试**

Run: `make test`
Expected: PASS — 所有包，含新增 4 个 store 测试 + 7 个 bridge 测试。

- [ ] **Step 2: 跑 lint**

Run: `make lint`
Expected: 无 warning/error。

- [ ] **Step 3: 跑 frontend tsc**

Run: `cd frontend && pnpm exec tsc --noEmit`
Expected: 无类型错误。

- [ ] **Step 4: 构建 .app**

Run: `make build`
Expected: 成功生成 `bin/Pager.app`。

- [ ] **Step 5: 手动回归 Section A — 4 状态颜色 + event_type pill**

参考 `docs/regression/2026-06-01-ui-state-regression.md` Section A，跑 4 个事件，逐一确认：

| 事件 | 预期颜色 | 预期 event_type pill |
|---|---|---|
| PreToolUse + AskUserQuestion | WAITING 暖橙 #ff9500 | `PreToolUse` outline |
| PreToolUse + Edit + bypass | WORKING 绿 #30d158 | `PreToolUse` outline |
| Stop | DONE 黄褐 #cc9a00 | `Stop` outline |
| StopFailure | ERROR 红 #ff453a | `StopFailure` outline |

- [ ] **Step 6: 手动回归 Section I — SQLite 新列**

```bash
sqlite3 ~/.config/pager/pager.db "SELECT event_type, cwd, project_name FROM t_events ORDER BY id DESC LIMIT 10"
```

Expected: 每行 cwd / project_name 都非空，project_name 等于 cwd 末段。

- [ ] **Step 7: 手动回归 §5 — 5 类事件 content 不再为空**

触发以下事件后查 SQLite：

```bash
sqlite3 ~/.config/pager/pager.db "SELECT event_type, content FROM t_events WHERE event_type IN ('Error','MessageDisplay','PostToolBatch','TaskCompleted','SubagentStop') ORDER BY id DESC LIMIT 20"
```

Expected:
- `Error` 行：`<error_type>: <error_message>` 格式（除非 payload 双字段都空）
- `MessageDisplay` 行：CC 的 delta 文本
- `PostToolBatch` 行：逗号分隔的 tool_name 列表
- `TaskCompleted` 行：task_subject 文本
- `SubagentStop` 行：last_assistant_message 文本（如果有）；否则 agent_type

- [ ] **Step 8: 存量数据兼容性检查**

如果手头有 v1.5 时期建的 `~/.config/pager/pager.db`，备份后用新版本启动一次：

```bash
cp ~/.config/pager/pager.db /tmp/pager-v1.5-backup.db
open bin/Pager.app
# 让它跑 30 秒触发 migrate
```

然后查：

```bash
sqlite3 ~/.config/pager/pager.db "SELECT COUNT(*) FROM t_events WHERE cwd != '' AND project_name != ''"
sqlite3 ~/.config/pager/pager.db "SELECT COUNT(*) FROM t_events"
```

Expected: 两个 COUNT 数字相等（所有旧行都被 backfill 了）。

如果第一个 COUNT 比第二个小：说明 backfill SQL 有 session_key 不存在于 t_sessions 的孤儿行；这种情况下旧 cwd 仍然是 ''，是预期行为（subselect 返回 NULL，UPDATE 写不进去）。无需修复。

- [ ] **Step 9: （可选）合并提交一个汇总 commit message**

无需新 commit；本 task 只是验证。如果发现回归 bug，回到对应 task 修复并新建 commit。

---

## 完成准则

所有 12 个 task 全部 ✅ 后，本次 followup 实施完毕。状态：

- ✅ 5 节 spec 全部落地（§1 推迟）
- ✅ 11 个文件改动，每个独立 commit
- ✅ 7 个新 bridge 测试 + 2 个新 store 测试通过
- ✅ make build / make test / make lint 全绿
- ✅ Section A + Section I 手动回归通过

之后调用 `superpowers:finishing-a-development-branch` 收尾（合并到 main / 打 tag / 等）。
