package bridge

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
)

//go:embed extract_rules.yaml
var rulesYAML []byte

// Rules 表示解析后的 extract_rules.yaml v3。
// 顶层是 agent → 事件名 → 模板节点的二级 map。
// 每个事件下挂的 yaml.Node 是 ScalarNode（单 string 模板）或
// MappingNode（per-tool 分桶 + default）。
type Rules struct {
	Version int                  `yaml:"version"`
	Claude  map[string]yaml.Node `yaml:"claude"`
	Codex   map[string]yaml.Node `yaml:"codex"`
}

// LoadRules parses the embedded yaml.
func LoadRules() (*Rules, error) {
	var r Rules
	if err := yaml.Unmarshal(rulesYAML, &r); err != nil {
		return nil, err
	}
	r.resolveAliases()
	return &r, nil
}

// resolveAliases replaces AliasNode entries in each bucket with a copy of the
// node the alias points to. yaml.v3 stores aliases as AliasNode when the
// target type is map[string]yaml.Node; this post-load pass normalises them so
// callers always see MappingNode or ScalarNode.
func (r *Rules) resolveAliases() {
	for _, bucket := range []map[string]yaml.Node{r.Claude, r.Codex} {
		for k, v := range bucket {
			if v.Kind == yaml.AliasNode && v.Alias != nil {
				bucket[k] = *v.Alias
			}
		}
	}
}

// Lookup 返回 (agentID, eventType) 对应的 yaml.Node。
// agentID 必须是 entity.AgentClaudeCode / AgentCodex 中的常量值。
// 未命中返回 (zero, false)。
func (r *Rules) Lookup(agentID, eventType string) (yaml.Node, bool) {
	var bucket map[string]yaml.Node
	switch agentID {
	case "claude-code":
		bucket = r.Claude
	case "codex":
		bucket = r.Codex
	default:
		return yaml.Node{}, false
	}
	n, ok := bucket[eventType]
	return n, ok
}

// PickTemplate 在 MappingNode 中按 toolName 查模板，找不到回落 "default"。
// ScalarNode 直接返回 .Value，忽略 toolName。
// 既找不到 toolName 又没有 default 时返回空串。
func PickTemplate(n yaml.Node, toolName string) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.MappingNode:
		var defaultTmpl string
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == toolName {
				return v.Value
			}
			if k.Value == "default" {
				defaultTmpl = v.Value
			}
		}
		return defaultTmpl
	}
	return ""
}

// Evaluate executes a rule template against a JSON payload.
//
// Token forms:
//   - "{…}"    standard substitution (resolveToken)
//   - "<…>"    raw-form, currently only "<json:N>"
//
// Supported {…} bodies (cumulative DSL):
//   - "$.path"             gjson path
//   - "$.a // $.b"         first non-empty fallback chain
//   - "tool_name"          literal toolName arg
//   - "value"              raw payload (when payload is a JSON string literal)
//   - "$.path|filter[:arg]" filtered value
//
// Filters: firstline, firstpara, default:N, bool:T,F
//
// Failure-mode: missing paths, unknown tokens, malformed JSON all yield empty
// string for the substitution. Caller is responsible for any final fallback.
func Evaluate(rule string, payload []byte, toolName string) string {
	var b strings.Builder
	i := 0
	for i < len(rule) {
		switch rule[i] {
		case '{':
			end := strings.IndexByte(rule[i:], '}')
			if end == -1 {
				b.WriteByte(rule[i])
				i++
				continue
			}
			b.WriteString(resolveToken(rule[i+1:i+end], payload, toolName))
			i += end + 1
		case '<':
			end := strings.IndexByte(rule[i:], '>')
			if end == -1 {
				b.WriteByte(rule[i])
				i++
				continue
			}
			b.WriteString(resolveRawToken(rule[i+1:i+end], payload))
			i += end + 1
		default:
			b.WriteByte(rule[i])
			i++
		}
	}
	return b.String()
}

// resolveToken handles a single {…} expression body.
//
// Constraint: the literal substring "//" inside a token is reserved for the
// fallback operator. Avoid embedding raw "//" in path expressions or filter
// arguments (e.g., do not write {$.url|default:http://x}); use a different
// representation or escape mechanism if such literals are ever needed.
func resolveToken(token string, payload []byte, toolName string) string {
	token = strings.TrimLeft(token, " \t")

	// Fallback chain: split on " // " and return first non-empty.
	if strings.Contains(token, "//") {
		parts := strings.Split(token, "//")
		for _, p := range parts {
			v := resolveToken(strings.TrimSpace(p), payload, toolName)
			if v != "" {
				return v
			}
		}
		return ""
	}

	// Filter pipeline: split off filters first.
	pipe := strings.Split(token, "|")
	base := strings.TrimSpace(pipe[0])

	var raw string
	exists := true
	switch base {
	case "tool_name":
		raw = toolName
	case "value":
		// gjson @this returns the unquoted string when payload is a JSON string.
		raw = gjson.GetBytes(payload, "@this").String()
	default:
		if strings.HasPrefix(base, "$.") {
			res := gjson.GetBytes(payload, base[2:])
			exists = res.Exists()
			raw = res.String()
		} else {
			// Unknown base token.
			return ""
		}
	}

	for _, f := range pipe[1:] {
		raw, exists = applyFilter(strings.TrimLeft(f, " \t"), raw, exists)
	}
	return raw
}

// applyFilter mutates a value through one filter step. The "exists" flag is
// propagated so |default:X fires only when the upstream path is missing
// (not merely empty).
func applyFilter(filter, value string, exists bool) (string, bool) {
	name, arg, _ := strings.Cut(filter, ":")
	name = strings.TrimRight(name, " \t")
	switch name {
	case "firstline":
		if i := strings.IndexByte(value, '\n'); i != -1 {
			return value[:i], exists
		}
		return value, exists
	case "firstpara":
		if i := strings.Index(value, "\n\n"); i != -1 {
			return value[:i], exists
		}
		return value, exists
	case "default":
		if !exists {
			return arg, true
		}
		return value, exists
	case "bool":
		t, f, _ := strings.Cut(arg, ",")
		if value == "true" {
			return t, exists
		}
		return f, exists
	case "prefix":
		// prefix:<lit> — prepend lit if value non-empty; empty stays empty.
		if value == "" {
			return "", exists
		}
		return arg + value, exists
	case "lookup":
		// lookup:k1=v1,k2=v2,default=dv — switch table on value.
		table, def := parseLookupArg(arg)
		if v, ok := table[value]; ok {
			return v, true
		}
		return def, true
	case "pluck":
		// pluck:<field> — value should be a JSON array; extract field from each element.
		// gjson's "#.field" projection returns a JSON array of values.
		res := gjson.Get(value, "#."+arg)
		if !res.Exists() {
			return "", false
		}
		return res.Raw, true
	case "join":
		// join:<sep> — value should be a JSON array of strings; join with sep.
		// Non-array values pass through unchanged.
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err != nil {
			return value, exists
		}
		return strings.Join(arr, arg), exists
	}
	return value, exists
}

// parseLookupArg parses "k1=v1,k2=v2,default=dv" into (table, default).
// Special key "default" goes into the default; other keys go into the table.
// Both keys and values may be empty strings.
func parseLookupArg(arg string) (map[string]string, string) {
	table := map[string]string{}
	def := ""
	for _, pair := range strings.Split(arg, ",") {
		idx := strings.Index(pair, "=")
		if idx < 0 {
			continue
		}
		k, v := pair[:idx], pair[idx+1:]
		if k == "default" {
			def = v
			continue
		}
		table[k] = v
	}
	return table, def
}

// resolveRawToken handles <…> tokens. Currently only "json:N".
//
// "json:N" → JSON.stringify the payload, take first N runes, append "…" if
// truncated. Used as the last-resort fallback in extract_rules.yaml.
func resolveRawToken(token string, payload []byte) string {
	if strings.HasPrefix(token, "json:") {
		n := atoiOrZero(token[len("json:"):])
		s := string(payload)
		runes := []rune(s)
		if n <= 0 || len(runes) <= n {
			return s
		}
		return string(runes[:n]) + "…"
	}
	return ""
}

// atoiOrZero parses a non-negative int; returns 0 on any error.
func atoiOrZero(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
