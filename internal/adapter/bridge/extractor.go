package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
)

const contentMaxRunes = 60

// ExtractContent extracts a human-readable summary from tool name + input.
// Returns (contentRaw, content) where content is truncated to contentMaxRunes.
func ExtractContent(toolName string, toolInput json.RawMessage) (contentRaw, content string) {
	raw := extractRaw(toolName, toolInput)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = toolName
	}
	return raw, truncateRunes(raw, contentMaxRunes)
}

func extractRaw(toolName string, toolInput json.RawMessage) string {
	unmarshal := func(v any) bool {
		return json.Unmarshal(toolInput, v) == nil
	}

	switch toolName {
	case "Bash":
		var in BashInput
		if unmarshal(&in) && in.Command != "" {
			return in.Command
		}
	case "Edit":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "编辑 " + in.FilePath
		}
	case "Write":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "写入 " + in.FilePath
		}
	case "Read":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "读取 " + in.FilePath
		}
	case "Glob":
		var in GlobInput
		if unmarshal(&in) && in.Pattern != "" {
			return "查找 " + in.Pattern
		}
	case "Grep":
		var in GrepInput
		if unmarshal(&in) && in.Pattern != "" {
			return "搜索 " + in.Pattern
		}
	case "WebFetch":
		var in WebFetchInput
		if unmarshal(&in) && in.URL != "" {
			return "抓取 " + in.URL
		}
	case "WebSearch":
		var in WebSearchInput
		if unmarshal(&in) && in.Query != "" {
			return "搜索 " + in.Query
		}
	case "Task":
		var in TaskInput
		if unmarshal(&in) && in.Description != "" {
			return "子任务: " + in.Description
		}
	case "AskUserQuestion":
		var in AskUserQuestionInput
		if unmarshal(&in) && len(in.Questions) > 0 && in.Questions[0].Question != "" {
			return in.Questions[0].Question
		}
	case "Agent":
		var in AgentInput
		if unmarshal(&in) {
			if in.Description != "" {
				return "子任务: " + in.Description
			}
			if in.Prompt != "" {
				return "子任务: " + in.Prompt
			}
		}
	case "TaskCreate":
		var in TaskCreateInput
		if unmarshal(&in) && in.Subject != "" {
			return in.Subject
		}
	case "TaskUpdate":
		var in TaskUpdateInput
		if unmarshal(&in) {
			parts := []string{}
			if in.Subject != "" {
				parts = append(parts, in.Subject)
			}
			if in.Status != "" {
				parts = append(parts, "→"+in.Status)
			}
			if len(parts) > 0 {
				return strings.Join(parts, " ")
			}
		}
	case "TaskGet", "TaskList":
		return toolName
	case "TaskStop":
		return toolName
	case "NotebookEdit":
		var in struct {
			NotebookPath string `json:"notebook_path"`
		}
		if unmarshal(&in) && in.NotebookPath != "" {
			return in.NotebookPath
		}
	case "LSP":
		var in struct {
			Operation string `json:"operation"`
			FilePath  string `json:"filePath"`
		}
		if unmarshal(&in) && in.Operation != "" {
			return "LSP " + in.Operation + ": " + in.FilePath
		}
	}

	// MCP tools: mcp__github__create_pr etc.
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(toolName, "__")
		return fmt.Sprintf("MCP: %s", parts[len(parts)-1])
	}

	return toolName
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ExtractEventContent extracts content from all non-tool event types.
// Returns (contentRaw, content). The convention is "raw data only, no i18n
// prefixes" — UI combines event-type labels with content for display.
//
// Exception: Notification events prepend a Chinese type label
// ([等授权]/[等输入]/[认证]) via notificationTypeLabel, so the type-derived
// hint survives storage and propagates to all consumers (UI, SQLite history,
// macOS system notifications). This is a deliberate v1.0 simplification —
// see docs/superpowers/specs/2026-06-03-pager-event-parsing-bugfix-design.md §3.3.
//
// Event types use CC-native CamelCase names (e.g., "Stop", "SessionStart").
func ExtractEventContent(eventType string, in *CCHookInput) (contentRaw, content string) {
	switch eventType {
	// Session layer
	case "SessionStart":
		return in.Source, in.Source
	case "SessionEnd":
		return in.Reason, in.Reason

	// Turn layer
	case "UserPromptSubmit":
		return in.Prompt, truncateRunes(in.Prompt, contentMaxRunes)
	case "UserPromptExpansion":
		raw := "/" + in.CommandName
		if in.CommandArgs != "" {
			raw += " " + in.CommandArgs
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	case "Stop":
		if in.LastAssistantMessage != "" {
			return in.LastAssistantMessage, truncateRunes(in.LastAssistantMessage, contentMaxRunes)
		}
		return in.StopReason, in.StopReason
	case "StopFailure":
		raw := in.ErrorType + ": " + in.ErrorMessage
		if raw == ": " {
			return "", ""
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	case "Error":
		raw := in.ErrorType + ": " + in.ErrorMessage
		if raw == ": " {
			return "", ""
		}
		return raw, truncateRunes(raw, contentMaxRunes)

	// Agent & Task layer
	case "SubagentStart":
		raw := firstNonEmpty(in.AgentType, in.AgentID)
		return raw, raw
	case "SubagentStop":
		if in.LastAssistantMessage != "" {
			return in.LastAssistantMessage, truncateRunes(in.LastAssistantMessage, contentMaxRunes)
		}
		return in.AgentType, in.AgentType
	case "TaskCreated":
		return in.TaskSubject, truncateRunes(in.TaskSubject, contentMaxRunes)
	case "TaskCompleted":
		return in.TaskSubject, truncateRunes(in.TaskSubject, contentMaxRunes)

	// Tool layer (non-standard events that still have tool info)
	case "PostToolUseFailure":
		raw := in.ToolName + ": " + in.ToolError
		return raw, truncateRunes(raw, contentMaxRunes)
	case "PermissionRequest":
		return ExtractContent(in.ToolName, in.ToolInput)
	case "PermissionDenied":
		raw := in.ToolName + ": " + in.DenialReason
		return raw, truncateRunes(raw, contentMaxRunes)
	case "PostToolBatch":
		names := extractBatchToolNames(in.ToolCalls)
		if len(names) == 0 {
			return "", ""
		}
		raw := strings.Join(names, ", ")
		return raw, truncateRunes(raw, contentMaxRunes)

	// Context layer
	case "PreCompact":
		return in.Trigger, in.Trigger
	case "PostCompact":
		return "", ""
	case "InstructionsLoaded":
		return in.FilePath, truncateRunes(in.FilePath, contentMaxRunes)

	// MCP & UI layer
	case "Notification":
		msg := in.Message
		if msg == "" {
			msg = in.NotificationType
		}
		raw := notificationTypeLabel(in.NotificationType) + msg
		return raw, truncateRunes(raw, contentMaxRunes)
	case "Elicitation":
		raw := in.ServerName + ": " + in.Message
		return raw, truncateRunes(raw, contentMaxRunes)
	case "ElicitationResult":
		return in.ServerName, in.ServerName
	case "MessageDisplay":
		return in.Delta, truncateRunes(in.Delta, contentMaxRunes)

	default:
		return "", ""
	}
}

// extractBatchToolNames parses PostToolBatch's tool_calls JSON array
// (each element having a tool_name) and returns the names in order.
// On parse failure or nil input, returns nil.
func extractBatchToolNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var calls []struct {
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(raw, &calls); err != nil {
		return nil
	}
	names := make([]string, 0, len(calls))
	for _, c := range calls {
		if c.ToolName != "" {
			names = append(names, c.ToolName)
		}
	}
	return names
}

// firstNonEmpty returns the first non-empty string from vals, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// notificationTypeLabel returns a Chinese-bracketed prefix for known
// notification_type values, or empty string for unknown types.
func notificationTypeLabel(ntype string) string {
	switch ntype {
	case "permission_prompt":
		return "[等授权] "
	case "idle_prompt":
		return "[等输入] "
	case "auth_success":
		return "[认证] " // Bug 1 already drops these at bridge entry; defensive.
	default:
		return ""
	}
}
