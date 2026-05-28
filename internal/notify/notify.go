package notify

import (
	"fmt"
	"os/exec"
	"strings"

	"pager/internal/event"
)

// Show triggers a macOS system notification based on event type.
// Uses osascript. Only fires for pre_tool_use, stop, and error events.
func Show(e *event.AgentEvent) {
	var title, body string
	switch e.EventType {
	case event.EventPreToolUse:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
		if body == "" {
			body = "等待确认: " + e.ToolName
		}
	case event.EventStop:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = "任务完成"
	case event.EventError:
		title = fmt.Sprintf("%s · %s [错误]", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
	default:
		return
	}
	showOsascript(title, body)
}

func showOsascript(title, body string) {
	title = sanitize(title)
	body = sanitize(body)
	script := fmt.Sprintf(
		`display notification "%s" with title "Pager" subtitle "%s" sound name "Tink"`,
		body, title,
	)
	_ = exec.Command("osascript", "-e", script).Run()
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	r := []rune(s)
	if len(r) > 100 {
		s = string(r[:100]) + "…"
	}
	return s
}

func agentLabel(agent string) string {
	switch agent {
	case event.AgentClaudeCode:
		return "Claude Code"
	case event.AgentCodex:
		return "Codex"
	default:
		return agent
	}
}

func lastPath(p string) string {
	parts := strings.Split(strings.TrimRight(p, "/"), "/")
	if len(parts) == 0 || p == "" {
		return p
	}
	return parts[len(parts)-1]
}
