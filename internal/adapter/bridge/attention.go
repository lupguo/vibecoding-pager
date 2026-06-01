package bridge

import "pager/internal/domain/entity"

// attentionTools are tools that always require user attention regardless of permission mode.
var attentionTools = map[string]bool{
	"AskUserQuestion": true,
}

// DetermineAttentionLevel decides the attention level based on event type, permission mode, and tool name.
// Event types use CC-native CamelCase (e.g., "PreToolUse", "Stop", "StopFailure").
func DetermineAttentionLevel(eventType, permissionMode, toolName string) string {
	switch eventType {
	// Done — session/agent ended
	case "Stop", "SessionEnd", "SubagentStop":
		return entity.AttentionDone

	// Attention — user action needed
	case "StopFailure", "PermissionRequest", "Notification", "Elicitation",
		"PostToolUseFailure", "PermissionDenied":
		return entity.AttentionAttention

	// Tool — depends on mode and tool type
	case "PreToolUse":
		if attentionTools[toolName] {
			return entity.AttentionAttention
		}
		if permissionMode == "bypassPermissions" {
			return entity.AttentionRunning
		}
		return entity.AttentionAttention

	// Running — informational
	case "PostToolUse", "PostToolBatch":
		return entity.AttentionRunning

	// Default — running (SessionStart, UserPromptSubmit, SubagentStart, TaskCreated, etc.)
	default:
		return entity.AttentionRunning
	}
}
