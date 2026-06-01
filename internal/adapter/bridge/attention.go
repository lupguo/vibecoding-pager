package bridge

import "pager/internal/domain/entity"

// attentionTools are tools that always require user attention regardless of permission mode.
// These tools involve direct user interaction (asking questions, etc.)
var attentionTools = map[string]bool{
	"AskUserQuestion": true,
}

// DetermineAttentionLevel decides the attention level based on event type, permission mode, and tool name.
//
// Rules:
//   - stop/error → done
//   - post_tool_use → running (tool already executed)
//   - pre_tool_use + attentionTool → attention (always, even with bypassPermissions)
//   - pre_tool_use + bypassPermissions → running (will auto-execute)
//   - pre_tool_use + anything else → attention (needs user approve)
func DetermineAttentionLevel(eventType, permissionMode, toolName string) string {
	switch eventType {
	case entity.EventStop, entity.EventError, entity.EventSubagentStop:
		return entity.AttentionDone
	case entity.EventPostToolUse:
		return entity.AttentionRunning
	case entity.EventPreToolUse:
		// Tools that always need user attention (e.g. asking user a question)
		if attentionTools[toolName] {
			return entity.AttentionAttention
		}
		if permissionMode == "bypassPermissions" {
			return entity.AttentionRunning
		}
		return entity.AttentionAttention
	default:
		return entity.AttentionRunning
	}
}
