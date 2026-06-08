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
// CC-Internal 例外：Tencent 的 claude-code-internal fork 不继承 Anthropic 上游
// 对 settings.local.json 的支持（v1.1.9 验证过），只读 settings.json。
// 写到 .local.json 看起来一切正常但运行时 hooks 永远不触发——
// 这种"沉默失败"很难诊断，所以这里直接写 settings.json，并由 legacyCleanup
// 清掉 .local.json 残留以免老用户升级后双发。
var targets = []Target{
	{"CC", "~/.claude/settings.local.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.json", "settings"},
	{"CodeBuddy", "~/.codebuddy/settings.local.json", "settings"},
	{"Codex", "~/.codex/hooks.json", "settings"},
}

// legacyCleanupTargets 列出"过去的写入位置"，每次安装都扫一遍把里面的
// pager 条目清掉（用户其他配置原样保留），避免新旧位置双发 hook。
//
// - CC / CodeBuddy: 老 installer 直接写 settings.json，现在写 .local.json
// - CC-Internal:    *反向* — 早期版本错写 .local.json (v1.1.9 fork 不读它，hooks 静默失效)，
//                   现在改回 settings.json。
// - Codex:           不在列表，原生只读 ~/.codex/hooks.json，没有分层。
var legacyCleanupTargets = []Target{
	{"CC", "~/.claude/settings.json", "settings"},
	{"CC-Internal", "~/.claude-internal/settings.local.json", "settings"},
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
