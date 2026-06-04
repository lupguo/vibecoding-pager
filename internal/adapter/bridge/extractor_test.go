package bridge

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractContent_Bash(t *testing.T) {
	input := json.RawMessage(`{"command":"go build ./..."}`)
	raw, content := ExtractContent("pre", "Bash", input)
	if raw != "go build ./..." {
		t.Errorf("raw = %q, want %q", raw, "go build ./...")
	}
	if content != "go build ./..." {
		t.Errorf("content = %q, want %q", content, "go build ./...")
	}
}

func TestExtractContent_Edit(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/src/main.go"}`)
	raw, content := ExtractContent("pre", "Edit", input)
	if raw != "编辑 /src/main.go" {
		t.Errorf("raw = %q, want %q", raw, "编辑 /src/main.go")
	}
	if content != "编辑 /src/main.go" {
		t.Errorf("content = %q, want %q", content, "编辑 /src/main.go")
	}
}

func TestExtractContent_Write(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/out.txt"}`)
	raw, content := ExtractContent("pre", "Write", input)
	if raw != "写入 /tmp/out.txt" {
		t.Errorf("raw = %q, want %q", raw, "写入 /tmp/out.txt")
	}
	if content != "写入 /tmp/out.txt" {
		t.Errorf("content = %q, want %q", content, "写入 /tmp/out.txt")
	}
}

func TestExtractContent_Read(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/etc/hosts"}`)
	raw, content := ExtractContent("pre", "Read", input)
	if raw != "读取 /etc/hosts" {
		t.Errorf("raw = %q, want %q", raw, "读取 /etc/hosts")
	}
	if content != "读取 /etc/hosts" {
		t.Errorf("content = %q, want %q", content, "读取 /etc/hosts")
	}
}

func TestExtractContent_Glob(t *testing.T) {
	input := json.RawMessage(`{"pattern":"**/*.go"}`)
	raw, _ := ExtractContent("pre", "Glob", input)
	if raw != "查找 **/*.go" {
		t.Errorf("raw = %q, want %q", raw, "查找 **/*.go")
	}
}

func TestExtractContent_Grep(t *testing.T) {
	input := json.RawMessage(`{"pattern":"TODO"}`)
	raw, _ := ExtractContent("pre", "Grep", input)
	if raw != "搜索 TODO" {
		t.Errorf("raw = %q, want %q", raw, "搜索 TODO")
	}
}

func TestExtractContent_WebFetch(t *testing.T) {
	input := json.RawMessage(`{"url":"https://example.com"}`)
	raw, _ := ExtractContent("pre", "WebFetch", input)
	if raw != "抓取 https://example.com" {
		t.Errorf("raw = %q, want %q", raw, "抓取 https://example.com")
	}
}

func TestExtractContent_WebSearch(t *testing.T) {
	input := json.RawMessage(`{"query":"golang wails v3"}`)
	raw, _ := ExtractContent("pre", "WebSearch", input)
	if raw != "搜索 golang wails v3" {
		t.Errorf("raw = %q, want %q", raw, "搜索 golang wails v3")
	}
}

func TestExtractContent_Task(t *testing.T) {
	input := json.RawMessage(`{"description":"Run linter"}`)
	raw, _ := ExtractContent("pre", "Task", input)
	if raw != "子任务: Run linter" {
		t.Errorf("raw = %q, want %q", raw, "子任务: Run linter")
	}
}

func TestExtractContent_MCP(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("pre", "mcp__github__create_pr", input)
	if raw != "MCP: create_pr" {
		t.Errorf("raw = %q, want %q", raw, "MCP: create_pr")
	}
}

func TestExtractContent_Unknown(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("pre", "SomeNewTool", input)
	if raw != "SomeNewTool" {
		t.Errorf("raw = %q, want %q", raw, "SomeNewTool")
	}
}

func TestExtractContent_Truncation(t *testing.T) {
	longCmd := "echo 'this is a very long command that definitely exceeds sixty characters limit for display'"
	input, _ := json.Marshal(BashInput{Command: longCmd})
	raw, content := ExtractContent("pre", "Bash", json.RawMessage(input))
	if raw != longCmd {
		t.Errorf("raw should be untruncated")
	}
	runes := []rune(content)
	if len(runes) != 61 {
		t.Errorf("content rune len = %d, want 61 (60 + ellipsis)", len(runes))
	}
	if string(runes[60]) != "…" {
		t.Errorf("last rune should be ellipsis, got %q", string(runes[60]))
	}
}

func TestExtractContent_EmptyInput(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("pre", "Bash", input)
	if raw != "Bash" {
		t.Errorf("raw = %q, want %q", raw, "Bash")
	}
}

func TestExtractContent_AskUserQuestion(t *testing.T) {
	input := `{"questions":[{"question":"你偏好哪种 UI 风格？","header":"UI","options":[],"multiSelect":false}]}`
	_, content := ExtractContent("pre", "AskUserQuestion", json.RawMessage(input))
	if !strings.Contains(content, "你偏好哪种 UI 风格") {
		t.Errorf("content = %q, want to contain question text", content)
	}
}

func TestExtractContent_Agent(t *testing.T) {
	input := `{"prompt":"Review the code for security issues","description":"Security review"}`
	_, content := ExtractContent("pre", "Agent", json.RawMessage(input))
	if !strings.Contains(content, "Security review") {
		t.Errorf("content = %q, want to contain description", content)
	}
}

func TestExtractContent_AgentFallbackToPrompt(t *testing.T) {
	input := `{"prompt":"Review the code for security issues"}`
	raw, _ := ExtractContent("pre", "Agent", json.RawMessage(input))
	if !strings.Contains(raw, "Review the code") {
		t.Errorf("raw = %q, want to contain prompt text", raw)
	}
}

func TestExtractContent_TaskCreate(t *testing.T) {
	input := json.RawMessage(`{"subject":"Fix login bug","description":"Form crashes on submit","activeForm":"Fixing login"}`)
	raw, content := ExtractContent("pre", "TaskCreate", input)
	if raw != "Fix login bug" {
		t.Errorf("raw: got %q", raw)
	}
	if content != "Fix login bug" {
		t.Errorf("content: got %q", content)
	}
}

func TestExtractContent_TaskUpdate(t *testing.T) {
	input := json.RawMessage(`{"taskId":"1","status":"completed","subject":"Fix login bug"}`)
	raw, content := ExtractContent("pre", "TaskUpdate", input)
	if raw != "Fix login bug →completed" {
		t.Errorf("raw: got %q", raw)
	}
	if content != "Fix login bug →completed" {
		t.Errorf("content: got %q", content)
	}
}

func TestExtractContent_TaskUpdate_StatusOnly(t *testing.T) {
	input := json.RawMessage(`{"taskId":"1","status":"in_progress"}`)
	raw, content := ExtractContent("pre", "TaskUpdate", input)
	if raw != "→in_progress" {
		t.Errorf("raw: got %q", raw)
	}
	if content != "→in_progress" {
		t.Errorf("content: got %q", content)
	}
}

func TestExtractContent_TaskGet(t *testing.T) {
	input := json.RawMessage(`{"taskId":"1"}`)
	raw, _ := ExtractContent("pre", "TaskGet", input)
	if raw != "TaskGet" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractContent_TaskList(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("pre", "TaskList", input)
	if raw != "TaskList" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractContent_LSP(t *testing.T) {
	input := json.RawMessage(`{"operation":"goToDefinition","filePath":"main.go","line":10,"character":5}`)
	raw, _ := ExtractContent("pre", "LSP", input)
	if raw != "LSP goToDefinition: main.go" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractContent_NotebookEdit(t *testing.T) {
	input := json.RawMessage(`{"notebook_path":"/tmp/test.ipynb","new_source":"print('hi')"}`)
	raw, _ := ExtractContent("pre", "NotebookEdit", input)
	if raw != "/tmp/test.ipynb" {
		t.Errorf("raw: got %q", raw)
	}
}

// --- ExtractEventContent tests ---

func TestExtractEventContent_Stop_WithMessage(t *testing.T) {
	in := &CCHookInput{
		LastAssistantMessage: "I've completed the refactoring. All tests pass.",
		StopReason:           "end_turn",
	}
	raw, content := ExtractEventContent("Stop", in)
	if raw != "I've completed the refactoring. All tests pass." {
		t.Errorf("raw: got %q", raw)
	}
	if len([]rune(content)) > 61 {
		t.Errorf("content too long: %d runes", len([]rune(content)))
	}
}

func TestExtractEventContent_Stop_NoMessage(t *testing.T) {
	in := &CCHookInput{StopReason: "end_turn"}
	raw, _ := ExtractEventContent("Stop", in)
	if raw != "end_turn" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Stop_Empty(t *testing.T) {
	in := &CCHookInput{}
	raw, _ := ExtractEventContent("Stop", in)
	if raw != "" {
		t.Errorf("raw: got %q, want empty", raw)
	}
}

func TestExtractEventContent_StopFailure(t *testing.T) {
	in := &CCHookInput{ErrorType: "rate_limit", ErrorMessage: "Too many requests"}
	raw, _ := ExtractEventContent("StopFailure", in)
	if raw != "rate_limit: Too many requests" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Notification(t *testing.T) {
	in := &CCHookInput{NotificationType: "permission_prompt", Message: "Approve file edit?"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等授权] Approve file edit?" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Notification_NoMessage(t *testing.T) {
	in := &CCHookInput{NotificationType: "idle_prompt", Message: ""}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等输入] idle_prompt" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_TaskCreated(t *testing.T) {
	in := &CCHookInput{TaskSubject: "Implement auth flow", TaskDescription: "Add JWT tokens"}
	raw, _ := ExtractEventContent("TaskCreated", in)
	if raw != "Implement auth flow" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_SubagentStart(t *testing.T) {
	in := &CCHookInput{AgentType: "general-purpose"}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "general-purpose" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_UserPromptExpansion(t *testing.T) {
	in := &CCHookInput{CommandName: "brainstorming", CommandArgs: "design auth"}
	raw, _ := ExtractEventContent("UserPromptExpansion", in)
	if raw != "/brainstorming design auth" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_Elicitation(t *testing.T) {
	in := &CCHookInput{ServerName: "github_mcp", Message: "Enter token"}
	raw, _ := ExtractEventContent("Elicitation", in)
	if raw != "github_mcp: Enter token" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_InstructionsLoaded(t *testing.T) {
	in := &CCHookInput{FilePath: "/project/CLAUDE.md", LoadReason: "session_start"}
	raw, _ := ExtractEventContent("InstructionsLoaded", in)
	if raw != "/project/CLAUDE.md" {
		t.Errorf("raw: got %q", raw)
	}
}

func TestExtractEventContent_SessionStart(t *testing.T) {
	in := &CCHookInput{Source: "startup"}
	raw, _ := ExtractEventContent("SessionStart", in)
	if raw != "startup" {
		t.Errorf("raw: got %q", raw)
	}
}

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

func TestExtractEventContent_StopFailure_BothEmpty_ReturnsEmpty(t *testing.T) {
	in := &CCHookInput{}
	raw, content := ExtractEventContent("StopFailure", in)
	if raw != "" || content != "" {
		t.Errorf("expected empty, got raw=%q content=%q", raw, content)
	}
}

func TestExtractEventContent_SessionEnd_UsesReason(t *testing.T) {
	in := &CCHookInput{Reason: "clear"}
	raw, content := ExtractEventContent("SessionEnd", in)
	if raw != "clear" || content != "clear" {
		t.Errorf("got (%q, %q), want (clear, clear)", raw, content)
	}
}

func TestExtractEventContent_SessionEnd_EmptyReason(t *testing.T) {
	in := &CCHookInput{Reason: ""}
	raw, content := ExtractEventContent("SessionEnd", in)
	if raw != "" || content != "" {
		t.Errorf("expected empty, got (%q, %q)", raw, content)
	}
}

func TestExtractEventContent_SubagentStart_AgentTypePresent(t *testing.T) {
	in := &CCHookInput{AgentType: "general-purpose", AgentID: "agent-xyz"}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "general-purpose" {
		t.Errorf("raw = %q, want %q", raw, "general-purpose")
	}
}

func TestExtractEventContent_SubagentStart_AgentIDFallback(t *testing.T) {
	in := &CCHookInput{AgentType: "", AgentID: "agent-66639e99"}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "agent-66639e99" {
		t.Errorf("raw = %q, want %q", raw, "agent-66639e99")
	}
}

func TestExtractEventContent_SubagentStart_BothEmpty(t *testing.T) {
	in := &CCHookInput{AgentType: "", AgentID: ""}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "" {
		t.Errorf("raw = %q, want empty", raw)
	}
}

func TestExtractEventContent_Notification_PermissionPrompt(t *testing.T) {
	in := &CCHookInput{NotificationType: "permission_prompt", Message: "needs your permission to use Bash"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等授权] needs your permission to use Bash" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_IdlePrompt(t *testing.T) {
	in := &CCHookInput{NotificationType: "idle_prompt", Message: "CodeBuddy is waiting for your input"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等输入] CodeBuddy is waiting for your input" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_UnknownType(t *testing.T) {
	in := &CCHookInput{NotificationType: "", Message: "raw message only"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "raw message only" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_EmptyMessageFallsBackToType(t *testing.T) {
	in := &CCHookInput{NotificationType: "idle_prompt", Message: ""}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等输入] idle_prompt" {
		t.Errorf("raw = %q", raw)
	}
}

// ─── Post phase ──────────────────────────────────────────────────────────────

func TestExtractContent_Post_Bash(t *testing.T) {
	raw, _ := ExtractContent("post", "Bash", []byte(`{"stdout":"hello\nworld","exitCode":0}`))
	if raw != "hello (exit=0)" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_Edit_Success(t *testing.T) {
	raw, _ := ExtractContent("post", "Edit", []byte(`{"success":true,"filePath":"/x.go"}`))
	if raw != "✓写入 /x.go" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_Edit_Failure(t *testing.T) {
	raw, _ := ExtractContent("post", "Edit", []byte(`{"success":false,"filePath":"/x.go"}`))
	if raw != "✗失败 /x.go" {
		t.Errorf("raw = %q", raw)
	}
}

// CC-INT shape: tool_response is an object containing answers array.
func TestExtractContent_Post_AskUserQuestion_Object(t *testing.T) {
	payload := []byte(`{"answers":["why? : because"]}`)
	raw, _ := ExtractContent("post", "AskUserQuestion", payload)
	if raw != "why? : because" {
		t.Errorf("raw = %q", raw)
	}
}

// CodeBuddy shape: tool_response is a bare JSON string.
func TestExtractContent_Post_AskUserQuestion_String(t *testing.T) {
	payload := []byte(`" · q1 → a1"`)
	raw, _ := ExtractContent("post", "AskUserQuestion", payload)
	// ExtractContent applies strings.TrimSpace, so the leading space in the
	// bare-string payload is dropped before truncation.
	if raw != "· q1 → a1" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_UnknownTool_DefaultObjectPaths(t *testing.T) {
	// stdout is in default.object_paths, so it resolves.
	raw, _ := ExtractContent("post", "MyTool", []byte(`{"stdout":"out"}`))
	if raw != "out" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_UnknownTool_FallbackJSON(t *testing.T) {
	// Payload has none of the default object_paths → fallback to <json:120>.
	raw, _ := ExtractContent("post", "MyTool", []byte(`{"weird_field":"x"}`))
	if raw != `{"weird_field":"x"}` {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Pre_UnknownToolFallsBackToToolName(t *testing.T) {
	raw, _ := ExtractContent("pre", "MyCustomTool", []byte(`{}`))
	if raw != "MyCustomTool" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_TruncatesTo60Runes(t *testing.T) {
	long := strings.Repeat("a", 100)
	_, content := ExtractContent("pre", "Bash", []byte(`{"command":"`+long+`"}`))
	if r := []rune(content); len(r) != 61 { // 60 + "…"
		t.Errorf("content rune length = %d, want 61", len(r))
	}
}
