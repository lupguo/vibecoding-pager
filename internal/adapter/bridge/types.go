package bridge

import "encoding/json"

// CCHookInput is the JSON structure CC passes via stdin to hooks.
type CCHookInput struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolUseID      string          `json:"tool_use_id"`
	PermissionMode string          `json:"permission_mode"`
	// SessionStart fields
	Source string `json:"source"` // startup | resume | clear
	// UserPromptSubmit fields
	Prompt string `json:"prompt"`
	// Notification fields
	Message string `json:"message"`
	// PreCompact fields
	Trigger string `json:"trigger"` // manual | auto
	// Stop/SubagentStop fields
	StopHookActive bool `json:"stop_hook_active"`
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
