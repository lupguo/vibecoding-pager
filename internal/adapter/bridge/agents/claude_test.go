package agents

import (
	"testing"

	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

func TestClaudeFamily_ID(t *testing.T) {
	if got := (ClaudeFamily{}).ID(); got != entity.AgentClaudeCode {
		t.Errorf("ID() = %q, want %q", got, entity.AgentClaudeCode)
	}
}

func TestClaudeFamily_Hooks_Count(t *testing.T) {
	hooks := ClaudeFamily{}.Hooks()
	if len(hooks) != 23 {
		t.Errorf("len(Hooks()) = %d, want 23", len(hooks))
	}
}

func TestClaudeFamily_Hooks_PreToolUseHasMatcher(t *testing.T) {
	hooks := ClaudeFamily{}.Hooks()
	for _, h := range hooks {
		if h.Event == "PreToolUse" {
			if h.Matcher != "*" {
				t.Errorf("PreToolUse.Matcher = %q, want %q", h.Matcher, "*")
			}
			return
		}
	}
	t.Error("PreToolUse not in Hooks()")
}

func TestClaudeFamily_Hooks_StopHasNoMatcher(t *testing.T) {
	hooks := ClaudeFamily{}.Hooks()
	for _, h := range hooks {
		if h.Event == "Stop" {
			if h.Matcher != "" {
				t.Errorf("Stop.Matcher = %q, want empty", h.Matcher)
			}
			return
		}
	}
	t.Error("Stop not in Hooks()")
}
