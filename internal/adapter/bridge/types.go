package bridge

import "encoding/json"

// CCHookInput is the JSON structure CC passes via stdin to hooks.
// All 22 registered hook events share this union struct.
// Different event types populate different fields; unused fields remain zero-value.
type CCHookInput struct {
	// ═══ Common (all events) ═══
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	CWD            string       `json:"cwd"`
	HookEventName  string       `json:"hook_event_name"`
	PermissionMode string       `json:"permission_mode"`
	Effort         *EffortLevel `json:"effort,omitempty"`

	// ═══ Session layer ═══
	Source       string `json:"source"`        // SessionStart: startup/resume/clear/compact
	Model        string `json:"model"`         // SessionStart
	SessionTitle string `json:"session_title"` // SessionStart

	// ═══ Turn layer ═══
	Prompt               string `json:"prompt"`                 // UserPromptSubmit
	ExpansionType        string `json:"expansion_type"`         // UserPromptExpansion: slash_command/mcp_prompt
	CommandName          string `json:"command_name"`           // UserPromptExpansion
	CommandArgs          string `json:"command_args"`           // UserPromptExpansion
	CommandSource        string `json:"command_source"`         // UserPromptExpansion: plugin/user/core
	StopReason           string `json:"stop_reason"`            // Stop: end_turn/max_tokens
	LastAssistantMessage string `json:"last_assistant_message"` // Stop: Claude's reply preview
	StopHookActive       bool   `json:"stop_hook_active"`       // Stop
	ErrorType            string `json:"error_type"`             // StopFailure: rate_limit/auth/billing...
	ErrorMessage         string `json:"error_message"`          // StopFailure

	// ═══ Tool layer ═══
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolUseID    string          `json:"tool_use_id"`
	ToolResult   json.RawMessage `json:"tool_result,omitempty"` // PostToolUse
	ToolError    string          `json:"error"`                 // PostToolUseFailure
	DenialReason string          `json:"denial_reason"`         // PermissionDenied
	ToolCalls    json.RawMessage `json:"tool_calls,omitempty"`  // PostToolBatch

	// ═══ Agent & Task layer ═══
	AgentID         string `json:"agent_id"`         // SubagentStart/Stop
	AgentType       string `json:"agent_type"`       // SubagentStart/Stop
	TaskID          string `json:"task_id"`          // TaskCreated/Completed
	TaskSubject     string `json:"task_subject"`     // TaskCreated/Completed
	TaskDescription string `json:"task_description"` // TaskCreated

	// ═══ Context layer ═══
	FilePath   string `json:"file_path"`   // InstructionsLoaded
	MemoryType string `json:"memory_type"` // InstructionsLoaded
	LoadReason string `json:"load_reason"` // InstructionsLoaded
	Trigger    string `json:"trigger"`     // PreCompact/PostCompact: manual/auto

	// ═══ MCP & UI layer ═══
	NotificationType string          `json:"notification_type"` // Notification: permission_prompt/idle_prompt
	Message          string          `json:"message"`           // Notification/Elicitation
	ServerName       string          `json:"server_name"`       // Elicitation
	Request          json.RawMessage `json:"request,omitempty"` // Elicitation
	UserResponse     string          `json:"user_response"`     // ElicitationResult
	Delta            string          `json:"delta"`             // MessageDisplay
}

// EffortLevel represents the effort level in CC's response.
type EffortLevel struct {
	Level string `json:"level"` // low/medium/high/xhigh/max
}

// BashInput is tool_input for Bash tool.
type BashInput struct {
	Command string `json:"command"`
}

// FileInput is tool_input for Edit/Write/Read tools.
type FileInput struct {
	FilePath string `json:"file_path"`
}

// GlobInput is tool_input for Glob tool.
type GlobInput struct {
	Pattern string `json:"pattern"`
}

// GrepInput is tool_input for Grep tool.
type GrepInput struct {
	Pattern string `json:"pattern"`
}

// WebFetchInput is tool_input for WebFetch tool.
type WebFetchInput struct {
	URL string `json:"url"`
}

// WebSearchInput is tool_input for WebSearch tool.
type WebSearchInput struct {
	Query string `json:"query"`
}

// TaskInput is tool_input for Task (subagent) tool.
type TaskInput struct {
	Description string `json:"description"`
}

// AskUserQuestionInput is tool_input for AskUserQuestion tool.
type AskUserQuestionInput struct {
	Questions []struct {
		Question string `json:"question"`
	} `json:"questions"`
}

// AgentInput is tool_input for Agent tool.
type AgentInput struct {
	Prompt      string `json:"prompt"`
	Description string `json:"description"`
}

// TaskCreateInput is tool_input for TaskCreate tool.
type TaskCreateInput struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	ActiveForm  string `json:"activeForm"`
}

// TaskUpdateInput is tool_input for TaskUpdate tool.
type TaskUpdateInput struct {
	TaskID  string `json:"taskId"`
	Status  string `json:"status"`
	Subject string `json:"subject"`
}
