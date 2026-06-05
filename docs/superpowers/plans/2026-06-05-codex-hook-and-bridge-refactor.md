# Codex Hook 接入 & Bridge 重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 pager bridge 重构为 Agent 接口 + 纯 DSL 抽取架构，并接入 Codex 0.137 hook 体系实现完整对等观测。

**Architecture:** 三层关注点分离：(1) Agent 接口拥有身份/输入 schema/hook 注册清单/早丢弃；(2) Envelope 提供主流程所需路由字段 + RawPayload；(3) DSL 引擎接 (template, payload) 出 string，agent 无关、event 无关。`extract_rules.yaml` 升级到 v3，事件名做一级 key，覆盖所有事件类型（不再有 pre/post/events 中间层）。

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3`, `github.com/tidwall/gjson`, `github.com/spf13/pflag`（新增）。

**Spec 来源:** `docs/superpowers/specs/2026-06-05-codex-hook-and-bridge-refactor-design.md`

---

## 阶段总览

| Phase | PR | 范围 | 任务数 |
|---|---|---|---|
| **Phase 1** | PR 1 | Bridge 重构（不接 Codex），CC/CodeBuddy 行为完全等效 | 16 |
| **Phase 2** | PR 2 | Codex 接入（agents/codex.go + YAML codex 段 + installer target） | 5 |
| **Phase 3** | PR 3 | CLAUDE.md / PRD.md 文档更新 + 旧二进制清理提示 | 2 |

每个 Phase 末尾有 **Review Checkpoint** —— 在下一个 Phase 开始前必须通过。

---

## File Structure

### Phase 1 创建的新文件

```
cmd/pager-bridge/
  main.go                                    # 主流程（替代 cmd/bridge/main.go）
  main_test.go                               # 单测：argv 解析、--print-hooks 行为

cmd/pager-installhooks/
  main.go                                    # installer 入口
  targets.go                                 # agent → settings 路径映射
  upsert.go                                  # append + 幂等的 hook 写入逻辑
  upsert_test.go                             # 单测：保留用户 entry、剔除 pager 旧名

internal/adapter/bridge/
  envelope.go                                # Envelope struct
  render.go                                  # Render 单一入口
  render_test.go                             # 端到端：(agent, event, tool, payload) → 字符串
  agents/
    agent.go                                 # Agent 接口 + HookSpec
    claude.go                                # ClaudeFamily 实现
    claude_test.go                           # ParseEnvelope/Accept/Hooks 单测
    registry.go                              # label → Agent 注册表
    registry_test.go                         # SelectByLabel / All 单测
```

### Phase 1 修改的文件

```
internal/adapter/bridge/rules.go             # Rules v3 + Lookup + PickTemplate + 链式 filter + 4 新 filter
internal/adapter/bridge/rules_test.go        # 全面重写（v3 + 4 新 filter + 链式）
internal/adapter/bridge/extract_rules.yaml   # v3 重组（事件名一级 key + claude 段）
go.mod / go.sum                              # 引入 spf13/pflag
Makefile                                     # BRIDGE_BIN / install-bridge / 删除 install-hooks 系列
```

### Phase 1 删除的文件

```
cmd/bridge/main.go                           # 改名到 cmd/pager-bridge/
cmd/bridge/main_test.go
internal/adapter/bridge/extractor.go         # ExtractContent / ExtractEventContent 全部被 Render 取代
internal/adapter/bridge/extractor_test.go
internal/adapter/bridge/types.go (大半)      # CCHookInput 及子 struct 删除
internal/adapter/bridge/types_test.go
scripts/install-hooks.sh                     # 改成 Go 实现
```

### Phase 2 创建/修改

```
internal/adapter/bridge/agents/codex.go      # 新建
internal/adapter/bridge/agents/codex_test.go # 新建
internal/adapter/bridge/agents/registry.go   # +1 行：注册 Codex
internal/adapter/bridge/extract_rules.yaml   # +codex 段
internal/domain/entity/event.go              # +EventPostCompact / EventSubagentStart
internal/domain/session/status_test.go       # +Codex 用例
cmd/pager-installhooks/targets.go            # +Codex Target
```

---

# Phase 1 — PR 1: Bridge Refactor

> **目标**：架构搬到位，**行为零变化**。CC / CodeBuddy 现网完全等效，Codex 还没接进来。
>
> **入口任务**：从 Task 1 开始，按顺序执行。
>
> **退出条件**（Review Checkpoint）：
> - `make test` 全绿
> - `make install-bridge` 跑通且 `~/.claude/settings.local.json` 写入 23 个 hook entry
> - 实际触发一次 CC 工具调用，session 列表/UI 行为与重构前一致

---

## Task 1: 新增 pflag 依赖 + 创建空骨架文件

**目的**：把所有新文件位置先占住，让后续任务可以直接 edit；同时锁定 pflag 版本。

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `cmd/pager-bridge/main.go`（最小可编译的 main）
- Create: `cmd/pager-installhooks/main.go`（最小可编译的 main）
- Create: `internal/adapter/bridge/envelope.go`
- Create: `internal/adapter/bridge/render.go`
- Create: `internal/adapter/bridge/agents/agent.go`
- Create: `internal/adapter/bridge/agents/claude.go`
- Create: `internal/adapter/bridge/agents/registry.go`

- [ ] **Step 1: 添加 pflag 依赖**

```bash
go get github.com/spf13/pflag@latest
```

- [ ] **Step 2: 创建 cmd/pager-bridge/main.go 骨架**

```go
package main

import (
	"github.com/spf13/pflag"
)

func main() {
	pflag.Parse()
	// TODO: implementation in Task 10
}
```

- [ ] **Step 3: 创建 cmd/pager-installhooks/main.go 骨架**

```go
package main

import (
	"github.com/spf13/pflag"
)

func main() {
	pflag.Parse()
	// TODO: implementation in Task 12
}
```

- [ ] **Step 4: 创建 envelope.go 占位（待 Task 2 填充）**

```go
package bridge
```

- [ ] **Step 5: 创建 render.go 占位**

```go
package bridge
```

- [ ] **Step 6: 创建 agents/agent.go 占位**

```go
package agents
```

- [ ] **Step 7: 创建 agents/claude.go 占位**

```go
package agents
```

- [ ] **Step 8: 创建 agents/registry.go 占位**

```go
package agents
```

- [ ] **Step 9: 验证整体编译通过**

Run:
```bash
go build ./...
```

Expected: 无报错（warning 可以有）。

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum cmd/pager-bridge cmd/pager-installhooks \
        internal/adapter/bridge/envelope.go \
        internal/adapter/bridge/render.go \
        internal/adapter/bridge/agents
git commit -m "chore(bridge): scaffold new packages and pflag dep"
```

---

## Task 2: 定义 Envelope + Agent 接口 + HookSpec

**目的**：锁定数据契约。后续任务都基于这两个类型。

**Files:**
- Modify: `internal/adapter/bridge/envelope.go`
- Modify: `internal/adapter/bridge/agents/agent.go`

- [ ] **Step 1: 实现 Envelope struct**

替换 `internal/adapter/bridge/envelope.go` 的全部内容：

```go
package bridge

import "encoding/json"

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
	ToolInput      json.RawMessage // 工具规则 pre 输入
	ToolResponse   json.RawMessage // 工具规则 post 输入
	TurnID         string          // Codex 才填，CC/CodeBuddy 留空
	RawPayload     json.RawMessage // 完整 stdin，DSL 读这个
}
```

- [ ] **Step 2: 实现 Agent 接口与 HookSpec**

替换 `internal/adapter/bridge/agents/agent.go` 的全部内容：

```go
package agents

import (
	"errors"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
)

// ErrInvalidPayload 表示 hook stdin 不是合法 JSON（或字段类型不匹配）。
// 主流程收到这个错误应该 silent return（hook 是 fire-and-forget）。
var ErrInvalidPayload = errors.New("agents: invalid hook payload")

// Agent 描述一个被 pager 支持的 coding agent（CC / CodeBuddy / Codex / ...）。
// 一个 Agent 实现可以服务多个 label——例如 ClaudeFamily{} 同时映射 "CC"、
// "CC-Internal"、"CodeBuddy" 三个 label，因为它们 schema 完全相同。
type Agent interface {
	// ID 返回写入 entity.AgentEvent.Agent 的稳定标识，
	// 同时是 extract_rules.yaml 的 namespace key。
	// 必须返回 entity.AgentXxx 中的常量。
	ID() string

	// ParseEnvelope 把 hook stdin 原始字节解为通用 Envelope。
	// 各 agent 在这里实现自己的 JSON schema 映射。
	// 返回 ErrInvalidPayload 表示 JSON 解码失败。
	ParseEnvelope(raw []byte) (*bridge.Envelope, error)

	// Accept 决定一个事件是否应该上报给 pager 服务器。
	// 返回 false 时事件被静默丢弃（例：CodeBuddy 的 auth_success Notification
	// 没有 session_id，落到下游会污染 session 列表）。
	Accept(eventType string, env *bridge.Envelope) bool

	// Hooks 返回该 agent 应该注册的全部 hook 事件清单。
	// pager-installhooks 通过 --print-hooks 子命令读取这个清单。
	Hooks() []HookSpec
}

// HookSpec 描述一个待注册的 hook 事件。
// Matcher 仅 PreToolUse / PostToolUse 等工具事件需要（"*" 表示匹配所有工具）；
// 非工具事件留空，installer 写入时不输出 matcher 字段。
type HookSpec struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
}
```

- [ ] **Step 3: 验证编译通过**

Run:
```bash
go build ./...
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/bridge/envelope.go internal/adapter/bridge/agents/agent.go
git commit -m "feat(bridge): define Envelope, Agent interface, HookSpec"
```

---

## Task 3: 实现 ClaudeFamily.ID() 与 Hooks()

**目的**：把 Agent 接口最静态的两个方法先实现。Hooks 是 23 个事件清单，由 install-hooks.sh 现有事件迁移而来。

**Files:**
- Modify: `internal/adapter/bridge/agents/claude.go`
- Create: `internal/adapter/bridge/agents/claude_test.go`

- [ ] **Step 1: 写失败的测试**

创建 `internal/adapter/bridge/agents/claude_test.go`：

```go
package agents

import (
	"testing"

	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

func TestClaudeFamily_ID(t *testing.T) {
	if got := (ClaudeFamily{}).ID(); got != entity.AgentClaudeCode {
		t.Errorf("ID() = %q, want %q", got, entity.AgentClaudeCode)
	}
}

func TestClaudeFamily_Hooks_Count(t *testing.T) {
	hooks := ClaudeFamily{}.Hooks()
	if len(hooks) != 23 {
		t.Errorf("len(Hooks()) = %d, want 23", len(hooks))
	}
}

func TestClaudeFamily_Hooks_PreToolUseHasMatcher(t *testing.T) {
	hooks := ClaudeFamily{}.Hooks()
	for _, h := range hooks {
		if h.Event == "PreToolUse" {
			if h.Matcher != "*" {
				t.Errorf("PreToolUse.Matcher = %q, want %q", h.Matcher, "*")
			}
			return
		}
	}
	t.Error("PreToolUse not in Hooks()")
}

func TestClaudeFamily_Hooks_StopHasNoMatcher(t *testing.T) {
	hooks := ClaudeFamily{}.Hooks()
	for _, h := range hooks {
		if h.Event == "Stop" {
			if h.Matcher != "" {
				t.Errorf("Stop.Matcher = %q, want empty", h.Matcher)
			}
			return
		}
	}
	t.Error("Stop not in Hooks()")
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestClaudeFamily -v
```

Expected: 编译失败，`ClaudeFamily` is not defined。

- [ ] **Step 3: 实现 ClaudeFamily.ID() 与 Hooks()**

替换 `internal/adapter/bridge/agents/claude.go` 的全部内容：

```go
package agents

import (
	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

// ClaudeFamily 是 Claude Code 与 schema 完全兼容的 agent 家族实现。
// 同时服务 "CC"、"CC-Internal"、"CodeBuddy" 三个 label。
type ClaudeFamily struct{}

func (ClaudeFamily) ID() string { return entity.AgentClaudeCode }

func (ClaudeFamily) ParseEnvelope(raw []byte) (*bridge.Envelope, error) {
	// TODO Task 4
	return nil, ErrInvalidPayload
}

func (ClaudeFamily) Accept(eventType string, env *bridge.Envelope) bool {
	// TODO Task 5
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

- [ ] **Step 4: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestClaudeFamily -v
```

Expected: 4 个 test 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/agents/claude.go internal/adapter/bridge/agents/claude_test.go
git commit -m "feat(agents): ClaudeFamily ID() and Hooks() implementation"
```

---

## Task 4: 实现 ClaudeFamily.ParseEnvelope（TDD）

**目的**：把 hook stdin JSON 解为 Envelope。覆盖 PreToolUse、Stop、Notification 三类典型 schema。

**Files:**
- Modify: `internal/adapter/bridge/agents/claude.go`
- Modify: `internal/adapter/bridge/agents/claude_test.go`

- [ ] **Step 1: 追加失败测试**

在 `claude_test.go` 文件末尾追加：

```go
import "encoding/json"

// (上面这行 import 如果已存在则跳过)

func TestClaudeFamily_ParseEnvelope_PreToolUseBash(t *testing.T) {
	raw := []byte(`{
		"session_id": "abc123",
		"transcript_path": "/tmp/transcript.jsonl",
		"cwd": "/home/user/project",
		"hook_event_name": "PreToolUse",
		"permission_mode": "default",
		"tool_name": "Bash",
		"tool_input": {"command": "ls -la"},
		"tool_use_id": "call_xyz"
	}`)
	env, err := ClaudeFamily{}.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope error = %v", err)
	}
	if env.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want %q", env.SessionID, "abc123")
	}
	if env.EventName != "PreToolUse" {
		t.Errorf("EventName = %q, want %q", env.EventName, "PreToolUse")
	}
	if env.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", env.ToolName, "Bash")
	}
	if env.PermissionMode != "default" {
		t.Errorf("PermissionMode = %q, want %q", env.PermissionMode, "default")
	}
	if string(env.ToolInput) != `{"command": "ls -la"}` {
		t.Errorf("ToolInput = %q, want raw json with command", string(env.ToolInput))
	}
	if string(env.RawPayload) != string(raw) {
		t.Error("RawPayload should equal input bytes")
	}
}

func TestClaudeFamily_ParseEnvelope_Stop(t *testing.T) {
	raw := []byte(`{
		"session_id": "s1",
		"hook_event_name": "Stop",
		"cwd": "/x"
	}`)
	env, err := ClaudeFamily{}.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope error = %v", err)
	}
	if env.EventName != "Stop" {
		t.Errorf("EventName = %q", env.EventName)
	}
	if env.ToolName != "" {
		t.Errorf("ToolName should be empty, got %q", env.ToolName)
	}
}

func TestClaudeFamily_ParseEnvelope_InvalidJSON(t *testing.T) {
	raw := []byte(`{not valid json`)
	_, err := ClaudeFamily{}.ParseEnvelope(raw)
	if err != ErrInvalidPayload {
		t.Errorf("err = %v, want ErrInvalidPayload", err)
	}
}

func TestClaudeFamily_ParseEnvelope_EmptyToolInputIsNil(t *testing.T) {
	raw := []byte(`{"session_id":"x","hook_event_name":"Stop"}`)
	env, err := ClaudeFamily{}.ParseEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env.ToolInput != nil && string(env.ToolInput) != "" {
		t.Errorf("ToolInput should be nil/empty, got %q", string(env.ToolInput))
	}
}

// 防止 import 优化器误删
var _ = json.RawMessage{}
```

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestClaudeFamily_ParseEnvelope -v
```

Expected: 4 个 test 失败（ParseEnvelope 当前总是返回 ErrInvalidPayload）。

- [ ] **Step 3: 实现 ParseEnvelope**

在 `claude.go` 中替换 ParseEnvelope 方法：

```go
func (ClaudeFamily) ParseEnvelope(raw []byte) (*bridge.Envelope, error) {
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
	return &bridge.Envelope{
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
```

并在 claude.go 顶部 import 中加 `"encoding/json"`。

- [ ] **Step 4: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestClaudeFamily_ParseEnvelope -v
```

Expected: 4 个 test 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/agents/claude.go internal/adapter/bridge/agents/claude_test.go
git commit -m "feat(agents): ClaudeFamily.ParseEnvelope decodes CC hook stdin"
```

---

## Task 5: 实现 ClaudeFamily.Accept（TDD）

**目的**：实现早丢弃规则。当前唯一规则是 CodeBuddy 的 `auth_success` Notification（无 session_id 的 Notification）。

**Files:**
- Modify: `internal/adapter/bridge/agents/claude.go`
- Modify: `internal/adapter/bridge/agents/claude_test.go`

- [ ] **Step 1: 追加失败测试**

在 `claude_test.go` 末尾追加：

```go
func TestClaudeFamily_Accept_NotificationWithoutSessionIDIsDropped(t *testing.T) {
	env := &bridge.Envelope{SessionID: ""}
	if (ClaudeFamily{}).Accept("Notification", env) {
		t.Error("Notification with empty SessionID should be dropped")
	}
}

func TestClaudeFamily_Accept_NotificationWithSessionIDIsKept(t *testing.T) {
	env := &bridge.Envelope{SessionID: "abc"}
	if !(ClaudeFamily{}).Accept("Notification", env) {
		t.Error("Notification with SessionID should be kept")
	}
}

func TestClaudeFamily_Accept_OtherEventsAlwaysKept(t *testing.T) {
	env := &bridge.Envelope{SessionID: ""}
	for _, evt := range []string{"PreToolUse", "Stop", "SessionStart"} {
		if !(ClaudeFamily{}).Accept(evt, env) {
			t.Errorf("event %q should be kept regardless of SessionID", evt)
		}
	}
}
```

并确保 `claude_test.go` 顶部 import 包含 `"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"`。

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestClaudeFamily_Accept -v
```

Expected: `TestClaudeFamily_Accept_NotificationWithoutSessionIDIsDropped` 失败（当前 Accept 永远返回 true）。

- [ ] **Step 3: 实现 Accept**

在 `claude.go` 替换 Accept 方法：

```go
func (ClaudeFamily) Accept(eventType string, env *bridge.Envelope) bool {
	// CodeBuddy 的 auth_success Notification 没 session_id，丢
	if eventType == "Notification" && env.SessionID == "" {
		return false
	}
	return true
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestClaudeFamily_Accept -v
```

Expected: 3 个 test 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/agents/claude.go internal/adapter/bridge/agents/claude_test.go
git commit -m "feat(agents): ClaudeFamily.Accept drops notification without session_id"
```

---

## Task 6: 创建 registry（TDD）

**目的**：把 label → Agent 的映射独立成单一注册点。

**Files:**
- Modify: `internal/adapter/bridge/agents/registry.go`
- Create: `internal/adapter/bridge/agents/registry_test.go`

- [ ] **Step 1: 写失败的测试**

创建 `internal/adapter/bridge/agents/registry_test.go`：

```go
package agents

import "testing"

func TestSelectByLabel_KnownLabels(t *testing.T) {
	tests := []struct {
		label string
		wantOK bool
	}{
		{"CC", true},
		{"CC-Internal", true},
		{"CodeBuddy", true},
		{"Unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			a, ok := SelectByLabel(tt.label)
			if ok != tt.wantOK {
				t.Errorf("SelectByLabel(%q) ok = %v, want %v", tt.label, ok, tt.wantOK)
			}
			if tt.wantOK && a == nil {
				t.Errorf("SelectByLabel(%q) returned nil agent", tt.label)
			}
		})
	}
}

func TestSelectByLabel_CCAndCodeBuddyMapToSameType(t *testing.T) {
	cc, _ := SelectByLabel("CC")
	cb, _ := SelectByLabel("CodeBuddy")
	if cc.ID() != cb.ID() {
		t.Errorf("CC.ID()=%q, CodeBuddy.ID()=%q, want same", cc.ID(), cb.ID())
	}
}

func TestAll_SortedByLabel(t *testing.T) {
	all := All()
	if len(all) < 3 {
		t.Errorf("len(All()) = %d, want >= 3", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Label > all[i].Label {
			t.Errorf("All() not sorted: %q > %q", all[i-1].Label, all[i].Label)
		}
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run "TestSelectByLabel|TestAll" -v
```

Expected: 编译失败（SelectByLabel 和 All 都未定义）。

- [ ] **Step 3: 实现 registry**

替换 `internal/adapter/bridge/agents/registry.go` 的全部内容：

```go
package agents

import "sort"

// registry 把命令行 --agent 参数（label）映射到具体 Agent 实例。
// 加新 agent 在这里加一行；ClaudeFamily 服务 schema 兼容的多个 label。
var registry = map[string]Agent{
	"CC":          ClaudeFamily{},
	"CC-Internal": ClaudeFamily{},
	"CodeBuddy":   ClaudeFamily{},
}

// SelectByLabel 根据 --agent 命令行参数返回对应 Agent 实例。
// 未知 label 返回 (nil, false)，主流程据此 silent return。
func SelectByLabel(label string) (Agent, bool) {
	a, ok := registry[label]
	return a, ok
}

// LabeledAgent 是 (label, agent) 的配对，All() 返回类型。
type LabeledAgent struct {
	Label string
	Agent Agent
}

// All 返回所有已注册的 Agent，按 label 字典序。
// pager-installhooks 用这个枚举所有 settings 文件，避免在 installer 里
// 重复维护 label 列表。
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

- [ ] **Step 4: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run "TestSelectByLabel|TestAll" -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/agents/registry.go internal/adapter/bridge/agents/registry_test.go
git commit -m "feat(agents): registry with SelectByLabel and All"
```

---

## Task 7: DSL 链式 filter 支持 + 4 个新 filter

**目的**：扩展 `Evaluate` 让一个 token 可以串多个 filter（`{$.x|f1:a|f2:b}`），并实现 `prefix` / `lookup` / `pluck` / `join` 四个新 filter。

**Files:**
- Modify: `internal/adapter/bridge/rules.go`
- Modify: `internal/adapter/bridge/rules_test.go`

- [ ] **Step 1: 先看现有 resolveToken 结构**

Run:
```bash
grep -n "resolveToken\|applyFilter\|case \"firstline\"" internal/adapter/bridge/rules.go
```

记录当前 filter 实现的位置（应该在 rules.go 的下半部分）。

- [ ] **Step 2: 写失败的链式 filter 测试**

在 `internal/adapter/bridge/rules_test.go` 末尾追加：

```go
// ─── 链式 filter ──────────────────────────────────────────────────────────────

func TestEvaluate_ChainedFilters(t *testing.T) {
	got := Evaluate(
		"{$.tool_calls|pluck:tool_name|join:, }",
		[]byte(`{"tool_calls":[{"tool_name":"Bash"},{"tool_name":"Read"},{"tool_name":"Grep"}]}`),
		"PostToolBatch",
	)
	if got != "Bash, Read, Grep" {
		t.Errorf("got %q, want %q", got, "Bash, Read, Grep")
	}
}

func TestEvaluate_ChainedFilters_EmptyArray(t *testing.T) {
	got := Evaluate(
		"{$.tool_calls|pluck:tool_name|join:, }",
		[]byte(`{"tool_calls":[]}`),
		"PostToolBatch",
	)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── prefix filter ────────────────────────────────────────────────────────────

func TestEvaluate_PrefixFilter_NonEmpty(t *testing.T) {
	got := Evaluate("{$.command_args|prefix: }", []byte(`{"command_args":"foo"}`), "")
	if got != " foo" {
		t.Errorf("got %q, want %q", got, " foo")
	}
}

func TestEvaluate_PrefixFilter_Empty(t *testing.T) {
	got := Evaluate("{$.command_args|prefix: }", []byte(`{"command_args":""}`), "")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestEvaluate_PrefixFilter_MissingField(t *testing.T) {
	got := Evaluate("{$.x|prefix:[]}", []byte(`{}`), "")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── lookup filter ────────────────────────────────────────────────────────────

func TestEvaluate_LookupFilter_Hit(t *testing.T) {
	got := Evaluate(
		"{$.notification_type|lookup:permission_prompt=[等授权] ,idle_prompt=[等输入] ,default=}",
		[]byte(`{"notification_type":"permission_prompt"}`),
		"",
	)
	if got != "[等授权] " {
		t.Errorf("got %q, want %q", got, "[等授权] ")
	}
}

func TestEvaluate_LookupFilter_Miss_HasDefault(t *testing.T) {
	got := Evaluate(
		"{$.x|lookup:foo=F,default=DEFAULT}",
		[]byte(`{"x":"bar"}`),
		"",
	)
	if got != "DEFAULT" {
		t.Errorf("got %q, want %q", got, "DEFAULT")
	}
}

func TestEvaluate_LookupFilter_Miss_NoDefault(t *testing.T) {
	got := Evaluate("{$.x|lookup:foo=F}", []byte(`{"x":"bar"}`), "")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── pluck filter ─────────────────────────────────────────────────────────────

func TestEvaluate_PluckFilter_ReturnsArray(t *testing.T) {
	got := Evaluate(
		"{$.calls|pluck:name}",
		[]byte(`{"calls":[{"name":"a"},{"name":"b"}]}`),
		"",
	)
	if got != `["a","b"]` {
		t.Errorf("got %q, want %q", got, `["a","b"]`)
	}
}

func TestEvaluate_PluckFilter_EmptyArray(t *testing.T) {
	got := Evaluate("{$.calls|pluck:name}", []byte(`{"calls":[]}`), "")
	if got != `[]` && got != "" {
		t.Errorf("got %q, want [] or empty", got)
	}
}

// ─── join filter ──────────────────────────────────────────────────────────────

func TestEvaluate_JoinFilter_StringArray(t *testing.T) {
	// 这里直接传一个 JSON 数组字面量给 token，模拟 pluck 的输出
	got := Evaluate(`{$.arr|join:, }`, []byte(`{"arr":["x","y","z"]}`), "")
	if got != "x, y, z" {
		t.Errorf("got %q, want %q", got, "x, y, z")
	}
}

func TestEvaluate_JoinFilter_NotAnArray(t *testing.T) {
	got := Evaluate(`{$.s|join:,}`, []byte(`{"s":"foo"}`), "")
	if got != "foo" {  // 非数组直接透传
		t.Errorf("got %q, want %q", got, "foo")
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestEvaluate_(Chained|Prefix|Lookup|Pluck|Join)" -v
```

Expected: 全部失败（链式语法和新 filter 都不支持）。

- [ ] **Step 4: 升级 resolveToken 支持链式**

打开 `internal/adapter/bridge/rules.go`，找到 `resolveToken` 函数（之前应该是处理单个 filter 的）。替换为：

```go
// resolveToken handles a single {…} expression body.
//
// Constraint: the literal substring "//" inside a token is reserved for the
// fallback operator. Avoid embedding raw "//" in path expressions or filter
// arguments.
//
// Filter chaining: tokens of the form "$.path|f1:a|f2:b|f3" apply filters
// left-to-right.
func resolveToken(token string, payload []byte, toolName string) string {
	token = strings.TrimSpace(token)

	// Fallback chain: split on " // " and return first non-empty.
	if strings.Contains(token, "//") {
		parts := strings.Split(token, "//")
		for _, p := range parts {
			v := resolveToken(strings.TrimSpace(p), payload, toolName)
			if v != "" {
				return v
			}
		}
		return ""
	}

	// Split into head + filters by "|"
	parts := strings.Split(token, "|")
	head := strings.TrimSpace(parts[0])
	value := resolveHead(head, payload, toolName)
	for _, f := range parts[1:] {
		fname, farg := splitFilter(strings.TrimSpace(f))
		value = applyFilter(fname, farg, value)
	}
	return value
}

// resolveHead handles the first segment of a token (path / literal / value / tool_name).
// Was previously inline at top of resolveToken.
func resolveHead(head string, payload []byte, toolName string) string {
	switch {
	case head == "tool_name":
		return toolName
	case head == "value":
		// raw payload as a JSON string literal
		var s string
		if err := json.Unmarshal(payload, &s); err == nil {
			return s
		}
		return ""
	case strings.HasPrefix(head, "$."):
		res := gjson.GetBytes(payload, head[2:])
		if !res.Exists() {
			return ""
		}
		// For nested structures, return Raw (JSON form) so that downstream
		// filters like pluck / join can process arrays.
		if res.IsArray() || res.IsObject() {
			return res.Raw
		}
		return res.String()
	}
	return ""
}

// splitFilter parses "name:arg" into ("name", "arg"). If no colon, returns (whole, "").
func splitFilter(f string) (name, arg string) {
	idx := strings.Index(f, ":")
	if idx < 0 {
		return f, ""
	}
	return f[:idx], f[idx+1:]
}
```

注意：你需要保留原来的 `resolveRawToken`（处理 `<json:N>`）不变。

- [ ] **Step 5: 实现 applyFilter（含 4 个新 filter）**

在 `rules.go` 中添加（如果 `applyFilter` 已存在，替换它）：

```go
// applyFilter dispatches to a named filter implementation.
// Filters operate on the running string value; pluck temporarily produces
// a JSON array string consumed by join.
func applyFilter(name, arg, value string) string {
	switch name {
	case "firstline":
		if i := strings.IndexByte(value, '\n'); i >= 0 {
			return value[:i]
		}
		return value
	case "firstpara":
		if i := strings.Index(value, "\n\n"); i >= 0 {
			return value[:i]
		}
		return value
	case "default":
		if value == "" {
			return arg
		}
		return value
	case "bool":
		// arg = "trueLabel,falseLabel"
		labels := strings.SplitN(arg, ",", 2)
		if len(labels) != 2 {
			return value
		}
		switch value {
		case "true":
			return labels[0]
		case "false":
			return labels[1]
		}
		return value
	case "prefix":
		if value == "" {
			return ""
		}
		return arg + value
	case "lookup":
		// arg = "k1=v1,k2=v2,default=dv"
		table, def := parseLookupArg(arg)
		if v, ok := table[value]; ok {
			return v
		}
		return def
	case "pluck":
		// value should be a JSON array; arg is the field name
		res := gjson.Get(value, "#."+arg)
		if !res.Exists() {
			return ""
		}
		return res.Raw
	case "join":
		// value should be a JSON array of strings; arg is the separator
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err != nil {
			return value // not an array, pass through
		}
		return strings.Join(arr, arg)
	}
	return value
}

// parseLookupArg parses "k1=v1,k2=v2,default=dv" into (table, default).
// Special key "default" goes into the default; other keys go into the table.
// Both keys and values may be empty strings.
func parseLookupArg(arg string) (map[string]string, string) {
	table := map[string]string{}
	def := ""
	for _, pair := range strings.Split(arg, ",") {
		idx := strings.Index(pair, "=")
		if idx < 0 {
			continue
		}
		k, v := pair[:idx], pair[idx+1:]
		if k == "default" {
			def = v
			continue
		}
		table[k] = v
	}
	return table, def
}
```

确保 `rules.go` 顶部 import 包含 `"encoding/json"`、`"github.com/tidwall/gjson"`、`"strings"`。

- [ ] **Step 6: 运行 filter 测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestEvaluate_(Chained|Prefix|Lookup|Pluck|Join)" -v
```

Expected: 全部 PASS。

- [ ] **Step 7: 运行所有现有 Evaluate 测试，确认没回归**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestEvaluate" -v
```

Expected: 全部 PASS（包括之前已存在的 fallback、firstline、bool 等）。

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/bridge/rules.go internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): chained filters + prefix/lookup/pluck/join"
```

---

## Task 8: 升级 extract_rules.yaml 到 v3 + Rules struct + Lookup + PickTemplate

**目的**：把 YAML 重组为"事件名做一级 key"，定义 v3 加载逻辑。

**Files:**
- Modify: `internal/adapter/bridge/extract_rules.yaml`
- Modify: `internal/adapter/bridge/rules.go`
- Modify: `internal/adapter/bridge/rules_test.go`

- [ ] **Step 1: 先备份现有 YAML（仅本地，不 commit 备份）**

Run:
```bash
cp internal/adapter/bridge/extract_rules.yaml /tmp/extract_rules_v1.backup.yaml
```

- [ ] **Step 2: 重写 extract_rules.yaml 为 v3**

完整替换 `internal/adapter/bridge/extract_rules.yaml`：

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
    default:         "{tool_name}"

  PostToolUse:
    Bash:            "{$.tool_response.stdout|firstline} (exit={$.tool_response.exitCode|default:0})"
    Read:            "已读 {$.tool_response.file.numLines|default:?} 行"
    Edit:            "{$.tool_response.success|bool:✓写入,✗失败} {$.tool_response.filePath}"
    Write:           "{$.tool_response.success|bool:✓写入,✗失败} {$.tool_response.filePath}"
    AskUserQuestion: "{$.tool_response.answers.0 // $.tool_response}"
    Agent:           "{$.tool_response.0.text|firstpara}"
    default:         "{$.tool_response.stdout // $.tool_response.text // $.tool_response.message // $.tool_response.result // $.tool_response.summary // $.tool_response}"

  PermissionRequest: *claude_pre
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
```

- [ ] **Step 3: 写失败的 Rules 加载测试**

替换 `rules_test.go` 中的 `TestLoadRules_HasPreAndPost`（这个针对 v1，现在不适用）：

找到旧的 `TestLoadRules_HasPreAndPost` 函数并删除。在该位置追加：

```go
// ─── Rules v3 加载与 Lookup ───────────────────────────────────────────────────

func TestLoadRules_Version(t *testing.T) {
	rules, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if rules.Version != 3 {
		t.Errorf("Version = %d, want 3", rules.Version)
	}
}

func TestLoadRules_ClaudeBucketPopulated(t *testing.T) {
	rules, _ := LoadRules()
	if _, ok := rules.Claude["PreToolUse"]; !ok {
		t.Error("rules.Claude missing PreToolUse")
	}
	if _, ok := rules.Claude["Stop"]; !ok {
		t.Error("rules.Claude missing Stop")
	}
}

func TestLookup_ClaudePreToolUseIsMappingNode(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("claude-code", "PreToolUse")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.MappingNode {
		t.Errorf("Kind = %v, want MappingNode", n.Kind)
	}
}

func TestLookup_ClaudeStopIsScalarNode(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("claude-code", "Stop")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.ScalarNode {
		t.Errorf("Kind = %v, want ScalarNode", n.Kind)
	}
}

func TestLookup_PermissionRequestEqualsPreToolUse(t *testing.T) {
	rules, _ := LoadRules()
	pre, _ := rules.Lookup("claude-code", "PreToolUse")
	pr, _ := rules.Lookup("claude-code", "PermissionRequest")
	if pre.Kind != pr.Kind {
		t.Errorf("PreToolUse.Kind=%v, PermissionRequest.Kind=%v", pre.Kind, pr.Kind)
	}
	// MappingNode 的 Content 长度（key+value 对的数量）应该相等
	if len(pre.Content) != len(pr.Content) {
		t.Errorf("PreToolUse content len=%d, PermissionRequest content len=%d",
			len(pre.Content), len(pr.Content))
	}
}

func TestLookup_UnknownAgentReturnsFalse(t *testing.T) {
	rules, _ := LoadRules()
	if _, ok := rules.Lookup("nonsense", "Stop"); ok {
		t.Error("expected ok=false for unknown agent")
	}
}

func TestLookup_UnknownEventReturnsFalse(t *testing.T) {
	rules, _ := LoadRules()
	if _, ok := rules.Lookup("claude-code", "NoSuchEvent"); ok {
		t.Error("expected ok=false for unknown event")
	}
}

// ─── PickTemplate ─────────────────────────────────────────────────────────────

func TestPickTemplate_ScalarNode_ReturnsValue(t *testing.T) {
	rules, _ := LoadRules()
	n, _ := rules.Lookup("claude-code", "Stop")
	got := PickTemplate(n, "Bash")
	if got != "{$.last_assistant_message} // {$.stop_reason}" {
		t.Errorf("got %q", got)
	}
}

func TestPickTemplate_MappingNode_ToolHit(t *testing.T) {
	rules, _ := LoadRules()
	n, _ := rules.Lookup("claude-code", "PreToolUse")
	got := PickTemplate(n, "Bash")
	if got != "{$.tool_input.command}" {
		t.Errorf("got %q", got)
	}
}

func TestPickTemplate_MappingNode_FallsBackToDefault(t *testing.T) {
	rules, _ := LoadRules()
	n, _ := rules.Lookup("claude-code", "PreToolUse")
	got := PickTemplate(n, "UnknownTool")
	if got != "{tool_name}" {
		t.Errorf("got %q, want default template", got)
	}
}
```

并确保 `rules_test.go` 顶部 import 包含 `"gopkg.in/yaml.v3"`.

- [ ] **Step 4: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestLoadRules|TestLookup|TestPickTemplate" -v
```

Expected: 编译失败（Rules 结构和 Lookup/PickTemplate 都未升级）。

- [ ] **Step 5: 升级 Rules struct + 实现 Lookup + PickTemplate**

替换 `internal/adapter/bridge/rules.go` 中的 Rules 部分：

```go
// Rules 表示解析后的 extract_rules.yaml v3。
// 顶层是 agent → 事件名 → 模板节点的二级 map。
// 每个事件下挂的 yaml.Node 是 ScalarNode（单 string 模板）或
// MappingNode（per-tool 分桶 + default）。
type Rules struct {
	Version int                  `yaml:"version"`
	Claude  map[string]yaml.Node `yaml:"claude"`
	Codex   map[string]yaml.Node `yaml:"codex"`
}

// Lookup 返回 (agentID, eventType) 对应的 yaml.Node。
// agentID 必须是 entity.AgentClaudeCode / AgentCodex 中的常量值。
// 未命中返回 (zero, false)。
func (r *Rules) Lookup(agentID, eventType string) (yaml.Node, bool) {
	var bucket map[string]yaml.Node
	switch agentID {
	case "claude-code":
		bucket = r.Claude
	case "codex":
		bucket = r.Codex
	default:
		return yaml.Node{}, false
	}
	n, ok := bucket[eventType]
	return n, ok
}

// PickTemplate 在 MappingNode 中按 toolName 查模板，找不到回落 "default"。
// ScalarNode 直接返回 .Value，忽略 toolName。
// 既找不到 toolName 又没有 default 时返回空串。
func PickTemplate(n yaml.Node, toolName string) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.MappingNode:
		var defaultTmpl string
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == toolName {
				return v.Value
			}
			if k.Value == "default" {
				defaultTmpl = v.Value
			}
		}
		return defaultTmpl
	}
	return ""
}
```

把旧的 `Pre` 和 `Post` 字段删除（如果还存在）。如果旧的 LoadRules 函数依赖 `Pre`/`Post`，更新它（应该只是 `yaml.Unmarshal`，不依赖具体字段）。

- [ ] **Step 6: 运行 v3 测试**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestLoadRules|TestLookup|TestPickTemplate" -v
```

Expected: 全部 PASS。

- [ ] **Step 7: 处理仍引用旧 Rules.Pre/Post 的代码（中间态）**

此时 `extractor.go`、`extractor_test.go`、`cmd/bridge/main.go`、`cmd/bridge/main_test.go` 还在仓库里，引用已不存在的 `Rules.Pre` / `Rules.Post`。它们会在 Task 11 整体删除，但当前 Task 必须让 build 不破——给四个文件都加 `//go:build never` 临时排除：

```bash
for f in internal/adapter/bridge/extractor.go \
         internal/adapter/bridge/extractor_test.go \
         cmd/bridge/main.go \
         cmd/bridge/main_test.go; do
  if [ -f "$f" ]; then
    # 在文件首行插入 build 约束（前面要有空行才能让 Go 识别）
    printf '//go:build never\n\n%s\n' "$(cat "$f")" > "$f.tmp" && mv "$f.tmp" "$f"
  fi
done
```

注：每个文件首行变成 `//go:build never`，紧跟一个空行，再是原 `package bridge` / `package main`。这是 Go 标准 build constraint 写法，让该文件完全不参与编译。

- [ ] **Step 8: 再次确认编译与测试**

```bash
go build ./...
go test ./internal/adapter/bridge/ -v
```

Expected: 全部 PASS。带 `//go:build never` 的文件被忽略。

- [ ] **Step 9: Commit**

```bash
git add internal/adapter/bridge/extract_rules.yaml \
        internal/adapter/bridge/rules.go \
        internal/adapter/bridge/rules_test.go \
        internal/adapter/bridge/extractor.go \
        internal/adapter/bridge/extractor_test.go
git commit -m "feat(bridge): YAML v3 + Rules.Lookup + PickTemplate"
```

---

## Task 9: 实现 Render 单一入口（TDD）

**目的**：把 (agentID, eventType, toolName, payload) 串起来产出 (raw, content)。

**Files:**
- Modify: `internal/adapter/bridge/render.go`
- Create: `internal/adapter/bridge/render_test.go`

- [ ] **Step 1: 写失败的测试**

创建 `internal/adapter/bridge/render_test.go`：

```go
package bridge

import "testing"

func TestRender_PreToolUse_Bash(t *testing.T) {
	raw := []byte(`{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`)
	got, content := Render("claude-code", "PreToolUse", "Bash", raw)
	if got != "ls -la" {
		t.Errorf("raw=%q, want %q", got, "ls -la")
	}
	if content != "ls -la" {
		t.Errorf("content=%q", content)
	}
}

func TestRender_PostToolUse_Bash(t *testing.T) {
	raw := []byte(`{"tool_name":"Bash","tool_response":{"stdout":"line1\nline2","exitCode":0}}`)
	got, _ := Render("claude-code", "PostToolUse", "Bash", raw)
	if got != "line1 (exit=0)" {
		t.Errorf("got %q, want %q", got, "line1 (exit=0)")
	}
}

func TestRender_SessionStart(t *testing.T) {
	raw := []byte(`{"source":"startup"}`)
	got, _ := Render("claude-code", "SessionStart", "", raw)
	if got != "startup" {
		t.Errorf("got %q, want %q", got, "startup")
	}
}

func TestRender_PermissionDenied(t *testing.T) {
	raw := []byte(`{"tool_name":"Bash","denial_reason":"too risky"}`)
	got, _ := Render("claude-code", "PermissionDenied", "", raw)
	if got != "Bash: too risky" {
		t.Errorf("got %q, want %q", got, "Bash: too risky")
	}
}

func TestRender_PostToolBatch_PluckJoin(t *testing.T) {
	raw := []byte(`{"tool_calls":[{"tool_name":"Bash"},{"tool_name":"Read"},{"tool_name":"Grep"}]}`)
	got, _ := Render("claude-code", "PostToolBatch", "", raw)
	if got != "Bash, Read, Grep" {
		t.Errorf("got %q, want %q", got, "Bash, Read, Grep")
	}
}

func TestRender_Notification_Lookup(t *testing.T) {
	raw := []byte(`{"notification_type":"permission_prompt","message":"need approval"}`)
	got, _ := Render("claude-code", "Notification", "", raw)
	want := "[等授权] need approval"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_PermissionRequestSameAsPreToolUse(t *testing.T) {
	raw := []byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`)
	pre, _ := Render("claude-code", "PreToolUse", "Bash", raw)
	pr, _ := Render("claude-code", "PermissionRequest", "Bash", raw)
	if pre != pr {
		t.Errorf("PreToolUse=%q, PermissionRequest=%q (anchor not working?)", pre, pr)
	}
}

func TestRender_UnknownEventReturnsEmpty(t *testing.T) {
	got, content := Render("claude-code", "NoSuchEvent", "", []byte(`{}`))
	if got != "" || content != "" {
		t.Errorf("unknown event should return empty, got (%q,%q)", got, content)
	}
}

func TestRender_UnknownAgentReturnsEmpty(t *testing.T) {
	got, _ := Render("nonsense", "Stop", "", []byte(`{}`))
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestRender_TruncatesLongContent(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz"
	for len([]rune(long)) < 100 {
		long += "abcdefghij"
	}
	raw := []byte(`{"prompt":"` + long + `"}`)
	_, content := Render("claude-code", "UserPromptSubmit", "", raw)
	if len([]rune(content)) > 60+1 { // 60 + ellipsis
		t.Errorf("content not truncated: %d runes", len([]rune(content)))
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/ -run TestRender -v
```

Expected: 编译失败（Render 未定义）。

- [ ] **Step 3: 实现 Render**

替换 `internal/adapter/bridge/render.go` 全部内容：

```go
package bridge

import "strings"

const contentMaxRunes = 60

// Render 把 (agent, event, tool, payload) 解析为展示字符串。
// payload 必须是 hook stdin 的完整原文。
// 非工具事件传 toolName="" 即可，YAML 该事件下若是 ScalarNode 不走 toolName 分支。
func Render(agentID, eventType, toolName string, payload []byte) (raw, content string) {
	rules, err := LoadRules()
	if err != nil {
		return "", ""
	}
	node, ok := rules.Lookup(agentID, eventType)
	if !ok {
		return "", ""
	}
	template := PickTemplate(node, toolName)
	if template == "" {
		return "", ""
	}
	raw = strings.TrimSpace(Evaluate(template, payload, toolName))
	return raw, truncateRunes(raw, contentMaxRunes)
}

// truncateRunes returns at most n runes of s, appending '…' if truncated.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/ -run TestRender -v
```

Expected: 全部 PASS。**特别关注 `TestRender_PermissionRequestSameAsPreToolUse`**——这是 YAML anchor 的关键验证。

如果 anchor 测试失败：检查 yaml.Node 的实际结构。可能需要在 LoadRules 之后做一次"显式展开 alias"的步骤：

```go
// 在 LoadRules 末尾追加 alias 展开（如果 yaml.v3 没自动展开）
func (r *Rules) resolveAliases() {
	for _, bucket := range []map[string]yaml.Node{r.Claude, r.Codex} {
		for k, v := range bucket {
			if v.Kind == yaml.AliasNode && v.Alias != nil {
				bucket[k] = *v.Alias
			}
		}
	}
}
```

并在 LoadRules 返回前调用。但优先验证默认行为；如果默认 yaml.v3 已展开 alias，就不需要这一步。

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/render.go internal/adapter/bridge/render_test.go
git commit -m "feat(bridge): Render single entry for all event types"
```

---

## Task 10: 实现 cmd/pager-bridge/main.go（含 --print-hooks）

**目的**：把主流程串起来。`--print-hooks` 子命令是 installer 的协议接口。

**Files:**
- Modify: `cmd/pager-bridge/main.go`
- Create: `cmd/pager-bridge/main_test.go`

- [ ] **Step 1: 实现主流程**

替换 `cmd/pager-bridge/main.go` 全部内容：

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge/agents"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

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
		return // 未知 agent，silent
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

func printHooksJSON(agentLabel string) {
	agent, ok := agents.SelectByLabel(agentLabel)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown agent: %q\n", agentLabel)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(agent.Hooks()); err != nil {
		os.Exit(1)
	}
}

func detectTTY() string {
	out, err := exec.Command("tty").Output()
	if err == nil {
		t := strings.TrimSpace(string(out))
		if t != "" && t != "not a tty" {
			return t
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Step 2: 写 --print-hooks 测试**

创建 `cmd/pager-bridge/main_test.go`：

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildBinary compiles cmd/pager-bridge into a temp binary for use in tests.
// Returns the path; callers should remove it.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "pager-bridge")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestPrintHooks_CC(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--print-hooks", "--agent", "CC").Output()
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	var hooks []struct {
		Event   string `json:"event"`
		Matcher string `json:"matcher"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &hooks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(hooks) != 23 {
		t.Errorf("hooks count = %d, want 23", len(hooks))
	}
}

func TestPrintHooks_UnknownAgent(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--print-hooks", "--agent", "Nonsense")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err == nil {
		t.Error("expected non-zero exit for unknown agent")
	}
}
```

- [ ] **Step 3: 运行测试**

Run:
```bash
go test ./cmd/pager-bridge/ -v
```

Expected: PASS。

- [ ] **Step 4: 手动验证 --print-hooks**

Run:
```bash
go run ./cmd/pager-bridge --print-hooks --agent CC | head -10
```

Expected: 输出 JSON 数组，前几条是 PreToolUse / PostToolUse 等。

- [ ] **Step 5: Commit**

```bash
git add cmd/pager-bridge/
git commit -m "feat(bridge): cmd/pager-bridge main with --print-hooks"
```

---

## Task 11: 删除旧的 extractor.go / types.go（CCHookInput）/ cmd/bridge/

**目的**：清理被 Render + Envelope 替代的旧代码。

**Files:**
- Delete: `internal/adapter/bridge/extractor.go`
- Delete: `internal/adapter/bridge/extractor_test.go`
- Modify: `internal/adapter/bridge/types.go`（删除 CCHookInput 及其下所有 sub-input struct，保留 BashInput 等仅当被外部引用——通过 grep 确认没引用）
- Delete: `internal/adapter/bridge/types_test.go`
- Delete: `cmd/bridge/main.go`
- Delete: `cmd/bridge/main_test.go`

- [ ] **Step 1: 确认无外部引用 CCHookInput**

Run:
```bash
grep -rn "CCHookInput\|bridge\.BashInput\|bridge\.FileInput\|bridge\.GlobInput\|bridge\.GrepInput" \
  --include="*.go" \
  --exclude-dir=cmd/bridge \
  --exclude-dir=internal/adapter/bridge \
  .
```

Expected: 无输出（确认 CCHookInput 等没被外部包引用）。

如果有外部引用，**停下**——这意味着 spec 漏了一个修改点，需要先把那个引用迁移到 Envelope 才能删除。

- [ ] **Step 2: 删除 extractor.go 和 extractor_test.go**

```bash
git rm internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
```

- [ ] **Step 3: 清理 types.go（保留不被 CCHookInput 引用的部分；如果整个文件都是 CCHookInput 的衍生 struct，整个删除）**

打开 `internal/adapter/bridge/types.go` 看一下：

```bash
cat internal/adapter/bridge/types.go
```

如果整个文件都是 CCHookInput + BashInput/EditInput 等 sub-input struct（spec 第 4.2 节确认它们不再需要），整个文件删除：

```bash
git rm internal/adapter/bridge/types.go internal/adapter/bridge/types_test.go
```

- [ ] **Step 4: 删除旧 cmd/bridge**

```bash
git rm -r cmd/bridge
```

- [ ] **Step 5: 验证编译**

Run:
```bash
go build ./...
```

Expected: 无报错。

如果报错 "undefined: bridge.ExtractContent" 或类似，意味着还有引用旧函数的代码。可能在 `internal/adapter/httpapi/` 或别处——通过 grep 找出来：

```bash
grep -rn "bridge\.ExtractContent\|bridge\.ExtractEventContent\|bridge\.CCHookInput" --include="*.go" .
```

修掉这些引用（多半是测试，删除即可）。

- [ ] **Step 6: 运行所有测试**

Run:
```bash
make test
```

Expected: 全部 PASS。

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(bridge): remove ExtractContent/CCHookInput, replaced by Render+Envelope"
```

---

## Task 12: 实现 cmd/pager-installhooks（targets + upsert + main）

**目的**：把原 install-hooks.sh 的功能用 Go 重写，加上 append+幂等、--print-hooks 协议消费、binary 改名迁移。

**Files:**
- Modify: `cmd/pager-installhooks/main.go`
- Create: `cmd/pager-installhooks/targets.go`
- Create: `cmd/pager-installhooks/upsert.go`
- Create: `cmd/pager-installhooks/upsert_test.go`

- [ ] **Step 1: 实现 targets.go**

创建 `cmd/pager-installhooks/targets.go`：

```go
package main

// Target 描述一个 agent 对应的写入目标。
type Target struct {
	AgentLabel   string
	SettingsFile string // 路径，~ 会展开
	Format       string // "settings"（顶层 settings 对象）| "hooks-only"（顶层就是 hooks）
}

// targets 是 installer 的目标清单。
// 注意：CC-Internal 这里写 settings.local.json 是 spec 的选择；如 PR1 实施时
// 验证 CC-Internal loader 不识别 .local.json，则改回 settings.json。
var targets = []Target{
	{"CC", "~/.claude/settings.local.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.local.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.local.json", "settings"},
	// "Codex" 在 PR 2 阶段加进来
}

// expandPath 把 "~/foo" 展开为绝对路径。
func expandPath(p string) string {
	if len(p) >= 2 && p[:2] == "~/" {
		home, err := osUserHomeDir()
		if err != nil {
			return p
		}
		return home + p[1:]
	}
	return p
}
```

并在文件末尾加一个间接调用，方便测试 mock：

```go
import "os"

var osUserHomeDir = os.UserHomeDir
```

- [ ] **Step 2: 实现 upsert.go（含 isPagerHookGroup）**

创建 `cmd/pager-installhooks/upsert.go`：

```go
package main

import (
	"path/filepath"
	"strings"
)

const (
	legacyBinaryName  = "vibecoding-pager-cc-bridge"
	currentBinaryName = "pager-bridge"
)

// HookEntry 是 hooks JSON 中单个 hook 命令条目。
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	Async   bool   `json:"async,omitempty"`
}

// HookGroup 是同一个事件下挂的一组 hook。
type HookGroup struct {
	Hooks   []HookEntry `json:"hooks"`
	Matcher string      `json:"matcher,omitempty"`
}

// HookSpec 是从 pager-bridge --print-hooks 接收的事件描述。
type HookSpec struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
}

// isPagerHookGroup 判断 settings 中的某个 hook group 是否由 pager 写入。
// 用 basename 严格匹配，避免误判用户自己的 /custom/pager-bridge-helper 之类。
func isPagerHookGroup(g HookGroup) bool {
	for _, h := range g.Hooks {
		parts := strings.Fields(h.Command)
		if len(parts) == 0 {
			continue
		}
		base := filepath.Base(parts[0])
		if base == currentBinaryName || base == legacyBinaryName {
			return true
		}
	}
	return false
}

// buildHookGroup 构造一个新的 pager hook group。
func buildHookGroup(spec HookSpec, agentLabel, bridgePath string) HookGroup {
	cmd := bridgePath + " --event " + spec.Event + " --agent " + agentLabel
	g := HookGroup{
		Hooks: []HookEntry{
			{Type: "command", Command: cmd, Timeout: 5, Async: true},
		},
	}
	if spec.Matcher != "" {
		g.Matcher = spec.Matcher
	}
	return g
}

// upsertHooks 把 specs 的 pager 条目写到 settings.hooks 下，保留用户自定义条目。
// 同一事件下有 pager 旧条目（按 binary basename 识别）的，先剔除再 append。
// 入参 settings 可能是 nil-map；返回的 settings 永远非 nil。
func upsertHooks(settings map[string]any, agentLabel string, specs []HookSpec, bridgePath string) map[string]any {
	if settings == nil {
		settings = map[string]any{}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, spec := range specs {
		// 取 existing 数组（types 会是 []any）
		var existing []any
		if v, ok := hooks[spec.Event].([]any); ok {
			existing = v
		}
		// 1. 剔除所有 pager 旧 entry（含 legacy 名）
		filtered := make([]any, 0, len(existing))
		for _, item := range existing {
			g, err := decodeGroupFromAny(item)
			if err == nil && isPagerHookGroup(g) {
				continue
			}
			filtered = append(filtered, item)
		}
		// 2. append 当前 pager entry
		newGroup := buildHookGroup(spec, agentLabel, bridgePath)
		filtered = append(filtered, hookGroupToAny(newGroup))
		hooks[spec.Event] = filtered
	}
	settings["hooks"] = hooks
	return settings
}

// decodeGroupFromAny 把 JSON 反序列化产生的 map[string]any 转为 typed HookGroup。
// 失败时返回 zero 值和 error。
func decodeGroupFromAny(item any) (HookGroup, error) {
	m, ok := item.(map[string]any)
	if !ok {
		return HookGroup{}, errNotMap
	}
	var g HookGroup
	if hs, ok := m["hooks"].([]any); ok {
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			entry := HookEntry{
				Type:    asString(hm["type"]),
				Command: asString(hm["command"]),
				Timeout: asInt(hm["timeout"]),
				Async:   asBool(hm["async"]),
			}
			g.Hooks = append(g.Hooks, entry)
		}
	}
	g.Matcher = asString(m["matcher"])
	return g, nil
}

// hookGroupToAny 把 typed HookGroup 转为 map[string]any，便于写回 settings。
func hookGroupToAny(g HookGroup) map[string]any {
	hooks := make([]any, 0, len(g.Hooks))
	for _, h := range g.Hooks {
		entry := map[string]any{"type": h.Type, "command": h.Command}
		if h.Timeout > 0 {
			entry["timeout"] = h.Timeout
		}
		if h.Async {
			entry["async"] = h.Async
		}
		hooks = append(hooks, entry)
	}
	out := map[string]any{"hooks": hooks}
	if g.Matcher != "" {
		out["matcher"] = g.Matcher
	}
	return out
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
```

并在文件末尾或顶部添加 `errNotMap`：

```go
import "errors"

var errNotMap = errors.New("hook entry is not a map")
```

- [ ] **Step 3: 写 upsert 测试**

创建 `cmd/pager-installhooks/upsert_test.go`：

```go
package main

import (
	"reflect"
	"testing"
)

func TestUpsert_FreshSettings(t *testing.T) {
	settings := map[string]any{}
	specs := []HookSpec{
		{Event: "PreToolUse", Matcher: "*"},
		{Event: "Stop"},
	}
	out := upsertHooks(settings, "CC", specs, "/bin/pager-bridge")
	hooks := out["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("PreToolUse len = %d, want 1", len(pre))
	}
	g, _ := decodeGroupFromAny(pre[0])
	if g.Matcher != "*" {
		t.Errorf("matcher = %q, want *", g.Matcher)
	}
	if g.Hooks[0].Command != "/bin/pager-bridge --event PreToolUse --agent CC" {
		t.Errorf("command = %q", g.Hooks[0].Command)
	}
}

func TestUpsert_ReplacesLegacyEntry(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/old/path/vibecoding-pager-cc-bridge --event PreToolUse --agent CC",
						},
					},
					"matcher": "*",
				},
			},
		},
	}
	specs := []HookSpec{{Event: "PreToolUse", Matcher: "*"}}
	out := upsertHooks(settings, "CC", specs, "/new/pager-bridge")
	pre := out["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("PreToolUse len = %d, want 1 (legacy entry should be replaced)", len(pre))
	}
	g, _ := decodeGroupFromAny(pre[0])
	if g.Hooks[0].Command != "/new/pager-bridge --event PreToolUse --agent CC" {
		t.Errorf("command not replaced: %q", g.Hooks[0].Command)
	}
}

func TestUpsert_PreservesUserCustomEntry(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/usr/local/bin/my-custom-hook --foo",
						},
					},
					"matcher": "*",
				},
			},
		},
	}
	specs := []HookSpec{{Event: "PreToolUse", Matcher: "*"}}
	out := upsertHooks(settings, "CC", specs, "/bin/pager-bridge")
	pre := out["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 2 {
		t.Errorf("PreToolUse len = %d, want 2 (user + pager)", len(pre))
	}
	// 验证用户的条目还在
	foundCustom := false
	for _, item := range pre {
		g, _ := decodeGroupFromAny(item)
		if len(g.Hooks) > 0 && g.Hooks[0].Command == "/usr/local/bin/my-custom-hook --foo" {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Error("user's custom hook was lost")
	}
}

func TestUpsert_Idempotent(t *testing.T) {
	settings := map[string]any{}
	specs := []HookSpec{{Event: "Stop"}}
	out1 := upsertHooks(settings, "CC", specs, "/bin/pager-bridge")
	// 注意：upsertHooks 修改的 map 是 out1 自己；这里把它再传一次，模拟二次调用
	out2 := upsertHooks(out1, "CC", specs, "/bin/pager-bridge")
	// 二次后 Stop 下应该仍然只有一条 entry
	stop := out2["hooks"].(map[string]any)["Stop"].([]any)
	if len(stop) != 1 {
		t.Errorf("after 2nd run, Stop has %d entries, want 1", len(stop))
	}
}

func TestIsPagerHookGroup_CurrentBinary(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{{Command: "/usr/local/bin/pager-bridge --event Stop"}},
	}
	if !isPagerHookGroup(g) {
		t.Error("should detect pager-bridge")
	}
}

func TestIsPagerHookGroup_LegacyBinary(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{{Command: "/some/path/vibecoding-pager-cc-bridge --event Stop"}},
	}
	if !isPagerHookGroup(g) {
		t.Error("should detect legacy binary name")
	}
}

func TestIsPagerHookGroup_UserBinary(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{{Command: "/usr/local/bin/pager-bridge-helper --foo"}},
	}
	if isPagerHookGroup(g) {
		t.Error("should NOT match suffix-only matches")
	}
}

// 防止 reflect 包未使用
var _ = reflect.DeepEqual
```

- [ ] **Step 4: 运行 upsert 测试**

Run:
```bash
go test ./cmd/pager-installhooks/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 实现 main.go**

替换 `cmd/pager-installhooks/main.go` 全部内容：

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	var (
		agentFilter string
		dryRun      bool
		uninstall   bool
		bridgePath  string
		verbose     bool
	)
	pflag.StringVar(&agentFilter, "agent", "", "only install for this agent label (default: all)")
	pflag.BoolVar(&dryRun, "dry-run", false, "show what would change, do not write")
	pflag.BoolVar(&uninstall, "uninstall", false, "remove pager hook entries from all settings files")
	pflag.StringVar(&bridgePath, "bridge", defaultBridgePath(), "path to pager-bridge binary")
	pflag.BoolVar(&verbose, "verbose", false, "print per-target operation log")
	pflag.Parse()

	for _, t := range targets {
		if agentFilter != "" && t.AgentLabel != agentFilter {
			continue
		}
		if err := processTarget(t, bridgePath, uninstall, dryRun, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", t.AgentLabel, err)
			os.Exit(1)
		}
	}
}

func defaultBridgePath() string {
	self, err := os.Executable()
	if err != nil {
		return "pager-bridge"
	}
	return filepath.Join(filepath.Dir(self), "pager-bridge")
}

// processTarget 对一个 Target 完成 read-modify-write。
func processTarget(t Target, bridgePath string, uninstall, dryRun, verbose bool) error {
	path := expandPath(t.SettingsFile)

	// 取 specs（仅 install 时；uninstall 不需要）
	var specs []HookSpec
	if !uninstall {
		s, err := fetchSpecs(bridgePath, t.AgentLabel)
		if err != nil {
			return fmt.Errorf("fetch specs: %w", err)
		}
		specs = s
	}

	// 读 settings
	settings, err := readSettings(path, t.Format)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	// 修改
	if uninstall {
		settings = removeAllPagerHooks(settings)
	} else {
		settings = upsertHooks(settings, t.AgentLabel, specs, bridgePath)
	}

	// 写
	if dryRun {
		buf, _ := json.MarshalIndent(settings, "", "  ")
		fmt.Printf("[%s] would write to %s:\n%s\n", t.AgentLabel, path, string(buf))
		return nil
	}
	if err := writeSettings(path, t.Format, settings); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if verbose {
		op := "installed"
		if uninstall {
			op = "uninstalled"
		}
		fmt.Printf("[%s] %s %d events at %s\n", t.AgentLabel, op, len(specs), path)
	}
	return nil
}

// fetchSpecs 通过 pager-bridge --print-hooks 取事件清单。
func fetchSpecs(bridgePath, agentLabel string) ([]HookSpec, error) {
	cmd := exec.Command(bridgePath, "--print-hooks", "--agent", agentLabel)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var specs []HookSpec
	if err := json.Unmarshal(bytes.TrimSpace(out), &specs); err != nil {
		return nil, err
	}
	return specs, nil
}

// readSettings 读 path 下的 JSON 文件。
// Format=settings: 顶层是完整 settings 对象，hooks 是其下一个 key。
// Format=hooks-only: 顶层就是 hooks 对象本身——读出来后包装成 {hooks: ...}。
func readSettings(path, format string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}
	if format == "hooks-only" {
		return map[string]any{"hooks": top}, nil
	}
	return top, nil
}

// writeSettings 把 settings 写回 path（按 Format 决定顶层结构）。
func writeSettings(path, format string, settings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var top any = settings
	if format == "hooks-only" {
		top = settings["hooks"]
		if top == nil {
			top = map[string]any{}
		}
	}
	buf, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

// removeAllPagerHooks 把所有事件下的 pager 条目剔除。
// 用户自定义条目保留；若整个事件下没剩内容则把该 key 整体移除。
func removeAllPagerHooks(settings map[string]any) map[string]any {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return settings
	}
	for event, raw := range hooks {
		arr, _ := raw.([]any)
		filtered := make([]any, 0, len(arr))
		for _, item := range arr {
			g, err := decodeGroupFromAny(item)
			if err == nil && isPagerHookGroup(g) {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = filtered
		}
	}
	settings["hooks"] = hooks
	return settings
}

// 防止 strings 未使用警告
var _ = strings.Contains
```

- [ ] **Step 6: 验证编译**

Run:
```bash
go build ./cmd/pager-installhooks/
```

Expected: 无报错。

- [ ] **Step 7: 端到端 dry-run 验证**

先把 bridge build 出来：

```bash
mkdir -p bin
go build -o bin/pager-bridge ./cmd/pager-bridge
go build -o bin/pager-installhooks ./cmd/pager-installhooks
```

然后跑 dry-run：

```bash
./bin/pager-installhooks --bridge $(pwd)/bin/pager-bridge --agent CC --dry-run
```

Expected: 输出 "[CC] would write to /Users/.../.claude/settings.local.json:" 后跟 settings JSON，里面 hooks 段含 23 个事件条目。

- [ ] **Step 8: Commit**

```bash
git add cmd/pager-installhooks/
git commit -m "feat(installhooks): Go installer with append+idempotent upsert"
```

---

## Task 13: Makefile 改造

**目的**：把 BRIDGE_BIN 改名、删除 install-hooks 系列 target、加 installer target。

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: 看现有 Makefile 相关段落**

Run:
```bash
grep -n "BRIDGE_BIN\|install-bridge\|install-hooks\|bridge:" Makefile
```

记录行号。

- [ ] **Step 2: 改写 Makefile 关键段**

打开 `Makefile`，找到并替换：

旧（约第 16 行 `.PHONY:` 那行）：
```makefile
.PHONY: dev build run clean test bridge install-bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop
```

新：
```makefile
.PHONY: dev build run clean test bridge installer install-bridge frontend-deps frontend-build bindings icon lint stop
```

旧（BRIDGE_BIN 定义那行）：
```makefile
BRIDGE_BIN := $(PWD)/bin/vibecoding-pager-cc-bridge
```

新：
```makefile
BRIDGE_BIN    := $(PWD)/bin/pager-bridge
INSTALLER_BIN := $(PWD)/bin/pager-installhooks
```

旧（bridge target，可能像 `bridge: ; go build -o $(BRIDGE_BIN) ./cmd/bridge`）：
```makefile
bridge:
	@mkdir -p bin
	go build -o $(BRIDGE_BIN) ./cmd/bridge
```

新：
```makefile
bridge:
	@mkdir -p bin
	go build -o $(BRIDGE_BIN) ./cmd/pager-bridge

installer:
	@mkdir -p bin
	go build -o $(INSTALLER_BIN) ./cmd/pager-installhooks
```

旧（install-bridge target）：
```makefile
install-bridge: bridge
	./scripts/install-hooks.sh
	@echo "✓ Installed bridge..."
```

新：
```makefile
install-bridge: bridge installer
	$(INSTALLER_BIN) --bridge $(BRIDGE_BIN) --verbose
	@echo "✓ pager-bridge installed at $(BRIDGE_BIN)"
	@stat -f "  mtime: %Sm  sha: $$(shasum -a 256 $(BRIDGE_BIN) | cut -d' ' -f1)" $(BRIDGE_BIN)
```

删除整段（约第 58-68 行）：
```makefile
install-hooks: bridge
	./scripts/install-hooks.sh CC $(BRIDGE_BIN)

install-hooks-codebuddy: bridge
	./scripts/install-hooks.sh --agent CodeBuddy --settings_file ~/.codebuddy/settings.json --bridge $(BRIDGE_BIN)

install-hooks-cc-internal: bridge
	./scripts/install-hooks.sh --agent CC-Internal --settings_file ~/.claude-internal/settings.json --bridge $(BRIDGE_BIN)
```

更新 help target（第 137-140 行附近的 echo）：

旧：
```
@echo "  make install-hooks  Install CC hooks into ~/.claude/settings.json"
@echo "  make install-hooks-codebuddy  Install CodeBuddy hooks into ~/.codebuddy/settings.json"
```

新：
```
@echo "  make install-bridge       Build and install hooks for all agents"
@echo "  make installer            Build pager-installhooks (Go-based hook installer)"
```

- [ ] **Step 3: 验证 Makefile 语法**

Run:
```bash
make help | head -20
```

Expected: 输出 help 文本，没有 syntax error。

- [ ] **Step 4: 运行 make bridge 与 make installer**

Run:
```bash
make bridge
make installer
ls -la bin/
```

Expected: `bin/pager-bridge` 和 `bin/pager-installhooks` 都存在。

- [ ] **Step 5: 删除 scripts/install-hooks.sh**

```bash
git rm scripts/install-hooks.sh
```

- [ ] **Step 6: Commit**

```bash
git add Makefile
git commit -m "build(make): rename binary to pager-bridge, drop shell installer"
```

---

## Task 14: 迁移老用户 settings.json 中的遗留 entry

**目的**：spec §4.8 要求 installer 清理 `~/.claude/settings.json`（非 .local.json）和同类文件中可能存在的老 vibecoding-pager-cc-bridge 条目。否则升级后 CC 会同时加载旧+新两套 hook，要么双发要么调用不存在的旧二进制。

**Files:**
- Modify: `cmd/pager-installhooks/targets.go`
- Modify: `cmd/pager-installhooks/main.go`
- Modify: `cmd/pager-installhooks/upsert_test.go`

- [ ] **Step 1: 在 targets.go 添加 legacy 清理目标**

打开 `cmd/pager-installhooks/targets.go`，在 `targets` 切片下面追加：

```go
// legacyCleanupTargets 是早期版本写过的非 .local.json 路径。
// 升级流程会扫这些文件，把里面的 pager 条目清掉（用户其他配置原样保留），
// 防止 settings.json 与 settings.local.json 双发 hook。
var legacyCleanupTargets = []Target{
	{"CC", "~/.claude/settings.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.json", "settings"},
}
```

注意：Codex 不在 legacy 清理列表中——它原生只读 `~/.codex/hooks.json` 这一个文件，没有 settings.json 这种"主+local"分层。

- [ ] **Step 2: 在 upsert_test.go 加测试**

在 `upsert_test.go` 末尾追加：

```go
func TestRemoveAllPagerHooks_LegacyEntriesCleared(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/old/path/vibecoding-pager-cc-bridge --event PreToolUse --agent CC",
						},
					},
					"matcher": "*",
				},
			},
		},
		"permissions": map[string]any{
			"allow": []any{"Bash(ls:*)"},
		},
	}
	out := removeAllPagerHooks(settings)
	hooks := out["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("PreToolUse should be removed when only legacy pager entry existed")
	}
	// 用户其他配置（permissions）原样保留
	if _, ok := out["permissions"]; !ok {
		t.Error("permissions should be preserved")
	}
}

func TestRemoveAllPagerHooks_PreservesUserHooks(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				// pager 旧条目
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/old/vibecoding-pager-cc-bridge --event PreToolUse --agent CC",
						},
					},
				},
				// 用户自定义条目
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/usr/local/bin/my-tool",
						},
					},
				},
			},
		},
	}
	out := removeAllPagerHooks(settings)
	pre := out["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Errorf("PreToolUse len = %d, want 1 (user entry preserved)", len(pre))
	}
}
```

- [ ] **Step 3: 运行测试，确认通过**

Run:
```bash
go test ./cmd/pager-installhooks/ -run "TestRemoveAllPagerHooks" -v
```

Expected: 全部 PASS（`removeAllPagerHooks` 在 Task 12 已实现）。

- [ ] **Step 4: 在 main.go 加 legacy 清理 phase**

打开 `cmd/pager-installhooks/main.go`，在 `main()` 函数中——当前 `for _, t := range targets` 循环之**前**——加入：

```go
// Legacy cleanup phase: scan primary settings.json files for any pager
// entries left over from earlier versions (which wrote there directly),
// and remove them. User's non-pager entries are preserved.
// Skipped when --uninstall is set (the regular pass below will clean these too).
if !uninstall && !dryRun {
	for _, t := range legacyCleanupTargets {
		path := expandPath(t.SettingsFile)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		settings, err := readSettings(path, t.Format)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s legacy] read error: %v\n", t.AgentLabel, err)
			continue
		}
		cleaned := removeAllPagerHooks(settings)
		if err := writeSettings(path, t.Format, cleaned); err != nil {
			fmt.Fprintf(os.Stderr, "[%s legacy] write error: %v\n", t.AgentLabel, err)
			continue
		}
		if verbose {
			fmt.Printf("[%s legacy] cleaned pager entries in %s\n", t.AgentLabel, path)
		}
	}
}
```

注意：当 `--uninstall` 时跳过这个 phase（下面的常规循环走 `removeAllPagerHooks` 路径，会处理 .local.json；本 phase 只针对升级场景的 settings.json 主文件）。

但其实 `--uninstall` 也应该清理 legacy 路径——否则用户卸载 pager 后老条目还在 settings.json 里。所以再加一段：

```go
// Uninstall: also clean legacy paths
if uninstall {
	for _, t := range legacyCleanupTargets {
		path := expandPath(t.SettingsFile)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		settings, _ := readSettings(path, t.Format)
		settings = removeAllPagerHooks(settings)
		_ = writeSettings(path, t.Format, settings)
		if verbose {
			fmt.Printf("[%s legacy] uninstalled pager entries from %s\n", t.AgentLabel, path)
		}
	}
}
```

- [ ] **Step 5: 端到端 dry-run 验证**

先准备一个含老条目的 fixture：

```bash
mkdir -p /tmp/pager-legacy-test/.claude
cat > /tmp/pager-legacy-test/.claude/settings.json << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/old/path/vibecoding-pager-cc-bridge --event PreToolUse --agent CC"
          }
        ],
        "matcher": "*"
      }
    ]
  },
  "permissions": {
    "allow": ["Bash(ls:*)"]
  }
}
EOF
```

然后用临时 HOME 跑 installer：

```bash
HOME=/tmp/pager-legacy-test ./bin/pager-installhooks \
  --bridge $(pwd)/bin/pager-bridge --verbose
```

Expected: 输出包含 `[CC legacy] cleaned pager entries in /tmp/pager-legacy-test/.claude/settings.json`。

检查文件：

```bash
cat /tmp/pager-legacy-test/.claude/settings.json
```

Expected: `permissions` 段还在，`hooks.PreToolUse` 数组为空或键被整体移除。

```bash
cat /tmp/pager-legacy-test/.claude/settings.local.json
```

Expected: 这个新文件被创建，里面是 23 个新 pager-bridge 事件。

清理：
```bash
rm -rf /tmp/pager-legacy-test
```

- [ ] **Step 6: Commit**

```bash
git add cmd/pager-installhooks/
git commit -m "feat(installhooks): cleanup legacy pager entries in settings.json"
```

---

## Task 15: 端到端集成验证（CC/CodeBuddy 行为等效）

**目的**：在真实环境跑一遍 `make install-bridge`，验证 CC/CodeBuddy 触发的事件被 pager 接收的 ContentRaw 与重构前一致。

**Files:**
- 不修改代码，仅运行验证。

- [ ] **Step 1: 跑 make install-bridge**

```bash
make install-bridge
```

Expected: 三家 settings.local.json 都被写入，输出 `[CC] installed 23 events ...`、`[CC-Internal] installed 23 events ...`、`[CodeBuddy] installed 23 events ...`。

- [ ] **Step 2: 检查写入的文件结构**

```bash
cat ~/.claude/settings.local.json | python3 -m json.tool | head -30
```

Expected: 顶层有 `hooks` 字段，下面 23 个事件，每个事件下有 1 个 group（matcher 仅工具事件存在）。

- [ ] **Step 3: 启动 pager**

```bash
make dev &
sleep 5
```

或者如果有现成的 Pager.app，启动它。

- [ ] **Step 4: 在另一个 terminal 触发 CC 事件**

启动 Claude Code，让它执行一个简单的 `Bash` 命令（例如 "ls"）。

或者直接手动模拟 hook 输入：

```bash
echo '{"session_id":"test123","hook_event_name":"PreToolUse","cwd":"/tmp","tool_name":"Bash","tool_input":{"command":"ls"},"tool_use_id":"call1"}' | \
  ./bin/pager-bridge --event PreToolUse --agent CC
```

- [ ] **Step 5: 检查 pager 收到的 ContentRaw**

在 pager 数据库或 UI 检查最新一条记录。如果走 SQLite：

```bash
sqlite3 ~/Library/Application\ Support/VibeCoding\ Pager/pager.db \
  "SELECT event_type, content_raw, content FROM events ORDER BY id DESC LIMIT 3"
```

或者通过 dev 模式的 BaseDir：

```bash
sqlite3 .config/pager/pager.db \
  "SELECT event_type, content_raw, content FROM events ORDER BY id DESC LIMIT 3"
```

Expected: 看到 `PreToolUse | ls | ls` 这种条目。content_raw 应该和老代码输出一致（`{$.command}` → `{$.tool_input.command}` 的迁移不改变实际值）。

- [ ] **Step 6: 触发更多事件类型做对比**

至少触发：
- 一次 PostToolUse（Bash 命令完成）
- 一次 SessionStart（重启 CC）
- 一次 Stop（一轮对话结束）

每个事件的 ContentRaw 都应该与重构前等效。

- [ ] **Step 7: 关停 pager 并确认无 panic**

```bash
make stop  # 或者 Ctrl+C 关 dev 进程
```

- [ ] **Step 8: 清理可能存在的旧二进制**

```bash
ls -la bin/vibecoding-pager-cc-bridge 2>/dev/null && \
  echo "⚠️  Old binary still exists. Will be cleaned in PR3 docs."
```

- [ ] **Step 9: 整体 make test**

```bash
make test
```

Expected: 全部 PASS。

- [ ] **Step 10: Commit（无代码变化时跳过）**

如果中间有调整 fixture / 修补 bug，commit 它们：

```bash
git status
# 如有改动:
git add -A
git commit -m "test(bridge): integration verification adjustments"
```

---

## Task 16: PR1 合并前最后检查

**目的**：把 PR1 完整状态打包给 reviewer。

**Files:**
- 不修改代码。

- [ ] **Step 1: 检查 git 状态干净**

```bash
git status
```

Expected: working tree clean。

- [ ] **Step 2: 跑全套测试**

```bash
make lint
make test
```

Expected: 全部 PASS。

- [ ] **Step 3: 用 git log 看 PR1 的所有 commit**

```bash
git log --oneline main..HEAD
```

Expected: 看到 ~13-15 个 commit，每个职责单一。

- [ ] **Step 4: 总结 PR1 改动**

写一段总结到 `/tmp/pr1-summary.md`（不进 git）：

```bash
cat > /tmp/pr1-summary.md << 'EOF'
# PR1: Bridge Refactor

## 范围
- Agent 接口抽象（agents/agent.go + claude.go + registry.go）
- Envelope 替代 CCHookInput
- DSL v3：YAML 升 v3、Evaluate 链式 filter、4 个新 filter（prefix/lookup/pluck/join）
- Render 单一入口
- cmd/pager-bridge 替代 cmd/bridge
- cmd/pager-installhooks 替代 install-hooks.sh
- 二进制改名 vibecoding-pager-cc-bridge → pager-bridge

## 行为变化
- ❌ 无（CC/CodeBuddy 完全等效）

## 测试覆盖
- agents 单测 16 个
- rules 单测（v3 + 链式 + 4 filter） ~25 个
- render 单测 9 个
- installer upsert 单测 7 个
- pager-bridge --print-hooks 集成测试 2 个

## 后续 PR
- PR2 接 Codex
- PR3 文档收尾
EOF
echo "Summary written to /tmp/pr1-summary.md"
```

---

## Phase 1 Review Checkpoint ⚠️

**进入 Phase 2 前必须满足**：

- [ ] `make test` 全绿（含 lint）
- [ ] `make install-bridge` 跑成功，三家 settings.local.json 写入正确
- [ ] CC 实操触发 PreToolUse + PostToolUse + Stop，pager UI 显示与重构前一致
- [ ] CodeBuddy 实操（如果可用）同上
- [ ] CC-Internal 验证 settings.local.json 是否被识别——如果不识别，回退到 settings.json
- [ ] git log 干净，每个 commit 有意义
- [ ] 旧文件全部删除（cmd/bridge、scripts/install-hooks.sh、extractor.go、CCHookInput）
- [ ] 旧二进制 bin/vibecoding-pager-cc-bridge 已被 installer 检测到 settings.json 中残留并清理

**reviewer 应当**：
- 确认所有重构无业务行为变化
- 重点 review：YAML anchor 的 yaml.v3 处理、链式 filter 解析、isPagerHookGroup 的 basename 匹配严谨度
- 在合并前手动测试一次 `--dry-run` 与 `--uninstall`

通过后才能开始 Phase 2。

---

# Phase 2 — PR 2: Codex 接入

> **目标**：把 Codex 0.137 接入完整对等观测。所有 10 个 Codex 事件能进入 pager，session 状态正确。
>
> **依赖**：Phase 1 完整通过。
>
> **退出条件**：在 `/private/data/projects/github.com/sapaude/project_grace` 启动 codex，触发 10 类事件后 pager 列表上看到 codex 会话，状态机走完整周期。

---

## Task 17: 新增事件常量 + 状态机用例

**Files:**
- Modify: `internal/domain/entity/event.go`
- Modify: `internal/domain/session/status_test.go`

- [ ] **Step 1: 在 entity/event.go 添加常量**

打开 `internal/domain/entity/event.go`，在现有 Event 常量段（约第 11-26 行）末尾追加：

```go
const (
	// ... existing constants ...
	EventPostCompact   = "PostCompact"
	EventSubagentStart = "SubagentStart"
)
```

注意：如果常量是分多个 const 段定义的，找到包含 `EventPreCompact` 的那段，在它末尾追加这两个。

- [ ] **Step 2: 写状态机失败测试**

打开 `internal/domain/session/status_test.go`，在合适位置追加（参考已有 PreToolUse / SessionStart 的写法）：

```go
func TestDeriveStatus_PostCompact(t *testing.T) {
	got := DeriveStatus("PostCompact", "", "", false)
	if got != entity.StatusWorking {
		t.Errorf("PostCompact status = %v, want StatusWorking", got)
	}
}

func TestDeriveStatus_SubagentStart(t *testing.T) {
	got := DeriveStatus("SubagentStart", "", "", false)
	if got != entity.StatusWorking {
		t.Errorf("SubagentStart status = %v, want StatusWorking", got)
	}
}

func TestDeriveStatus_CodexSessionStart(t *testing.T) {
	got := DeriveStatus("SessionStart", "", "", false)
	if got != entity.StatusWorking {
		t.Errorf("SessionStart status = %v, want StatusWorking", got)
	}
}
```

- [ ] **Step 3: 运行测试**

Run:
```bash
go test ./internal/domain/session/ -run "TestDeriveStatus_(PostCompact|SubagentStart|CodexSessionStart)" -v
```

Expected: 全部 PASS（DeriveStatus 默认分支返回 StatusWorking，常量加进来后即生效）。

- [ ] **Step 4: Commit**

```bash
git add internal/domain/entity/event.go internal/domain/session/status_test.go
git commit -m "feat(domain): EventPostCompact and EventSubagentStart constants"
```

---

## Task 18: 实现 Codex Agent

**Files:**
- Create: `internal/adapter/bridge/agents/codex.go`
- Create: `internal/adapter/bridge/agents/codex_test.go`
- Modify: `internal/adapter/bridge/agents/registry.go`

- [ ] **Step 1: 写失败测试**

创建 `internal/adapter/bridge/agents/codex_test.go`：

```go
package agents

import (
	"testing"

	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

func TestCodex_ID(t *testing.T) {
	if got := (Codex{}).ID(); got != entity.AgentCodex {
		t.Errorf("ID() = %q, want %q", got, entity.AgentCodex)
	}
}

func TestCodex_Hooks_Count(t *testing.T) {
	if got := len(Codex{}.Hooks()); got != 10 {
		t.Errorf("len(Hooks()) = %d, want 10", got)
	}
}

func TestCodex_Hooks_NoSessionEnd(t *testing.T) {
	for _, h := range Codex{}.Hooks() {
		if h.Event == "SessionEnd" {
			t.Error("Codex should NOT have SessionEnd hook")
		}
	}
}

func TestCodex_ParseEnvelope_PreToolUseExecCommand(t *testing.T) {
	raw := []byte(`{
		"session_id": "codex-sess-1",
		"turn_id": "turn-42",
		"transcript_path": "/tmp/t.jsonl",
		"cwd": "/x",
		"hook_event_name": "PreToolUse",
		"tool_name": "exec_command",
		"tool_input": {"cmd":"ls","workdir":"/x","yield_time_ms":1000,"max_output_tokens":4000},
		"tool_use_id": "call_001"
	}`)
	env, err := Codex{}.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope error = %v", err)
	}
	if env.SessionID != "codex-sess-1" {
		t.Errorf("SessionID = %q", env.SessionID)
	}
	if env.TurnID != "turn-42" {
		t.Errorf("TurnID = %q, want turn-42", env.TurnID)
	}
	if env.ToolName != "exec_command" {
		t.Errorf("ToolName = %q", env.ToolName)
	}
}

func TestCodex_ParseEnvelope_SessionStartTrigger(t *testing.T) {
	raw := []byte(`{
		"session_id": "s",
		"hook_event_name": "SessionStart",
		"trigger": "startup"
	}`)
	env, err := Codex{}.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope error = %v", err)
	}
	if env.EventName != "SessionStart" {
		t.Errorf("EventName = %q", env.EventName)
	}
	// trigger 字段不在 Envelope 上；DSL 应该从 RawPayload 读
	// 验证 RawPayload 完整
	if len(env.RawPayload) == 0 {
		t.Error("RawPayload should be non-empty")
	}
}

func TestCodex_Accept_AlwaysTrue(t *testing.T) {
	if !(Codex{}).Accept("Notification", &bridge.Envelope{}) {
		t.Error("Codex.Accept should always be true")
	}
}
```

并确保 import 包含 `"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"`。

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -run TestCodex -v
```

Expected: 编译失败（Codex 未定义）。

- [ ] **Step 3: 实现 Codex agent**

创建 `internal/adapter/bridge/agents/codex.go`：

```go
package agents

import (
	"encoding/json"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

// Codex 是 Codex CLI 的 Agent 实现。
// 对应 Codex 0.137 的 hook schema：sessions 用 trigger 而非 source；
// 多一个 turn_id 字段；不发 SessionEnd / Notification / Elicitation 等
// 13 个 CC 专属事件。
type Codex struct{}

func (Codex) ID() string { return entity.AgentCodex }

func (Codex) ParseEnvelope(raw []byte) (*bridge.Envelope, error) {
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
	return &bridge.Envelope{
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

func (Codex) Accept(string, *bridge.Envelope) bool { return true }

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

- [ ] **Step 4: 注册 Codex 到 registry**

打开 `internal/adapter/bridge/agents/registry.go`，在 registry map 中追加：

```go
var registry = map[string]Agent{
	"CC":          ClaudeFamily{},
	"CC-Internal": ClaudeFamily{},
	"CodeBuddy":   ClaudeFamily{},
	"Codex":       Codex{},  // 新增
}
```

- [ ] **Step 5: 在 registry_test.go 加 Codex 用例**

打开 `internal/adapter/bridge/agents/registry_test.go`，在 `TestSelectByLabel_KnownLabels` 的 tests slice 里追加：

```go
{"Codex", true},
```

并新增一个测试：

```go
func TestSelectByLabel_CodexHasOwnID(t *testing.T) {
	codex, ok := SelectByLabel("Codex")
	if !ok || codex == nil {
		t.Fatal("Codex not registered")
	}
	cc, _ := SelectByLabel("CC")
	if codex.ID() == cc.ID() {
		t.Error("Codex should have a different ID from CC")
	}
}
```

- [ ] **Step 6: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/agents/ -v
```

Expected: 全部 PASS。

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/bridge/agents/
git commit -m "feat(agents): Codex implementation and registry entry"
```

---

## Task 19: 在 extract_rules.yaml 添加 codex 段

**Files:**
- Modify: `internal/adapter/bridge/extract_rules.yaml`
- Modify: `internal/adapter/bridge/render_test.go`

- [ ] **Step 1: 写失败的 Render 测试（codex 路径）**

在 `render_test.go` 末尾追加：

```go
func TestRender_Codex_PreToolUse_ExecCommand(t *testing.T) {
	raw := []byte(`{"tool_name":"exec_command","tool_input":{"cmd":"ls -la","workdir":"/x"}}`)
	got, _ := Render("codex", "PreToolUse", "exec_command", raw)
	if got != "ls -la" {
		t.Errorf("got %q, want %q", got, "ls -la")
	}
}

func TestRender_Codex_PostToolUse_ExecCommand_StringResponse(t *testing.T) {
	// Codex 的 tool_response 是单 string
	raw := []byte(`{"tool_name":"exec_command","tool_response":"Chunk ID: abc\nProcess exited with code 0\nOutput:\n/path"}`)
	got, _ := Render("codex", "PostToolUse", "exec_command", raw)
	if got != "Chunk ID: abc" {
		t.Errorf("got %q, want %q", got, "Chunk ID: abc")
	}
}

func TestRender_Codex_SessionStart_TriggerNotSource(t *testing.T) {
	raw := []byte(`{"trigger":"startup"}`)
	got, _ := Render("codex", "SessionStart", "", raw)
	if got != "startup" {
		t.Errorf("got %q, want %q", got, "startup")
	}
}

func TestRender_Codex_PermissionRequest_ReusesPreToolUse(t *testing.T) {
	raw := []byte(`{"tool_name":"exec_command","tool_input":{"cmd":"rm -rf /"}}`)
	got, _ := Render("codex", "PermissionRequest", "exec_command", raw)
	if got != "rm -rf /" {
		t.Errorf("got %q, want %q", got, "rm -rf /")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestRender_Codex" -v
```

Expected: 全部失败（codex 段尚未在 YAML 中）。

- [ ] **Step 3: 在 extract_rules.yaml 末尾追加 codex 段**

打开 `internal/adapter/bridge/extract_rules.yaml`，在文件末尾追加：

```yaml

codex:
  PreToolUse: &codex_pre
    exec_command: "{$.tool_input.cmd}"
    write_stdin:  "stdin→session {$.tool_input.session_id}"
    default:      "{tool_name}"

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

- [ ] **Step 4: 运行测试，确认通过**

Run:
```bash
go test ./internal/adapter/bridge/ -run "TestRender_Codex" -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 也跑一遍 codex 段的 Lookup 测试**

追加到 `rules_test.go`：

```go
func TestLookup_CodexPreToolUseIsMappingNode(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("codex", "PreToolUse")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.MappingNode {
		t.Errorf("Kind = %v", n.Kind)
	}
}

func TestLookup_CodexSessionStartIsScalar(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("codex", "SessionStart")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.ScalarNode {
		t.Errorf("Kind = %v", n.Kind)
	}
	if n.Value != "{$.trigger}" {
		t.Errorf("Value = %q", n.Value)
	}
}
```

Run:
```bash
go test ./internal/adapter/bridge/ -v
```

Expected: 全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/bridge/extract_rules.yaml \
        internal/adapter/bridge/render_test.go \
        internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): codex section in extract_rules.yaml"
```

---

## Task 20: installer 加 Codex Target

**Files:**
- Modify: `cmd/pager-installhooks/targets.go`

- [ ] **Step 1: 添加 Codex target**

打开 `cmd/pager-installhooks/targets.go`，在 `targets` 切片末尾追加：

```go
var targets = []Target{
	{"CC", "~/.claude/settings.local.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.local.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.local.json", "settings"},
	{"Codex", "~/.codex/hooks.json", "hooks-only"},  // 新增
}
```

- [ ] **Step 2: 验证 dry-run**

```bash
make installer
./bin/pager-installhooks --bridge $(pwd)/bin/pager-bridge --agent Codex --dry-run
```

Expected: 输出 `[Codex] would write to /Users/.../.codex/hooks.json:` 后跟一个**顶层就是 hooks 对象**的 JSON（没有外层的 `{"hooks":{...}}`）。

- [ ] **Step 3: Commit**

```bash
git add cmd/pager-installhooks/targets.go
git commit -m "feat(installhooks): add Codex target"
```

---

## Task 21: Codex 集成验证

**Files:**
- 不修改代码。

- [ ] **Step 1: 构建并安装**

```bash
make install-bridge
```

Expected: 输出包含 `[Codex] installed 10 events at /Users/.../.codex/hooks.json`。

- [ ] **Step 2: 检查 ~/.codex/hooks.json 内容**

```bash
cat ~/.codex/hooks.json | python3 -m json.tool | head -30
```

Expected: 顶层就是一个对象，下面是 10 个事件 key。每个事件下挂一个 group，含 `command: "/path/to/pager-bridge --event <X> --agent Codex"`。

- [ ] **Step 3: 启动 pager**

```bash
make dev &
sleep 5
```

- [ ] **Step 4: 在 project_grace 启动 codex**

```bash
cd /private/data/projects/github.com/sapaude/project_grace
codex
```

第一次启动会弹 **"Hooks need review"** 对话框。

选 **"Trust all and continue"**。

- [ ] **Step 5: 在 codex 里跑几个简单命令**

让 codex 做：
- 一次 `ls`（触发 PreToolUse + PostToolUse with exec_command）
- 一次问题（触发 UserPromptSubmit）
- 退出（触发 Stop）

- [ ] **Step 6: 检查 pager 收到 codex 事件**

```bash
sqlite3 .config/pager/pager.db \
  "SELECT agent, event_type, content_raw FROM events \
   WHERE agent='codex' \
   ORDER BY id DESC LIMIT 10"
```

Expected: 看到 codex 的 SessionStart / PreToolUse / PostToolUse / UserPromptSubmit / Stop 等事件，content_raw 与命令对应（"ls" 等）。

- [ ] **Step 7: 检查 pager UI**

打开 pager 界面，应该有一条 codex 会话的卡片，工具图标走完整周期（working → working → done）。

- [ ] **Step 8: 关停**

```bash
make stop
```

- [ ] **Step 9: 跑全套测试**

```bash
make test
```

Expected: 全部 PASS。

- [ ] **Step 10: Commit（如有 fixture / bug 修补）**

```bash
git status
# 如有改动:
git add -A
git commit -m "test(codex): integration verification"
```

---

## Phase 2 Review Checkpoint ⚠️

**进入 Phase 3 前必须满足**：

- [ ] `make test` 全绿
- [ ] `~/.codex/hooks.json` 写入正确（10 个事件，顶层结构正确）
- [ ] codex 在 project_grace 启动后，pager 收到 SessionStart/PreToolUse/PostToolUse/Stop 至少各一次
- [ ] pager UI 上 codex 会话状态机走通（working → done）
- [ ] codex 的 first-time trust 流程顺畅（无 hook trust 错误）
- [ ] 检查过 pager-installhooks --uninstall 也能清理 ~/.codex/hooks.json

通过后才能开始 Phase 3。

---

# Phase 3 — PR 3: 文档与边角

> **目标**：CLAUDE.md / PRD.md 反映 Codex 已接入的事实；旧二进制清理提示。

---

## Task 22: 更新 CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: 把 vibecoding-pager-cc-bridge 全部替换为 pager-bridge**

```bash
grep -n "vibecoding-pager-cc-bridge" CLAUDE.md
```

记录所有出现位置，逐一替换。

- [ ] **Step 2: "Bridge Binary Lifecycle" 节改写**

打开 CLAUDE.md，找到该节（约第 81 行附近），把所有 `vibecoding-pager-cc-bridge` 改成 `pager-bridge`，把所有 `~/.claude/settings.json` / `~/.codebuddy/settings.json` 在描述时分别改成 `~/.claude/settings.local.json` / `~/.codebuddy/settings.local.json`（保留对 settings.json 不被污染的解释）。

更新文字大概像：

```markdown
## Bridge Binary Lifecycle

After modifying `internal/adapter/bridge/*` or `cmd/pager-bridge/*`, run:

`make install-bridge`

This rebuilds `bin/pager-bridge` and runs `bin/pager-installhooks`,
which writes hooks into:

- `~/.claude/settings.local.json` (CC)
- `~/.claude-internal/settings.local.json` (CC-Internal)
- `~/.codebuddy/settings.local.json` (CodeBuddy)
- `~/.codex/hooks.json` (Codex)

The installer is **append + idempotent**: existing user-defined hooks in
the same event are preserved; pager's previous hook entries (including the
legacy `vibecoding-pager-cc-bridge` binary name) are detected by binary
basename and replaced.
```

- [ ] **Step 3: 新增 "Codex Hook Integration" 节**

在 "Bridge Binary Lifecycle" 节之后插入：

````markdown
## Codex Hook Integration

Codex 0.137+ has a hook system that pager接入via `~/.codex/hooks.json`.
Schema parity with Claude Code is high (same `session_id` / `tool_name` /
`tool_input` etc), with two key differences handled by the Codex agent:

- `SessionStart` carries `trigger` (startup/resume/clear/compact) instead of
  `source`
- All shell-like operations (Bash equivalents, file edits via `apply_patch`)
  go through `exec_command` rather than named tools

**First-run trust prompt:** After `make install-bridge`, the next time you
start codex it will prompt:

```
1 hook is new or changed.
Hooks need review
Hooks can run outside the sandbox after you trust them.
[Trust all and continue]  [Continue without trusting (hooks won't run)]
```

Pick **Trust all and continue**. Codex stores a hash of `hooks.json` and
won't prompt again until the file changes.

To bypass interactive prompt in CI scenarios, run codex with
`--dangerously-bypass-hook-trust`. Don't use this in regular dev work.

**Codex events covered (10):** PreToolUse, PostToolUse, PermissionRequest,
PreCompact, PostCompact, SessionStart, UserPromptSubmit, SubagentStart,
SubagentStop, Stop.
````

- [ ] **Step 4: 更新 "Key Files" 表**

找到 Key Files 章节（约第 200 行附近），更新这两行：

```markdown
| `cmd/pager-bridge/main.go` | pager-bridge entry point |
| `cmd/pager-installhooks/main.go` | hook installer (replaces install-hooks.sh) |
```

并在 Configuration Paths 节补充 settings.local.json 的角色：

```markdown
The installer writes pager's hooks into each agent's **`settings.local.json`**
(or `hooks.json` for Codex), keeping the user's primary `settings.json` untouched.
```

- [ ] **Step 5: 更新 Configuration Paths 表中的 mode 行**

找到表格中的 dev / production 行，把 `vibecoding-pager-cc-bridge` 改成 `pager-bridge`。

- [ ] **Step 6: 验证修改完整**

```bash
grep -n "vibecoding-pager-cc-bridge" CLAUDE.md
```

Expected: 只剩"legacy"字眼的描述（installer 兼容旧名），没有作为可执行路径出现。

- [ ] **Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): codex integration + pager-bridge naming + settings.local.json"
```

---

## Task 23: PRD.md 更新 + 旧二进制清理提示

**Files:**
- Modify: `docs/PRD.md`（如必要）

- [ ] **Step 1: 检查 PRD.md 是否提到 Codex 状态**

```bash
grep -n "[Cc]odex\|未接通\|reserved\|TBD" docs/PRD.md | head -20
```

如果 PRD 里写了 "Codex 字段保留未接通" 之类，改为 "Codex 完整接通（v1.5）"。

如果没有相关内容，跳过此 Step。

- [ ] **Step 2: 在 install-bridge 输出加旧二进制清理提示**

打开 `Makefile`，找到 install-bridge target：

```makefile
install-bridge: bridge installer
	$(INSTALLER_BIN) --bridge $(BRIDGE_BIN) --verbose
	@echo "✓ pager-bridge installed at $(BRIDGE_BIN)"
	@stat -f "  mtime: %Sm  sha: $$(shasum -a 256 $(BRIDGE_BIN) | cut -d' ' -f1)" $(BRIDGE_BIN)
	@if [ -f bin/vibecoding-pager-cc-bridge ]; then \
	  echo ""; \
	  echo "ℹ️  Legacy binary bin/vibecoding-pager-cc-bridge exists. Safe to remove with:"; \
	  echo "    rm bin/vibecoding-pager-cc-bridge"; \
	fi
```

- [ ] **Step 3: 验证**

```bash
make install-bridge
```

Expected: 末尾若 bin/vibecoding-pager-cc-bridge 还在，输出清理提示。

- [ ] **Step 4: 写一段 README 提示（可选）**

如果 README.md 提到了 vibecoding-pager-cc-bridge，更新它。否则跳过。

- [ ] **Step 5: Commit**

```bash
git add Makefile docs/PRD.md README.md
git commit -m "docs: PRD codex status + Makefile legacy binary cleanup hint"
```

---

## Phase 3 Review Checkpoint ⚠️

**Phase 3 完成条件**：

- [ ] `grep -n "vibecoding-pager-cc-bridge" CLAUDE.md` 只剩 legacy 描述
- [ ] CLAUDE.md 有 Codex Hook Integration 节
- [ ] make install-bridge 末尾若发现旧二进制会提示
- [ ] PRD.md 中 Codex 状态正确

---

# 完整退出标准

整个 plan 全部完成后：

- [ ] 三个 PR 全部 review 通过、合入 main
- [ ] `make test` / `make lint` 全绿
- [ ] 实操测试：CC + CodeBuddy + Codex 各跑一遍，session 状态机正确
- [ ] 用户的旧 `~/.claude/settings.json` 没被改（pager 现在写 .local.json）
- [ ] `make install-bridge` 幂等：跑两次结果相同
- [ ] `pager-installhooks --uninstall` 能清理掉所有 pager 痕迹

完成。
