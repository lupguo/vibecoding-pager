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
