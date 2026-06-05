package agents

import (
	"encoding/json"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

// ClaudeFamily 是 Claude Code 与 schema 完全兼容的 agent 家族实现。
// 同时服务 "CC"、"CC-Internal"、"CodeBuddy" 三个 label。
type ClaudeFamily struct{}

func (ClaudeFamily) ID() string { return entity.AgentClaudeCode }

func (ClaudeFamily) ParseEnvelope(raw []byte) (*bridge.Envelope, error) {
	var w struct {
		SessionID      string          `json:"session_id"`
		TranscriptPath string          `json:"transcript_path"`
		CWD            string          `json:"cwd"`
		HookEventName  string          `json:"hook_event_name"`
		PermissionMode string          `json:"permission_mode"`
		ToolName       string          `json:"tool_name"`
		ToolInput      json.RawMessage `json:"tool_input"`
		ToolUseID      string          `json:"tool_use_id"`
		ToolResponse   json.RawMessage `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, ErrInvalidPayload
	}
	return &bridge.Envelope{
		SessionID:      w.SessionID,
		TranscriptPath: w.TranscriptPath,
		CWD:            w.CWD,
		EventName:      w.HookEventName,
		PermissionMode: w.PermissionMode,
		ToolName:       w.ToolName,
		ToolUseID:      w.ToolUseID,
		ToolInput:      w.ToolInput,
		ToolResponse:   w.ToolResponse,
		RawPayload:     raw,
	}, nil
}

func (ClaudeFamily) Accept(eventType string, env *bridge.Envelope) bool {
	// TODO Task 5
	return true
}

func (ClaudeFamily) Hooks() []HookSpec {
	return []HookSpec{
		{Event: "PreToolUse", Matcher: "*"},
		{Event: "PostToolUse", Matcher: "*"},
		{Event: "PostToolUseFailure", Matcher: "*"},
		{Event: "PermissionRequest", Matcher: "*"},
		{Event: "PermissionDenied", Matcher: "*"},
		{Event: "PostToolBatch"},
		{Event: "SessionStart"},
		{Event: "SessionEnd"},
		{Event: "UserPromptSubmit"},
		{Event: "UserPromptExpansion"},
		{Event: "Stop"},
		{Event: "StopFailure"},
		{Event: "SubagentStart"},
		{Event: "SubagentStop"},
		{Event: "TaskCreated"},
		{Event: "TaskCompleted"},
		{Event: "Notification"},
		{Event: "PreCompact"},
		{Event: "PostCompact"},
		{Event: "InstructionsLoaded"},
		{Event: "Elicitation"},
		{Event: "MessageDisplay"},
		{Event: "ElicitationResult"},
	}
}
