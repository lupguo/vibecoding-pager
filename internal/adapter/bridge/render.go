package bridge

import "strings"

const contentMaxRunes = 60

// Render 把 (agent, event, tool, payload) 解析为展示字符串。
// payload 必须是 hook stdin 的完整原文。
// 非工具事件传 toolName="" 即可，YAML 该事件下若是 ScalarNode 不走 toolName 分支。
//
// 返回值:
//   - raw:     完整的解析结果（未截断），可存入 ContentRaw
//   - content: 截断到 contentMaxRunes 个 rune 的展示版本
func Render(agentID, eventType, toolName string, payload []byte) (raw, content string) {
	rules, err := LoadRules()
	if err != nil {
		return "", ""
	}
	node, ok := rules.Lookup(agentID, eventType)
	if !ok {
		return "", ""
	}
	template := PickTemplate(node, toolName)
	if template == "" {
		return "", ""
	}
	raw = strings.TrimSpace(Evaluate(template, payload, toolName))
	return raw, truncateRunes(raw, contentMaxRunes)
}

// truncateRunes returns at most n runes of s, appending '…' if truncated.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
