package main

import (
	"testing"

	"pager/internal/adapter/bridge"
)

func TestParseArgs_NewFormat(t *testing.T) {
	eventType, agent := parseArgs([]string{"--event", "StopFailure", "--agent", "CC-INT"})
	if eventType != "StopFailure" {
		t.Errorf("eventType: got %q, want %q", eventType, "StopFailure")
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
	eventType, agent := parseArgs([]string{"--event", "Notification"})
	if eventType != "Notification" {
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

func TestShouldDrop_NotificationWithoutSession(t *testing.T) {
	in := &bridge.CCHookInput{SessionID: ""}
	if !shouldDrop("Notification", in) {
		t.Error("expected drop for Notification without session_id")
	}
}

func TestShouldDrop_NotificationWithSession(t *testing.T) {
	in := &bridge.CCHookInput{SessionID: "abc-123"}
	if shouldDrop("Notification", in) {
		t.Error("expected keep for Notification with session_id")
	}
}

func TestShouldDrop_PreToolUseWithoutSession(t *testing.T) {
	in := &bridge.CCHookInput{SessionID: ""}
	if shouldDrop("PreToolUse", in) {
		t.Error("expected keep for PreToolUse without session_id (fallback path)")
	}
}
