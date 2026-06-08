package entity

import (
	"encoding/json"
	"time"
)

// EventType constants. Values are CamelCase to match the hook event names
// emitted by upstream agents (Claude Code, CodeBuddy, claude-code-internal,
// Codex). pager-bridge passes them through verbatim via --event. They MUST
// stay in sync with the switch cases in session.DeriveStatus.
//
// Coverage: 23 CC-family events (CC / CC-Internal / CodeBuddy share schema)
// plus 10 Codex events (subset; Codex doesn't emit Notification / Elicitation
// / SessionEnd / PostToolBatch / TaskCreated / TaskCompleted /
// MessageDisplay / InstructionsLoaded / UserPromptExpansion /
// ElicitationResult / PostToolUseFailure / StopFailure / Error).
const (
	EventPreToolUse         = "PreToolUse"
	EventPostToolUse        = "PostToolUse"
	EventStop               = "Stop"
	EventStopFailure        = "StopFailure"
	EventError              = "Error"
	EventNotification       = "Notification"
	EventSessionStart       = "SessionStart"
	EventSessionEnd         = "SessionEnd"
	EventUserPromptSubmit   = "UserPromptSubmit"
	EventSubagentStop       = "SubagentStop"
	EventPreCompact         = "PreCompact"
	EventPermissionRequest  = "PermissionRequest"
	EventPermissionDenied   = "PermissionDenied"
	EventElicitation        = "Elicitation"
	EventPostToolUseFailure = "PostToolUseFailure"
	EventPostCompact        = "PostCompact"
	EventSubagentStart      = "SubagentStart"
)

// Agent constants — values written into AgentEvent.Agent and the
// extract_rules.yaml namespace key.
const (
	AgentClaudeCode = "claude-code" // CC, CC-Internal, CodeBuddy (shared schema)
	AgentCodex      = "codex"       // Codex CLI 0.137+
)

// SessionStatus is the single source of truth for session UX state.
// Values are mutually exclusive; UI renders one tag per card.
type SessionStatus string

const (
	StatusWorking SessionStatus = "working" // active, no user action needed
	StatusWaiting SessionStatus = "waiting" // user action required (any reason)
	StatusDone    SessionStatus = "done"    // ended cleanly
	StatusError   SessionStatus = "error"   // ended with failure
)

// AgentEvent is the single cross-layer data structure.
// Bridge fills all fields; server and UI are read-only consumers.
type AgentEvent struct {
	Agent          string          `json:"agent"`
	Host           string          `json:"host"`
	CWD            string          `json:"cwd"`
	TTY            string          `json:"tty"`
	SessionID      string          `json:"session_id"`
	TermProgram    string          `json:"term_program"`
	ITermSessionID string          `json:"iterm_session_id,omitempty"`
	EventType      string          `json:"event_type"`
	ToolName       string          `json:"tool_name"`
	ToolUseID      string          `json:"tool_use_id"`
	Content        string          `json:"content"`
	ContentRaw     string          `json:"content_raw"`
	AgentLabel     string          `json:"agent_label"`
	PermissionMode string          `json:"permission_mode,omitempty"`
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// SessionKey returns the unique session identifier.
// Prefers the agent-native session_id (set by all 4 agents); falls back to
// host:cwd:tty triple when an event arrives without one (e.g. early CodeBuddy
// notifications before authentication completes).
func (e *AgentEvent) SessionKey() string {
	if e.SessionID != "" {
		return e.SessionID
	}
	return e.Host + ":" + e.CWD + ":" + e.TTY
}
