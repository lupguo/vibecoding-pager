//go:build never

package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)


func main() {
	defer os.Exit(0)

	eventType, agentLabel := parseArgs(os.Args[1:])

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}

	if os.Getenv("PAGER_DEBUG") == "1" {
		_ = os.WriteFile("/tmp/pager-raw.json", raw, 0644)
	}

	var in bridge.CCHookInput
	_ = json.Unmarshal(raw, &in)

	if shouldDrop(eventType, &in) {
		return
	}

	// Extract content based on event type. PreToolUse / Permission* dispatch to
	// rules.Pre with tool_input; PostToolUse to rules.Post with tool_response.
	var contentRaw, content string
	switch eventType {
	case "PreToolUse", "PermissionRequest", "PermissionDenied":
		contentRaw, content = bridge.ExtractContent("pre", in.ToolName, in.ToolInput)
	case "PostToolUse":
		contentRaw, content = bridge.ExtractContent("post", in.ToolName, in.ToolResult)
	default:
		contentRaw, content = bridge.ExtractEventContent(eventType, &in)
	}

	e := entity.AgentEvent{
		Agent:          entity.AgentClaudeCode,
		Host:           "local",
		SessionID:      in.SessionID,
		CWD:            firstNonEmpty(in.CWD, os.Getenv("PWD")),
		TTY:            detectTTY(),
		TermProgram:    os.Getenv("TERM_PROGRAM"),
		ITermSessionID: os.Getenv("ITERM_SESSION_ID"),
		EventType:      eventType,
		ToolName:       in.ToolName,
		ToolUseID:      in.ToolUseID,
		Content:        content,
		ContentRaw:     contentRaw,
		AgentLabel:     agentLabel,
		PermissionMode: in.PermissionMode,
		RawPayload:     raw,
		Timestamp:      time.Now(),
	}

	bridge.PostEvent(e)
}

// parseArgs extracts event type and agent label from command-line args.
// Supports both formats:
//
//	New: vibecoding-pager-cc-bridge --event <type> --agent <label>
//	Old: vibecoding-pager-cc-bridge <type> [--agent <label>]
func parseArgs(args []string) (eventType, agentLabel string) {
	eventType = "unknown"
	agentLabel = "CC" // default

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--event" && i+1 < len(args):
			eventType = args[i+1]
			i++ // skip next
		case args[i] == "--agent" && i+1 < len(args):
			agentLabel = args[i+1]
			i++ // skip next
		case !strings.HasPrefix(args[i], "--") && eventType == "unknown":
			eventType = args[i]
		}
	}
	return
}

// shouldDrop returns true for events that have no actionable signal and would
// pollute the session list. Currently only filters CodeBuddy's auth_success
// Notification (which carries no session_id, would trigger the host:cwd:tty
// fallback, and make session_key look like a path).
func shouldDrop(eventType string, in *bridge.CCHookInput) bool {
	if eventType == "Notification" && in.SessionID == "" {
		return true
	}
	return false
}

func detectTTY() string {
	out, err := exec.Command("tty").Output()
	if err == nil {
		t := strings.TrimSpace(string(out))
		if t != "" && t != "not a tty" {
			return t
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
