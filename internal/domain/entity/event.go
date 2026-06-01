package entity

import (
	"encoding/json"
	"time"
)

// EventType constants
const (
	EventPreToolUse       = "pre_tool_use"
	EventPostToolUse      = "post_tool_use"
	EventStop             = "stop"
	EventError            = "error"
	EventNotification     = "notification"
	EventSessionStart     = "session_start"
	EventUserPromptSubmit = "user_prompt_submit"
	EventSubagentStop     = "subagent_stop"
	EventPreCompact       = "pre_compact"
)

// Agent constants
const (
	AgentClaudeCode = "claude-code"
	AgentCodex      = "codex"
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
	Agent          string `json:"agent"`
	Host           string `json:"host"`
	CWD            string `json:"cwd"`
	TTY            string `json:"tty"`
	SessionID      string `json:"session_id"`
	TermProgram    string `json:"term_program"`
	ITermSessionID string `json:"iterm_session_id,omitempty"`
	EventType      string `json:"event_type"`
	ToolName       string `json:"tool_name"`
	ToolUseID      string `json:"tool_use_id"`
	Content        string `json:"content"`
	ContentRaw     string `json:"content_raw"`
	AgentLabel     string `json:"agent_label"`
	PermissionMode string `json:"permission_mode,omitempty"`
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// SessionKey returns the unique session identifier.
// Prefers CC-native session_id; falls back to host:cwd:tty triple.
func (e *AgentEvent) SessionKey() string {
	if e.SessionID != "" {
		return e.SessionID
	}
	return e.Host + ":" + e.CWD + ":" + e.TTY
}
