package notify

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

const NotificationIDPrefix = "evt-"

// osascriptRunner is the indirection point that lets tests verify the
// fallback path is invoked without actually spawning osascript. Production
// runs use runOsascript; tests substitute a recording func.
var osascriptRunner = runOsascript

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

// ShowEvent dispatches a system notification for an event.
//
// Two backends are supported:
//
//   - svc != nil → Wails-native NotificationService (used when the binary
//     runs inside a .app bundle; supports click-to-deep-link via UserInfo).
//   - svc == nil → osascript fallback (used when the binary runs unbundled
//     via `make dev` / `make run`; click handlers not available, but
//     notifications still appear in the macOS Notification Center).
//
// The split exists because Wails' NotificationService.Startup hard-fails
// without a valid CFBundleIdentifier, so app.go skips registering the service
// in unbundled mode (see internal/wails/bundle.go). Without the osascript
// fallback below, that skip would silently drop every notification.
func ShowEvent(svc *notifications.NotificationService, e *entity.AgentEvent, lang string) {
	label := e.AgentLabel
	if label == "" {
		label = agentLabel(e.Agent)
	}
	title := sanitize(fmt.Sprintf("%s · %s", label, lastPath(e.CWD)))
	body := extractBody(e)
	body = sanitize(body)

	if svc == nil {
		// unbundled fallback: osascript, no deep-link support
		args := buildOsascriptCmd(title, body)
		if err := osascriptRunner(args); err != nil {
			slog.Warn("osascript notification failed", "module", "notify", "err", err, "event", e.EventType)
		}
		return
	}

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

// extractBody picks the most informative non-empty string from the event for
// the notification body.
func extractBody(e *entity.AgentEvent) string {
	if e.Content != "" {
		return e.Content
	}
	if e.ToolName != "" {
		return e.ToolName
	}
	return e.EventType
}

// buildOsascriptCmd returns the [-e, <script>] argv suffix for `osascript`.
// AppleScript double-quoted strings interpret backslash as an escape, so
// backslash MUST be escaped first (otherwise we'd double-escape any
// pre-existing escape sequence). Newlines are flattened to spaces because
// the whole script is a single -e argument.
func buildOsascriptCmd(title, body string) []string {
	script := fmt.Sprintf(
		`display notification "%s" with title "Pager" subtitle "%s" sound name "Tink"`,
		osascriptEscape(body), osascriptEscape(title),
	)
	return []string{"-e", script}
}

func osascriptEscape(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func runOsascript(args []string) error {
	return exec.Command("osascript", args...).Run()
}

// sanitize normalizes user-provided content for display in a notification:
// truncates to 100 runes (macOS notification banners crop anything longer)
// and replaces newlines with spaces. Quote/backslash escaping is intentionally
// NOT done here — that's handled by the per-backend builder (osascriptEscape
// for the AppleScript path, Wails handles its own).
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
