package main

import "os"

// Target 描述一个 agent 对应的写入目标。
type Target struct {
	AgentLabel   string
	SettingsFile string // 路径，~ 会展开
	Format       string // 当前只有 "settings"：顶层 {"hooks": {<event>: [...]}}
	                    // CC/CodeBuddy 写到 settings.local.json，Codex 写到 hooks.json，
	                    // schema 完全一致（codex-rs/config/src/hook_config.rs::HooksFile 即 {"hooks": ...}）
}

// targets 是 installer 的目标清单。
// 所有 4 个 agent 都用同一份 schema：顶层一个 "hooks" 对象，下面挂 event → MatcherGroup[]。
//
// 写入位置选 settings.json (而不是 settings.local.json)：
//   - Anthropic CC v2 文档（https://code.claude.com/docs/en/hooks）只在
//     **.claude/settings.local.json**（per-project、git 仓库内）这一档
//     列了 .local.json，user-level 只有 ~/.claude/settings.json。
//     ~/.claude/settings.local.json 不在加载列表里。
//   - CC-Internal (Tencent fork v1.1.9) 经验证不读 .local.json 覆盖层，
//     hooks 静默不触发。
//   - CodeBuddy 共享同一加载逻辑。
//   - Codex 走自己的 ~/.codex/hooks.json，跟这条无关。
//
// 写入 user-global settings.json 是所有 agent 都识别的最大公约数。
var targets = []Target{
	{"CC", "~/.claude/settings.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.json", "settings"},
	{"Codex", "~/.codex/hooks.json", "settings"},
}

// legacyCleanupTargets 列出"过去的写入位置"，每次安装都扫一遍把里面的
// pager 条目清掉（用户其他配置原样保留），避免新旧位置双发 hook。
//
// 早期版本错写到 ~/.claude/settings.local.json 等位置（按 Anthropic
// per-project 配置文件的命名误推）。这些位置 user-level 不被加载，pager
// 条目留在里面是死代码，但万一未来 fork 真支持了，会变双发。
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
