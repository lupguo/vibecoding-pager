package session

import (
	"testing"

	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name       string
		eventType  string
		toolName   string
		permMode   string
		hasAskUser bool
		want       entity.SessionStatus
	}{
		// Terminal failures
		{"StopFailure → error", "StopFailure", "", "", false, entity.StatusError},
		{"Error → error", "Error", "", "", false, entity.StatusError},

		// User-attention events
		{"PermissionRequest → waiting", "PermissionRequest", "", "", false, entity.StatusWaiting},
		{"PermissionDenied → waiting", "PermissionDenied", "", "", false, entity.StatusWaiting},
		{"Notification → waiting", "Notification", "", "", false, entity.StatusWaiting},
		{"Elicitation → waiting", "Elicitation", "", "", false, entity.StatusWaiting},
		{"PostToolUseFailure → waiting", "PostToolUseFailure", "", "", false, entity.StatusWaiting},

		// Clean termination
		{"Stop (no pending) → done", "Stop", "", "", false, entity.StatusDone},
		{"Stop (askUser pending) → waiting", "Stop", "", "", true, entity.StatusWaiting},
		{"SessionEnd → done", "SessionEnd", "", "", false, entity.StatusDone},
		{"SubagentStop → done", "SubagentStop", "", "", false, entity.StatusDone},

		// Tool-use
		{"PreToolUse AskUserQuestion → waiting", "PreToolUse", "AskUserQuestion", "default", false, entity.StatusWaiting},
		{"PreToolUse bypass → working", "PreToolUse", "Edit", "bypassPermissions", false, entity.StatusWorking},
		{"PreToolUse default → waiting", "PreToolUse", "Edit", "default", false, entity.StatusWaiting},
		{"PreToolUse acceptEdits → waiting (non-bypass)", "PreToolUse", "Edit", "acceptEdits", false, entity.StatusWaiting},

		// Default running
		{"PostToolUse → working", "PostToolUse", "Edit", "default", false, entity.StatusWorking},
		{"PostToolBatch → working", "PostToolBatch", "", "", false, entity.StatusWorking},
		{"SessionStart → working", "SessionStart", "", "", false, entity.StatusWorking},
		{"unknown → working", "TaskCreated", "", "", false, entity.StatusWorking},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveStatus(tc.eventType, tc.toolName, tc.permMode, tc.hasAskUser)
			if got != tc.want {
				t.Errorf("DeriveStatus(%q,%q,%q,%v) = %q; want %q",
					tc.eventType, tc.toolName, tc.permMode, tc.hasAskUser, got, tc.want)
			}
		})
	}
}

func TestDeriveStatus_PostCompact(t *testing.T) {
	got := DeriveStatus("PostCompact", "", "", false)
	if got != entity.StatusWorking {
		t.Errorf("PostCompact status = %v, want StatusWorking", got)
	}
}

func TestDeriveStatus_SubagentStart(t *testing.T) {
	got := DeriveStatus("SubagentStart", "", "", false)
	if got != entity.StatusWorking {
		t.Errorf("SubagentStart status = %v, want StatusWorking", got)
	}
}

// Codex emits SessionStart on resume; same mapping as CC.
func TestDeriveStatus_CodexSessionStart(t *testing.T) {
	got := DeriveStatus("SessionStart", "", "", false)
	if got != entity.StatusWorking {
		t.Errorf("SessionStart status = %v, want StatusWorking", got)
	}
}
