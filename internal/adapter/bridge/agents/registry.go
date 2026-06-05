package agents

import "sort"

// registry 把命令行 --agent 参数（label）映射到具体 Agent 实例。
// 加新 agent 在这里加一行；ClaudeFamily 服务 schema 兼容的多个 label。
var registry = map[string]Agent{
	"CC":          ClaudeFamily{},
	"CC-Internal": ClaudeFamily{},
	"CodeBuddy":   ClaudeFamily{},
}

// SelectByLabel 根据 --agent 命令行参数返回对应 Agent 实例。
// 未知 label 返回 (nil, false)，主流程据此 silent return。
func SelectByLabel(label string) (Agent, bool) {
	a, ok := registry[label]
	return a, ok
}

// LabeledAgent 是 (label, agent) 的配对，All() 返回类型。
type LabeledAgent struct {
	Label string
	Agent Agent
}

// All 返回所有已注册的 Agent，按 label 字典序。
// pager-installhooks 用这个枚举所有 settings 文件，避免在 installer 里
// 重复维护 label 列表。
func All() []LabeledAgent {
	labels := make([]string, 0, len(registry))
	for l := range registry {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	out := make([]LabeledAgent, 0, len(labels))
	for _, l := range labels {
		out = append(out, LabeledAgent{Label: l, Agent: registry[l]})
	}
	return out
}
