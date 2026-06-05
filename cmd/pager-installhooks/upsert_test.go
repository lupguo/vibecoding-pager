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
