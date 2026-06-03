# Pager Event Parsing 4-Bug Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix four classes of event-parsing defects in `pager-cc-bridge`: (1) drop CodeBuddy's session-less `auth_success` Notification, (2) replace single-shot `ExtractContent` with phase-aware (Pre/Post) YAML rule-driven extractor that handles cross-agent `tool_response` schema differences via `gjson`, (3) prepend `[等授权]/[等输入]/[认证]` labels to Notification content, (4) repair stale binary deployment + missing field mappings (`SessionEnd.reason`, `SubagentStart.agent_id`).

**Architecture:** Single-process changes confined to `internal/adapter/bridge/` and `cmd/bridge/`. Pager main process is untouched. Introduce one third-party dependency (`github.com/tidwall/gjson`) scoped to the bridge package only; rest of repo stays stdlib-only. Configuration-driven extraction via `extract_rules.yaml` (go:embed) + a minimal hand-written DSL evaluator (~80 lines, no template engine). Dev tooling closes the deployment gap that caused 5 of the 6 empty-content event types.

**Tech Stack:** Go 1.25, `github.com/tidwall/gjson` v1.18+, `gopkg.in/yaml.v3`, `embed` stdlib, existing testing stdlib.

---

## File Structure

### New files
- `internal/adapter/bridge/extract_rules.yaml` — Pre/Post extraction rules per tool (go:embed source)
- `internal/adapter/bridge/rules.go` — yaml loader + mini-DSL evaluator (path/fallback/filters/json-truncate)
- `internal/adapter/bridge/rules_test.go` — DSL unit tests (one test per DSL feature)

### Modified files
- `internal/adapter/bridge/types.go` — add `Reason string \`json:"reason"\`` field
- `internal/adapter/bridge/extractor.go` — rewrite `ExtractContent` signature to `(phase, toolName, payload)` driven by yaml; add Notification `notificationTypeLabel`; fix `SessionEnd` and `SubagentStart` cases
- `internal/adapter/bridge/extractor_test.go` — migrate existing tests to new signature; add Post-phase tests covering CC-INT and CodeBuddy AskUserQuestion divergence
- `cmd/bridge/main.go` — add `shouldDrop` guard; pass phase to `ExtractContent` (`"pre"` for PreToolUse/PermissionRequest/PermissionDenied, `"post"` for PostToolUse, sourcing payload from `ToolInput` or `ToolResult` accordingly)
- `cmd/bridge/main_test.go` — add `TestShouldDrop`
- `go.mod` / `go.sum` — register gjson + yaml.v3
- `Makefile` — add `install-bridge` target; make `dev`/`run`/`build` depend on it
- `CLAUDE.md` — add "Bridge Binary Lifecycle" section

### Untouched
- `internal/domain/**`, `internal/wails/**`, `internal/infra/**`, `frontend/**`

---

## Task 1: Add gjson + yaml.v3 dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add dependencies via go get**

Run from repo root:
```bash
go get github.com/tidwall/gjson@v1.18.0
go get gopkg.in/yaml.v3@v3.0.1
```

Expected: `go.mod` gains two `require` entries; `go.sum` updated with checksums.

- [ ] **Step 2: Verify modules resolve and existing tests still pass**

Run:
```bash
go mod tidy
go build ./...
go test ./...
```

Expected: All existing tests pass. No build errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(bridge): add gjson + yaml.v3 dependencies for rule-driven extractor"
```

---

## Task 2: Bug 1 — shouldDrop guard for session-less Notification

**Files:**
- Modify: `cmd/bridge/main.go`
- Modify: `cmd/bridge/main_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/bridge/main_test.go`:

```go
import (
	"pager/internal/adapter/bridge"
)

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

(Note: if `cmd/bridge/main_test.go` does not yet `import "pager/internal/adapter/bridge"`, add it to the existing import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/bridge/ -run TestShouldDrop -v`
Expected: FAIL with `undefined: shouldDrop`.

- [ ] **Step 3: Implement shouldDrop**

In `cmd/bridge/main.go`, add this function below `isToolEvent`:

```go
// shouldDrop returns true for events that have no actionable signal and would
// pollute the session list. Currently only filters CodeBuddy's auth_success
// Notification (which carries no session_id and would trigger the host:cwd:tty
// fallback, making session_key look like a path).
func shouldDrop(eventType string, in *bridge.CCHookInput) bool {
	if eventType == "Notification" && in.SessionID == "" {
		return true
	}
	return false
}
```

Insert the guard call inside `main`, immediately after `_ = json.Unmarshal(raw, &in)` and before the `var contentRaw, content string` declaration:

```go
	if shouldDrop(eventType, &in) {
		return
	}
```

- [ ] **Step 4: Run tests to verify all pass**

Run: `go test ./cmd/bridge/ -v`
Expected: PASS for all `TestShouldDrop_*` plus existing `TestParseArgs_*`, `TestIsToolEvent`.

- [ ] **Step 5: Commit**

```bash
git add cmd/bridge/main.go cmd/bridge/main_test.go
git commit -m "fix(bridge): drop session-less Notification (Bug 1)

CodeBuddy auth_success and similar lifecycle Notifications carry no
session_id, triggering the host:cwd:tty fallback and creating
session_keys that look like file paths. Drop them at bridge entry."
```

---

## Task 3: Bug 4b — SessionEnd uses reason field

**Files:**
- Modify: `internal/adapter/bridge/types.go`
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractEventContent_SessionEnd_UsesReason(t *testing.T) {
	in := &CCHookInput{Reason: "clear"}
	raw, content := ExtractEventContent("SessionEnd", in)
	if raw != "clear" {
		t.Errorf("raw = %q, want %q", raw, "clear")
	}
	if content != "clear" {
		t.Errorf("content = %q, want %q", content, "clear")
	}
}

func TestExtractEventContent_SessionEnd_EmptyReason(t *testing.T) {
	in := &CCHookInput{Reason: ""}
	raw, content := ExtractEventContent("SessionEnd", in)
	if raw != "" || content != "" {
		t.Errorf("expected empty raw/content, got %q/%q", raw, content)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_SessionEnd -v`
Expected: FAIL with `unknown field Reason in struct literal of type CCHookInput`.

- [ ] **Step 3: Add Reason field to CCHookInput**

In `internal/adapter/bridge/types.go`, find the "Session layer" comment block (around line 18) and add `Reason` next to the existing session fields:

```go
	// ═══ Session layer ═══
	Source       string `json:"source"`        // SessionStart: startup/resume/clear/compact
	Model        string `json:"model"`         // SessionStart
	SessionTitle string `json:"session_title"` // SessionStart
	Reason       string `json:"reason"`        // SessionEnd: other/clear/compact/logout
```

- [ ] **Step 4: Update SessionEnd case in ExtractEventContent**

In `internal/adapter/bridge/extractor.go`, find:

```go
	case "SessionEnd":
		return "", ""
```

Replace with:

```go
	case "SessionEnd":
		return in.Reason, in.Reason
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_SessionEnd -v`
Expected: PASS.

- [ ] **Step 6: Run full bridge package tests**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: All existing tests still PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/bridge/types.go internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): SessionEnd content uses reason field (Bug 4b)

raw_payload carries reason='other'/'clear'/'compact'/'logout'.
Was previously hard-coded to empty string."
```

---

## Task 4: Bug 4c — SubagentStart falls back to agent_id

**Files:**
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractEventContent_SubagentStart_AgentTypePresent(t *testing.T) {
	in := &CCHookInput{AgentType: "general-purpose", AgentID: "agent-xyz"}
	raw, _ := ExtractEventContent("SubagentStart", in)
	if raw != "general-purpose" {
		t.Errorf("raw = %q, want %q", raw, "general-purpose")
	}
}

func TestExtractEventContent_SubagentStart_AgentIDFallback(t *testing.T) {
	// CodeBuddy emits agent_id only, no agent_type
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

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_SubagentStart -v`
Expected: `TestExtractEventContent_SubagentStart_AgentIDFallback` FAILs (current code returns `""` because `AgentType` is empty).

- [ ] **Step 3: Update SubagentStart case**

In `internal/adapter/bridge/extractor.go`, find:

```go
	case "SubagentStart":
		return in.AgentType, in.AgentType
```

Replace with:

```go
	case "SubagentStart":
		raw := firstNonEmpty(in.AgentType, in.AgentID)
		return raw, raw
```

Then ensure a `firstNonEmpty` helper exists in this package. If `cmd/bridge/main.go` already defines one, do **not** depend on it (different package). Add this helper at the bottom of `internal/adapter/bridge/extractor.go`:

```go
// firstNonEmpty returns the first non-empty string from the given args, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: PASS for all `TestExtractEventContent_SubagentStart_*`.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): SubagentStart falls back to agent_id when agent_type missing (Bug 4c)

CodeBuddy SubagentStart raw_payload only carries agent_id; agent_type
field is absent. Previous implementation returned empty content."
```

---

## Task 5: Bug 3 — Notification type label

**Files:**
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`
- Modify: `internal/adapter/bridge/types.go` (only if `NotificationType` field is missing — verify first)

- [ ] **Step 1: Verify NotificationType field exists**

Check `internal/adapter/bridge/types.go` already has:
```go
	NotificationType string          `json:"notification_type"` // Notification: permission_prompt/idle_prompt
```
It should be present (it is in the current code). If it's missing, add it under the "MCP & UI layer" section. Otherwise skip to Step 2.

- [ ] **Step 2: Write the failing tests**

Append to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractEventContent_Notification_PermissionPrompt(t *testing.T) {
	in := &CCHookInput{
		NotificationType: "permission_prompt",
		Message:          "needs your permission to use Bash",
	}
	raw, _ := ExtractEventContent("Notification", in)
	want := "[等授权] needs your permission to use Bash"
	if raw != want {
		t.Errorf("raw = %q, want %q", raw, want)
	}
}

func TestExtractEventContent_Notification_IdlePrompt(t *testing.T) {
	in := &CCHookInput{
		NotificationType: "idle_prompt",
		Message:          "CodeBuddy is waiting for your input",
	}
	raw, _ := ExtractEventContent("Notification", in)
	want := "[等输入] CodeBuddy is waiting for your input"
	if raw != want {
		t.Errorf("raw = %q, want %q", raw, want)
	}
}

func TestExtractEventContent_Notification_AuthSuccess(t *testing.T) {
	in := &CCHookInput{
		NotificationType: "auth_success",
		Message:          "auth_success: example-user (Tencent)",
	}
	raw, _ := ExtractEventContent("Notification", in)
	want := "[认证] auth_success: example-user (Tencent)"
	if raw != want {
		t.Errorf("raw = %q, want %q", raw, want)
	}
}

func TestExtractEventContent_Notification_UnknownType(t *testing.T) {
	in := &CCHookInput{
		NotificationType: "",
		Message:          "raw message only",
	}
	raw, _ := ExtractEventContent("Notification", in)
	want := "raw message only"
	if raw != want {
		t.Errorf("raw = %q, want %q", raw, want)
	}
}

func TestExtractEventContent_Notification_EmptyMessageFallsBackToType(t *testing.T) {
	in := &CCHookInput{
		NotificationType: "idle_prompt",
		Message:          "",
	}
	raw, _ := ExtractEventContent("Notification", in)
	want := "[等输入] idle_prompt"
	if raw != want {
		t.Errorf("raw = %q, want %q", raw, want)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run TestExtractEventContent_Notification -v`
Expected: FAIL — current code returns `msg` without label prefix.

- [ ] **Step 4: Implement notificationTypeLabel and update Notification case**

In `internal/adapter/bridge/extractor.go`, find:

```go
	case "Notification":
		raw := in.Message
		if raw == "" {
			raw = in.NotificationType
		}
		return raw, truncateRunes(raw, contentMaxRunes)
```

Replace with:

```go
	case "Notification":
		msg := in.Message
		if msg == "" {
			msg = in.NotificationType
		}
		raw := notificationTypeLabel(in.NotificationType) + msg
		return raw, truncateRunes(raw, contentMaxRunes)
```

Add this helper at the bottom of `extractor.go` (next to `firstNonEmpty` from Task 4):

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
		return "[认证] " // Bug 1 already drops these at bridge entry; this is defensive.
	default:
		return ""
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: PASS for all 5 new tests, all existing tests still PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go
git commit -m "fix(bridge): prepend type label to Notification content (Bug 3)

[等授权]/[等输入]/[认证] prefixes derived from notification_type
make raw Notification messages distinguishable in the session card."
```

---

## Task 6: Create extract_rules.yaml + minimal evaluator

**Files:**
- Create: `internal/adapter/bridge/extract_rules.yaml`
- Create: `internal/adapter/bridge/rules.go`
- Create: `internal/adapter/bridge/rules_test.go`

This task lays the foundation: yaml file, embed loader, and a minimal `Evaluate` function supporting only `{$.path}` substitution + literal text. Subsequent tasks (7, 8, 9) extend the evaluator incrementally.

- [ ] **Step 1: Create the yaml rules file**

Create `internal/adapter/bridge/extract_rules.yaml`:

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

- [ ] **Step 2: Write the failing test for minimal evaluator**

Create `internal/adapter/bridge/rules_test.go`:

```go
package bridge

import (
	"testing"
)

func TestEvaluate_LiteralOnly(t *testing.T) {
	got := Evaluate("hello world", []byte(`{}`), "Bash")
	if got != "hello world" {
		t.Errorf("Evaluate() = %q, want %q", got, "hello world")
	}
}

func TestEvaluate_SimplePath(t *testing.T) {
	got := Evaluate("{$.command}", []byte(`{"command":"go test"}`), "Bash")
	if got != "go test" {
		t.Errorf("Evaluate() = %q, want %q", got, "go test")
	}
}

func TestEvaluate_PathWithLiteralPrefix(t *testing.T) {
	got := Evaluate("编辑 {$.file_path}", []byte(`{"file_path":"/src/main.go"}`), "Edit")
	if got != "编辑 /src/main.go" {
		t.Errorf("Evaluate() = %q, want %q", got, "编辑 /src/main.go")
	}
}

func TestEvaluate_NestedPath(t *testing.T) {
	got := Evaluate("{$.questions.0.question}", []byte(`{"questions":[{"question":"why?"}]}`), "AskUserQuestion")
	if got != "why?" {
		t.Errorf("Evaluate() = %q, want %q", got, "why?")
	}
}

func TestEvaluate_MissingPathReturnsEmpty(t *testing.T) {
	got := Evaluate("{$.missing}", []byte(`{"command":"x"}`), "Bash")
	if got != "" {
		t.Errorf("Evaluate() = %q, want empty", got)
	}
}

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
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/bridge/ -run TestEvaluate -v`
Expected: FAIL with `undefined: Evaluate` and `undefined: LoadRules`.

- [ ] **Step 4: Implement minimal rules.go**

Create `internal/adapter/bridge/rules.go`:

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

// Rules represents the parsed extract_rules.yaml. The Post values are kept as
// yaml.Node so that we can distinguish a plain string ("{value}") from a
// structured rule (with string:/object:/object_paths: keys) at evaluation time.
type Rules struct {
	Version int                  `yaml:"version"`
	Pre     map[string]string    `yaml:"pre"`
	Post    map[string]yaml.Node `yaml:"post"`
}

// LoadRules parses the embedded yaml. Returns a populated *Rules or error.
func LoadRules() (*Rules, error) {
	var r Rules
	if err := yaml.Unmarshal(rulesYAML, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Evaluate executes a rule template against a JSON payload, returning the
// resolved string. Unknown DSL constructs return empty string for that
// substitution; the caller is responsible for fallback chaining.
//
// Supported in this task (extended in tasks 7-9):
//   - literal text
//   - {$.path} — gjson path resolution
func Evaluate(rule string, payload []byte, toolName string) string {
	var b strings.Builder
	i := 0
	for i < len(rule) {
		if rule[i] == '{' {
			end := strings.IndexByte(rule[i:], '}')
			if end == -1 {
				b.WriteByte(rule[i])
				i++
				continue
			}
			token := rule[i+1 : i+end]
			b.WriteString(resolveToken(token, payload, toolName))
			i += end + 1
			continue
		}
		b.WriteByte(rule[i])
		i++
	}
	return b.String()
}

// resolveToken handles a single {…} expression body.
// Extended progressively in tasks 7-9.
func resolveToken(token string, payload []byte, toolName string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(token, "$.") {
		return gjson.GetBytes(payload, token[2:]).String()
	}
	return ""
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -run "TestEvaluate|TestLoadRules" -v`
Expected: PASS for all 6 tests in this task.

- [ ] **Step 6: Run full package tests**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: All existing tests + 6 new tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/bridge/extract_rules.yaml internal/adapter/bridge/rules.go internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): add yaml rule loader + minimal DSL evaluator

extract_rules.yaml is embedded; LoadRules parses Pre/Post per-tool
templates. Evaluate supports literal text and {\$.path} substitution.
Filters, fallbacks, and json-truncate added in subsequent commits."
```

---

## Task 7: DSL — fallback (`//`), `{value}`, `{tool_name}` tokens

**Files:**
- Modify: `internal/adapter/bridge/rules.go`
- Modify: `internal/adapter/bridge/rules_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/adapter/bridge/rules_test.go`:

```go
func TestEvaluate_FallbackOperator(t *testing.T) {
	// First path empty, second has value → second wins.
	got := Evaluate("{$.description // $.prompt}", []byte(`{"prompt":"P"}`), "Agent")
	if got != "P" {
		t.Errorf("Evaluate() = %q, want %q", got, "P")
	}
}

func TestEvaluate_FallbackOperator_FirstWins(t *testing.T) {
	got := Evaluate("{$.description // $.prompt}", []byte(`{"description":"D","prompt":"P"}`), "Agent")
	if got != "D" {
		t.Errorf("Evaluate() = %q, want %q", got, "D")
	}
}

func TestEvaluate_FallbackOperator_BothEmpty(t *testing.T) {
	got := Evaluate("{$.a // $.b}", []byte(`{}`), "Agent")
	if got != "" {
		t.Errorf("Evaluate() = %q, want empty", got)
	}
}

func TestEvaluate_ToolNameToken(t *testing.T) {
	got := Evaluate("{tool_name}", []byte(`{}`), "MyTool")
	if got != "MyTool" {
		t.Errorf("Evaluate() = %q, want %q", got, "MyTool")
	}
}

func TestEvaluate_ValueToken_StringPayload(t *testing.T) {
	// {value} resolves to the entire payload when payload is a JSON string literal.
	got := Evaluate("{value}", []byte(`"hello"`), "AskUserQuestion")
	if got != "hello" {
		t.Errorf("Evaluate() = %q, want %q", got, "hello")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run "TestEvaluate_(Fallback|ToolName|Value)" -v`
Expected: FAIL — `resolveToken` does not yet handle `//`, `tool_name`, or `value`.

- [ ] **Step 3: Extend resolveToken**

In `internal/adapter/bridge/rules.go`, replace the existing `resolveToken` with:

```go
// resolveToken handles a single {…} expression body.
// Supported (cumulative across tasks 6-9):
//   - "$.path"             gjson path
//   - "$.a // $.b"         first non-empty fallback chain
//   - "tool_name"          literal toolName arg
//   - "value"              raw payload as string (when payload is a JSON string)
//   - "$.path|filter:arg"  filtered value (Task 8)
//   - "<json:N>"           stringify payload, truncate to N runes + "…" (Task 9)
func resolveToken(token string, payload []byte, toolName string) string {
	token = strings.TrimSpace(token)
	switch token {
	case "tool_name":
		return toolName
	case "value":
		// gjson on a payload that is itself a JSON string literal returns
		// the unquoted string via the @this path.
		return gjson.GetBytes(payload, "@this").String()
	}
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
	if strings.HasPrefix(token, "$.") {
		return gjson.GetBytes(payload, token[2:]).String()
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -run TestEvaluate -v`
Expected: PASS for all `TestEvaluate_*`.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/rules.go internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): DSL evaluator supports // fallback, tool_name, value tokens"
```

---

## Task 8: DSL — filter pipeline (`|firstline`, `|firstpara`, `|default:N`, `|bool:T,F`)

**Files:**
- Modify: `internal/adapter/bridge/rules.go`
- Modify: `internal/adapter/bridge/rules_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/adapter/bridge/rules_test.go`:

```go
func TestEvaluate_FilterFirstline(t *testing.T) {
	got := Evaluate("{$.stdout|firstline}", []byte(`{"stdout":"line1\nline2\nline3"}`), "Bash")
	if got != "line1" {
		t.Errorf("Evaluate() = %q, want %q", got, "line1")
	}
}

func TestEvaluate_FilterFirstline_NoNewline(t *testing.T) {
	got := Evaluate("{$.stdout|firstline}", []byte(`{"stdout":"single line"}`), "Bash")
	if got != "single line" {
		t.Errorf("Evaluate() = %q, want %q", got, "single line")
	}
}

func TestEvaluate_FilterFirstpara(t *testing.T) {
	got := Evaluate("{$.text|firstpara}", []byte(`{"text":"para1 line1\npara1 line2\n\npara2"}`), "Agent")
	if got != "para1 line1\npara1 line2" {
		t.Errorf("Evaluate() = %q, want %q", got, "para1 line1\npara1 line2")
	}
}

func TestEvaluate_FilterDefault_PathPresent(t *testing.T) {
	got := Evaluate("{$.exitCode|default:0}", []byte(`{"exitCode":7}`), "Bash")
	if got != "7" {
		t.Errorf("Evaluate() = %q, want %q", got, "7")
	}
}

func TestEvaluate_FilterDefault_PathMissing(t *testing.T) {
	got := Evaluate("{$.exitCode|default:0}", []byte(`{}`), "Bash")
	if got != "0" {
		t.Errorf("Evaluate() = %q, want %q", got, "0")
	}
}

func TestEvaluate_FilterBool_True(t *testing.T) {
	got := Evaluate("{$.success|bool:✓写入,✗失败}", []byte(`{"success":true}`), "Edit")
	if got != "✓写入" {
		t.Errorf("Evaluate() = %q, want %q", got, "✓写入")
	}
}

func TestEvaluate_FilterBool_False(t *testing.T) {
	got := Evaluate("{$.success|bool:✓写入,✗失败}", []byte(`{"success":false}`), "Edit")
	if got != "✗失败" {
		t.Errorf("Evaluate() = %q, want %q", got, "✗失败")
	}
}

func TestEvaluate_FilterChain_FullBashRule(t *testing.T) {
	// The full Post.Bash template from extract_rules.yaml.
	got := Evaluate(
		"{$.stdout|firstline} (exit={$.exitCode|default:0})",
		[]byte(`{"stdout":"hello\nworld","exitCode":0}`),
		"Bash",
	)
	if got != "hello (exit=0)" {
		t.Errorf("Evaluate() = %q, want %q", got, "hello (exit=0)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run TestEvaluate_Filter -v`
Expected: FAIL — filters not implemented.

- [ ] **Step 3: Extend resolveToken with filter pipeline**

In `internal/adapter/bridge/rules.go`, replace `resolveToken` with the version below (incorporates filters on top of Task 7's fallback/value/tool_name):

```go
// resolveToken handles a single {…} expression body.
// Supported (cumulative):
//   - "$.path"                   gjson path
//   - "$.a // $.b"               first non-empty fallback chain
//   - "tool_name"                literal toolName arg
//   - "value"                    raw payload as string
//   - "$.path|filter[:arg]"      filtered value
//
// Filters: firstline, firstpara, default:N, bool:T,F
func resolveToken(token string, payload []byte, toolName string) string {
	token = strings.TrimSpace(token)
	// Fallback chain takes precedence — split first.
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

	// Split filters off.
	pipe := strings.Split(token, "|")
	base := strings.TrimSpace(pipe[0])

	var raw string
	exists := true
	switch base {
	case "tool_name":
		raw = toolName
	case "value":
		raw = gjson.GetBytes(payload, "@this").String()
	default:
		if strings.HasPrefix(base, "$.") {
			res := gjson.GetBytes(payload, base[2:])
			exists = res.Exists()
			raw = res.String()
		}
	}

	for _, f := range pipe[1:] {
		raw, exists = applyFilter(strings.TrimSpace(f), raw, exists)
	}
	return raw
}

// applyFilter mutates a value through one filter step, propagating an
// "exists" flag so that |default:X can fire only when the upstream path is
// missing (not merely empty).
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -run TestEvaluate -v`
Expected: PASS for all `TestEvaluate_*` (Tasks 6, 7, 8 combined).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/rules.go internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): DSL evaluator supports filter pipeline (firstline, firstpara, default, bool)"
```

---

## Task 9: DSL — `<json:N>` truncated JSON dump

**Files:**
- Modify: `internal/adapter/bridge/rules.go`
- Modify: `internal/adapter/bridge/rules_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/adapter/bridge/rules_test.go`:

```go
func TestEvaluate_JSONTruncate_Short(t *testing.T) {
	// Payload short enough that no truncation happens.
	got := Evaluate("<json:120>", []byte(`{"k":"v"}`), "X")
	if got != `{"k":"v"}` {
		t.Errorf("Evaluate() = %q, want %q", got, `{"k":"v"}`)
	}
}

func TestEvaluate_JSONTruncate_Long(t *testing.T) {
	long := []byte(`{"k":"` + strings.Repeat("a", 300) + `"}`)
	got := Evaluate("<json:50>", long, "X")
	// Want: first 50 runes of the JSON serialization + "…"
	if !strings.HasSuffix(got, "…") {
		t.Errorf("Evaluate() = %q, want suffix …", got)
	}
	if runes := []rune(got); len(runes) != 51 { // 50 + ellipsis
		t.Errorf("rune length = %d, want 51", len(runes))
	}
}

func TestEvaluate_JSONTruncate_Embedded(t *testing.T) {
	got := Evaluate("prefix:<json:5> suffix", []byte(`{"k":"v"}`), "X")
	// "prefix:" + first 5 runes of `{"k":"v"}` ("{\"k\":") + "…" + " suffix"
	want := `prefix:{"k":… suffix`
	if got != want {
		t.Errorf("Evaluate() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run TestEvaluate_JSONTruncate -v`
Expected: FAIL.

- [ ] **Step 3: Implement `<json:N>` token**

In `internal/adapter/bridge/rules.go`:

(a) Update the `Evaluate` function to detect the `<json:N>` form (delimited by `<…>` instead of `{…}`):

Replace the existing `Evaluate` body with:

```go
// Evaluate executes a rule template against a JSON payload.
//
// Token forms:
//   - "{…}"    standard substitution (resolveToken)
//   - "<…>"    raw-form, currently only "<json:N>"
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

// resolveRawToken handles <…> tokens. Currently only "json:N".
func resolveRawToken(token string, payload []byte) string {
	if strings.HasPrefix(token, "json:") {
		nStr := token[len("json:"):]
		n := atoiOrZero(nStr)
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/bridge/ -run TestEvaluate -v`
Expected: PASS for all `TestEvaluate_*`.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/bridge/rules.go internal/adapter/bridge/rules_test.go
git commit -m "feat(bridge): DSL <json:N> token for stringify-and-truncate fallback"
```

---

## Task 10: Refactor ExtractContent to phase-aware, yaml-driven

**Files:**
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/extractor_test.go`
- Modify: `cmd/bridge/main.go`

This task replaces the per-tool `switch` statement in `ExtractContent` with yaml-rule lookup. It also adds a `RenderPostRule` helper for the structured Post entries (which can be `string`, `{string:..., object:...}`, or default-with-fallback chain). Existing `ExtractContent` callers (in `ExtractEventContent.PermissionRequest` and in `cmd/bridge/main.go`) update to the new signature.

- [ ] **Step 1: Write failing tests for the new signature**

Append to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractContent_Pre_Bash(t *testing.T) {
	raw, content := ExtractContent("pre", "Bash", []byte(`{"command":"go test"}`))
	if raw != "go test" {
		t.Errorf("raw = %q, want %q", raw, "go test")
	}
	if content != "go test" {
		t.Errorf("content = %q", content)
	}
}

func TestExtractContent_Pre_Edit(t *testing.T) {
	raw, _ := ExtractContent("pre", "Edit", []byte(`{"file_path":"/src/x.go"}`))
	if raw != "编辑 /src/x.go" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Pre_UnknownToolFallsBackToToolName(t *testing.T) {
	raw, _ := ExtractContent("pre", "MyCustomTool", []byte(`{}`))
	if raw != "MyCustomTool" {
		t.Errorf("raw = %q, want %q", raw, "MyCustomTool")
	}
}

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

func TestExtractContent_Post_AskUserQuestion_Object(t *testing.T) {
	// CC-INT shape: tool_response is an object containing answers.
	payload := []byte(`{"answers":["why? : because"]}`)
	raw, _ := ExtractContent("post", "AskUserQuestion", payload)
	if raw != "why? : because" {
		t.Errorf("raw = %q", raw)
	}
}

func TestExtractContent_Post_AskUserQuestion_String(t *testing.T) {
	// CodeBuddy shape: tool_response is a bare JSON string.
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

func TestExtractContent_TruncatesTo60Runes(t *testing.T) {
	long := strings.Repeat("a", 100)
	_, content := ExtractContent("pre", "Bash", []byte(`{"command":"`+long+`"}`))
	if r := []rune(content); len(r) != 61 { // 60 + "…"
		t.Errorf("content rune length = %d, want 61", len(r))
	}
}
```

(Note: `strings` may already be imported in `extractor_test.go`; if not, add it.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/bridge/ -run TestExtractContent_(Pre|Post|Truncates) -v`
Expected: FAIL — `ExtractContent` still has old `(toolName, toolInput)` signature.

- [ ] **Step 3: Refactor ExtractContent in extractor.go**

In `internal/adapter/bridge/extractor.go`:

(a) Replace the existing `ExtractContent` and `extractRaw` functions (lines 11-135 in the current file) with:

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
// phase   = "pre"   → payload is tool_input  (use rules.Pre[toolName])
// phase   = "post"  → payload is tool_response (use rules.Post[toolName])
//
// Falls back to rules.default for unknown tools, and to toolName itself if
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

// renderPre looks up a Pre rule and evaluates it. Falls back to the "default"
// entry if toolName is not registered.
func renderPre(toolName string, payload []byte) string {
	rule, ok := loadedRules.Pre[toolName]
	if !ok {
		rule = loadedRules.Pre["default"]
	}
	return Evaluate(rule, payload, toolName)
}

// renderPost looks up a Post rule and evaluates it. Post entries can be:
//   - a plain string template (always applied regardless of payload type)
//   - a structured node with keys: string (used when payload is a JSON string),
//     object (used when payload is a JSON object), object_paths (a list of
//     gjson paths probed in order), fallback (last-resort template).
//
// For unknown tools, the "default" entry is used the same way.
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

// RenderPostRule walks a Post yaml.Node according to the rules above.
func RenderPostRule(node yaml.Node, payload []byte, toolName string) string {
	// Plain string template.
	if node.Kind == yaml.ScalarNode {
		return Evaluate(node.Value, payload, toolName)
	}
	// Mapping with string:/object:/object_paths:/fallback: keys.
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

	// Detect payload shape — JSON string vs object/array vs other.
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
// Avoids importing bytes to keep this file slim.
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

(b) The existing `extractRaw` function and the long per-tool switch block can now be **deleted entirely**. The MCP-tool special case (`mcp__github__create_pr`) was handled by `extractRaw`; keep parity by adding to `renderPre`:

After `renderPre`, add:

```go
// init-time augment: register the MCP catch-all rule. We do this at init
// rather than in yaml so the prefix-matching logic stays in code.
// (No-op if loadedRules is not yet ready.)
```

Actually simpler — handle MCP inside `renderPre`:

Replace the `renderPre` body with:

```go
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
```

(c) Update the `ExtractEventContent` PermissionRequest case (currently calls `ExtractContent(in.ToolName, in.ToolInput)`). Find:

```go
	case "PermissionRequest":
		return ExtractContent(in.ToolName, in.ToolInput)
```

Replace with:

```go
	case "PermissionRequest":
		return ExtractContent("pre", in.ToolName, in.ToolInput)
```

(d) Add `"gopkg.in/yaml.v3"` to the `import` block in `extractor.go` (needed for the `yaml.Node` type used by `RenderPostRule`).

- [ ] **Step 4: Update cmd/bridge/main.go to pass phase**

In `cmd/bridge/main.go`, find the existing block:

```go
	// Extract content based on event type
	var contentRaw, content string
	if isToolEvent(eventType) {
		contentRaw, content = bridge.ExtractContent(in.ToolName, in.ToolInput)
	} else {
		contentRaw, content = bridge.ExtractEventContent(eventType, &in)
	}
```

Replace with:

```go
	// Extract content based on event type
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

The old `isToolEvent` function in `main.go` is now unreferenced by `main()`, but it is still tested by `TestIsToolEvent`. Keep it for the test (no behavior change).

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: All bridge tests + cmd/bridge tests PASS, including the new Pre/Post coverage from Step 1.

- [ ] **Step 6: Run go vet to catch unused imports**

Run: `go vet ./...`
Expected: clean (no warnings about unused `fmt` import in extractor.go etc.).

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/bridge/extractor.go internal/adapter/bridge/extractor_test.go cmd/bridge/main.go
git commit -m "refactor(bridge): phase-aware ExtractContent driven by extract_rules.yaml (Bug 2)

ExtractContent(phase, toolName, payload) replaces the per-tool switch.
Pre uses tool_input → rules.Pre[tool]; Post uses tool_response →
rules.Post[tool] with string/object schema dispatch and default
object_paths/fallback chain. Cross-agent shape differences (CC-INT
object answers vs CodeBuddy bare-string tool_response) are handled
uniformly. MCP tool prefix matching preserved in renderPre."
```

---

## Task 11: Migrate existing Pre tests to new signature

**Files:**
- Modify: `internal/adapter/bridge/extractor_test.go`

The pre-existing `TestExtractContent_Bash`, `TestExtractContent_Edit`, etc. (lines 9-110 of the original file) call the old 2-arg signature. After Task 10 they will not compile. Update them.

- [ ] **Step 1: Identify all old-signature call sites**

Run: `grep -n "ExtractContent(\"" internal/adapter/bridge/extractor_test.go || grep -n 'ExtractContent("' internal/adapter/bridge/extractor_test.go`
The current `extractor_test.go` has ~10 functions calling `ExtractContent("ToolName", input)`.

- [ ] **Step 2: Migrate each call to the new signature**

In `internal/adapter/bridge/extractor_test.go`, replace every occurrence of:

```go
ExtractContent("Bash", input)
ExtractContent("Edit", input)
ExtractContent("Write", input)
ExtractContent("Read", input)
ExtractContent("Glob", input)
ExtractContent("Grep", input)
ExtractContent("WebFetch", input)
ExtractContent("WebSearch", input)
ExtractContent("Task", input)
ExtractContent("AskUserQuestion", input)
ExtractContent("Agent", input)
ExtractContent("TaskCreate", input)
ExtractContent("TaskUpdate", input)
ExtractContent("TaskGet", input)
ExtractContent("TaskList", input)
ExtractContent("NotebookEdit", input)
ExtractContent("LSP", input)
ExtractContent("UnknownTool", input)
ExtractContent("mcp__github__create_pr", input)
```

…with the corresponding 3-arg form:

```go
ExtractContent("pre", "Bash", input)
ExtractContent("pre", "Edit", input)
…
```

(Use `sed -i '' 's/ExtractContent("\([A-Za-z_]*\)",/ExtractContent("pre", "\1",/g' internal/adapter/bridge/extractor_test.go` if you prefer batch — verify the diff afterwards.)

- [ ] **Step 3: Run tests**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: all PASS.

If any test fails because the rule's output differs slightly from the old hand-coded format (e.g., trailing whitespace, missing prefix), update either the yaml template or the test expectation — whichever matches the spec § 3.2.2.

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/bridge/extractor_test.go
git commit -m "test(bridge): migrate Pre-phase tests to phase-aware ExtractContent signature"
```

---

## Task 12: Makefile install-bridge target + dev/run/build prerequisites

**Files:**
- Modify: `Makefile`

This is the dev-tooling fix for Bug 4a. The hook command path in `~/.claude/settings.json` and `~/.codebuddy/settings.json` is `/data/projects/github.com/sapaude/pager/bin/pager-cc-bridge`, which equals `$(BIN_DIR)/pager-cc-bridge` produced by `make bridge`. So no copy is needed — but `make bridge` must be auto-invoked by the dev workflow, which today it isn't (`run: build-go` and `dev: stop` skip it).

- [ ] **Step 1: Add install-bridge target**

In `Makefile`, find the `bridge` target (around line 47):

```makefile
## Build pager-cc-bridge binary
bridge:
	go build -o $(BRIDGE_BIN) ./cmd/bridge
```

Insert immediately after it:

```makefile
## Build bridge and report deployment metadata
install-bridge: bridge
	@echo "✓ bridge built at $(BRIDGE_BIN)"
	@echo "  mtime: $$(date -r $(BRIDGE_BIN) '+%F %T')"
	@echo "  sha:   $$(shasum -a 256 $(BRIDGE_BIN) | cut -c1-12)"
```

- [ ] **Step 2: Add install-bridge to .PHONY**

Find the `.PHONY` line near the top:

```makefile
.PHONY: dev build run clean test bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop
```

Replace with:

```makefile
.PHONY: dev build run clean test bridge install-bridge install-hooks install-hooks-codebuddy frontend-deps frontend-build bindings icon lint stop
```

- [ ] **Step 3: Make dev/run/build depend on install-bridge**

Find:

```makefile
dev: stop
	wails3 dev -config ./build/config.yml -port $(VITE_PORT)
```

Replace with:

```makefile
dev: stop install-bridge
	wails3 dev -config ./build/config.yml -port $(VITE_PORT)
```

Find:

```makefile
build: frontend-build
	wails3 package
```

Replace with:

```makefile
build: frontend-build install-bridge
	wails3 package
```

Find:

```makefile
run: build-go
	./$(APP_BIN)
```

Replace with:

```makefile
run: build-go install-bridge
	./$(APP_BIN)
```

- [ ] **Step 4: Add install-bridge to help text**

Find the `help` target:

```makefile
	@echo "  make bridge         Build pager-cc-bridge"
	@echo "  make install-hooks  Install CC hooks into ~/.claude/settings.json"
```

Insert between them:

```makefile
	@echo "  make install-bridge Build bridge with deployment verification"
```

- [ ] **Step 5: Verify Makefile syntax**

Run: `make help`
Expected: prints the help text without errors; "make install-bridge" line is visible.

- [ ] **Step 6: Run install-bridge**

Run: `make install-bridge`
Expected: builds without error; prints `✓ bridge built at bin/pager-cc-bridge`, mtime, sha.

- [ ] **Step 7: Commit**

```bash
git add Makefile
git commit -m "build(make): install-bridge target + dev/run/build prerequisite (Bug 4a)

Bug 4a root cause was a stale bridge binary (6/01 build missing 6/02
field-alignment refactor). Wiring install-bridge into the dev workflow
prevents this in future."
```

---

## Task 13: CLAUDE.md Bridge Binary Lifecycle section

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Locate insertion point**

Open `CLAUDE.md`. Find the existing "## Code Conventions" section. The new section will be inserted **before** it, after the existing "## Tech Stack" or "## Non-Negotiable Decisions" section (whichever comes last before "## Code Conventions").

- [ ] **Step 2: Add the section**

Insert this block (preserving the surrounding `##` heading style):

```markdown
## Bridge Binary Lifecycle

After modifying `internal/adapter/bridge/*` or `cmd/bridge/*`, run:

```bash
make install-bridge
```

This rebuilds `bin/pager-cc-bridge` (the binary that CC/CodeBuddy hooks invoke
via `~/.claude/settings.json` and `~/.codebuddy/settings.json`) and prints its
mtime + sha so you can confirm deployment.

**Why this matters:** the 2026-06-03 root-cause investigation of "TaskCreated /
PostToolBatch / SessionEnd / SubagentStart all empty content" found that
events were being recorded against a **stale** bridge binary that predated
commit `1ba099f` (CC field alignment). New rules in extractor.go are inert
until the binary is rebuilt — this is by design (hooks are external
processes), and `make install-bridge` is the discipline that closes the gap.

`make dev` / `make run` / `make build` already depend on `install-bridge`, so
in practice you only need to run it explicitly when iterating on the bridge
without restarting the Pager app.
```

- [ ] **Step 3: Verify**

Run: `grep -A2 "Bridge Binary Lifecycle" CLAUDE.md`
Expected: shows the new section heading + first line.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): bridge binary lifecycle discipline (Bug 4a)"
```

---

## Task 14: End-to-end integration verification

**Files:**
- (verification only — no code changes)

This task is the empirical close-out: rebuild bridge, generate real events, query the SQLite DB, confirm content non-empty rate ≥ 95%.

- [ ] **Step 1: Rebuild and confirm bridge mtime**

Run:
```bash
make install-bridge
```
Expected: prints `✓ bridge built at bin/pager-cc-bridge` with current timestamp.

- [ ] **Step 2: Verify bridge processes a TaskCreated event correctly**

With the Pager app running (`make dev` or `make run`), pipe a test event through the freshly-built bridge:

```bash
echo '{"session_id":"e2e-test-001","cwd":"/tmp","hook_event_name":"TaskCreated","task_id":"99","task_subject":"e2e verification"}' \
  | ./bin/pager-cc-bridge --event TaskCreated --agent E2E
```

Then query the DB:

```bash
sqlite3 ~/.config/pager/pager.db \
  "SELECT id, agent_label, event_type, content, content_raw FROM t_events WHERE agent_label='E2E' ORDER BY id DESC LIMIT 1"
```
Expected: `content` and `content_raw` both equal `e2e verification`.

- [ ] **Step 3: Verify Bug 1 — auth_success Notification is dropped**

```bash
echo '{"hook_event_name":"Notification","notification_type":"auth_success","message":"auth_success: e2e","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event Notification --agent E2E

sqlite3 ~/.config/pager/pager.db \
  "SELECT COUNT(*) FROM t_events WHERE agent_label='E2E' AND content LIKE '%e2e%' AND timestamp > datetime('now','-1 minute')"
```
Expected: count is 1 (only the TaskCreated from Step 2 — the auth_success Notification was dropped at bridge).

- [ ] **Step 4: Verify Bug 3 — permission_prompt Notification gets label**

```bash
echo '{"session_id":"e2e-test-002","hook_event_name":"Notification","notification_type":"permission_prompt","message":"needs your permission to use Bash","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event Notification --agent E2E

sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='Notification' ORDER BY id DESC LIMIT 1"
```
Expected: `[等授权] needs your permission to use Bash`.

- [ ] **Step 5: Verify Bug 2 — PostToolUse(AskUserQuestion) takes from tool_response**

CodeBuddy form (string tool_response):

```bash
PAYLOAD='{"session_id":"e2e-test-003","hook_event_name":"PostToolUse","tool_name":"AskUserQuestion","tool_use_id":"u1","tool_input":{"questions":[{"question":"q?"}]},"tool_response":" · q? → answer A","cwd":"/tmp"}'
echo "$PAYLOAD" | ./bin/pager-cc-bridge --event PostToolUse --agent E2E

sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='PostToolUse' ORDER BY id DESC LIMIT 1"
```
Expected: ` · q? → answer A` (CodeBuddy bare-string form resolves via `string: "{value}"`).

CC-INT form (object tool_response):

```bash
PAYLOAD='{"session_id":"e2e-test-004","hook_event_name":"PostToolUse","tool_name":"AskUserQuestion","tool_use_id":"u2","tool_input":{"questions":[{"question":"q?"}]},"tool_response":{"answers":["q? : answer B"]},"cwd":"/tmp"}'
echo "$PAYLOAD" | ./bin/pager-cc-bridge --event PostToolUse --agent E2E

sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='PostToolUse' AND content LIKE '%answer B%' ORDER BY id DESC LIMIT 1"
```
Expected: `q? : answer B` (CC-INT object form resolves via `object: "{$.answers.0}"`).

- [ ] **Step 6: Verify Bug 4b — SessionEnd uses reason**

```bash
echo '{"session_id":"e2e-test-005","hook_event_name":"SessionEnd","reason":"compact","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event SessionEnd --agent E2E

sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='SessionEnd' ORDER BY id DESC LIMIT 1"
```
Expected: `compact`.

- [ ] **Step 7: Verify Bug 4c — SubagentStart fallback to agent_id**

```bash
echo '{"session_id":"e2e-test-006","hook_event_name":"SubagentStart","agent_id":"agent-7777","cwd":"/tmp"}' \
  | ./bin/pager-cc-bridge --event SubagentStart --agent E2E

sqlite3 ~/.config/pager/pager.db \
  "SELECT content FROM t_events WHERE agent_label='E2E' AND event_type='SubagentStart' ORDER BY id DESC LIMIT 1"
```
Expected: `agent-7777`.

- [ ] **Step 8: Run a real CC + CodeBuddy session and check non-empty rate**

Use a real CC session (e.g. `claude` CLI in a project) for ≥ 5 turns, similarly for CodeBuddy. Then:

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
Expected: every event_type with `total > 0` shows `non_empty_pct ≥ 95.0` except possibly `SessionEnd` (which can have `reason=""` legitimately, not a defect).

- [ ] **Step 9: Final commit (if any cleanup needed)**

If the verification surfaced any minor mismatch (e.g., a yaml template needs a tweak), commit it now. Otherwise this task has no commit.

```bash
# Only if changes were needed:
git add <changed files>
git commit -m "fix(bridge): minor template tweak from e2e verification"
```

---

## Self-Review

### Spec Coverage

| Spec section | Implementing task(s) |
|---|---|
| § 3.1 Bug 1 shouldDrop | Task 2 |
| § 3.2.1 gjson dependency | Task 1 |
| § 3.2.2 yaml file | Task 6 |
| § 3.2.3 Mini-DSL | Tasks 6 (path/literal), 7 (fallback/value/tool_name), 8 (filters), 9 (json:N) |
| § 3.2.4 Code changes (extractor.go, main.go) | Task 10 |
| § 3.2.5 Failure-mode (always-fallback) | Task 10 (`renderPre`/`renderPost` chain), Task 6 (default rule), Task 11 (test coverage of unknown tools) |
| § 3.3 Bug 3 Notification label | Task 5 |
| § 3.4.1 SessionEnd reason | Task 3 |
| § 3.4.2 SubagentStart fallback | Task 4 |
| § 3.4.3 dev tooling | Tasks 12, 13 |
| § 3.4.4 historical backfill | **Out of scope** per spec § 6 |
| § 4 Test plan | Task 2 (Bug 1), Task 3 (4b), Task 4 (4c), Task 5 (Bug 3), Tasks 6-9 (DSL units), Task 10 (Pre/Post integration), Task 14 (end-to-end + non-empty rate ≥ 95%) |

No gaps found.

### Placeholder Scan

Scanned for forbidden patterns (`TBD`, `TODO`, "implement later", "appropriate error handling", "similar to Task N"): none present. Every code step contains complete, paste-ready code.

### Type/Signature Consistency

- `ExtractContent(phase, toolName string, payload json.RawMessage) (string, string)` — defined in Task 10, used by Task 10 (PermissionRequest case), Task 10 main.go update, Task 11 test migration. ✓
- `Evaluate(rule string, payload []byte, toolName string) string` — defined in Task 6, extended in 7/8/9. ✓
- `LoadRules() (*Rules, error)` and `Rules{Version, Pre, Post}` — defined in Task 6, used in Task 10. ✓
- `RenderPostRule(node yaml.Node, payload []byte, toolName string) string` — defined in Task 10. ✓
- `notificationTypeLabel(ntype string) string`, `firstNonEmpty(...)` — defined in Tasks 4/5, no later renames. ✓
- `shouldDrop(eventType string, in *bridge.CCHookInput) bool` — defined and tested in Task 2. ✓

No drift between task definitions and consumers.

### Notes for the Implementer

- **Task 10 is the largest single change** (~150 lines refactored across 3 files). If it fails midway, `git restore` extractor.go and main.go and re-attempt step-by-step — no other task depends on partial Task-10 state.
- **Task 14 requires Pager app running**; without it, bridge POSTs return network error and rows never reach the DB. Use `make dev` in another terminal first.
- **Filter argument with comma** (`bool:✓写入,✗失败`): the implementation uses `strings.Cut(arg, ",")` which only splits on the first comma. If a filter argument ever needs a literal comma, the DSL needs escape support — out of scope; document a limitation if it ever bites.
