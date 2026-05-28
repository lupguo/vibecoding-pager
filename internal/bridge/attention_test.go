package bridge

import (
	"testing"

	"pager/internal/event"
)

func TestDetermineAttentionLevel_Stop(t *testing.T) {
	level := DetermineAttentionLevel(event.EventStop, "bypassPermissions")
	if level != event.AttentionDone {
		t.Errorf("got %q, want %q", level, event.AttentionDone)
	}
}

func TestDetermineAttentionLevel_Error(t *testing.T) {
	level := DetermineAttentionLevel(event.EventError, "")
	if level != event.AttentionDone {
		t.Errorf("got %q, want %q", level, event.AttentionDone)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Bypass(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPreToolUse, "bypassPermissions")
	if level != event.AttentionRunning {
		t.Errorf("got %q, want %q", level, event.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Default(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPreToolUse, "default")
	if level != event.AttentionAttention {
		t.Errorf("got %q, want %q", level, event.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Empty(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPreToolUse, "")
	if level != event.AttentionAttention {
		t.Errorf("got %q, want %q", level, event.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PostToolUse(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPostToolUse, "default")
	if level != event.AttentionRunning {
		t.Errorf("got %q, want %q", level, event.AttentionRunning)
	}
}
