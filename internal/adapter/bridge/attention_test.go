package bridge

import (
	"testing"

	"pager/internal/domain/entity"
)

func TestDetermineAttentionLevel_Stop(t *testing.T) {
	level := DetermineAttentionLevel("Stop", "bypassPermissions", "Bash")
	if level != entity.AttentionDone {
		t.Errorf("got %q, want %q", level, entity.AttentionDone)
	}
}

func TestDetermineAttentionLevel_SessionEnd(t *testing.T) {
	level := DetermineAttentionLevel("SessionEnd", "", "")
	if level != entity.AttentionDone {
		t.Errorf("got %q, want %q", level, entity.AttentionDone)
	}
}

func TestDetermineAttentionLevel_SubagentStop(t *testing.T) {
	level := DetermineAttentionLevel("SubagentStop", "", "")
	if level != entity.AttentionDone {
		t.Errorf("got %q, want %q", level, entity.AttentionDone)
	}
}

func TestDetermineAttentionLevel_StopFailure(t *testing.T) {
	level := DetermineAttentionLevel("StopFailure", "", "")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PermissionRequest(t *testing.T) {
	level := DetermineAttentionLevel("PermissionRequest", "", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_Notification(t *testing.T) {
	level := DetermineAttentionLevel("Notification", "", "")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_Elicitation(t *testing.T) {
	level := DetermineAttentionLevel("Elicitation", "", "")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PostToolUseFailure(t *testing.T) {
	level := DetermineAttentionLevel("PostToolUseFailure", "", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PermissionDenied(t *testing.T) {
	level := DetermineAttentionLevel("PermissionDenied", "", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Bypass(t *testing.T) {
	level := DetermineAttentionLevel("PreToolUse", "bypassPermissions", "Bash")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Default(t *testing.T) {
	level := DetermineAttentionLevel("PreToolUse", "default", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PreToolUse_AskUserQuestion_Bypass(t *testing.T) {
	// AskUserQuestion should ALWAYS be attention, even with bypassPermissions
	level := DetermineAttentionLevel("PreToolUse", "bypassPermissions", "AskUserQuestion")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PostToolUse(t *testing.T) {
	level := DetermineAttentionLevel("PostToolUse", "default", "Bash")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_PostToolBatch(t *testing.T) {
	level := DetermineAttentionLevel("PostToolBatch", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_SessionStart(t *testing.T) {
	level := DetermineAttentionLevel("SessionStart", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_UserPromptSubmit(t *testing.T) {
	level := DetermineAttentionLevel("UserPromptSubmit", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_SubagentStart(t *testing.T) {
	level := DetermineAttentionLevel("SubagentStart", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_TaskCreated(t *testing.T) {
	level := DetermineAttentionLevel("TaskCreated", "", "")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}
