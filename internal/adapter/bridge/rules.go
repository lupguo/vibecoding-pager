package bridge

import (
	_ "embed"
	"strings"

	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
)

//go:embed extract_rules.yaml
var rulesYAML []byte

// Rules represents the parsed extract_rules.yaml. Post values are kept as
// yaml.Node so we can distinguish a plain string template from a structured
// rule (with string:/object:/object_paths:/fallback: keys) at evaluation time.
type Rules struct {
	Version int                  `yaml:"version"`
	Pre     map[string]string    `yaml:"pre"`
	Post    map[string]yaml.Node `yaml:"post"`
}

// LoadRules parses the embedded yaml.
func LoadRules() (*Rules, error) {
	var r Rules
	if err := yaml.Unmarshal(rulesYAML, &r); err != nil {
		return nil, err
	}
	return &r, nil
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
	token = strings.TrimSpace(token)

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
		raw, exists = applyFilter(strings.TrimSpace(f), raw, exists)
	}
	return raw
}

// applyFilter mutates a value through one filter step. The "exists" flag is
// propagated so |default:X fires only when the upstream path is missing
// (not merely empty).
func applyFilter(filter, value string, exists bool) (string, bool) {
	name, arg, _ := strings.Cut(filter, ":")
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
	}
	return value, exists
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
