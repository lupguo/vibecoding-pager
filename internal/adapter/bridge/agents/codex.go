package agents

import (
	"encoding/json"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

// Codex 是 Codex CLI 的 Agent 实现。
//
// 对应 Codex 0.137 的 hook schema (codex-rs/config/src/hook_config.rs)：
//   - SessionStart 携带 trigger（startup / resume / clear / compact）而不是 CC 的 source。
//   - 多一个 turn_id 字段（CC 没有）。
//   - exec_command 工具替代 CC 的 Bash 与文件编辑（含 apply_patch）。
//   - 只发 10 个事件，比 CC 家族少 13 个 — 没有 Notification / Elicitation /
//     SessionEnd / PostToolBatch / TaskCreated / TaskCompleted / MessageDisplay /
//     InstructionsLoaded / UserPromptExpansion / ElicitationResult /
//     PostToolUseFailure / StopFailure / Error。
type Codex struct{}

func (Codex) ID() string { return entity.AgentCodex }

func (Codex) ParseEnvelope(raw []byte) (*bridge.Envelope, error) {
	var w struct {
		SessionID      string          `json:"session_id"`
		TurnID         string          `json:"turn_id"`
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
		TurnID:         w.TurnID,
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

func (Codex) Accept(string, *bridge.Envelope) bool { return true }

func (Codex) Hooks() []HookSpec {
	return []HookSpec{
		{Event: "PreToolUse", Matcher: "*"},
		{Event: "PostToolUse", Matcher: "*"},
		{Event: "PermissionRequest", Matcher: "*"},
		{Event: "PreCompact"},
		{Event: "PostCompact"},
		{Event: "SessionStart"},
		{Event: "UserPromptSubmit"},
		{Event: "SubagentStart"},
		{Event: "SubagentStop"},
		{Event: "Stop"},
	}
}
