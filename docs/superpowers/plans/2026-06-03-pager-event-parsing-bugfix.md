# Pager Event Parsing 4-Bug Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix four classes of event-parsing defects in `pager-cc-bridge`: (1) drop CodeBuddy's session-less `auth_success` Notification, (2) replace single-shot `ExtractContent` with phase-aware (Pre/Post) YAML rule-driven extractor that handles cross-agent `tool_response` schema differences via `gjson`, (3) prepend `[等授权]/[等输入]/[认证]` labels to Notification content, (4) repair stale binary deployment + missing field mappings (`SessionEnd.reason`, `SubagentStart.agent_id`).

**Architecture:** Single-process changes confined to `internal/adapter/bridge/` and `cmd/bridge/`. Pager main process untouched. Introduce `github.com/tidwall/gjson` + `gopkg.in/yaml.v3` scoped to the bridge package; rest of repo stays stdlib-only. Configuration-driven extraction via `extract_rules.yaml` (go:embed) + a minimal hand-written DSL evaluator (~120 lines, no template engine). Dev tooling closes the deployment gap that caused 5 of the 6 empty-content event types.

**Tech Stack:** Go 1.25, `github.com/tidwall/gjson` v1.18+, `gopkg.in/yaml.v3` v3.0.1, `embed` stdlib.

---

## File Structure

### New files
- `internal/adapter/bridge/extract_rules.yaml` — Pre/Post extraction rules per tool (go:embed source)
- `internal/adapter/bridge/rules.go` — yaml loader + mini-DSL evaluator
- `internal/adapter/bridge/rules_test.go` — DSL unit tests

### Modified files
- `internal/adapter/bridge/types.go` — add `Reason string \`json:"reason"\``
- `internal/adapter/bridge/extractor.go` — rewrite `ExtractContent(phase, toolName, payload)`; add `notificationTypeLabel`; fix `SessionEnd` and `SubagentStart` cases; remove old per-tool switch; preserve MCP prefix-matching
- `internal/adapter/bridge/extractor_test.go` — migrate Pre tests to new signature; add Post tests for CC-INT vs CodeBuddy AskUserQuestion divergence; add SessionEnd / SubagentStart / Notification cases
- `cmd/bridge/main.go` — add `shouldDrop` guard; pass phase to `ExtractContent`
- `cmd/bridge/main_test.go` — add `TestShouldDrop_*`
- `go.mod` / `go.sum` — register gjson + yaml.v3
- `Makefile` — add `install-bridge` target; make `dev`/`run`/`build` depend on it
- `CLAUDE.md` — add "Bridge Binary Lifecycle" section

### Untouched
- `internal/domain/**`, `internal/wails/**`, `internal/infra/**`, `frontend/**`

---

## Task 1: Quick fixes + deps (Bug 1 / 3 / 4b / 4c + gjson/yaml)

**Scope:** Five small, independent changes that don't depend on the DSL machinery. Done first because they unblock testing the rest in isolation.

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `cmd/bridge/main.go`, `cmd/bridge/main_test.go`
- Modify: `internal/adapter/bridge/types.go`
- Modify: `internal/adapter/bridge/extractor.go`, `internal/adapter/bridge/extractor_test.go`

### Subtask 1A — Add dependencies

- [ ] **Run:**
```bash
go get github.com/tidwall/gjson@v1.18.0
go get gopkg.in/yaml.v3@v3.0.1
go mod tidy
```

### Subtask 1B — Bug 1: shouldDrop guard

- [ ] **Append to `cmd/bridge/main_test.go`:**

```go
import (
	"pager/internal/adapter/bridge"
)

// (add this import to the existing import block; do not duplicate package decl)

func TestShouldDrop_NotificationWithoutSession(t *testing.T) {
	in := &bridge.CCHookInput{SessionID: ""}
	if !shouldDrop("Notification", in) {
		t.Error("expected drop for Notification without session_id")
	}
}

func TestShouldDrop_NotificationWithSession(t *testing.T) {
	in := &bridge.CCHookInput{SessionID: "abc-123"}
	if shouldDrop("Notification", in) {
		t.Error("expected keep for Notification with session_id")
	}
}

func TestShouldDrop_PreToolUseWithoutSession(t *testing.T) {
	in := &bridge.CCHookInput{SessionID: ""}
	if shouldDrop("PreToolUse", in) {
		t.Error("expected keep for PreToolUse without session_id (fallback path)")
	}
}
```

- [ ] **In `cmd/bridge/main.go`, add below `isToolEvent`:**

```go
// shouldDrop returns true for events that have no actionable signal and would
// pollute the session list. Currently only filters CodeBuddy's auth_success
// Notification (which carries no session_id, would trigger the host:cwd:tty
// fallback, and make session_key look like a path).
func shouldDrop(eventType string, in *bridge.CCHookInput) bool {
	if eventType == "Notification" && in.SessionID == "" {
		return true
	}
	return false
}
```

- [ ] **Insert the guard call inside `main`**, immediately after `_ = json.Unmarshal(raw, &in)` and before the `var contentRaw, content string` declaration:

```go
	if shouldDrop(eventType, &in) {
		return
	}
```

### Subtask 1C — Bug 4b: SessionEnd.reason

- [ ] **In `internal/adapter/bridge/types.go`, add to the "Session layer" block:**

```go
	Reason       string `json:"reason"`        // SessionEnd: other/clear/compact/logout
```

- [ ] **In `internal/adapter/bridge/extractor.go`, replace:**

```go
	case "SessionEnd":
		return "", ""
```

with:

```go
	case "SessionEnd":
		return in.Reason, in.Reason
```

- [ ] **Append to `extractor_test.go`:**

```go
func TestExtractEventContent_SessionEnd_UsesReason(t *testing.T) {
	in := &CCHookInput{Reason: "clear"}
	raw, content := ExtractEventContent("SessionEnd", in)
	if raw != "clear" || content != "clear" {
		t.Errorf("got (%q, %q), want (clear, clear)", raw, content)
	}
}

func TestExtractEventContent_SessionEnd_EmptyReason(t *testing.T) {
	in := &CCHookInput{Reason: ""}
	raw, content := ExtractEventContent("SessionEnd", in)
	if raw != "" || content != "" {
		t.Errorf("expected empty, got (%q, %q)", raw, content)
	}
}
```

### Subtask 1D — Bug 4c: SubagentStart.agent_id fallback

- [ ] **In `internal/adapter/bridge/extractor.go`, replace:**

```go
	case "SubagentStart":
		return in.AgentType, in.AgentType
```

with:

```go
	case "SubagentStart":
		raw := firstNonEmpty(in.AgentType, in.AgentID)
		return raw, raw
```

- [ ] **Add helper at the bottom of `extractor.go` (it does not exist in this package — `cmd/bridge/main.go` has its own copy in package main):**

```go
// firstNonEmpty returns the first non-empty string from vals, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Append to `extractor_test.go`:**

```go
func TestExtractEventContent_SubagentStart_AgentTypePresent(t *testing.T) {
	in := &CCHookInput{AgentType: "general-purpose", AgentID: "agent-xyz"}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "general-purpose" {
		t.Errorf("raw = %q, want %q", raw, "general-purpose")
	}
}

func TestExtractEventContent_SubagentStart_AgentIDFallback(t *testing.T) {
	in := &CCHookInput{AgentType: "", AgentID: "agent-66639e99"}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "agent-66639e99" {
		t.Errorf("raw = %q, want %q", raw, "agent-66639e99")
	}
}

func TestExtractEventContent_SubagentStart_BothEmpty(t *testing.T) {
	in := &CCHookInput{AgentType: "", AgentID: ""}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "" {
		t.Errorf("raw = %q, want empty", raw)
	}
}
```

### Subtask 1E — Bug 3: Notification type label

- [ ] **In `internal/adapter/bridge/extractor.go`, replace:**

```go
	case "Notification":
		raw := in.Message
		if raw == "" {
			raw = in.NotificationType
		}
		return raw, truncateRunes(raw, contentMaxRunes)
```

with:

```go
	case "Notification":
		msg := in.Message
		if msg == "" {
			msg = in.NotificationType
		}
		raw := notificationTypeLabel(in.NotificationType) + msg
		return raw, truncateRunes(raw, contentMaxRunes)
```

- [ ] **Add helper at the bottom of `extractor.go` (next to `firstNonEmpty`):**

```go
// notificationTypeLabel returns a Chinese-bracketed prefix for known
// notification_type values, or empty string for unknown types.
func notificationTypeLabel(ntype string) string {
	switch ntype {
	case "permission_prompt":
		return "[等授权] "
	case "idle_prompt":
		return "[等输入] "
	case "auth_success":
		return "[认证] " // Bug 1 already drops these at bridge entry; defensive.
	default:
		return ""
	}
}
```

- [ ] **Append to `extractor_test.go`:**

```go
func TestExtractEventContent_Notification_PermissionPrompt(t *testing.T) {
	in := &CCHookInput{NotificationType: "permission_prompt", Message: "needs your permission to use Bash"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等授权] needs your permission to use Bash" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_IdlePrompt(t *testing.T) {
	in := &CCHookInput{NotificationType: "idle_prompt", Message: "CodeBuddy is waiting for your input"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等输入] CodeBuddy is waiting for your input" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_AuthSuccess(t *testing.T) {
	in := &CCHookInput{NotificationType: "auth_success", Message: "auth_success: example-user"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[认证] auth_success: example-user" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_UnknownType(t *testing.T) {
	in := &CCHookInput{NotificationType: "", Message: "raw message only"}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "raw message only" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractEventContent_Notification_EmptyMessageFallsBackToType(t *testing.T) {
	in := &CCHookInput{NotificationType: "idle_prompt", Message: ""}
	raw, _ := ExtractEventContent("Notification", in)
	if raw != "[等输入] idle_prompt" {
		t.Errorf("raw = %q", raw)
	}
}
```

### Verification

- [ ] **Run full test suite:**
```bash
go test ./...
```
Expected: all PASS, no regressions.

- [ ] **Run go vet:**
```bash
go vet ./...
```
Expected: clean.

### Commit

- [ ] **Single commit covering all 5 subtasks:**
```bash
git add go.mod go.sum cmd/bridge/main.go cmd/bridge/main_test.go \
        internal/adapter/bridge/types.go \
        internal/adapter/bridge/extractor.go \
        internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): four quick-win event-parsing fixes (Bug 1/3/4b/4c)

- Bug 1: drop CodeBuddy auth_success Notification (no session_id)
- Bug 3: prepend [等授权]/[等输入]/[认证] type label to Notification
- Bug 4b: SessionEnd content uses reason field (was hardcoded empty)
- Bug 4c: SubagentStart falls back to agent_id when agent_type missing

Also adds gjson + yaml.v3 deps used by Task 2's rule-driven extractor."
```

---

## Task 2: DSL evaluator + extract_rules.yaml

**Scope:** Build the rule-driven extraction machinery as a self-contained module. No callers wired up yet — Task 3 hooks it in. Validated entirely through unit tests.

**Files:**
- Create: `internal/adapter/bridge/extract_rules.yaml`
- Create: `internal/adapter/bridge/rules.go`
- Create: `internal/adapter/bridge/rules_test.go`

### Subtask 2A — extract_rules.yaml

- [ ] **Create `internal/adapter/bridge/extract_rules.yaml`:**

```yaml
version: 1

# Pre phase: extract action description from tool_input.
pre:
  Bash:            "{$.command}"
  Edit:            "编辑 {$.file_path}"
  Write:           "写入 {$.file_path}"
  Read:            "读取 {$.file_path}"
  Glob:            "查找 {$.pattern}"
  Grep:            "搜索 {$.pattern}"
  WebFetch:        "抓取 {$.url}"
  WebSearch:       "搜索 {$.query}"
  AskUserQuestion: "{$.questions.0.question}"
  Agent:           "子任务: {$.description // $.prompt}"
  Task:            "子任务: {$.description}"
  TaskCreate:      "{$.subject}"
  TaskUpdate:      "{$.subject} →{$.status}"
  NotebookEdit:    "{$.notebook_path}"
  LSP:             "LSP {$.operation}: {$.filePath}"
  default:         "{tool_name}"

# Post phase: extract result summary from tool_response.
post:
  Bash:            "{$.stdout|firstline} (exit={$.exitCode|default:0})"
  Read:            "已读 {$.file.numLines|default:?} 行"
  Edit:            "{$.success|bool:✓写入,✗失败} {$.filePath}"
  Write:           "{$.success|bool:✓写入,✗失败} {$.filePath}"
  AskUserQuestion:
    string:        "{value}"
    object:        "{$.answers.0}"
  Agent:           "{$.0.text|firstpara}"
  default:
    string:        "{value}"
    object_paths:
      - "$.answers.0"
      - "$.stdout"
      - "$.text"
      - "$.message"
      - "$.result"
      - "$.summary"
    fallback:      "<json:120>"
```

### Subtask 2B — rules.go (full DSL implementation)

- [ ] **Create `internal/adapter/bridge/rules.go`:**

```go
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
```

### Subtask 2C — rules_test.go (one test per DSL feature)

- [ ] **Create `internal/adapter/bridge/rules_test.go`:**

```go
package bridge

import (
	"strings"
	"testing"
)

// ─── Loader ──────────────────────────────────────────────────────────────────

func TestLoadRules_HasPreAndPost(t *testing.T) {
	rules, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if rules.Version != 1 {
		t.Errorf("rules.Version = %d, want 1", rules.Version)
	}
	if _, ok := rules.Pre["Bash"]; !ok {
		t.Error("rules.Pre missing Bash")
	}
	if _, ok := rules.Post["Bash"]; !ok {
		t.Error("rules.Post missing Bash")
	}
	if _, ok := rules.Post["AskUserQuestion"]; !ok {
		t.Error("rules.Post missing AskUserQuestion")
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
```

### Verification

- [ ] **Run rules tests:**
```bash
go test ./internal/adapter/bridge/ -run "TestEvaluate|TestLoadRules" -v
```
Expected: all PASS.

- [ ] **Run full suite (rules.go shouldn't break anything else):**
```bash
go test ./...
```

### Commit

- [ ] **Single commit:**
```bash
git add internal/adapter/bridge/extract_rules.yaml \
        internal/adapter/bridge/rules.go \
        internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): yaml rule loader + mini-DSL evaluator (Bug 2 prep)

Self-contained module: extract_rules.yaml (go:embed) defines per-tool
Pre/Post extraction templates; rules.go loads + evaluates them.

DSL supports: {\$.path}, {\$.a // \$.b} fallback chain, {tool_name},
{value}, |firstline / |firstpara / |default:N / |bool:T,F filters,
<json:N> stringify-and-truncate. Failure-mode is total: missing paths
and unknown tokens yield empty substitution; caller decides fallback.

ExtractContent will switch to this in Task 3."
```

---

## Task 3: Wire DSL into ExtractContent (phase-aware refactor)

**Scope:** Replace per-tool `switch` in `ExtractContent` with rule-driven dispatch; update `cmd/bridge/main.go` to pass phase + correct payload field; migrate existing Pre tests; add Post tests for cross-agent schema.

**Files:**
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`
- Modify: `cmd/bridge/main.go`

### Subtask 3A — Refactor ExtractContent

- [ ] **In `internal/adapter/bridge/extractor.go`, add `"gopkg.in/yaml.v3"` to the import block** (needed for `yaml.Node` in the new helpers).

- [ ] **Replace the existing `ExtractContent` (and the entire `extractRaw` function — lines ~11-135 of the original file)** with this block:

```go
// loadedRules is initialized once at package init and reused across calls.
// LoadRules failure is fatal — extract_rules.yaml is embedded, so a parse
// error means the binary itself is malformed.
var loadedRules *Rules

func init() {
	r, err := LoadRules()
	if err != nil {
		panic("bridge: failed to load embedded extract_rules.yaml: " + err.Error())
	}
	loadedRules = r
}

// ExtractContent extracts a (raw, truncated) summary from a tool event payload.
//
//	phase = "pre"  → payload is tool_input,    use rules.Pre[toolName]
//	phase = "post" → payload is tool_response, use rules.Post[toolName]
//
// Falls back to rules.default for unknown tools, then to toolName itself if
// every rule resolution yields empty.
func ExtractContent(phase, toolName string, payload json.RawMessage) (contentRaw, content string) {
	var raw string
	switch phase {
	case "pre":
		raw = renderPre(toolName, payload)
	case "post":
		raw = renderPost(toolName, payload)
	default:
		raw = toolName
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = toolName
	}
	return raw, truncateRunes(raw, contentMaxRunes)
}

// renderPre looks up a Pre rule. Preserves the legacy MCP-prefix special case
// (mcp__github__create_pr → "MCP: create_pr") that the old extractRaw had.
func renderPre(toolName string, payload []byte) string {
	if rule, ok := loadedRules.Pre[toolName]; ok {
		return Evaluate(rule, payload, toolName)
	}
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(toolName, "__")
		return "MCP: " + parts[len(parts)-1]
	}
	return Evaluate(loadedRules.Pre["default"], payload, toolName)
}

// renderPost dispatches by payload shape (string vs object) when the rule is
// structured. Falls back to default rule for unknown tools.
func renderPost(toolName string, payload []byte) string {
	node, ok := loadedRules.Post[toolName]
	if !ok {
		node, ok = loadedRules.Post["default"]
		if !ok {
			return toolName
		}
	}
	return RenderPostRule(node, payload, toolName)
}

// RenderPostRule walks a Post yaml.Node:
//   - ScalarNode: plain string template, applied directly
//   - MappingNode: branches by payload shape (string/object), then
//     object_paths probe list, then fallback template
func RenderPostRule(node yaml.Node, payload []byte, toolName string) string {
	if node.Kind == yaml.ScalarNode {
		return Evaluate(node.Value, payload, toolName)
	}
	if node.Kind != yaml.MappingNode {
		return ""
	}
	get := func(key string) (yaml.Node, bool) {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return *node.Content[i+1], true
			}
		}
		return yaml.Node{}, false
	}

	trimmed := bytesTrimSpace(payload)
	isString := len(trimmed) > 0 && trimmed[0] == '"'

	if isString {
		if n, ok := get("string"); ok {
			if v := Evaluate(n.Value, payload, toolName); v != "" {
				return v
			}
		}
	} else {
		if n, ok := get("object"); ok {
			if v := Evaluate(n.Value, payload, toolName); v != "" {
				return v
			}
		}
		if n, ok := get("object_paths"); ok && n.Kind == yaml.SequenceNode {
			for _, p := range n.Content {
				template := "{" + p.Value + "}"
				if v := Evaluate(template, payload, toolName); v != "" {
					return v
				}
			}
		}
	}
	if n, ok := get("fallback"); ok {
		return Evaluate(n.Value, payload, toolName)
	}
	return ""
}

// bytesTrimSpace returns payload without leading/trailing ASCII whitespace.
func bytesTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\n' || b[start] == '\r') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\n' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}
```

- [ ] **Update `ExtractEventContent.PermissionRequest` case** (the only intra-package caller of the old signature). Find:

```go
	case "PermissionRequest":
		return ExtractContent(in.ToolName, in.ToolInput)
```

Replace with:

```go
	case "PermissionRequest":
		return ExtractContent("pre", in.ToolName, in.ToolInput)
```

### Subtask 3B — Update cmd/bridge/main.go

- [ ] **In `cmd/bridge/main.go`, replace:**

```go
	// Extract content based on event type
	var contentRaw, content string
	if isToolEvent(eventType) {
		contentRaw, content = bridge.ExtractContent(in.ToolName, in.ToolInput)
	} else {
		contentRaw, content = bridge.ExtractEventContent(eventType, &in)
	}
```

with:

```go
	// Extract content based on event type. PreToolUse / Permission* dispatch to
	// rules.Pre with tool_input; PostToolUse to rules.Post with tool_response.
	var contentRaw, content string
	switch eventType {
	case "PreToolUse", "PermissionRequest", "PermissionDenied":
		contentRaw, content = bridge.ExtractContent("pre", in.ToolName, in.ToolInput)
	case "PostToolUse":
		contentRaw, content = bridge.ExtractContent("post", in.ToolName, in.ToolResult)
	default:
		contentRaw, content = bridge.ExtractEventContent(eventType, &in)
	}
```

(`isToolEvent` becomes unused by `main()` but is still covered by `TestIsToolEvent`. Keep it — no behavior change, single-test cost is trivial.)

### Subtask 3C — Migrate existing Pre tests

The existing `TestExtractContent_*` tests in `extractor_test.go` (~10 functions) call the old 2-arg signature. After Subtask 3A they won't compile.

- [ ] **Run a sed migration over the test file:**
```bash
sed -i '' 's/ExtractContent("\([A-Za-z_]*\)",/ExtractContent("pre", "\1",/g' \
    internal/adapter/bridge/extractor_test.go
```

- [ ] **Verify the diff** — `git diff internal/adapter/bridge/extractor_test.go` should show every old call now has `"pre"` as the first argument. Spot-check 2-3 lines manually.

- [ ] **Manually fix the MCP test if present.** Find:
```go
ExtractContent("pre", "mcp__github__create_pr", input)
```
Confirm it's correct (sed should have produced this).

### Subtask 3D — Add Post-phase tests (cross-agent coverage)

- [ ] **Append to `internal/adapter/bridge/extractor_test.go`:**

```go
// ─── Post phase ──────────────────────────────────────────────────────────────

func TestExtractContent_Post_Bash(t *testing.T) {
	raw, _ := ExtractContent("post", "Bash", []byte(`{"stdout":"hello\nworld","exitCode":0}`))
	if raw != "hello (exit=0)" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_Edit_Success(t *testing.T) {
	raw, _ := ExtractContent("post", "Edit", []byte(`{"success":true,"filePath":"/x.go"}`))
	if raw != "✓写入 /x.go" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_Edit_Failure(t *testing.T) {
	raw, _ := ExtractContent("post", "Edit", []byte(`{"success":false,"filePath":"/x.go"}`))
	if raw != "✗失败 /x.go" {
		t.Errorf("raw = %q", raw)
	}
}

// CC-INT shape: tool_response is an object containing answers array.
func TestExtractContent_Post_AskUserQuestion_Object(t *testing.T) {
	payload := []byte(`{"answers":["why? : because"]}`)
	raw, _ := ExtractContent("post", "AskUserQuestion", payload)
	if raw != "why? : because" {
		t.Errorf("raw = %q", raw)
	}
}

// CodeBuddy shape: tool_response is a bare JSON string.
func TestExtractContent_Post_AskUserQuestion_String(t *testing.T) {
	payload := []byte(`" · q1 → a1"`)
	raw, _ := ExtractContent("post", "AskUserQuestion", payload)
	if raw != " · q1 → a1" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_UnknownTool_DefaultObjectPaths(t *testing.T) {
	// stdout is in default.object_paths, so it resolves.
	raw, _ := ExtractContent("post", "MyTool", []byte(`{"stdout":"out"}`))
	if raw != "out" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_UnknownTool_FallbackJSON(t *testing.T) {
	// Payload has none of the default object_paths → fallback to <json:120>.
	raw, _ := ExtractContent("post", "MyTool", []byte(`{"weird_field":"x"}`))
	if raw != `{"weird_field":"x"}` {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Pre_UnknownToolFallsBackToToolName(t *testing.T) {
	raw, _ := ExtractContent("pre", "MyCustomTool", []byte(`{}`))
	if raw != "MyCustomTool" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_TruncatesTo60Runes(t *testing.T) {
	long := strings.Repeat("a", 100)
	_, content := ExtractContent("pre", "Bash", []byte(`{"command":"`+long+`"}`))
	if r := []rune(content); len(r) != 61 { // 60 + "…"
		t.Errorf("content rune length = %d, want 61", len(r))
	}
}
```

If `strings` isn't already imported in `extractor_test.go`, add it.

### Verification

- [ ] **Run all tests:**
```bash
go test ./...
```
Expected: all PASS, including original Pre tests (now with `"pre"` arg) + 9 new Post tests.

- [ ] **Run vet to catch unused imports** (the old `extractRaw` may have imported `fmt` for the MCP case which is now obsolete):
```bash
go vet ./...
```

### Commit

- [ ] **Single commit:**
```bash
git add internal/adapter/bridge/extractor.go \
        internal/adapter/bridge/extractor_test.go \
        cmd/bridge/main.go
git commit -m "refactor(bridge): phase-aware ExtractContent driven by extract_rules.yaml (Bug 2)

ExtractContent(phase, toolName, payload) replaces the per-tool switch
in extractor.go. Pre uses tool_input → rules.Pre[tool]; Post uses
tool_response → rules.Post[tool] with string/object schema dispatch
and default object_paths/fallback chain.

Cross-agent shape differences (CC-INT object answers vs CodeBuddy
bare-string tool_response) are now handled uniformly by the rule
engine. MCP tool prefix matching preserved in renderPre.

cmd/bridge/main.go dispatches by event_type to choose phase + payload."
```

---

## Task 4: Dev tooling — Makefile install-bridge + CLAUDE.md (Bug 4a)

**Scope:** Close the deployment gap that caused 5 of 6 empty-content event types in the original investigation. Hook command path equals `$(BIN_DIR)/pager-cc-bridge`, so no copy needed — just wire `make bridge` into the dev workflow.

**Files:**
- Modify: `Makefile`
- Modify: `CLAUDE.md`

### Subtask 4A — Makefile

- [ ] **Add `install-bridge` to the `.PHONY` line near the top.** Find:
```makefile
.PHONY: dev build run clean test bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop
```
Replace with:
```makefile
.PHONY: dev build run clean test bridge install-bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop
```

- [ ] **Insert `install-bridge` target immediately after the existing `bridge` target:**

```makefile
## Build bridge and report deployment metadata
install-bridge: bridge
	@echo "✓ bridge built at $(BRIDGE_BIN)"
	@echo "  mtime: $$(date -r $(BRIDGE_BIN) '+%F %T')"
	@echo "  sha:   $$(shasum -a 256 $(BRIDGE_BIN) | cut -c1-12)"
```

- [ ] **Wire `dev` / `run` / `build` to depend on `install-bridge`.** Make these three replacements:

| Find | Replace |
|---|---|
| `dev: stop` | `dev: stop install-bridge` |
| `build: frontend-build` | `build: frontend-build install-bridge` |
| `run: build-go` | `run: build-go install-bridge` |

- [ ] **Add `install-bridge` to the help output.** In the `help` target, find the line:
```makefile
	@echo "  make bridge         Build pager-cc-bridge"
```
Insert below it:
```makefile
	@echo "  make install-bridge Build bridge with deployment verification"
```

### Subtask 4B — CLAUDE.md

- [ ] **Locate insertion point.** Open `CLAUDE.md`. The new section goes **immediately before** the existing `## Code Conventions` section.

- [ ] **Insert this block:**

```markdown
## Bridge Binary Lifecycle

After modifying `internal/adapter/bridge/*` or `cmd/bridge/*`, run:

\`\`\`bash
make install-bridge
\`\`\`

This rebuilds `bin/pager-cc-bridge` (the binary that CC/CodeBuddy hooks invoke
via `~/.claude/settings.json` and `~/.codebuddy/settings.json`) and prints its
mtime + sha so you can confirm deployment.

**Why this matters:** the 2026-06-03 root-cause investigation of "TaskCreated /
PostToolBatch / SessionEnd / SubagentStart all empty content" found that
events were being recorded against a stale bridge binary that predated commit
`1ba099f` (CC field alignment refactor). New rules in extractor.go are inert
until the binary is rebuilt — this is by design (hooks are external
processes), and `make install-bridge` is the discipline that closes the gap.

`make dev` / `make run` / `make build` already depend on `install-bridge`, so
in practice you only need to run it explicitly when iterating on the bridge
without restarting the Pager app.
```

(The triple-backticks above are escaped only for the plan; in CLAUDE.md they should be plain backticks.)

### Verification

- [ ] **Run:**
```bash
make help
```
Expected: prints help including `make install-bridge` line.

- [ ] **Run:**
```bash
make install-bridge
```
Expected: builds, prints `✓ bridge built at bin/pager-cc-bridge` + mtime + sha.

- [ ] **Sanity:**
```bash
grep -A2 "Bridge Binary Lifecycle" CLAUDE.md
```
Expected: section heading + first line shown.

### Commit

- [ ] **Single commit:**
```bash
git add Makefile CLAUDE.md
git commit -m "build(make): install-bridge target + Bridge Binary Lifecycle docs (Bug 4a)

Bug 4a root cause was a stale bridge binary (6/01 build missing 6/02
field-alignment refactor). Wiring install-bridge into dev/run/build
prevents this in future; CLAUDE.md captures the discipline so future
me doesn't repeat the investigation."
```

---

## Task 5: End-to-end verification

**Scope:** Empirical close-out. No code changes (modulo any minor template tweaks surfaced by reality). Rebuild bridge, generate test events, query SQLite, confirm content non-empty rate ≥ 95%.

**Pre-req:** Pager app must be running (`make dev` in another terminal) — otherwise bridge POSTs return network error and rows never reach the DB.

### Subtask 5A — Rebuild and prove deployment

- [ ] **Run:**
```bash
make install-bridge
ls -la bin/pager-cc-bridge
```
Expected: mtime is "now" (within seconds).

### Subtask 5B — Per-bug spot checks

For each bug, fire a representative payload and assert the resulting DB row.

- [ ] **Bug 4a (TaskCreated has content):**
```bash
echo '{"session_id":"e2e-001","cwd":"/tmp","hook_event_name":"TaskCreated","task_id":"99","task_subject":"e2e verification"}' \
  | ./bin/pager-cc-bridge --event TaskCreated --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='TaskCreated' ORDER BY id DESC LIMIT 1"
```
Expected: `e2e verification`.

- [ ] **Bug 1 (auth_success Notification dropped):**
```bash
echo '{"hook_event_name":"Notification","notification_type":"auth_success","message":"x","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event Notification --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT COUNT(*) FROM t_events WHERE agent_label='E2E' AND event_type='Notification' AND timestamp > datetime('now','-1 minute')"
```
Expected: `0`.

- [ ] **Bug 3 (permission_prompt label):**
```bash
echo '{"session_id":"e2e-003","hook_event_name":"Notification","notification_type":"permission_prompt","message":"needs your permission to use Bash","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event Notification --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='Notification' ORDER BY id DESC LIMIT 1"
```
Expected: `[等授权] needs your permission to use Bash`.

- [ ] **Bug 2 PostToolUse — CodeBuddy string form:**
```bash
echo '{"session_id":"e2e-004","hook_event_name":"PostToolUse","tool_name":"AskUserQuestion","tool_use_id":"u1","tool_input":{"questions":[{"question":"q?"}]},"tool_response":" · q? → answer A","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event PostToolUse --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='PostToolUse' ORDER BY id DESC LIMIT 1"
```
Expected: ` · q? → answer A`.

- [ ] **Bug 2 PostToolUse — CC-INT object form:**
```bash
echo '{"session_id":"e2e-005","hook_event_name":"PostToolUse","tool_name":"AskUserQuestion","tool_use_id":"u2","tool_input":{"questions":[{"question":"q?"}]},"tool_response":{"answers":["q? : answer B"]},"cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event PostToolUse --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='PostToolUse' AND content LIKE '%answer B%' ORDER BY id DESC LIMIT 1"
```
Expected: `q? : answer B`.

- [ ] **Bug 4b (SessionEnd reason):**
```bash
echo '{"session_id":"e2e-006","hook_event_name":"SessionEnd","reason":"compact","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event SessionEnd --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='SessionEnd' ORDER BY id DESC LIMIT 1"
```
Expected: `compact`.

- [ ] **Bug 4c (SubagentStart agent_id):**
```bash
echo '{"session_id":"e2e-007","hook_event_name":"SubagentStart","agent_id":"agent-7777","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event SubagentStart --agent E2E
sleep 1
sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='SubagentStart' ORDER BY id DESC LIMIT 1"
```
Expected: `agent-7777`.

### Subtask 5C — Real-session non-empty rate

- [ ] **Run a real CC + CodeBuddy session.** In separate terminals, do a real interactive session in each (≥ 5 turns each, including at least one PreToolUse, PostToolUse, AskUserQuestion, Stop). Then query:

```bash
sqlite3 ~/.config/pager/pager.db <<'SQL'
SELECT
  event_type,
  COUNT(*) AS total,
  SUM(CASE WHEN content='' AND content_raw='' THEN 1 ELSE 0 END) AS empty,
  ROUND(100.0 * (COUNT(*) - SUM(CASE WHEN content='' AND content_raw='' THEN 1 ELSE 0 END)) / COUNT(*), 1) AS non_empty_pct
FROM t_events
WHERE id > (SELECT MAX(id) - 200 FROM t_events)
GROUP BY event_type
ORDER BY non_empty_pct ASC;
SQL
```

Expected: every event_type with `total > 0` shows `non_empty_pct ≥ 95.0`. The only legitimate exception is `SessionEnd` with `reason=""` (rare, not a defect).

### Subtask 5D — Cleanup commit (only if needed)

- [ ] **If verification surfaced a yaml template tweak**, commit it now. Otherwise this task has no commit:
```bash
# Only if changes were needed:
git add <changed files>
git commit -m "fix(bridge): minor template tweak from e2e verification"
```

---

## Self-Review

### Spec coverage

| Spec section | Implementing task |
|---|---|
| § 3.1 Bug 1 shouldDrop | Task 1B |
| § 3.2.1 gjson + yaml.v3 | Task 1A |
| § 3.2.2 yaml file | Task 2A |
| § 3.2.3 Mini-DSL | Task 2B (full implementation in one shot) |
| § 3.2.4 Code refactor | Task 3 |
| § 3.2.5 Failure-mode | Task 2B (`Evaluate` returns "" for unknowns) + Task 3A (`renderPre` / `renderPost` chain to default → toolName) |
| § 3.3 Bug 3 Notification label | Task 1E |
| § 3.4.1 SessionEnd.reason | Task 1C |
| § 3.4.2 SubagentStart fallback | Task 1D |
| § 3.4.3 dev tooling | Task 4 |
| § 3.4.4 historical backfill | **Out of scope** per spec § 6 |
| § 4 Test plan | Task 1 (4 quick fixes), Task 2 (DSL units), Task 3 (Pre/Post integration), Task 5 (end-to-end + non-empty rate) |

No gaps.

### Placeholder scan

Searched plan for: TBD, TODO, "implement later", "appropriate error handling", "similar to Task N". None present. Every code block is paste-ready.

### Type / signature consistency

- `ExtractContent(phase, toolName string, payload json.RawMessage) (string, string)` — defined Task 3A, used Task 3A (PermissionRequest), Task 3B (main.go), Task 3C (test migration), Task 3D (Post tests). ✓
- `Evaluate(rule string, payload []byte, toolName string) string` — defined Task 2B, used by Task 3A's `renderPre` / `RenderPostRule`. ✓
- `LoadRules() (*Rules, error)` and `Rules{Version, Pre, Post}` — defined Task 2B, used in Task 3A `init()`. ✓
- `RenderPostRule(node yaml.Node, payload []byte, toolName string) string` — defined Task 3A. ✓
- `notificationTypeLabel`, `firstNonEmpty`, `shouldDrop` — defined Task 1, no later renames. ✓

### Notes for the implementer

- **Task 3 is the largest single change** (~150 lines refactored across 3 files). If something fails midway, `git restore` extractor.go and main.go and re-attempt — no other task depends on partial Task 3 state.
- **Task 5 requires Pager app running**; without it bridge POSTs fail silently and rows never appear in DB. Check `lsof -i:7421` shows the Pager listener before running subtask 5B.
- **Filter argument with comma** (`bool:✓写入,✗失败`): `strings.Cut(arg, ",")` only splits on the first comma. If a future filter argument needs a literal comma, escape support must be added — out of scope.
- **MCP prefix special case** (`mcp__github__create_pr` → `MCP: create_pr`) lives in `renderPre` not yaml — verify the existing test in `extractor_test.go` still passes after Task 3.
