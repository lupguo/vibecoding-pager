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

// legacyCleanupTargets 是早期版本写过的非 .local.json 路径。
// 升级流程会扫这些文件，把里面的 pager 条目清掉（用户其他配置原样保留），
// 防止 settings.json 与 settings.local.json 双发 hook。
// Codex 不在列表中——它原生只读 ~/.codex/hooks.json，没有 settings.json 主+local 分层。
var legacyCleanupTargets = []Target{
	{"CC", "~/.claude/settings.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.json", "settings"},
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
