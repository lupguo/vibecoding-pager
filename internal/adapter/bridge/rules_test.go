package bridge

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ─── Rules v3 加载与 Lookup ───────────────────────────────────────────────────

func TestLoadRules_Version(t *testing.T) {
	rules, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if rules.Version != 3 {
		t.Errorf("Version = %d, want 3", rules.Version)
	}
}

func TestLoadRules_ClaudeBucketPopulated(t *testing.T) {
	rules, _ := LoadRules()
	if _, ok := rules.Claude["PreToolUse"]; !ok {
		t.Error("rules.Claude missing PreToolUse")
	}
	if _, ok := rules.Claude["Stop"]; !ok {
		t.Error("rules.Claude missing Stop")
	}
}

func TestLookup_ClaudePreToolUseIsMappingNode(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("claude-code", "PreToolUse")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.MappingNode {
		t.Errorf("Kind = %v, want MappingNode", n.Kind)
	}
}

func TestLookup_ClaudeStopIsScalarNode(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("claude-code", "Stop")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.ScalarNode {
		t.Errorf("Kind = %v, want ScalarNode", n.Kind)
	}
}

func TestLookup_PermissionRequestEqualsPreToolUse(t *testing.T) {
	rules, _ := LoadRules()
	pre, _ := rules.Lookup("claude-code", "PreToolUse")
	pr, _ := rules.Lookup("claude-code", "PermissionRequest")
	if pre.Kind != pr.Kind {
		t.Errorf("PreToolUse.Kind=%v, PermissionRequest.Kind=%v", pre.Kind, pr.Kind)
	}
	if len(pre.Content) != len(pr.Content) {
		t.Errorf("PreToolUse content len=%d, PermissionRequest content len=%d",
			len(pre.Content), len(pr.Content))
	}
}

func TestLookup_UnknownAgentReturnsFalse(t *testing.T) {
	rules, _ := LoadRules()
	if _, ok := rules.Lookup("nonsense", "Stop"); ok {
		t.Error("expected ok=false for unknown agent")
	}
}

func TestLookup_UnknownEventReturnsFalse(t *testing.T) {
	rules, _ := LoadRules()
	if _, ok := rules.Lookup("claude-code", "NoSuchEvent"); ok {
		t.Error("expected ok=false for unknown event")
	}
}

// ─── PickTemplate ─────────────────────────────────────────────────────────────

func TestPickTemplate_ScalarNode_ReturnsValue(t *testing.T) {
	rules, _ := LoadRules()
	n, _ := rules.Lookup("claude-code", "Stop")
	got := PickTemplate(n, "Bash")
	if got != "{$.last_assistant_message} // {$.stop_reason}" {
		t.Errorf("got %q", got)
	}
}

func TestPickTemplate_MappingNode_ToolHit(t *testing.T) {
	rules, _ := LoadRules()
	n, _ := rules.Lookup("claude-code", "PreToolUse")
	got := PickTemplate(n, "Bash")
	if got != "{$.tool_input.command}" {
		t.Errorf("got %q", got)
	}
}

func TestPickTemplate_MappingNode_FallsBackToDefault(t *testing.T) {
	rules, _ := LoadRules()
	n, _ := rules.Lookup("claude-code", "PreToolUse")
	got := PickTemplate(n, "UnknownTool")
	if got != "{tool_name}" {
		t.Errorf("got %q, want default template", got)
	}
}

// ─── Basic substitution ──────────────────────────────────────────────────────

func TestEvaluate_LiteralOnly(t *testing.T) {
	if got := Evaluate("hello world", []byte(`{}`), "Bash"); got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_SimplePath(t *testing.T) {
	if got := Evaluate("{$.command}", []byte(`{"command":"go test"}`), "Bash"); got != "go test" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_PathWithLiteralPrefix(t *testing.T) {
	got := Evaluate("编辑 {$.file_path}", []byte(`{"file_path":"/src/main.go"}`), "Edit")
	if got != "编辑 /src/main.go" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_NestedPath(t *testing.T) {
	got := Evaluate("{$.questions.0.question}", []byte(`{"questions":[{"question":"why?"}]}`), "AskUserQuestion")
	if got != "why?" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_MissingPathReturnsEmpty(t *testing.T) {
	if got := Evaluate("{$.missing}", []byte(`{"command":"x"}`), "Bash"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── Fallback / value / tool_name ────────────────────────────────────────────

func TestEvaluate_FallbackOperator_SecondWins(t *testing.T) {
	got := Evaluate("{$.description // $.prompt}", []byte(`{"prompt":"P"}`), "Agent")
	if got != "P" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FallbackOperator_FirstWins(t *testing.T) {
	got := Evaluate("{$.description // $.prompt}", []byte(`{"description":"D","prompt":"P"}`), "Agent")
	if got != "D" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FallbackOperator_BothEmpty(t *testing.T) {
	if got := Evaluate("{$.a // $.b}", []byte(`{}`), "Agent"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestEvaluate_ToolNameToken(t *testing.T) {
	if got := Evaluate("{tool_name}", []byte(`{}`), "MyTool"); got != "MyTool" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_ValueToken_StringPayload(t *testing.T) {
	if got := Evaluate("{value}", []byte(`"hello"`), "AskUserQuestion"); got != "hello" {
		t.Errorf("got %q", got)
	}
}

// ─── Filters ─────────────────────────────────────────────────────────────────

func TestEvaluate_FilterFirstline(t *testing.T) {
	got := Evaluate("{$.stdout|firstline}", []byte(`{"stdout":"line1\nline2"}`), "Bash")
	if got != "line1" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterFirstline_NoNewline(t *testing.T) {
	got := Evaluate("{$.stdout|firstline}", []byte(`{"stdout":"single line"}`), "Bash")
	if got != "single line" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterFirstpara(t *testing.T) {
	got := Evaluate("{$.text|firstpara}", []byte(`{"text":"p1 line1\np1 line2\n\np2"}`), "Agent")
	if got != "p1 line1\np1 line2" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterDefault_PathPresent(t *testing.T) {
	got := Evaluate("{$.exitCode|default:0}", []byte(`{"exitCode":7}`), "Bash")
	if got != "7" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterDefault_PathMissing(t *testing.T) {
	got := Evaluate("{$.exitCode|default:0}", []byte(`{}`), "Bash")
	if got != "0" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterBool_True(t *testing.T) {
	got := Evaluate("{$.success|bool:✓写入,✗失败}", []byte(`{"success":true}`), "Edit")
	if got != "✓写入" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterBool_False(t *testing.T) {
	got := Evaluate("{$.success|bool:✓写入,✗失败}", []byte(`{"success":false}`), "Edit")
	if got != "✗失败" {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_FilterChain_FullBashPostRule(t *testing.T) {
	got := Evaluate(
		"{$.stdout|firstline} (exit={$.exitCode|default:0})",
		[]byte(`{"stdout":"hello\nworld","exitCode":0}`),
		"Bash",
	)
	if got != "hello (exit=0)" {
		t.Errorf("got %q", got)
	}
}

// ─── <json:N> raw token ──────────────────────────────────────────────────────

func TestEvaluate_JSONTruncate_Short(t *testing.T) {
	if got := Evaluate("<json:120>", []byte(`{"k":"v"}`), "X"); got != `{"k":"v"}` {
		t.Errorf("got %q", got)
	}
}

func TestEvaluate_JSONTruncate_Long(t *testing.T) {
	long := []byte(`{"k":"` + strings.Repeat("a", 300) + `"}`)
	got := Evaluate("<json:50>", long, "X")
	if !strings.HasSuffix(got, "…") {
		t.Errorf("got %q, want suffix …", got)
	}
	if r := []rune(got); len(r) != 51 {
		t.Errorf("rune length = %d, want 51", len(r))
	}
}

func TestEvaluate_JSONTruncate_Embedded(t *testing.T) {
	got := Evaluate("prefix:<json:5> suffix", []byte(`{"k":"v"}`), "X")
	want := `prefix:{"k":… suffix`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ─── Branch coverage: unknown tokens / malformed input ───────────────────────

func TestEvaluate_UnknownBaseToken(t *testing.T) {
	// Token body that's neither $.path, tool_name, nor value should yield empty.
	// Protects against typos in rule YAML.
	if got := Evaluate("{foobar}", []byte(`{"foo":"bar"}`), "X"); got != "" {
		t.Errorf("got %q, want empty for unknown base token", got)
	}
}

func TestEvaluate_UnknownRawToken(t *testing.T) {
	// <…> form other than json:N should yield empty.
	if got := Evaluate("<base64:50>", []byte(`{"k":"v"}`), "X"); got != "" {
		t.Errorf("got %q, want empty for unknown raw token", got)
	}
}

func TestEvaluate_UnmatchedBrace(t *testing.T) {
	// Unmatched { is written literally; the rest of the rule continues.
	if got := Evaluate("foo {bar", []byte(`{}`), "X"); got != "foo {bar" {
		t.Errorf("got %q, want literal pass-through", got)
	}
}

func TestEvaluate_FilterFirstline_EmptyStdout(t *testing.T) {
	// Path exists but value is empty string. Critical case: this is the exact
	// "PostToolUse with empty stdout" pattern that motivated the bugfix.
	got := Evaluate("{$.stdout|firstline}", []byte(`{"stdout":""}`), "Bash")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestEvaluate_FilterChain_DefaultThenFirstline(t *testing.T) {
	// |default:N triggers when path missing; the synthesized value then flows
	// through subsequent filters. Verifies exists-flag propagation.
	got := Evaluate("{$.stdout|default:fallback line\nignored|firstline}", []byte(`{}`), "Bash")
	if got != "fallback line" {
		t.Errorf("got %q, want %q", got, "fallback line")
	}
}

// TestEvaluate_FallbackOperator_DocumentsHTTPURLLimitation pins the documented
// constraint that "//" inside a token body is reserved for the fallback
// operator. Future authors trying to use raw URL literals like "http://x" in
// a default: filter argument will trip this — by design.
//
// Trace for input "{$.url|default:http://x}" against payload "{}":
//   1. Token body "$.url|default:http://x" contains "//" → fallback chain
//   2. Split on "//": ["$.url|default:http:", "/x"]
//   3. Recurse on first part "$.url|default:http:":
//      - $.url path missing (exists=false)
//      - filter default:http: triggers, returns "http:"
//   4. First non-empty wins → "http:"
//
// The user wanted "http://x". They got "http:". The // got eaten as a
// fallback delimiter. This test exists so any future change to the parse
// order of // vs | gets caught and forces a deliberate decision.
func TestEvaluate_FallbackOperator_DocumentsHTTPURLLimitation(t *testing.T) {
	got := Evaluate("{$.url|default:http://x}", []byte(`{}`), "X")
	if got != "http:" {
		t.Errorf("got %q, want %q — // constraint may have changed; update doc + DSL", got, "http:")
	}
}

// ─── 链式 filter ──────────────────────────────────────────────────────────────

func TestEvaluate_ChainedFilters(t *testing.T) {
	got := Evaluate(
		"{$.tool_calls|pluck:tool_name|join:, }",
		[]byte(`{"tool_calls":[{"tool_name":"Bash"},{"tool_name":"Read"},{"tool_name":"Grep"}]}`),
		"PostToolBatch",
	)
	if got != "Bash, Read, Grep" {
		t.Errorf("got %q, want %q", got, "Bash, Read, Grep")
	}
}

func TestEvaluate_ChainedFilters_EmptyArray(t *testing.T) {
	got := Evaluate(
		"{$.tool_calls|pluck:tool_name|join:, }",
		[]byte(`{"tool_calls":[]}`),
		"PostToolBatch",
	)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── prefix filter ────────────────────────────────────────────────────────────

func TestEvaluate_PrefixFilter_NonEmpty(t *testing.T) {
	got := Evaluate("{$.command_args|prefix: }", []byte(`{"command_args":"foo"}`), "")
	if got != " foo" {
		t.Errorf("got %q, want %q", got, " foo")
	}
}

func TestEvaluate_PrefixFilter_Empty(t *testing.T) {
	got := Evaluate("{$.command_args|prefix: }", []byte(`{"command_args":""}`), "")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestEvaluate_PrefixFilter_MissingField(t *testing.T) {
	got := Evaluate("{$.x|prefix:[]}", []byte(`{}`), "")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── lookup filter ────────────────────────────────────────────────────────────

func TestEvaluate_LookupFilter_Hit(t *testing.T) {
	got := Evaluate(
		"{$.notification_type|lookup:permission_prompt=[等授权] ,idle_prompt=[等输入] ,default=}",
		[]byte(`{"notification_type":"permission_prompt"}`),
		"",
	)
	if got != "[等授权] " {
		t.Errorf("got %q, want %q", got, "[等授权] ")
	}
}

func TestEvaluate_LookupFilter_Miss_HasDefault(t *testing.T) {
	got := Evaluate(
		"{$.x|lookup:foo=F,default=DEFAULT}",
		[]byte(`{"x":"bar"}`),
		"",
	)
	if got != "DEFAULT" {
		t.Errorf("got %q, want %q", got, "DEFAULT")
	}
}

func TestEvaluate_LookupFilter_Miss_NoDefault(t *testing.T) {
	got := Evaluate("{$.x|lookup:foo=F}", []byte(`{"x":"bar"}`), "")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── pluck filter ─────────────────────────────────────────────────────────────

func TestEvaluate_PluckFilter_ReturnsArray(t *testing.T) {
	got := Evaluate(
		"{$.calls|pluck:name}",
		[]byte(`{"calls":[{"name":"a"},{"name":"b"}]}`),
		"",
	)
	if got != `["a","b"]` {
		t.Errorf("got %q, want %q", got, `["a","b"]`)
	}
}

func TestEvaluate_PluckFilter_EmptyArray(t *testing.T) {
	got := Evaluate("{$.calls|pluck:name}", []byte(`{"calls":[]}`), "")
	if got != `[]` && got != "" {
		t.Errorf("got %q, want [] or empty", got)
	}
}

// ─── join filter ──────────────────────────────────────────────────────────────

func TestEvaluate_JoinFilter_StringArray(t *testing.T) {
	got := Evaluate(`{$.arr|join:, }`, []byte(`{"arr":["x","y","z"]}`), "")
	if got != "x, y, z" {
		t.Errorf("got %q, want %q", got, "x, y, z")
	}
}

func TestEvaluate_JoinFilter_NotAnArray(t *testing.T) {
	got := Evaluate(`{$.s|join:,}`, []byte(`{"s":"foo"}`), "")
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

// ─── Codex bucket Lookup ──────────────────────────────────────────────────────

func TestLookup_CodexPreToolUseIsMappingNode(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("codex", "PreToolUse")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.MappingNode {
		t.Errorf("Kind = %v", n.Kind)
	}
}

func TestLookup_CodexSessionStartIsScalar(t *testing.T) {
	rules, _ := LoadRules()
	n, ok := rules.Lookup("codex", "SessionStart")
	if !ok {
		t.Fatal("Lookup miss")
	}
	if n.Kind != yaml.ScalarNode {
		t.Errorf("Kind = %v", n.Kind)
	}
	if n.Value != "{$.trigger}" {
		t.Errorf("Value = %q", n.Value)
	}
}
