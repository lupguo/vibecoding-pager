package bridge

import (
	"testing"

	"pager/internal/domain/entity"
)

func TestDetermineAttentionLevel_Stop(t *testing.T) {
	level := DetermineAttentionLevel(entity.EventStop, "bypassPermissions", "Bash")
	if level != entity.AttentionDone {
		t.Errorf("got %q, want %q", level, entity.AttentionDone)
	}
}

func TestDetermineAttentionLevel_Error(t *testing.T) {
	level := DetermineAttentionLevel(entity.EventError, "", "")
	if level != entity.AttentionDone {
		t.Errorf("got %q, want %q", level, entity.AttentionDone)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Bypass(t *testing.T) {
	level := DetermineAttentionLevel(entity.EventPreToolUse, "bypassPermissions", "Bash")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Default(t *testing.T) {
	level := DetermineAttentionLevel(entity.EventPreToolUse, "default", "Bash")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Empty(t *testing.T) {
	level := DetermineAttentionLevel(entity.EventPreToolUse, "", "Edit")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PostToolUse(t *testing.T) {
	level := DetermineAttentionLevel(entity.EventPostToolUse, "default", "Bash")
	if level != entity.AttentionRunning {
		t.Errorf("got %q, want %q", level, entity.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_AskUserQuestion_Bypass(t *testing.T) {
	// AskUserQuestion should ALWAYS be attention, even with bypassPermissions
	level := DetermineAttentionLevel(entity.EventPreToolUse, "bypassPermissions", "AskUserQuestion")
	if level != entity.AttentionAttention {
		t.Errorf("got %q, want %q", level, entity.AttentionAttention)
	}
}
