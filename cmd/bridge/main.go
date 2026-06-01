package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"pager/internal/adapter/bridge"
	"pager/internal/domain/entity"
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

	// Extract content based on event type
	var contentRaw, content string
	if isToolEvent(eventType) {
		contentRaw, content = bridge.ExtractContent(in.ToolName, in.ToolInput)
	} else {
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
//	New: pager-cc-bridge --event <type> --agent <label>
//	Old: pager-cc-bridge <type> [--agent <label>]
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

// isToolEvent returns true if the event type involves tool use with tool_input.
func isToolEvent(eventType string) bool {
	switch eventType {
	case "PreToolUse", "PostToolUse", "PermissionRequest", "PermissionDenied":
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
