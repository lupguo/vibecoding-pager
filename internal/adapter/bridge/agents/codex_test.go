package agents

import (
	"testing"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

func TestCodex_ID(t *testing.T) {
	if got := (Codex{}).ID(); got != entity.AgentCodex {
		t.Errorf("ID() = %q, want %q", got, entity.AgentCodex)
	}
}

func TestCodex_Hooks_Count(t *testing.T) {
	if got := len((Codex{}).Hooks()); got != 10 {
		t.Errorf("len(Hooks()) = %d, want 10", got)
	}
}

func TestCodex_Hooks_NoSessionEnd(t *testing.T) {
	for _, h := range (Codex{}).Hooks() {
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
	env, err := (Codex{}).ParseEnvelope(raw)
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
	env, err := (Codex{}).ParseEnvelope(raw)
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
