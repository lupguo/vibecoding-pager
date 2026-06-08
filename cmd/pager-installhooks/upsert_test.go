package main

import (
	"testing"
)

func TestUpsert_FreshSettings(t *testing.T) {
	settings := map[string]any{}
	specs := []HookSpec{
		{Event: "PreToolUse", Matcher: "*"},
		{Event: "Stop"},
	}
	out := upsertHooks(settings, "CC", specs, "/bin/pager-bridge")
	hooks := out["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("PreToolUse len = %d, want 1", len(pre))
	}
	g, _ := decodeGroupFromAny(pre[0])
	if g.Matcher != "*" {
		t.Errorf("matcher = %q, want *", g.Matcher)
	}
	if g.Hooks[0].Command != "/bin/pager-bridge --event PreToolUse --agent CC" {
		t.Errorf("command = %q", g.Hooks[0].Command)
	}
}

func TestUpsert_ReplacesLegacyEntry(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/old/path/vibecoding-pager-cc-bridge --event PreToolUse --agent CC",
						},
					},
					"matcher": "*",
				},
			},
		},
	}
	specs := []HookSpec{{Event: "PreToolUse", Matcher: "*"}}
	out := upsertHooks(settings, "CC", specs, "/new/pager-bridge")
	pre := out["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("PreToolUse len = %d, want 1 (legacy entry should be replaced)", len(pre))
	}
	g, _ := decodeGroupFromAny(pre[0])
	if g.Hooks[0].Command != "/new/pager-bridge --event PreToolUse --agent CC" {
		t.Errorf("command not replaced: %q", g.Hooks[0].Command)
	}
}

func TestUpsert_PreservesUserCustomEntry(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/usr/local/bin/my-custom-hook --foo",
						},
					},
					"matcher": "*",
				},
			},
		},
	}
	specs := []HookSpec{{Event: "PreToolUse", Matcher: "*"}}
	out := upsertHooks(settings, "CC", specs, "/bin/pager-bridge")
	pre := out["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 2 {
		t.Errorf("PreToolUse len = %d, want 2 (user + pager)", len(pre))
	}
	foundCustom := false
	for _, item := range pre {
		g, _ := decodeGroupFromAny(item)
		if len(g.Hooks) > 0 && g.Hooks[0].Command == "/usr/local/bin/my-custom-hook --foo" {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Error("user's custom hook was lost")
	}
}

func TestUpsert_Idempotent(t *testing.T) {
	settings := map[string]any{}
	specs := []HookSpec{{Event: "Stop"}}
	out1 := upsertHooks(settings, "CC", specs, "/bin/pager-bridge")
	out2 := upsertHooks(out1, "CC", specs, "/bin/pager-bridge")
	stop := out2["hooks"].(map[string]any)["Stop"].([]any)
	if len(stop) != 1 {
		t.Errorf("after 2nd run, Stop has %d entries, want 1", len(stop))
	}
}

func TestIsPagerHookGroup_CurrentBinary(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{{Command: "/usr/local/bin/pager-bridge --event Stop"}},
	}
	if !isPagerHookGroup(g) {
		t.Error("should detect pager-bridge")
	}
}

func TestIsPagerHookGroup_LegacyBinary(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{{Command: "/some/path/vibecoding-pager-cc-bridge --event Stop"}},
	}
	if !isPagerHookGroup(g) {
		t.Error("should detect legacy binary name")
	}
}

func TestIsPagerHookGroup_UserBinary(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{{Command: "/usr/local/bin/pager-bridge-helper --foo"}},
	}
	if isPagerHookGroup(g) {
		t.Error("should NOT match suffix-only matches")
	}
}

func TestRemoveAllPagerHooks_LegacyEntriesCleared(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/old/path/vibecoding-pager-cc-bridge --event PreToolUse --agent CC",
						},
					},
					"matcher": "*",
				},
			},
		},
		"permissions": map[string]any{
			"allow": []any{"Bash(ls:*)"},
		},
	}
	out := removeAllPagerHooks(settings)
	hooks := out["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("PreToolUse should be removed when only legacy pager entry existed")
	}
	// 用户其他配置（permissions）原样保留
	if _, ok := out["permissions"]; !ok {
		t.Error("permissions should be preserved")
	}
}

func TestRemoveAllPagerHooks_PreservesUserHooks(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				// pager 旧条目
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/old/vibecoding-pager-cc-bridge --event PreToolUse --agent CC",
						},
					},
				},
				// 用户自定义条目
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/usr/local/bin/my-tool",
						},
					},
				},
			},
		},
	}
	out := removeAllPagerHooks(settings)
	pre := out["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Errorf("PreToolUse len = %d, want 1 (user entry preserved)", len(pre))
	}
}

// TestTargets_Paths pins each agent's write target. CC-Internal in particular
// must write to settings.json (NOT settings.local.json) because the Tencent
// claude-code-internal v1.1.9 fork does not load .local.json overlays.
// Writing to .local.json appears to succeed but hooks never fire — the silent
// failure was the root cause of the 2026-06-08 CC-Internal regression.
func TestTargets_Paths(t *testing.T) {
	want := map[string]struct {
		settings string
		legacy   string // empty if none
	}{
		"CC":          {"~/.claude/settings.local.json", "~/.claude/settings.json"},
		"CC-Internal": {"~/.claude-internal/settings.json", "~/.claude-internal/settings.local.json"},
		"CodeBuddy":   {"~/.codebuddy/settings.local.json", "~/.codebuddy/settings.json"},
		"Codex":       {"~/.codex/hooks.json", ""},
	}
	for _, tgt := range targets {
		w, ok := want[tgt.AgentLabel]
		if !ok {
			t.Errorf("unexpected agent %q", tgt.AgentLabel)
			continue
		}
		if tgt.SettingsFile != w.settings {
			t.Errorf("%s SettingsFile = %q, want %q", tgt.AgentLabel, tgt.SettingsFile, w.settings)
		}
		if tgt.Format != "settings" {
			t.Errorf("%s Format = %q, want settings", tgt.AgentLabel, tgt.Format)
		}
	}
	for label, w := range want {
		if w.legacy == "" {
			// Codex must NOT have a legacy entry.
			for _, l := range legacyCleanupTargets {
				if l.AgentLabel == label {
					t.Errorf("%s should not have a legacy cleanup entry but found %q", label, l.SettingsFile)
				}
			}
			continue
		}
		var got string
		for _, l := range legacyCleanupTargets {
			if l.AgentLabel == label {
				got = l.SettingsFile
				break
			}
		}
		if got != w.legacy {
			t.Errorf("%s legacy cleanup path = %q, want %q", label, got, w.legacy)
		}
	}
}
