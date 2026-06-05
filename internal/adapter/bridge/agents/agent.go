package agents

import (
	"errors"

	"github.com/lupguo/vibecoding-pager/internal/adapter/bridge"
)

// ErrInvalidPayload 表示 hook stdin 不是合法 JSON（或字段类型不匹配）。
// 主流程收到这个错误应该 silent return（hook 是 fire-and-forget）。
var ErrInvalidPayload = errors.New("agents: invalid hook payload")

// Agent 描述一个被 pager 支持的 coding agent（CC / CodeBuddy / Codex / ...）。
// 一个 Agent 实现可以服务多个 label——例如 ClaudeFamily{} 同时映射 "CC"、
// "CC-Internal"、"CodeBuddy" 三个 label，因为它们 schema 完全相同。
type Agent interface {
	// ID 返回写入 entity.AgentEvent.Agent 的稳定标识，
	// 同时是 extract_rules.yaml 的 namespace key。
	// 必须返回 entity.AgentXxx 中的常量。
	ID() string

	// ParseEnvelope 把 hook stdin 原始字节解为通用 Envelope。
	// 各 agent 在这里实现自己的 JSON schema 映射。
	// 返回 ErrInvalidPayload 表示 JSON 解码失败。
	ParseEnvelope(raw []byte) (*bridge.Envelope, error)

	// Accept 决定一个事件是否应该上报给 pager 服务器。
	// 返回 false 时事件被静默丢弃（例：CodeBuddy 的 auth_success Notification
	// 没有 session_id，落到下游会污染 session 列表）。
	Accept(eventType string, env *bridge.Envelope) bool

	// Hooks 返回该 agent 应该注册的全部 hook 事件清单。
	// pager-installhooks 通过 --print-hooks 子命令读取这个清单。
	Hooks() []HookSpec
}

// HookSpec 描述一个待注册的 hook 事件。
// Matcher 仅 PreToolUse / PostToolUse 等工具事件需要（"*" 表示匹配所有工具）；
// 非工具事件留空，installer 写入时不输出 matcher 字段。
type HookSpec struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
}
