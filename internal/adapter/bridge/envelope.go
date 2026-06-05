package bridge

import "encoding/json"

// Envelope 是 hook stdin 经 Agent 解析后的通用形态，承担两个角色：
//   1. 给主流程提供路由 / 状态机所需的核心字段
//   2. 通过 RawPayload 字段把完整 JSON 透传给 DSL 规则引擎
//
// 注意：本结构体只放"主流程使用"的字段。事件特定字段（source / reason /
// last_assistant_message / notification_type / agent_type / ...）一律不进，
// 它们由 DSL 直接从 RawPayload 通过 gjson 路径读取。
type Envelope struct {
	SessionID      string          // Registry / SessionKey
	TranscriptPath string          // 日志展示
	CWD            string          // SessionKey fallback
	EventName      string          // hook_event_name
	PermissionMode string          // DeriveStatus 输入
	ToolName       string          // PreToolUse/PostToolUse 模板路由
	ToolUseID      string          // PostToolUse 配对
	ToolInput      json.RawMessage // 工具规则 pre 输入
	ToolResponse   json.RawMessage // 工具规则 post 输入
	TurnID         string          // Codex 才填，CC/CodeBuddy 留空
	RawPayload     json.RawMessage // 完整 stdin，DSL 读这个
}
