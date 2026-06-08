package main

import "os"

// Target 描述一个 agent 对应的写入目标。
type Target struct {
	AgentLabel   string
	SettingsFile string // 路径，~ 会展开
	Format       string // 当前只有 "settings"：顶层 {"hooks": {<event>: [...]}}
}

// targets 是 installer 的目标清单。所有 4 个 agent 都用同一份顶层 schema：
// {"hooks": {<event>: [MatcherGroup, ...]}}。
//
// 写入位置选 settings.json (而不是 settings.local.json)：
//   - Anthropic Claude Code v2 文档 (https://code.claude.com/docs/en/hooks)
//     只在 **per-project** 这一档（<repo>/.claude/settings.local.json，
//     gitignored）支持 .local.json。user-level 的 ~/.claude/settings.local.json
//     不在加载链上。
//   - CC-Internal (Tencent fork v1.1.9) 验证过同样不读 .local.json 覆盖层。
//   - CodeBuddy fork 行为一致。
//   - Codex 走自己的 ~/.codex/hooks.json，没有 .local.json 分层。
//
// 写 settings.json 是所有 agent 都识别的最大公约数。
var targets = []Target{
	{"CC", "~/.claude/settings.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.json", "settings"},
	{"Codex", "~/.codex/hooks.json", "settings"},
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
