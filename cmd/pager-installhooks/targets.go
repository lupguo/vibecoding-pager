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

// legacyCleanupTargets 列出"过去的写入位置"，每次安装都扫一遍把里面的
// pager 条目清掉（用户其他配置原样保留），避免新旧位置双发 hook。
//
// 早期 installer 误把 hooks 写到 ~/.<agent>/settings.local.json（按 Anthropic
// per-project 配置文件命名误推）。那些位置在 user-level 不会被任何 agent 加载，
// 留在那里是死代码 — 但万一未来 fork 真支持了会变双发。
//
// Codex 不在列表 — 原生只读 ~/.codex/hooks.json，没有分层。
var legacyCleanupTargets = []Target{
	{"CC", "~/.claude/settings.local.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.local.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.local.json", "settings"},
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
