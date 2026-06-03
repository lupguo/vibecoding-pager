package bridge

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

const contentMaxRunes = 60

// loadedRules is initialized once at package init and reused across calls.
// LoadRules failure is fatal — extract_rules.yaml is embedded, so a parse
// error means the binary itself is malformed.
var loadedRules *Rules

func init() {
	r, err := LoadRules()
	if err != nil {
		panic("bridge: failed to load embedded extract_rules.yaml: " + err.Error())
	}
	loadedRules = r
}

// ExtractContent extracts a (raw, truncated) summary from a tool event payload.
//
//	phase = "pre"  → payload is tool_input,    use rules.Pre[toolName]
//	phase = "post" → payload is tool_response, use rules.Post[toolName]
//
// Falls back to rules.default for unknown tools, then to toolName itself if
// every rule resolution yields empty.
func ExtractContent(phase, toolName string, payload json.RawMessage) (contentRaw, content string) {
	var raw string
	switch phase {
	case "pre":
		raw = renderPre(toolName, payload)
	case "post":
		raw = renderPost(toolName, payload)
	default:
		raw = toolName
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = toolName
	}
	return raw, truncateRunes(raw, contentMaxRunes)
}

// renderPre looks up a Pre rule. Preserves the legacy MCP-prefix special case
// (mcp__github__create_pr → "MCP: create_pr") that the old extractRaw had.
func renderPre(toolName string, payload []byte) string {
	if rule, ok := loadedRules.Pre[toolName]; ok {
		return Evaluate(rule, payload, toolName)
	}
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(toolName, "__")
		return "MCP: " + parts[len(parts)-1]
	}
	return Evaluate(loadedRules.Pre["default"], payload, toolName)
}

// renderPost dispatches by payload shape (string vs object) when the rule is
// structured. Falls back to default rule for unknown tools.
func renderPost(toolName string, payload []byte) string {
	node, ok := loadedRules.Post[toolName]
	if !ok {
		node, ok = loadedRules.Post["default"]
		if !ok {
			return toolName
		}
	}
	return renderPostRule(node, payload, toolName)
}

// renderPostRule walks a Post yaml.Node:
//   - ScalarNode: plain string template, applied directly
//   - MappingNode: branches by payload shape (string/object), then
//     object_paths probe list, then fallback template
func renderPostRule(node yaml.Node, payload []byte, toolName string) string {
	if node.Kind == yaml.ScalarNode {
		return Evaluate(node.Value, payload, toolName)
	}
	if node.Kind != yaml.MappingNode {
		return ""
	}
	get := func(key string) (yaml.Node, bool) {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return *node.Content[i+1], true
			}
		}
		return yaml.Node{}, false
	}

	trimmed := bytesTrimSpace(payload)
	isString := len(trimmed) > 0 && trimmed[0] == '"'

	if isString {
		if n, ok := get("string"); ok {
			if v := Evaluate(n.Value, payload, toolName); v != "" {
				return v
			}
		}
	} else {
		if n, ok := get("object"); ok {
			if v := Evaluate(n.Value, payload, toolName); v != "" {
				return v
			}
		}
		if n, ok := get("object_paths"); ok && n.Kind == yaml.SequenceNode {
			for _, p := range n.Content {
				template := "{" + p.Value + "}"
				if v := Evaluate(template, payload, toolName); v != "" {
					return v
				}
			}
		}
	}
	if n, ok := get("fallback"); ok {
		return Evaluate(n.Value, payload, toolName)
	}
	return ""
}

// bytesTrimSpace returns payload without leading/trailing ASCII whitespace.
func bytesTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\n' || b[start] == '\r') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\n' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
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
		return ExtractContent("pre", in.ToolName, in.ToolInput)
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

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
