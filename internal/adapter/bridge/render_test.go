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
