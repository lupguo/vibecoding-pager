package agents

import (
	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

// ClaudeFamily 是 Claude Code 与 schema 完全兼容的 agent 家族实现。
// 同时服务 "CC"、"CC-Internal"、"CodeBuddy" 三个 label。
type ClaudeFamily struct{}

func (ClaudeFamily) ID() string { return entity.AgentClaudeCode }

func (ClaudeFamily) ParseEnvelope(raw []byte) (*bridge.Envelope, error) {
	// TODO Task 4
	return nil, ErrInvalidPayload
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
