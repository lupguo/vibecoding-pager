package bridge

import "pager/internal/event"

// DetermineAttentionLevel decides the attention level based on event type and permission mode.
//
// Rules:
//   - stop/error → done
//   - post_tool_use → running (tool already executed)
//   - pre_tool_use + bypassPermissions → running (will auto-execute)
//   - pre_tool_use + anything else → attention (needs user approve)
func DetermineAttentionLevel(eventType, permissionMode string) string {
	switch eventType {
	case event.EventStop, event.EventError:
		return event.AttentionDone
	case event.EventPostToolUse:
		return event.AttentionRunning
	case event.EventPreToolUse:
		if permissionMode == "bypassPermissions" {
			return event.AttentionRunning
		}
		return event.AttentionAttention
	default:
		return event.AttentionRunning
	}
}
