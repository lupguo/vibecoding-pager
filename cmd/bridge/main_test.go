package main

import "testing"

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

func TestIsToolEvent(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{"PreToolUse", true},
		{"PostToolUse", true},
		{"PermissionRequest", true},
		{"PermissionDenied", true},
		{"Stop", false},
		{"Notification", false},
		{"SessionStart", false},
	}
	for _, tt := range tests {
		got := isToolEvent(tt.eventType)
		if got != tt.want {
			t.Errorf("isToolEvent(%q): got %v, want %v", tt.eventType, got, tt.want)
		}
	}
}
