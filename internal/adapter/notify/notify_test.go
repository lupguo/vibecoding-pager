package notify

import (
	"testing"

	"pager/internal/domain/entity"
)

func TestShouldNotify_AttentionOnly(t *testing.T) {
	cases := []struct {
		ev   *entity.AgentEvent
		want bool
	}{
		{&entity.AgentEvent{EventType: "StopFailure"}, true},
		{&entity.AgentEvent{EventType: "PermissionRequest"}, true},
		{&entity.AgentEvent{EventType: "Stop"}, false},
		{&entity.AgentEvent{EventType: "PostToolUse", ToolName: "Edit", PermissionMode: "default"}, false},
		{&entity.AgentEvent{EventType: "PreToolUse", ToolName: "Edit", PermissionMode: "default"}, true},
	}
	for _, tc := range cases {
		if got := ShouldNotify(tc.ev, "attention_only"); got != tc.want {
			t.Errorf("ShouldNotify(%q) = %v; want %v", tc.ev.EventType, got, tc.want)
		}
	}
}

func TestShouldNotifyByConfig(t *testing.T) {
	cfg := map[string][]string{
		"CC": {"PreToolUse", "Stop"},
	}
	if !ShouldNotifyByConfig(&entity.AgentEvent{AgentLabel: "CC", EventType: "Stop"}, cfg) {
		t.Error("CC + Stop should notify")
	}
	if ShouldNotifyByConfig(&entity.AgentEvent{AgentLabel: "CC", EventType: "Notification"}, cfg) {
		t.Error("CC + Notification should NOT notify")
	}
}
