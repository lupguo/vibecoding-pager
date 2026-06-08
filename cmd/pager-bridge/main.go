package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge/agents"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
	infralog "github.com/lupguo/vibecoding-pager/internal/infra/log"
)

func main() {
	defer os.Exit(0)

	var (
		agentLabel string
		eventType  string
		printHooks bool
		debug      bool
	)
	pflag.StringVar(&agentLabel, "agent", "", "agent label (CC/CC-Internal/CodeBuddy/Codex)")
	pflag.StringVar(&eventType, "event", "", "hook event name")
	pflag.BoolVar(&printHooks, "print-hooks", false, "print hook spec JSON for given --agent and exit")
	pflag.BoolVar(&debug, "debug", false, "dump raw stdin to /tmp/pager-raw.json")
	pflag.Parse()

	// Log to stderr; --debug bumps level to DEBUG, otherwise WARN keeps the
	// hook subprocess output quiet under normal operation.
	level := slog.LevelWarn
	if debug || os.Getenv("PAGER_DEBUG") == "1" {
		level = slog.LevelDebug
	}
	infralog.Init(level)
	log := infralog.Module("bridge.cli")

	if printHooks {
		printHooksJSON(agentLabel)
		return
	}

	agent, ok := agents.SelectByLabel(agentLabel)
	if !ok {
		return
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	if debug || os.Getenv("PAGER_DEBUG") == "1" {
		_ = os.WriteFile("/tmp/pager-raw.json", raw, 0644)
	}

	env, err := agent.ParseEnvelope(raw)
	if err != nil {
		// Surface parse failures so malformed agent payloads (e.g. unescaped
		// control chars in JSON strings) don't disappear silently. Hooks are
		// fire-and-forget so this only shows up in the agent's terminal, but
		// it's the only signal a user has when an event is dropped at the
		// envelope-parsing stage.
		log.Warn("parse envelope", "event", eventType, "agent", agentLabel, "err", err)
		return
	}
	if !agent.Accept(eventType, env) {
		return
	}

	contentRaw, content := bridge.Render(agent.ID(), eventType, env.ToolName, raw)

	bridge.PostEvent(entity.AgentEvent{
		Agent:          agent.ID(),
		AgentLabel:     agentLabel,
		EventType:      eventType,
		SessionID:      env.SessionID,
		CWD:            firstNonEmpty(env.CWD, os.Getenv("PWD")),
		TTY:            detectTTY(),
		Host:           "local",
		TermProgram:    os.Getenv("TERM_PROGRAM"),
		ITermSessionID: os.Getenv("ITERM_SESSION_ID"),
		ToolName:       env.ToolName,
		ToolUseID:      env.ToolUseID,
		PermissionMode: env.PermissionMode,
		Content:        content,
		ContentRaw:     contentRaw,
		RawPayload:     raw,
		Timestamp:      time.Now(),
	})
}

func printHooksJSON(agentLabel string) {
	agent, ok := agents.SelectByLabel(agentLabel)
	if !ok {
		// User-facing CLI error before exit — keep as plain stderr line so
		// installer's output stays parseable.
		fmt.Fprintf(os.Stderr, "unknown agent: %q\n", agentLabel)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(agent.Hooks()); err != nil {
		os.Exit(1)
	}
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
