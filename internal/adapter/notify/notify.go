package notify

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

const NotificationIDPrefix = "evt-"

// ShouldNotifyByConfig checks if an event should trigger a system notification
// based on the per-agent notification_events config.
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

// ShouldNotify determines whether an event should trigger a notification based on level.
func ShouldNotify(e *entity.AgentEvent, level string) bool {
	status := session.DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, false)
	switch level {
	case "all":
		return status == entity.StatusWaiting || status == entity.StatusDone || status == entity.StatusError
	case "attention_only":
		return status == entity.StatusWaiting || status == entity.StatusError
	default:
		return false
	}
}

// ShowEvent dispatches a Wails-native notification for an event.
// svc is the registered NotificationService; nil-safe (no-op).
func ShowEvent(svc *notifications.NotificationService, e *entity.AgentEvent, lang string) {
	if svc == nil {
		return
	}
	label := e.AgentLabel
	if label == "" {
		label = agentLabel(e.Agent)
	}
	title := fmt.Sprintf("%s · %s", label, lastPath(e.CWD))
	body := e.Content
	if body == "" {
		if e.ToolName != "" {
			body = e.ToolName
		} else {
			body = e.EventType
		}
	}
	body = sanitize(body)
	title = sanitize(title)

	id := fmt.Sprintf("%s%s-%d", NotificationIDPrefix, e.SessionKey(), time.Now().UnixNano())
	err := svc.SendNotification(notifications.NotificationOptions{
		ID:    id,
		Title: title,
		Body:  body,
		Data:  map[string]any{"session_id": e.SessionKey()},
	})
	if err != nil {
		slog.Warn("notification send failed", "module", "notify", "err", err, "event", e.EventType)
	}
}

func sanitize(s string) string {
	r := []rune(s)
	if len(r) > 100 {
		s = string(r[:100]) + "…"
	}
	s = strings.ReplaceAll(s, "\n", " ")
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
