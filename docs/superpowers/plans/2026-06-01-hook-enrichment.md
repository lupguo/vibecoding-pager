# Hook 体系丰富化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand Pager's hook system from 3 to 22 CC events, fix content extraction for all event types, and add a user-configurable notification settings panel.

**Architecture:** Bridge binary receives all CC hook events via stdin JSON, extracts meaningful content, determines attention level, and POSTs to the Pager HTTP server. The Settings panel allows users to configure which events trigger macOS system notifications.

**Tech Stack:** Go 1.25, Wails v3, React 18, TypeScript, Tailwind CSS, zustand, i18next

---

### Task 1: CCHookInput Struct Expansion

**Files:**
- Modify: `internal/adapter/bridge/types.go`

- [ ] **Step 1: Write test for new struct fields**

Create `internal/adapter/bridge/types_test.go`:

```go
package bridge

import (
	"encoding/json"
	"testing"
)

func TestCCHookInput_StopFields(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "Stop",
		"stop_reason": "end_turn",
		"last_assistant_message": "I completed the task"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.StopReason != "end_turn" {
		t.Errorf("StopReason: got %q, want %q", in.StopReason, "end_turn")
	}
	if in.LastAssistantMessage != "I completed the task" {
		t.Errorf("LastAssistantMessage: got %q", in.LastAssistantMessage)
	}
}

func TestCCHookInput_TaskCreatedFields(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "TaskCreated",
		"task_id": "t1",
		"task_title": "Fix login bug",
		"task_description": "The login form crashes on submit"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.TaskTitle != "Fix login bug" {
		t.Errorf("TaskTitle: got %q", in.TaskTitle)
	}
	if in.TaskDescription != "The login form crashes on submit" {
		t.Errorf("TaskDescription: got %q", in.TaskDescription)
	}
}

func TestCCHookInput_NotificationFields(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "Notification",
		"notification_type": "permission_prompt",
		"message": "Waiting for approval"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.NotificationType != "permission_prompt" {
		t.Errorf("NotificationType: got %q", in.NotificationType)
	}
	if in.Message != "Waiting for approval" {
		t.Errorf("Message: got %q", in.Message)
	}
}

func TestCCHookInput_StopFailureFields(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "StopFailure",
		"error_type": "rate_limit",
		"error_message": "Too many requests"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.ErrorType != "rate_limit" {
		t.Errorf("ErrorType: got %q", in.ErrorType)
	}
	if in.ErrorMessage != "Too many requests" {
		t.Errorf("ErrorMessage: got %q", in.ErrorMessage)
	}
}

func TestCCHookInput_SubagentFields(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "SubagentStart",
		"agent_id": "agent-123",
		"agent_type": "general-purpose"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.AgentID != "agent-123" {
		t.Errorf("AgentID: got %q", in.AgentID)
	}
	if in.AgentType != "general-purpose" {
		t.Errorf("AgentType: got %q", in.AgentType)
	}
}

func TestCCHookInput_ElicitationFields(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "Elicitation",
		"server_name": "my_mcp_server",
		"message": "Enter your API key"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.ServerName != "my_mcp_server" {
		t.Errorf("ServerName: got %q", in.ServerName)
	}
}

func TestCCHookInput_EffortLevel(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"effort": {"level": "high"}
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Effort == nil || in.Effort.Level != "high" {
		t.Errorf("Effort: got %+v", in.Effort)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/bridge/ -run TestCCHookInput -v`
Expected: FAIL — fields like `StopReason`, `LastAssistantMessage`, `TaskTitle` etc. don't exist yet.

- [ ] **Step 3: Implement the expanded CCHookInput struct**

Replace the entire `CCHookInput` struct and related types in `internal/adapter/bridge/types.go`:

```go
package bridge

import "encoding/json"

// CCHookInput is the JSON structure CC passes via stdin to hooks.
// All 22 registered hook events share this union struct.
// Different event types populate different fields; unused fields remain zero-value.
type CCHookInput struct {
	// ═══ Common (all events) ═══
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	CWD            string       `json:"cwd"`
	HookEventName  string       `json:"hook_event_name"`
	PermissionMode string       `json:"permission_mode"`
	Effort         *EffortLevel `json:"effort,omitempty"`

	// ═══ Session layer ═══
	Source       string `json:"source"`        // SessionStart: startup/resume/clear/compact
	Model        string `json:"model"`         // SessionStart
	SessionTitle string `json:"session_title"` // SessionStart

	// ═══ Turn layer ═══
	Prompt               string `json:"prompt"`                 // UserPromptSubmit
	ExpansionType        string `json:"expansion_type"`         // UserPromptExpansion: slash_command/mcp_prompt
	CommandName          string `json:"command_name"`           // UserPromptExpansion
	CommandArgs          string `json:"command_args"`           // UserPromptExpansion
	CommandSource        string `json:"command_source"`         // UserPromptExpansion: plugin/user/core
	StopReason           string `json:"stop_reason"`            // Stop: end_turn/max_tokens
	LastAssistantMessage string `json:"last_assistant_message"` // Stop: Claude's reply preview
	StopHookActive       bool   `json:"stop_hook_active"`       // Stop
	ErrorType            string `json:"error_type"`             // StopFailure: rate_limit/auth/billing...
	ErrorMessage         string `json:"error_message"`          // StopFailure

	// ═══ Tool layer ═══
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolUseID    string          `json:"tool_use_id"`
	ToolResult   json.RawMessage `json:"tool_result,omitempty"` // PostToolUse
	ToolError    string          `json:"error"`                 // PostToolUseFailure
	DenialReason string          `json:"denial_reason"`         // PermissionDenied
	ToolCalls    json.RawMessage `json:"tool_calls,omitempty"`  // PostToolBatch

	// ═══ Agent & Task layer ═══
	AgentID         string `json:"agent_id"`          // SubagentStart/Stop
	AgentType       string `json:"agent_type"`        // SubagentStart/Stop
	TaskID          string `json:"task_id"`           // TaskCreated/Completed
	TaskTitle       string `json:"task_title"`        // TaskCreated/Completed
	TaskDescription string `json:"task_description"`  // TaskCreated

	// ═══ Context layer ═══
	FilePath   string `json:"file_path"`   // InstructionsLoaded
	MemoryType string `json:"memory_type"` // InstructionsLoaded
	LoadReason string `json:"load_reason"` // InstructionsLoaded
	Trigger    string `json:"trigger"`     // PreCompact/PostCompact: manual/auto

	// ═══ MCP & UI layer ═══
	NotificationType string          `json:"notification_type"` // Notification: permission_prompt/idle_prompt
	Message          string          `json:"message"`           // Notification/Elicitation
	ServerName       string          `json:"server_name"`       // Elicitation
	Request          json.RawMessage `json:"request,omitempty"` // Elicitation
	UserResponse     string          `json:"user_response"`     // ElicitationResult
	MessageText      string          `json:"message_text"`      // MessageDisplay
}

// EffortLevel represents the effort level in CC's response.
type EffortLevel struct {
	Level string `json:"level"` // low/medium/high/xhigh/max
}

// BashInput is tool_input for Bash tool.
type BashInput struct {
	Command string `json:"command"`
}

// FileInput is tool_input for Edit/Write/Read tools.
type FileInput struct {
	FilePath string `json:"file_path"`
}

// GlobInput is tool_input for Glob tool.
type GlobInput struct {
	Pattern string `json:"pattern"`
}

// GrepInput is tool_input for Grep tool.
type GrepInput struct {
	Pattern string `json:"pattern"`
}

// WebFetchInput is tool_input for WebFetch tool.
type WebFetchInput struct {
	URL string `json:"url"`
}

// WebSearchInput is tool_input for WebSearch tool.
type WebSearchInput struct {
	Query string `json:"query"`
}

// TaskInput is tool_input for Task (subagent) tool.
type TaskInput struct {
	Description string `json:"description"`
}

// AskUserQuestionInput is tool_input for AskUserQuestion tool.
type AskUserQuestionInput struct {
	Questions []struct {
		Question string `json:"question"`
	} `json:"questions"`
}

// AgentInput is tool_input for Agent tool.
type AgentInput struct {
	Prompt      string `json:"prompt"`
	Description string `json:"description"`
}

// TaskCreateInput is tool_input for TaskCreate tool.
type TaskCreateInput struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	ActiveForm  string `json:"activeForm"`
}

// TaskUpdateInput is tool_input for TaskUpdate tool.
type TaskUpdateInput struct {
	TaskID  string `json:"taskId"`
	Status  string `json:"status"`
	Subject string `json:"subject"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/bridge/ -run TestCCHookInput -v`
Expected: All 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/types.go internal/adapter/bridge/types_test.go
git commit -m "feat(bridge): expand CCHookInput to cover all 22 hook event fields"
```

---

### Task 2: Content Extraction Fix — Tool Events

**Files:**
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`

- [ ] **Step 1: Write failing tests for TaskCreate/TaskUpdate extraction**

Add to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractContent_TaskCreate(t *testing.T) {
	input := json.RawMessage(`{"subject":"Fix login bug","description":"Form crashes on submit","activeForm":"Fixing login"}`)
	raw, content := ExtractContent("TaskCreate", input)
	if raw != "任务: Fix login bug" {
		t.Errorf("raw: got %q", raw)
	}
	if content != "任务: Fix login bug" {
		t.Errorf("content: got %q", content)
	}
}

func TestExtractContent_TaskUpdate(t *testing.T) {
	input := json.RawMessage(`{"taskId":"1","status":"completed","subject":"Fix login bug"}`)
	raw, content := ExtractContent("TaskUpdate", input)
	if raw != "任务: Fix login bug →completed" {
		t.Errorf("raw: got %q", raw)
	}
	if content != "任务: Fix login bug →completed" {
		t.Errorf("content: got %q", content)
	}
}

func TestExtractContent_TaskUpdate_StatusOnly(t *testing.T) {
	input := json.RawMessage(`{"taskId":"1","status":"in_progress"}`)
	raw, content := ExtractContent("TaskUpdate", input)
	if raw != "任务: →in_progress" {
		t.Errorf("raw: got %q", raw)
	}
	if content != "任务: →in_progress" {
		t.Errorf("content: got %q", content)
	}
}

func TestExtractContent_TaskGet(t *testing.T) {
	input := json.RawMessage(`{"taskId":"1"}`)
	raw, _ := ExtractContent("TaskGet", input)
	if raw != "查看任务" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractContent_TaskList(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("TaskList", input)
	if raw != "查看任务" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractContent_LSP(t *testing.T) {
	input := json.RawMessage(`{"operation":"goToDefinition","filePath":"main.go","line":10,"character":5}`)
	raw, _ := ExtractContent("LSP", input)
	if raw != "LSP goToDefinition: main.go" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractContent_NotebookEdit(t *testing.T) {
	input := json.RawMessage(`{"notebook_path":"/tmp/test.ipynb","new_source":"print('hi')"}`)
	raw, _ := ExtractContent("NotebookEdit", input)
	if raw != "编辑 /tmp/test.ipynb" {
		t.Errorf("raw: got %q", raw)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run "TestExtractContent_Task|TestExtractContent_LSP|TestExtractContent_Notebook" -v`
Expected: FAIL — cases for TaskCreate/TaskUpdate/LSP/NotebookEdit not implemented.

- [ ] **Step 3: Add tool extraction cases in extractor.go**

In `internal/adapter/bridge/extractor.go`, add these cases inside the `extractRaw` function's switch, before the MCP tools check:

```go
	case "TaskCreate":
		var in TaskCreateInput
		if unmarshal(&in) && in.Subject != "" {
			return "任务: " + in.Subject
		}
	case "TaskUpdate":
		var in TaskUpdateInput
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
		return "停止任务"
	case "NotebookEdit":
		var in struct {
			NotebookPath string `json:"notebook_path"`
		}
		if unmarshal(&in) && in.NotebookPath != "" {
			return "编辑 " + in.NotebookPath
		}
	case "LSP":
		var in struct {
			Operation string `json:"operation"`
			FilePath  string `json:"filePath"`
		}
		if unmarshal(&in) && in.Operation != "" {
			return "LSP " + in.Operation + ": " + in.FilePath
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -run "TestExtractContent_Task|TestExtractContent_LSP|TestExtractContent_Notebook" -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "feat(bridge): add content extraction for TaskCreate/Update/LSP/Notebook"
```

---

### Task 3: Content Extraction — Non-Tool Events (Stop, Notification, etc.)

**Files:**
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`

- [ ] **Step 1: Write failing tests for event content extraction**

Add to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractEventContent_Stop_WithMessage(t *testing.T) {
	in := &CCHookInput{
		LastAssistantMessage: "I've completed the refactoring. All tests pass.",
		StopReason:           "end_turn",
	}
	raw, content := ExtractEventContent("stop", in)
	if raw != "I've completed the refactoring. All tests pass." {
		t.Errorf("raw: got %q", raw)
	}
	// content should be truncated to 60 runes
	if len([]rune(content)) > 61 { // 60 + possible "…"
		t.Errorf("content too long: %d runes", len([]rune(content)))
	}
}

func TestExtractEventContent_Stop_NoMessage(t *testing.T) {
	in := &CCHookInput{StopReason: "end_turn"}
	raw, _ := ExtractEventContent("stop", in)
	if raw != "回复完成: end_turn" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Stop_Empty(t *testing.T) {
	in := &CCHookInput{}
	raw, _ := ExtractEventContent("stop", in)
	if raw != "回复完成" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_StopFailure(t *testing.T) {
	in := &CCHookInput{ErrorType: "rate_limit", ErrorMessage: "Too many requests"}
	raw, _ := ExtractEventContent("stop_failure", in)
	if raw != "rate_limit: Too many requests" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Notification(t *testing.T) {
	in := &CCHookInput{NotificationType: "permission_prompt", Message: "Approve file edit?"}
	raw, _ := ExtractEventContent("notification", in)
	if raw != "Approve file edit?" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Notification_NoMessage(t *testing.T) {
	in := &CCHookInput{NotificationType: "idle_prompt", Message: ""}
	raw, _ := ExtractEventContent("notification", in)
	if raw != "idle_prompt" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_TaskCreated(t *testing.T) {
	in := &CCHookInput{TaskTitle: "Implement auth flow", TaskDescription: "Add JWT tokens"}
	raw, _ := ExtractEventContent("task_created", in)
	if raw != "任务: Implement auth flow" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_SubagentStart(t *testing.T) {
	in := &CCHookInput{AgentType: "general-purpose"}
	raw, _ := ExtractEventContent("subagent_start", in)
	if raw != "子Agent启动: general-purpose" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_UserPromptExpansion(t *testing.T) {
	in := &CCHookInput{CommandName: "brainstorming", CommandArgs: "design auth"}
	raw, _ := ExtractEventContent("user_prompt_expansion", in)
	if raw != "/brainstorming design auth" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Elicitation(t *testing.T) {
	in := &CCHookInput{ServerName: "github_mcp", Message: "Enter token"}
	raw, _ := ExtractEventContent("elicitation", in)
	if raw != "MCP表单: github_mcp - Enter token" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_InstructionsLoaded(t *testing.T) {
	in := &CCHookInput{FilePath: "/project/CLAUDE.md", LoadReason: "session_start"}
	raw, _ := ExtractEventContent("instructions_loaded", in)
	if raw != "加载: /project/CLAUDE.md" {
		t.Errorf("raw: got %q", raw)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run "TestExtractEventContent_Stop|TestExtractEventContent_Notification|TestExtractEventContent_Task|TestExtractEventContent_Subagent|TestExtractEventContent_User|TestExtractEventContent_Elicit|TestExtractEventContent_Instruct" -v`
Expected: FAIL — new event types not handled, Stop doesn't read `LastAssistantMessage`.

- [ ] **Step 3: Rewrite ExtractEventContent with full event routing**

Replace the `ExtractEventContent` function in `internal/adapter/bridge/extractor.go`:

```go
// ExtractEventContent extracts content from all non-tool event types.
// Returns (contentRaw, content).
func ExtractEventContent(eventType string, in *CCHookInput) (contentRaw, content string) {
	switch eventType {
	// Session layer
	case "session_start":
		raw := "会话启动: " + in.Source
		return raw, truncateRunes(raw, contentMaxRunes)
	case "session_end":
		return "会话结束", "会话结束"

	// Turn layer
	case "user_prompt_submit":
		raw := in.Prompt
		if raw == "" {
			raw = "用户输入"
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	case "user_prompt_expansion":
		raw := "/" + in.CommandName
		if in.CommandArgs != "" {
			raw += " " + in.CommandArgs
		}
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

	// Agent & Task layer
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

	// Tool layer (non-standard events that still have tool info)
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

	// Context layer
	case "pre_compact":
		raw := "上下文压缩: " + in.Trigger
		return raw, raw
	case "post_compact":
		return "压缩完成", "压缩完成"
	case "instructions_loaded":
		raw := "加载: " + in.FilePath
		return raw, truncateRunes(raw, contentMaxRunes)

	// MCP & UI layer
	case "notification":
		raw := in.Message
		if raw == "" {
			raw = in.NotificationType
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	case "elicitation":
		raw := "MCP表单: " + in.ServerName + " - " + in.Message
		return raw, truncateRunes(raw, contentMaxRunes)
	case "elicitation_result":
		raw := "MCP响应: " + in.ServerName
		return raw, truncateRunes(raw, contentMaxRunes)
	case "message_display":
		raw := in.MessageText
		if raw == "" {
			raw = "消息输出"
		}
		return raw, truncateRunes(raw, contentMaxRunes)

	default:
		return eventType, eventType
	}
}
```

- [ ] **Step 4: Run all extractor tests**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: All PASS (including existing tests for session_start, user_prompt_submit, subagent_stop, pre_compact).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "feat(bridge): complete event content extraction for all 22 hook types"
```

---

### Task 4: Attention Level Logic Expansion

**Files:**
- Modify: `internal/adapter/bridge/attention.go`
- Modify: `internal/adapter/bridge/attention_test.go`

- [ ] **Step 1: Write failing tests for new event types**

Add to `internal/adapter/bridge/attention_test.go`:

```go
func TestDetermineAttentionLevel_StopFailure(t *testing.T) {
	level := DetermineAttentionLevel("stop_failure", "", "")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PermissionRequest(t *testing.T) {
	level := DetermineAttentionLevel("permission_request", "", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_Notification(t *testing.T) {
	level := DetermineAttentionLevel("notification", "", "")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_Elicitation(t *testing.T) {
	level := DetermineAttentionLevel("elicitation", "", "")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PostToolUseFailure(t *testing.T) {
	level := DetermineAttentionLevel("post_tool_use_failure", "", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PermissionDenied(t *testing.T) {
	level := DetermineAttentionLevel("permission_denied", "", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_SessionEnd(t *testing.T) {
	level := DetermineAttentionLevel("session_end", "", "")
	if level != entity.AttentionDone {
		t.Errorf("got %q, want %q", level, entity.AttentionDone)
	}
}

func TestDetermineAttentionLevel_SessionStart(t *testing.T) {
	level := DetermineAttentionLevel("session_start", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_PostToolBatch(t *testing.T) {
	level := DetermineAttentionLevel("post_tool_batch", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run "TestDetermineAttentionLevel_StopFailure|TestDetermineAttentionLevel_Permission|TestDetermineAttentionLevel_Notification|TestDetermineAttentionLevel_Elicitation|TestDetermineAttentionLevel_Session" -v`
Expected: FAIL — new event types not handled.

- [ ] **Step 3: Rewrite DetermineAttentionLevel**

Replace the function in `internal/adapter/bridge/attention.go`:

```go
package bridge

import "pager/internal/domain/entity"

// attentionTools are tools that always require user attention regardless of permission mode.
var attentionTools = map[string]bool{
	"AskUserQuestion": true,
}

// DetermineAttentionLevel decides the attention level based on event type, permission mode, and tool name.
func DetermineAttentionLevel(eventType, permissionMode, toolName string) string {
	switch eventType {
	// Done — session/agent ended
	case entity.EventStop, "session_end", entity.EventSubagentStop:
		return entity.AttentionDone

	// Attention — user action needed
	case "stop_failure", "permission_request", "notification", "elicitation",
		"post_tool_use_failure", "permission_denied":
		return entity.AttentionAttention

	// Tool — depends on mode and tool type
	case entity.EventPreToolUse:
		if attentionTools[toolName] {
			return entity.AttentionAttention
		}
		if permissionMode == "bypassPermissions" {
			return entity.AttentionRunning
		}
		return entity.AttentionAttention

	// Running — informational
	case entity.EventPostToolUse, "post_tool_batch":
		return entity.AttentionRunning

	// Default — running
	default:
		return entity.AttentionRunning
	}
}
```

- [ ] **Step 4: Run all attention tests**

Run: `go test ./internal/adapter/bridge/ -run TestDetermineAttentionLevel -v`
Expected: All PASS (existing + new).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/attention.go internal/adapter/bridge/attention_test.go
git commit -m "feat(bridge): expand attention logic for all 22 hook event types"
```

---

### Task 5: Bridge CLI Argument Refactor

**Files:**
- Modify: `cmd/bridge/main.go`

- [ ] **Step 1: Write test for new --event/--agent flag parsing**

Create `cmd/bridge/main_test.go`:

```go
package main

import "testing"

func TestParseArgs_NewFormat(t *testing.T) {
	eventType, agent := parseArgs([]string{"--event", "stop_failure", "--agent", "CC-INT"})
	if eventType != "stop_failure" {
		t.Errorf("eventType: got %q, want %q", eventType, "stop_failure")
	}
	if agent != "CC-INT" {
		t.Errorf("agent: got %q, want %q", agent, "CC-INT")
	}
}

func TestParseArgs_BackwardCompat(t *testing.T) {
	eventType, agent := parseArgs([]string{"pre_tool_use", "--agent", "CC"})
	if eventType != "pre_tool_use" {
		t.Errorf("eventType: got %q, want %q", eventType, "pre_tool_use")
	}
	if agent != "CC" {
		t.Errorf("agent: got %q, want %q", agent, "CC")
	}
}

func TestParseArgs_DefaultAgent(t *testing.T) {
	eventType, agent := parseArgs([]string{"--event", "notification"})
	if eventType != "notification" {
		t.Errorf("eventType: got %q", eventType)
	}
	if agent != "CC" {
		t.Errorf("agent: got %q, want default %q", agent, "CC")
	}
}

func TestParseArgs_Empty(t *testing.T) {
	eventType, agent := parseArgs([]string{})
	if eventType != "unknown" {
		t.Errorf("eventType: got %q, want %q", eventType, "unknown")
	}
	if agent != "CC" {
		t.Errorf("agent: got %q, want %q", agent, "CC")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/bridge/ -run TestParseArgs -v`
Expected: FAIL — `--event` flag not recognized.

- [ ] **Step 3: Update parseArgs in cmd/bridge/main.go**

Replace the `parseArgs` function:

```go
// parseArgs extracts event type and agent label from command-line args.
// Supports both formats:
//   New: pager-cc-bridge --event <type> --agent <label>
//   Old: pager-cc-bridge <type> [--agent <label>]
func parseArgs(args []string) (eventType, agentLabel string) {
	eventType = "unknown"
	agentLabel = "CC" // default

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--event" && i+1 < len(args):
			eventType = args[i+1]
			i++ // skip next
		case args[i] == "--agent" && i+1 < len(args):
			agentLabel = args[i+1]
			i++ // skip next
		case !strings.HasPrefix(args[i], "--") && eventType == "unknown":
			eventType = args[i]
		}
	}
	return
}
```

Also update `isToolEvent` in `cmd/bridge/main.go`:

```go
// isToolEvent returns true if the event type involves tool use with tool_input.
func isToolEvent(eventType string) bool {
	switch eventType {
	case entity.EventPreToolUse, entity.EventPostToolUse, "permission_request", "permission_denied":
		return true
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/bridge/ -run TestParseArgs -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/bridge/main.go cmd/bridge/main_test.go
git commit -m "feat(bridge): refactor CLI to --event/--agent flags with backward compat"
```

---

### Task 6: Notification Config — Backend

**Files:**
- Modify: `internal/infra/config/config.go`
- Modify: `internal/infra/config/config_test.go`
- Modify: `internal/adapter/notify/notify.go`
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Write test for new NotificationEvents config field**

Add to `internal/infra/config/config_test.go`:

```go
func TestDefaults_NotificationEvents(t *testing.T) {
	cfg := Defaults()
	if cfg.NotificationEvents == nil {
		t.Fatal("NotificationEvents should not be nil")
	}
	ccEvents, ok := cfg.NotificationEvents["CC"]
	if !ok {
		t.Fatal("expected CC key in NotificationEvents")
	}
	// Should contain stop_failure
	found := false
	for _, e := range ccEvents {
		if e == "stop_failure" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("CC events should include stop_failure, got: %v", ccEvents)
	}
}

func TestLoadFrom_WithNotificationEvents(t *testing.T) {
	tmp := t.TempDir() + "/settings.json"
	data := []byte(`{"notification_events":{"CC":["stop","notification"]}}`)
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NotificationEvents["CC"]) != 2 {
		t.Errorf("expected 2 events, got %d", len(cfg.NotificationEvents["CC"]))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/config/ -run "TestDefaults_Notification|TestLoadFrom_With" -v`
Expected: FAIL — `NotificationEvents` field doesn't exist.

- [ ] **Step 3: Add NotificationEvents to config.go**

Update `internal/infra/config/config.go`:

```go
type Settings struct {
	Language           string              `json:"language"`
	Theme              string              `json:"theme"`
	Opacity            int                 `json:"opacity"`
	HotkeyToggle       string              `json:"hotkey_toggle"`
	NotificationLevel  string              `json:"notification_level"`
	PopupWidth         int                 `json:"popup_width"`
	PopupPinned        bool                `json:"popup_pinned"`
	SessionLoadHours   int                 `json:"session_load_hours"`
	NotificationEvents map[string][]string `json:"notification_events"`
}

func Defaults() Settings {
	return Settings{
		Language:          "zh",
		Theme:             "system",
		Opacity:           75,
		HotkeyToggle:     "Alt+E",
		NotificationLevel: "attention_only",
		PopupWidth:        380,
		PopupPinned:       false,
		SessionLoadHours:  24,
		NotificationEvents: map[string][]string{
			"CC": {
				"stop_failure",
				"notification",
				"permission_request",
				"post_tool_use_failure",
				"elicitation",
			},
			"CC-INT": {
				"stop_failure",
				"notification",
			},
		},
	}
}
```

- [ ] **Step 4: Update notify.go to use NotificationEvents**

Add a new function in `internal/adapter/notify/notify.go`:

```go
// ShouldNotifyByConfig checks if an event should trigger notification
// based on the per-agent notification_events config.
func ShouldNotifyByConfig(e *entity.AgentEvent, notifEvents map[string][]string) bool {
	if notifEvents == nil {
		return false
	}
	events, ok := notifEvents[e.AgentLabel]
	if !ok {
		return false
	}
	for _, ev := range events {
		if ev == e.EventType {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Update app.go to use new notification logic**

In `internal/wails/app.go`, update the tracker callback (around line 64-67):

Replace:
```go
		if len(sessions) > 0 && sessions[0].LastEvent != nil {
			cfg, _ := config.LoadFrom(config.DefaultPath())
			notify.ShowFull(sessions[0].LastEvent, cfg.NotificationLevel, cfg.Language)
		}
```

With:
```go
		if len(sessions) > 0 && sessions[0].LastEvent != nil {
			cfg, _ := config.LoadFrom(config.DefaultPath())
			e := sessions[0].LastEvent
			if notify.ShouldNotifyByConfig(e, cfg.NotificationEvents) {
				notify.ShowFull(e, "all", cfg.Language)
			}
		}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/infra/config/ ./internal/adapter/notify/ -v`
Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/infra/config/config.go internal/infra/config/config_test.go internal/adapter/notify/notify.go internal/wails/app.go
git commit -m "feat: add per-agent notification_events config with backend support"
```

---

### Task 7: Settings UI — Notification Tab

**Files:**
- Create: `frontend/src/pages/settings/NotificationSettings.tsx`
- Modify: `frontend/src/pages/SettingsPanel.tsx`
- Modify: `frontend/src/pages/settings/GeneralSettings.tsx`
- Modify: `frontend/src/store/settings.ts`
- Modify: `frontend/src/i18n/locales/zh.json`
- Modify: `frontend/src/i18n/locales/en.json`

- [ ] **Step 1: Update settings store to include notification_events**

In `frontend/src/store/settings.ts`, update the `Settings` interface:

```typescript
export interface Settings {
  language: string
  theme: string
  opacity: number
  hotkey_toggle: string
  notification_level: string
  popup_width: number
  popup_pinned: boolean
  session_load_hours: number
  notification_events: Record<string, string[]>
}

const DEFAULT_SETTINGS: Settings = {
  language: 'zh',
  theme: 'system',
  opacity: 75,
  hotkey_toggle: 'Alt+E',
  notification_level: 'attention_only',
  popup_width: 380,
  popup_pinned: false,
  session_load_hours: 24,
  notification_events: {
    CC: ['stop_failure', 'notification', 'permission_request', 'post_tool_use_failure', 'elicitation'],
    'CC-INT': ['stop_failure', 'notification'],
  },
}
```

- [ ] **Step 2: Update i18n locale files**

Add to `frontend/src/i18n/locales/zh.json`:

```json
{
  "nav": {
    "general": "基本设置",
    "notifications": "通知",
    "about": "关于"
  },
  "notifications": {
    "title": "事件通知",
    "description": "开启的事件将触发 macOS 系统通知。所有事件均在会话面板中展示。",
    "presetRecommend": "推荐",
    "presetCritical": "仅关键",
    "presetAll": "全通知",
    "presetNone": "全关闭",
    "connected": "已接入",
    "notConnected": "未接入",
    "footer": "当前已开启 {{count}} 项通知 · 共 {{total}} 项事件"
  }
}
```

Add to `frontend/src/i18n/locales/en.json`:

```json
{
  "nav": {
    "general": "General",
    "notifications": "Notifications",
    "about": "About"
  },
  "notifications": {
    "title": "Event Notifications",
    "description": "Enabled events will trigger macOS system notifications. All events are shown in the session panel.",
    "presetRecommend": "Recommended",
    "presetCritical": "Critical Only",
    "presetAll": "All",
    "presetNone": "None",
    "connected": "Connected",
    "notConnected": "Not connected",
    "footer": "{{count}} notifications enabled · {{total}} total events"
  }
}
```

- [ ] **Step 3: Create NotificationSettings.tsx**

Create `frontend/src/pages/settings/NotificationSettings.tsx`:

```tsx
import { useTranslation } from 'react-i18next'
import { useSettingsStore } from '../../store/settings'

// All hook events grouped by domain
const EVENT_GROUPS = [
  {
    domain: 'SESSION',
    events: [
      { key: 'session_start', label: '会话启动', labelEn: 'Session Start' },
      { key: 'session_end', label: '会话结束', labelEn: 'Session End' },
    ],
  },
  {
    domain: 'TURN',
    events: [
      { key: 'user_prompt_submit', label: '用户输入', labelEn: 'User Input' },
      { key: 'stop', label: 'Agent 回复完成', labelEn: 'Agent Reply' },
      { key: 'stop_failure', label: 'Agent 错误', labelEn: 'Agent Error', desc: 'rate_limit / auth / billing' },
      { key: 'user_prompt_expansion', label: '命令展开', labelEn: 'Command Expansion' },
    ],
  },
  {
    domain: 'TOOL',
    events: [
      { key: 'pre_tool_use', label: '工具等待执行', labelEn: 'Tool Pending' },
      { key: 'post_tool_use', label: '工具执行完成', labelEn: 'Tool Done' },
      { key: 'post_tool_use_failure', label: '工具执行失败', labelEn: 'Tool Failed' },
      { key: 'permission_request', label: '权限请求', labelEn: 'Permission Request' },
      { key: 'permission_denied', label: '权限被拒', labelEn: 'Permission Denied' },
      { key: 'post_tool_batch', label: '批次完成', labelEn: 'Batch Done' },
    ],
  },
  {
    domain: 'AGENT & TASK',
    events: [
      { key: 'subagent_start', label: '子Agent 启动', labelEn: 'Subagent Start' },
      { key: 'subagent_stop', label: '子Agent 结束', labelEn: 'Subagent Stop' },
      { key: 'task_created', label: '任务创建', labelEn: 'Task Created' },
      { key: 'task_completed', label: '任务完成', labelEn: 'Task Completed' },
    ],
  },
  {
    domain: 'SYSTEM & MCP',
    events: [
      { key: 'notification', label: '系统通知', labelEn: 'Notification', desc: 'permission_prompt / idle_prompt' },
      { key: 'elicitation', label: 'MCP 表单请求', labelEn: 'MCP Elicitation' },
      { key: 'instructions_loaded', label: '指令加载', labelEn: 'Instructions Loaded' },
      { key: 'pre_compact', label: '上下文压缩', labelEn: 'Context Compact' },
      { key: 'message_display', label: '消息输出', labelEn: 'Message Display' },
    ],
  },
]

const PRESETS: Record<string, string[]> = {
  recommend: ['stop_failure', 'notification', 'permission_request', 'post_tool_use_failure', 'elicitation'],
  critical: ['stop_failure', 'permission_request'],
  all: EVENT_GROUPS.flatMap((g) => g.events.map((e) => e.key)),
  none: [],
}

const AGENTS = [
  { id: 'CC', color: '#007aff' },
  { id: 'CC-INT', color: '#bf5af2' },
  { id: 'Codex', color: '#ff9f0a' },
  { id: 'Gemini', color: '#30d158' },
]

export default function NotificationSettings() {
  const { t, i18n } = useTranslation()
  const isZh = i18n.language === 'zh'
  const settings = useSettingsStore((s) => s.settings)
  const updateSettings = useSettingsStore((s) => s.updateSettings)

  const activeAgent = 'CC' // TODO: make selectable
  const notifEvents = settings.notification_events || {}
  const agentEvents = notifEvents[activeAgent] || []

  const isEnabled = (eventKey: string) => agentEvents.includes(eventKey)
  const enabledCount = agentEvents.length
  const totalCount = EVENT_GROUPS.reduce((sum, g) => sum + g.events.length, 0)

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

  const connectedAgents = Object.keys(notifEvents)

  return (
    <div>
      <h3 className="text-[17px] font-semibold text-[--pager-text-primary] mb-4">
        {t('notifications.title')}
      </h3>

      {/* Agent Tabs */}
      <div className="flex gap-1 p-[3px] bg-[rgba(0,0,0,0.04)] dark:bg-[rgba(255,255,255,0.04)] rounded-[9px] mb-3">
        {AGENTS.map((agent) => {
          const connected = connectedAgents.includes(agent.id)
          return (
            <button
              key={agent.id}
              className={`flex-1 flex items-center justify-center gap-[4px] py-[5px] px-[6px] rounded-[7px] text-[11px] font-medium transition-all
                ${activeAgent === agent.id
                  ? 'bg-white dark:bg-[rgba(255,255,255,0.08)] shadow-sm font-semibold text-[--pager-text-primary]'
                  : connected
                    ? 'text-[--pager-text-muted]'
                    : 'text-[--pager-text-muted] opacity-40'
                }`}
            >
              <span
                className="w-[6px] h-[6px] rounded-full"
                style={{ background: agent.color }}
              />
              {agent.id}
              <span className={`text-[8px] px-[3px] py-[1px] rounded-[3px] font-semibold ${
                connected
                  ? 'bg-[rgba(48,209,88,0.12)] text-[#30d158]'
                  : 'bg-[rgba(0,0,0,0.04)] dark:bg-[rgba(255,255,255,0.04)] text-[--pager-text-muted]'
              }`}>
                {connected ? (isZh ? '已接入' : 'On') : (isZh ? '未接入' : 'Off')}
              </span>
            </button>
          )
        })}
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
```

- [ ] **Step 4: Update SettingsPanel.tsx — add Notifications tab, remove section label**

Replace `frontend/src/pages/SettingsPanel.tsx`:

```tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { useSettingsStore } from '../store/settings'
import GeneralSettings from './settings/GeneralSettings'
import NotificationSettings from './settings/NotificationSettings'
import DataSettings from './settings/DataSettings'
import AboutSettings from './settings/AboutSettings'

type Page = 'general' | 'notifications' | 'data' | 'about'

export default function SettingsPanel() {
  const { t, i18n } = useTranslation()
  const [page, setPage] = useState<Page>('general')
  const settings = useSettingsStore((s) => s.settings)
  const isZh = i18n.language === 'zh'

  const navItems: { id: Page; icon: string; label: string }[] = [
    { id: 'general', icon: '⚙️', label: t('nav.general') },
    { id: 'notifications', icon: '🔔', label: isZh ? '通知' : 'Notifications' },
    { id: 'data', icon: '💾', label: isZh ? '数据' : 'Data' },
    { id: 'about', icon: 'ℹ️', label: t('nav.about') },
  ]

  const handleClose = async () => {
    try {
      const { Hide } = await import('../../bindings/pager/internal/wails/windowbinding.js')
      await Hide()
    } catch {
      // Fallback: use window.close or just ignore
    }
  }

  return (
    <div className="w-full h-screen flex flex-col bg-[#f5f5f7] dark:bg-[#1e1e20] text-[--pager-text]"
         style={{ fontFamily: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif" }}>
      {/* Custom titlebar — draggable */}
      <div
        className="h-[38px] flex items-center justify-between px-3 border-b border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.06)] bg-[rgba(246,246,248,0.98)] dark:bg-[rgba(30,30,32,0.98)] shrink-0 select-none cursor-default"
        style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
      >
        <span className="text-[12px] font-semibold text-[--pager-text-primary] tracking-tight">
          {isZh ? '设置' : 'Settings'}
        </span>
        <button
          onClick={handleClose}
          className="w-5 h-5 flex items-center justify-center rounded hover:bg-[rgba(0,0,0,0.06)] dark:hover:bg-[rgba(255,255,255,0.08)] transition-colors"
          style={{ '--wails-draggable': 'none' } as React.CSSProperties}
          title="Close"
        >
          <X size={12} className="text-[--pager-text-muted]" />
        </button>
      </div>

      {/* Content area */}
      <div className="flex flex-1 overflow-hidden">
        <div className="w-[180px] min-w-[180px] bg-[rgba(246,246,248,0.98)] dark:bg-[rgba(30,30,32,0.98)] border-r border-[rgba(0,0,0,0.08)] dark:border-[rgba(255,255,255,0.06)] p-3 pt-2">
          {navItems.map((item) => (
            <button
              key={item.id}
              onClick={() => setPage(item.id)}
              className={`w-full text-left px-3 py-1.5 my-0.5 rounded-md text-[13px] flex items-center gap-2 transition-colors
                ${page === item.id
                  ? 'bg-[#007aff] text-white font-medium'
                  : 'text-[rgba(0,0,0,0.75)] dark:text-[rgba(255,255,255,0.6)] hover:bg-[rgba(0,0,0,0.04)] dark:hover:bg-[rgba(255,255,255,0.05)]'
                }`}
            >
              <span className="text-[14px]">{item.icon}</span>
              {item.label}
            </button>
          ))}
        </div>

        <div className="flex-1 p-5 overflow-y-auto bg-[#f5f5f7] dark:bg-[#2a2a2c]">
          {page === 'general' && <GeneralSettings />}
          {page === 'notifications' && <NotificationSettings />}
          {page === 'data' && <DataSettings />}
          {page === 'about' && <AboutSettings />}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Remove notification section from GeneralSettings.tsx**

In `frontend/src/pages/settings/GeneralSettings.tsx`, remove the entire `<Section label={t('general.notifications')}>...</Section>` block (lines 62-73 approximately).

- [ ] **Step 6: Run frontend type check**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/settings/NotificationSettings.tsx frontend/src/pages/SettingsPanel.tsx frontend/src/pages/settings/GeneralSettings.tsx frontend/src/store/settings.ts frontend/src/i18n/locales/zh.json frontend/src/i18n/locales/en.json
git commit -m "feat(ui): add Notifications tab to Settings panel"
```

---

### Task 8: Install Hooks Script + Build

**Files:**
- Create: `scripts/install-hooks.sh`
- Modify: `Makefile`

- [ ] **Step 1: Create install-hooks.sh**

Create `scripts/install-hooks.sh`:

```bash
#!/bin/bash
# Install Pager hook configuration into Claude Code settings.
# Usage: ./scripts/install-hooks.sh [AGENT_LABEL] [BRIDGE_PATH]
#
# AGENT_LABEL defaults to "CC"
# BRIDGE_PATH defaults to the bin/pager-cc-bridge relative to this script

set -euo pipefail

AGENT="${1:-CC}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BRIDGE_PATH="${2:-$SCRIPT_DIR/../bin/pager-cc-bridge}"
SETTINGS_FILE="$HOME/.claude/settings.json"

if [ ! -f "$BRIDGE_PATH" ]; then
  echo "Error: bridge binary not found at $BRIDGE_PATH"
  echo "Run 'make bridge' first."
  exit 1
fi

# Ensure settings directory exists
mkdir -p "$(dirname "$SETTINGS_FILE")"

# Create settings file if it doesn't exist
if [ ! -f "$SETTINGS_FILE" ]; then
  echo '{}' > "$SETTINGS_FILE"
fi

# All 22 hook events to register
EVENTS=(
  "SessionStart:session_start"
  "SessionEnd:session_end"
  "UserPromptSubmit:user_prompt_submit"
  "UserPromptExpansion:user_prompt_expansion"
  "Stop:stop"
  "StopFailure:stop_failure"
  "PreToolUse:pre_tool_use:*"
  "PostToolUse:post_tool_use:*"
  "PostToolUseFailure:post_tool_use_failure:*"
  "PostToolBatch:post_tool_batch"
  "PermissionRequest:permission_request:*"
  "PermissionDenied:permission_denied:*"
  "SubagentStart:subagent_start"
  "SubagentStop:subagent_stop"
  "TaskCreated:task_created"
  "TaskCompleted:task_completed"
  "Notification:notification"
  "PreCompact:pre_compact"
  "PostCompact:post_compact"
  "InstructionsLoaded:instructions_loaded"
  "Elicitation:elicitation"
  "MessageDisplay:message_display"
)

# Build the hooks JSON using python3 for reliable JSON manipulation
python3 << PYTHON
import json, sys

settings_file = "$SETTINGS_FILE"
bridge_path = "$BRIDGE_PATH"
agent = "$AGENT"

events = [
$(for e in "${EVENTS[@]}"; do
  IFS=':' read -r hook_name event_name matcher <<< "$e"
  if [ -n "$matcher" ]; then
    echo "    ('$hook_name', '$event_name', '$matcher'),"
  else
    echo "    ('$hook_name', '$event_name', None),"
  fi
done)
]

with open(settings_file, 'r') as f:
    settings = json.load(f)

if 'hooks' not in settings:
    settings['hooks'] = {}

for hook_name, event_name, matcher in events:
    hook_entry = {
        "type": "command",
        "command": f"{bridge_path} --event {event_name} --agent {agent}",
        "timeout": 5,
        "async": True
    }
    hook_group = {"hooks": [hook_entry]}
    if matcher:
        hook_group["matcher"] = matcher

    settings['hooks'][hook_name] = [hook_group]

with open(settings_file, 'w') as f:
    json.dump(settings, f, indent=2)

print(f"Installed {len(events)} hooks for agent '{agent}' in {settings_file}")
PYTHON

echo "Done. Bridge path: $BRIDGE_PATH"
```

- [ ] **Step 2: Make script executable**

```bash
chmod +x scripts/install-hooks.sh
```

- [ ] **Step 3: Update Makefile — add install-hooks target**

Add to `Makefile` after the `bridge:` target:

```makefile
## Install CC hooks into ~/.claude/settings.json
install-hooks: bridge
	./scripts/install-hooks.sh CC $(BRIDGE_BIN)
```

- [ ] **Step 4: Build bridge and install hooks**

Run: `make bridge && make install-hooks`
Expected: Bridge compiles, hooks installed with 22 events listed.

- [ ] **Step 5: Verify settings.json was updated**

Run: `cat ~/.claude/settings.json | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('hooks',{})), 'hooks registered')"`
Expected: `22 hooks registered`

- [ ] **Step 6: Commit**

```bash
git add scripts/install-hooks.sh Makefile
git commit -m "feat: add install-hooks script for 22 CC hook events"
```

---

### Task 9: Final Verification

- [ ] **Step 1: Run full Go test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 2: Run frontend lint**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Rebuild bridge binary**

Run: `make bridge`
Expected: Compiles without errors.

- [ ] **Step 4: Manual smoke test — trigger a hook**

Run: `echo '{"session_id":"test","hook_event_name":"Stop","stop_reason":"end_turn","last_assistant_message":"Done!"}' | ./bin/pager-cc-bridge --event stop --agent CC`
Expected: Exit 0. If Pager app is running, check the event in panel.

- [ ] **Step 5: Final commit (if any remaining changes)**

```bash
git status
# If clean, nothing to commit.
# If there are changes, stage and commit with appropriate message.
```

