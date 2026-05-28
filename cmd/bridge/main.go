package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"pager/internal/bridge"
	"pager/internal/event"
)

func main() {
	defer os.Exit(0)

	eventType := "unknown"
	if len(os.Args) > 1 {
		eventType = os.Args[1]
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}

	if os.Getenv("PAGER_DEBUG") == "1" {
		_ = os.WriteFile("/tmp/pager-raw.json", raw, 0644)
	}

	var in bridge.CCHookInput
	_ = json.Unmarshal(raw, &in)

	contentRaw, content := bridge.ExtractContent(in.ToolName, in.ToolInput)

	e := event.AgentEvent{
		Agent:          event.AgentClaudeCode,
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
		RawPayload:     raw,
		Timestamp:      time.Now(),
	}

	bridge.PostEvent(e)
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
