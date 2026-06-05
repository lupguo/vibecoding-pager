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
		"task_subject": "Fix login bug",
		"task_description": "The login form crashes on submit"
	}`
	var in CCHookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.TaskSubject != "Fix login bug" {
		t.Errorf("TaskSubject: got %q", in.TaskSubject)
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

// TestCCHookInput_PostToolUse_BashRoundTrip verifies that the tool_response
// field unmarshals into ToolResult. This guards against the field-tag drift
// caught by T5 (was `tool_result`, CC sends `tool_response`).
func TestCCHookInput_PostToolUse_BashRoundTrip(t *testing.T) {
	raw := []byte(`{
		"hook_event_name":"PostToolUse",
		"tool_name":"Bash",
		"tool_use_id":"u1",
		"tool_input":{"command":"echo hi"},
		"tool_response":{"stdout":"hi","exitCode":0}
	}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", in.ToolName)
	}
	if len(in.ToolResult) == 0 {
		t.Fatal("ToolResult is empty — likely tool_response field-tag drift")
	}
	// ExtractContent call removed: extractor.go is excluded during bridge refactor
	// (see //go:build never); content extraction is tested in extractor_test.go.
}

// TestCCHookInput_Stop_LastAssistantMessageRoundTrip pins the last_assistant_message
// field tag (used in Stop/SubagentStop event content extraction).
func TestCCHookInput_Stop_LastAssistantMessageRoundTrip(t *testing.T) {
	raw := []byte(`{
		"hook_event_name":"Stop",
		"last_assistant_message":"All done.",
		"stop_reason":"end_turn"
	}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.LastAssistantMessage != "All done." {
		t.Errorf("LastAssistantMessage = %q, want %q (field tag drift?)", in.LastAssistantMessage, "All done.")
	}
	if in.StopReason != "end_turn" {
		t.Errorf("StopReason = %q (field tag drift?)", in.StopReason)
	}
}

// TestCCHookInput_TaskCreated_RoundTrip pins task_subject (Bug 4a hot path).
func TestCCHookInput_TaskCreated_RoundTrip(t *testing.T) {
	raw := []byte(`{
		"hook_event_name":"TaskCreated",
		"task_id":"t1",
		"task_subject":"do the thing"
	}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.TaskSubject != "do the thing" {
		t.Errorf("TaskSubject = %q (field tag drift?)", in.TaskSubject)
	}
}

// TestCCHookInput_UserPromptExpansion_RoundTrip pins command_name + command_args.
func TestCCHookInput_UserPromptExpansion_RoundTrip(t *testing.T) {
	raw := []byte(`{
		"hook_event_name":"UserPromptExpansion",
		"command_name":"brainstorm",
		"command_args":"foo bar"
	}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.CommandName != "brainstorm" {
		t.Errorf("CommandName = %q (field tag drift?)", in.CommandName)
	}
	if in.CommandArgs != "foo bar" {
		t.Errorf("CommandArgs = %q (field tag drift?)", in.CommandArgs)
	}
}

// TestCCHookInput_MessageDisplay_RoundTrip pins delta.
func TestCCHookInput_MessageDisplay_RoundTrip(t *testing.T) {
	raw := []byte(`{"hook_event_name":"MessageDisplay","delta":"streamed text"}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Delta != "streamed text" {
		t.Errorf("Delta = %q (field tag drift?)", in.Delta)
	}
}

// TestCCHookInput_Notification_RoundTrip pins notification_type + message.
func TestCCHookInput_Notification_RoundTrip(t *testing.T) {
	raw := []byte(`{
		"hook_event_name":"Notification",
		"notification_type":"permission_prompt",
		"message":"needs your permission to use Bash"
	}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.NotificationType != "permission_prompt" {
		t.Errorf("NotificationType = %q (field tag drift?)", in.NotificationType)
	}
	if in.Message != "needs your permission to use Bash" {
		t.Errorf("Message = %q (field tag drift?)", in.Message)
	}
}

// TestCCHookInput_SessionEnd_RoundTrip pins reason (added by Task 1).
func TestCCHookInput_SessionEnd_RoundTrip(t *testing.T) {
	raw := []byte(`{"hook_event_name":"SessionEnd","reason":"compact"}`)
	var in CCHookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Reason != "compact" {
		t.Errorf("Reason = %q (field tag drift?)", in.Reason)
	}
}
