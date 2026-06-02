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
