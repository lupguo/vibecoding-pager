package notify

import (
	"fmt"
	"os/exec"
	"strings"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

// ShouldNotifyByConfig checks if an event should trigger a system notification
// based on the per-agent notification_events config.
// Returns true if the event's AgentLabel+EventType is in the configured list.
func ShouldNotifyByConfig(e *entity.AgentEvent, notifEvents map[string][]string) bool {
	if notifEvents == nil {
		return false
	}
	events, ok := notifEvents[e.AgentLabel]
	if !ok {
		return false
	}
	for _, ev := range events {
		if ev == e.EventType {
			return true
		}
	}
	return false
}

// ShouldNotify determines whether an event should trigger a system notification
// based on the configured notification level.
func ShouldNotify(e *entity.AgentEvent, level string) bool {
	status := session.DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, false)
	switch level {
	case "all":
		// Notify on any state transition the user cares about.
		return status == entity.StatusWaiting || status == entity.StatusDone || status == entity.StatusError
	case "attention_only":
		return status == entity.StatusWaiting || status == entity.StatusError
	default:
		return false
	}
}

// Notification text by language
var notifyText = map[string]map[string]string{
	"zh": {
		"waiting":  "等待确认: ",
		"finished": "任务完成",
		"error":    " [错误]",
	},
	"en": {
		"waiting":  "Waiting: ",
		"finished": "Task completed",
		"error":    " [Error]",
	},
}

func getText(lang, key string) string {
	if m, ok := notifyText[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return notifyText["zh"][key]
}

// ShowFull is the main notification entrypoint with all configuration.
func ShowFull(e *entity.AgentEvent, level string, lang string) {
	if !ShouldNotify(e, level) {
		return
	}

	var title, body string
	switch e.EventType {
	case entity.EventPreToolUse:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
		if body == "" {
			body = getText(lang, "waiting") + e.ToolName
		}
	case entity.EventStop:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = getText(lang, "finished")
	case entity.EventError:
		title = fmt.Sprintf("%s · %s%s", agentLabel(e.Agent), lastPath(e.CWD), getText(lang, "error"))
		body = e.Content
	default:
		return
	}
	showOsascript(title, body)
}

// Show triggers a macOS system notification based on event type.
// Uses osascript. Only fires for pre_tool_use, stop, and error events.
func Show(e *entity.AgentEvent) {
	var title, body string
	switch e.EventType {
	case entity.EventPreToolUse:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
		if body == "" {
			body = "等待确认: " + e.ToolName
		}
	case entity.EventStop:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = "任务完成"
	case entity.EventError:
		title = fmt.Sprintf("%s · %s [错误]", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
	default:
		return
	}
	showOsascript(title, body)
}

// ShowEvent triggers a macOS system notification for any event type.
// Works with CamelCase event types. Called after ShouldNotifyByConfig passes.
func ShowEvent(e *entity.AgentEvent, lang string) {
	label := e.AgentLabel
	if label == "" {
		label = agentLabel(e.Agent)
	}
	title := fmt.Sprintf("%s · %s", label, lastPath(e.CWD))

	body := e.Content
	if body == "" {
		body = e.EventType
		if e.ToolName != "" {
			body = e.ToolName
		}
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
	case entity.AgentClaudeCode:
		return "Claude Code"
	case entity.AgentCodex:
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
