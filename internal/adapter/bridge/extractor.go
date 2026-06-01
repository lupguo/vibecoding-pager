package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
)

const contentMaxRunes = 60

// ExtractContent extracts a human-readable summary from tool name + input.
// Returns (contentRaw, content) where content is truncated to contentMaxRunes.
func ExtractContent(toolName string, toolInput json.RawMessage) (contentRaw, content string) {
	raw := extractRaw(toolName, toolInput)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = toolName
	}
	return raw, truncateRunes(raw, contentMaxRunes)
}

func extractRaw(toolName string, toolInput json.RawMessage) string {
	unmarshal := func(v any) bool {
		return json.Unmarshal(toolInput, v) == nil
	}

	switch toolName {
	case "Bash":
		var in BashInput
		if unmarshal(&in) && in.Command != "" {
			return in.Command
		}
	case "Edit":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "编辑 " + in.FilePath
		}
	case "Write":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "写入 " + in.FilePath
		}
	case "Read":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "读取 " + in.FilePath
		}
	case "Glob":
		var in GlobInput
		if unmarshal(&in) && in.Pattern != "" {
			return "查找 " + in.Pattern
		}
	case "Grep":
		var in GrepInput
		if unmarshal(&in) && in.Pattern != "" {
			return "搜索 " + in.Pattern
		}
	case "WebFetch":
		var in WebFetchInput
		if unmarshal(&in) && in.URL != "" {
			return "抓取 " + in.URL
		}
	case "WebSearch":
		var in WebSearchInput
		if unmarshal(&in) && in.Query != "" {
			return "搜索 " + in.Query
		}
	case "Task":
		var in TaskInput
		if unmarshal(&in) && in.Description != "" {
			return "子任务: " + in.Description
		}
	case "AskUserQuestion":
		var in AskUserQuestionInput
		if unmarshal(&in) && len(in.Questions) > 0 && in.Questions[0].Question != "" {
			return in.Questions[0].Question
		}
	case "Agent":
		var in AgentInput
		if unmarshal(&in) {
			if in.Description != "" {
				return "子任务: " + in.Description
			}
			if in.Prompt != "" {
				return "子任务: " + in.Prompt
			}
		}
	}

	// MCP tools: mcp__github__create_pr etc.
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(toolName, "__")
		return fmt.Sprintf("MCP: %s", parts[len(parts)-1])
	}

	return toolName
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ExtractEventContent extracts content from event-level hooks (not tool-based).
// Returns (contentRaw, content).
func ExtractEventContent(eventType string, in *CCHookInput) (contentRaw, content string) {
	switch eventType {
	case "session_start":
		raw := "会话启动: " + in.Source
		return raw, truncateRunes(raw, contentMaxRunes)
	case "user_prompt_submit":
		raw := in.Prompt
		if raw == "" {
			raw = "用户输入"
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	case "subagent_stop":
		raw := "子Agent完成"
		return raw, raw
	case "pre_compact":
		raw := "对话压缩: " + in.Trigger
		return raw, truncateRunes(raw, contentMaxRunes)
	case "notification":
		raw := in.Message
		if raw == "" {
			raw = "通知"
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	default:
		return eventType, eventType
	}
}
