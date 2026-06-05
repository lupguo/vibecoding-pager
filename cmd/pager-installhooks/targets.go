package main

import "os"

// Target 描述一个 agent 对应的写入目标。
type Target struct {
	AgentLabel   string
	SettingsFile string // 路径，~ 会展开
	Format       string // "settings"（顶层 settings 对象）| "hooks-only"（顶层就是 hooks）
}

// targets 是 installer 的目标清单。
// 注意：CC-Internal 这里写 settings.local.json 是 spec 的选择；如 PR1 实施时
// 验证 CC-Internal loader 不识别 .local.json，则改回 settings.json。
var targets = []Target{
	{"CC", "~/.claude/settings.local.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.local.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.local.json", "settings"},
	// "Codex" 在 PR 2 阶段加进来
}

// osUserHomeDir is an indirection for testing.
var osUserHomeDir = os.UserHomeDir

// expandPath 把 "~/foo" 展开为绝对路径。
func expandPath(p string) string {
	if len(p) >= 2 && p[:2] == "~/" {
		home, err := osUserHomeDir()
		if err != nil {
			return p
		}
		return home + p[1:]
	}
	return p
}
