package main

import (
	"encoding/json"
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

// TestFormatTop_HooksOnlyUnwrapsHooksKey verifies that the Codex-style
// "hooks-only" format unwraps the internal {"hooks": {...}} envelope so
// that the on-disk JSON has event names at the top level (Codex's native
// schema for ~/.codex/hooks.json).
func TestFormatTop_HooksOnlyUnwrapsHooksKey(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{map[string]any{"matcher": "*"}},
		},
	}
	top := formatTop("hooks-only", settings)
	m, ok := top.(map[string]any)
	if !ok {
		t.Fatalf("top should be a map, got %T", top)
	}
	if _, hasWrapper := m["hooks"]; hasWrapper {
		t.Error("hooks-only output must not contain a top-level \"hooks\" key")
	}
	if _, hasStop := m["Stop"]; !hasStop {
		t.Error("hooks-only output should expose event names at top level (missing Stop)")
	}
}

// TestFormatTop_SettingsKeepsWrapper verifies the CC-family "settings"
// format leaves the {"hooks": {...}} envelope intact.
func TestFormatTop_SettingsKeepsWrapper(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{"Stop": []any{}},
		"permissions": map[string]any{
			"allow": []any{"Bash(ls:*)"},
		},
	}
	top := formatTop("settings", settings)
	m, ok := top.(map[string]any)
	if !ok {
		t.Fatalf("top should be a map, got %T", top)
	}
	if _, ok := m["hooks"]; !ok {
		t.Error("settings format should retain top-level \"hooks\" wrapper")
	}
	if _, ok := m["permissions"]; !ok {
		t.Error("settings format should retain user's other keys (permissions)")
	}
}

// TestFormatTop_HooksOnlyEmpty ensures that empty internal settings still
// produce a valid empty-object top-level for hooks-only format.
func TestFormatTop_HooksOnlyEmpty(t *testing.T) {
	top := formatTop("hooks-only", map[string]any{})
	buf, err := json.Marshal(top)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(buf) != "{}" {
		t.Errorf("empty hooks-only should marshal to {}, got %s", string(buf))
	}
}

// TestTargets_CodexHooksOnly pins the Codex target's contract: ~/.codex/hooks.json
// with the hooks-only format. If this test breaks, the installer's Codex
// integration needs revisiting.
func TestTargets_CodexHooksOnly(t *testing.T) {
	var codex *Target
	for i := range targets {
		if targets[i].AgentLabel == "Codex" {
			codex = &targets[i]
			break
		}
	}
	if codex == nil {
		t.Fatal("Codex target missing from targets slice")
	}
	if codex.SettingsFile != "~/.codex/hooks.json" {
		t.Errorf("Codex SettingsFile = %q, want ~/.codex/hooks.json", codex.SettingsFile)
	}
	if codex.Format != "hooks-only" {
		t.Errorf("Codex Format = %q, want hooks-only", codex.Format)
	}
	// And Codex must NOT appear in the legacy cleanup list (no settings.json predecessor).
	for _, l := range legacyCleanupTargets {
		if l.AgentLabel == "Codex" {
			t.Error("Codex must not be in legacyCleanupTargets — it had no pre-Phase-1 settings.json schema")
		}
	}
}
