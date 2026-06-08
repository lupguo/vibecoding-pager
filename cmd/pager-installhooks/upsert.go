package main

import (
	"errors"
	"path/filepath"
	"strings"
)

const (
	legacyBinaryName  = "vibecoding-pager-cc-bridge"
	currentBinaryName = "pager-bridge"
)

var errNotMap = errors.New("hook entry is not a map")

// HookEntry 是 hooks JSON 中单个 hook 命令条目。
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	Async   bool   `json:"async,omitempty"`
}

// HookGroup 是同一个事件下挂的一组 hook。
type HookGroup struct {
	Hooks   []HookEntry `json:"hooks"`
	Matcher string      `json:"matcher,omitempty"`
}

// HookSpec 是从 pager-bridge --print-hooks 接收的事件描述。
type HookSpec struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
}

// isPagerHookGroup 判断 settings 中的某个 hook group 是否由 pager 写入。
// 用 basename 严格匹配，避免误判用户自己的 /custom/pager-bridge-helper 之类。
func isPagerHookGroup(g HookGroup) bool {
	for _, h := range g.Hooks {
		parts := strings.Fields(h.Command)
		if len(parts) == 0 {
			continue
		}
		base := filepath.Base(parts[0])
		if base == currentBinaryName || base == legacyBinaryName {
			return true
		}
	}
	return false
}

// buildHookGroup 构造一个新的 pager hook group。
//
// async 字段是 per-agent 行为：
//   - CC / CC-Internal / CodeBuddy 支持 `async: true`，让 hook 完全非阻塞（fire-and-forget）。
//   - Codex 0.137 还不支持 async（codex-rs/hooks/src/engine/discovery.rs 会
//     "skipping async hook ..."），整条 hook 会被丢弃。所以 Codex 必须用 sync 形式
//     —— 配合 5s timeout，HTTP POST 在 localhost 上也就十几毫秒，不会真的卡 codex turn。
func buildHookGroup(spec HookSpec, agentLabel, bridgePath string) HookGroup {
	cmd := bridgePath + " --event " + spec.Event + " --agent " + agentLabel
	supportsAsync := agentLabel != "Codex"
	g := HookGroup{
		Hooks: []HookEntry{
			{Type: "command", Command: cmd, Timeout: 5, Async: supportsAsync},
		},
	}
	if spec.Matcher != "" {
		g.Matcher = spec.Matcher
	}
	return g
}

// upsertHooks 把 specs 的 pager 条目写到 settings.hooks 下，保留用户自定义条目。
// 同一事件下有 pager 旧条目（按 binary basename 识别）的，先剔除再 append。
// 入参 settings 可能是 nil-map；返回的 settings 永远非 nil。
func upsertHooks(settings map[string]any, agentLabel string, specs []HookSpec, bridgePath string) map[string]any {
	if settings == nil {
		settings = map[string]any{}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, spec := range specs {
		var existing []any
		if v, ok := hooks[spec.Event].([]any); ok {
			existing = v
		}
		filtered := make([]any, 0, len(existing))
		for _, item := range existing {
			g, err := decodeGroupFromAny(item)
			if err == nil && isPagerHookGroup(g) {
				continue
			}
			filtered = append(filtered, item)
		}
		newGroup := buildHookGroup(spec, agentLabel, bridgePath)
		filtered = append(filtered, hookGroupToAny(newGroup))
		hooks[spec.Event] = filtered
	}
	settings["hooks"] = hooks
	return settings
}

// decodeGroupFromAny 把 JSON 反序列化产生的 map[string]any 转为 typed HookGroup。
func decodeGroupFromAny(item any) (HookGroup, error) {
	m, ok := item.(map[string]any)
	if !ok {
		return HookGroup{}, errNotMap
	}
	var g HookGroup
	if hs, ok := m["hooks"].([]any); ok {
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			entry := HookEntry{
				Type:    asString(hm["type"]),
				Command: asString(hm["command"]),
				Timeout: asInt(hm["timeout"]),
				Async:   asBool(hm["async"]),
			}
			g.Hooks = append(g.Hooks, entry)
		}
	}
	g.Matcher = asString(m["matcher"])
	return g, nil
}

// hookGroupToAny 把 typed HookGroup 转为 map[string]any，便于写回 settings。
func hookGroupToAny(g HookGroup) map[string]any {
	hooks := make([]any, 0, len(g.Hooks))
	for _, h := range g.Hooks {
		entry := map[string]any{"type": h.Type, "command": h.Command}
		if h.Timeout > 0 {
			entry["timeout"] = h.Timeout
		}
		if h.Async {
			entry["async"] = h.Async
		}
		hooks = append(hooks, entry)
	}
	out := map[string]any{"hooks": hooks}
	if g.Matcher != "" {
		out["matcher"] = g.Matcher
	}
	return out
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
