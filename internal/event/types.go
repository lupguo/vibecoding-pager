package event

import (
	"encoding/json"
	"time"
)

// EventType constants
const (
	EventPreToolUse  = "pre_tool_use"
	EventPostToolUse = "post_tool_use"
	EventStop        = "stop"
	EventError       = "error"
	EventNotification = "notification"
)

// Agent constants
const (
	AgentClaudeCode = "claude-code"
	AgentCodex      = "codex"
)

// SessionStatus constants
const (
	StatusWaiting  = "waiting"
	StatusActive   = "active"
	StatusFinished = "finished"
	StatusError    = "error"
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
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// SessionKey returns the unique session identifier (host:cwd:tty triple).
func (e *AgentEvent) SessionKey() string {
	return e.Host + ":" + e.CWD + ":" + e.TTY
}
