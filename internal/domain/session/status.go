package session

import "pager/internal/domain/entity"

// Constants kept here to avoid magic strings. Mirrored against CC-native event types.
const (
	toolAskUserQuestion       = "AskUserQuestion"
	permModeBypassPermissions = "bypassPermissions"
)

// DeriveStatus folds (event_type, tool_name, permission_mode, hasPendingAskUser) into a
// single SessionStatus. The function is the only place where event_type → status mapping
// lives.
func DeriveStatus(eventType, toolName, permissionMode string, hasPendingAskUser bool) entity.SessionStatus {
	switch eventType {
	// terminal failures
	case entity.EventStopFailure, entity.EventError:
		return entity.StatusError

	// user-attention events
	case entity.EventPermissionRequest, entity.EventPermissionDenied, entity.EventNotification,
		entity.EventElicitation, entity.EventPostToolUseFailure:
		return entity.StatusWaiting

	// clean termination — but if AskUserQuestion is still pending, the agent is
	// waiting on the user, not finished.
	case entity.EventStop, entity.EventSessionEnd, entity.EventSubagentStop:
		if hasPendingAskUser {
			return entity.StatusWaiting
		}
		return entity.StatusDone

	// tool-use
	case entity.EventPreToolUse:
		if toolName == toolAskUserQuestion || permissionMode != permModeBypassPermissions {
			return entity.StatusWaiting
		}
		return entity.StatusWorking

	// running default — PostToolUse, PostToolBatch, SessionStart, TaskCreated, etc.
	default:
		return entity.StatusWorking
	}
}
