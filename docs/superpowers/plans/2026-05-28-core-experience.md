# Pager v1.1 Core Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Pager from a flat event list to an attention-graded notification system with unique session identity, rich content, and precise terminal jumping.

**Architecture:** Three-layer change — bridge (data enrichment + agent arg) → registry (session_id key + attention level) → frontend (v8 card design + 3-color filter). Each task is independently committable.

**Tech Stack:** Go 1.25, Wails v3 alpha.96, React 18, TypeScript, Tailwind CSS, zustand

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/event/types.go` | Modify | Add `AttentionLevel`, `AgentLabel`, `PermissionMode` fields |
| `internal/bridge/types.go` | Modify | Add `PermissionMode` to `CCHookInput`, add `AskUserQuestionInput`, `AgentInput` |
| `internal/bridge/extractor.go` | Modify | Enhance extraction for AskUserQuestion, Agent tools |
| `internal/bridge/extractor_test.go` | Modify | Add tests for new extraction cases |
| `internal/bridge/attention.go` | Create | `DetermineAttentionLevel()` function |
| `internal/bridge/attention_test.go` | Create | Tests for attention level logic |
| `cmd/bridge/main.go` | Modify | Parse `--agent` flag, extract `permission_mode`, set `AttentionLevel` |
| `internal/registry/registry.go` | Modify | SessionKey uses `session_id`, add `AttentionLevel` to Session |
| `internal/registry/registry_test.go` | Modify | Update tests for new SessionKey logic |
| `frontend/src/store/sessions.ts` | Modify | Add `filter` state, update Session interface |
| `frontend/src/App.tsx` | Rewrite | Three-color filter header |
| `frontend/src/components/SessionCard.tsx` | Rewrite | v8 card design |
| `frontend/src/components/SessionList.tsx` | Modify | Filter logic |
| `frontend/src/components/StatusIcon.tsx` | Remove | No longer needed (replaced by card background) |
| `frontend/src/index.css` | Modify | Dark/Light theme + pulse animation |
| `main.go` | Modify | Window hide-on-blur |
| `scripts/install-hooks.sh` | Modify | Accept `--agent` parameter |

---

### Task 1: AgentEvent — Add New Fields

**Files:**
- Modify: `internal/event/types.go`

- [ ] **Step 1: Add attention level constants and new fields**

```go
// Add after existing SessionStatus constants (line 28):

// AttentionLevel constants
const (
	AttentionAttention = "attention" // Needs user action (approve/answer)
	AttentionRunning   = "running"   // Executing in background
	AttentionDone      = "done"      // Session completed
)
```

Add three new fields to `AgentEvent` struct (after `ContentRaw` field):

```go
type AgentEvent struct {
	Agent          string `json:"agent"`
	Host           string `json:"host"`
	CWD            string `json:"cwd"`
	TTY            string `json:"tty"`
	SessionID      string `json:"session_id"`
	TermProgram    string `json:"term_program"`
	ITermSessionID string `json:"iterm_session_id,omitempty"`
	EventType      string `json:"event_type"`
	ToolName       string `json:"tool_name"`
	ToolUseID      string `json:"tool_use_id"`
	Content        string `json:"content"`
	ContentRaw     string `json:"content_raw"`
	AttentionLevel string `json:"attention_level"`          // NEW
	AgentLabel     string `json:"agent_label"`              // NEW
	PermissionMode string `json:"permission_mode,omitempty"` // NEW
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}
```

- [ ] **Step 2: Change SessionKey() to use session_id**

Replace the current `SessionKey()` method:

```go
// SessionKey returns the unique session identifier.
// Prefers CC-native session_id; falls back to host:cwd:tty triple.
func (e *AgentEvent) SessionKey() string {
	if e.SessionID != "" {
		return e.SessionID
	}
	return e.Host + ":" + e.CWD + ":" + e.TTY
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /data/projects/github.com/sapaude/pager && go build ./internal/event/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/event/types.go
git commit -m "feat(event): add AttentionLevel, AgentLabel, PermissionMode fields; use session_id as SessionKey"
```

---

### Task 2: Attention Level Logic

**Files:**
- Create: `internal/bridge/attention.go`
- Create: `internal/bridge/attention_test.go`

- [ ] **Step 1: Write the test file**

```go
// internal/bridge/attention_test.go
package bridge

import (
	"testing"

	"pager/internal/event"
)

func TestDetermineAttentionLevel_Stop(t *testing.T) {
	level := DetermineAttentionLevel(event.EventStop, "bypassPermissions")
	if level != event.AttentionDone {
		t.Errorf("got %q, want %q", level, event.AttentionDone)
	}
}

func TestDetermineAttentionLevel_Error(t *testing.T) {
	level := DetermineAttentionLevel(event.EventError, "")
	if level != event.AttentionDone {
		t.Errorf("got %q, want %q", level, event.AttentionDone)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Bypass(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPreToolUse, "bypassPermissions")
	if level != event.AttentionRunning {
		t.Errorf("got %q, want %q", level, event.AttentionRunning)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Default(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPreToolUse, "default")
	if level != event.AttentionAttention {
		t.Errorf("got %q, want %q", level, event.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PreToolUse_Empty(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPreToolUse, "")
	if level != event.AttentionAttention {
		t.Errorf("got %q, want %q", level, event.AttentionAttention)
	}
}

func TestDetermineAttentionLevel_PostToolUse(t *testing.T) {
	level := DetermineAttentionLevel(event.EventPostToolUse, "default")
	if level != event.AttentionRunning {
		t.Errorf("got %q, want %q", level, event.AttentionRunning)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

Run: `cd /data/projects/github.com/sapaude/pager && go test ./internal/bridge/ -run TestDetermineAttention -v`
Expected: FAIL — `DetermineAttentionLevel` undefined

- [ ] **Step 3: Implement DetermineAttentionLevel**

```go
// internal/bridge/attention.go
package bridge

import "pager/internal/event"

// DetermineAttentionLevel decides the attention level based on event type and permission mode.
//
// Rules:
//   - stop/error → done
//   - post_tool_use → running (tool already executed)
//   - pre_tool_use + bypassPermissions → running (will auto-execute)
//   - pre_tool_use + anything else → attention (needs user approve)
func DetermineAttentionLevel(eventType, permissionMode string) string {
	switch eventType {
	case event.EventStop, event.EventError:
		return event.AttentionDone
	case event.EventPostToolUse:
		return event.AttentionRunning
	case event.EventPreToolUse:
		if permissionMode == "bypassPermissions" {
			return event.AttentionRunning
		}
		return event.AttentionAttention
	default:
		return event.AttentionRunning
	}
}
```

- [ ] **Step 4: Run tests to confirm they pass**

Run: `cd /data/projects/github.com/sapaude/pager && go test ./internal/bridge/ -run TestDetermineAttention -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/attention.go internal/bridge/attention_test.go
git commit -m "feat(bridge): add DetermineAttentionLevel with permission_mode awareness"
```

---

### Task 3: Bridge — Parse --agent and permission_mode

**Files:**
- Modify: `internal/bridge/types.go`
- Modify: `cmd/bridge/main.go`

- [ ] **Step 1: Add PermissionMode to CCHookInput**

In `internal/bridge/types.go`, add field to `CCHookInput`:

```go
type CCHookInput struct {
	SessionID      string          `json:"session_id"`
	CWD            string          `json:"cwd"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolUseID      string          `json:"tool_use_id"`
	PermissionMode string          `json:"permission_mode"` // NEW
}
```

- [ ] **Step 2: Update cmd/bridge/main.go to parse --agent and set new fields**

Replace the entire `main()` function in `cmd/bridge/main.go`:

```go
package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"pager/internal/bridge"
	"pager/internal/event"
)

func main() {
	defer os.Exit(0)

	eventType, agentLabel := parseArgs(os.Args[1:])

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}

	if os.Getenv("PAGER_DEBUG") == "1" {
		_ = os.WriteFile("/tmp/pager-raw.json", raw, 0644)
	}

	var in bridge.CCHookInput
	_ = json.Unmarshal(raw, &in)

	contentRaw, content := bridge.ExtractContent(in.ToolName, in.ToolInput)
	attentionLevel := bridge.DetermineAttentionLevel(eventType, in.PermissionMode)

	e := event.AgentEvent{
		Agent:          event.AgentClaudeCode,
		Host:           "local",
		SessionID:      in.SessionID,
		CWD:            firstNonEmpty(in.CWD, os.Getenv("PWD")),
		TTY:            detectTTY(),
		TermProgram:    os.Getenv("TERM_PROGRAM"),
		ITermSessionID: os.Getenv("ITERM_SESSION_ID"),
		EventType:      eventType,
		ToolName:       in.ToolName,
		ToolUseID:      in.ToolUseID,
		Content:        content,
		ContentRaw:     contentRaw,
		AttentionLevel: attentionLevel,
		AgentLabel:     agentLabel,
		PermissionMode: in.PermissionMode,
		RawPayload:     raw,
		Timestamp:      time.Now(),
	}

	bridge.PostEvent(e)
}

// parseArgs extracts event type and --agent flag from command-line args.
// Usage: pager-cc-bridge <event_type> [--agent <label>]
func parseArgs(args []string) (eventType, agentLabel string) {
	eventType = "unknown"
	agentLabel = "CC" // default

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--agent" && i+1 < len(args):
			agentLabel = args[i+1]
			i++ // skip next
		case !strings.HasPrefix(args[i], "--") && eventType == "unknown":
			eventType = args[i]
		}
	}
	return
}

func detectTTY() string {
	out, err := exec.Command("tty").Output()
	if err == nil {
		t := strings.TrimSpace(string(out))
		if t != "" && t != "not a tty" {
			return t
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Step 3: Build bridge binary**

Run: `cd /data/projects/github.com/sapaude/pager && go build -o bin/pager-cc-bridge ./cmd/bridge/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/bridge/types.go cmd/bridge/main.go
git commit -m "feat(bridge): parse --agent flag and permission_mode from CC hook stdin"
```

---

### Task 4: Content Extraction Enhancement

**Files:**
- Modify: `internal/bridge/types.go`
- Modify: `internal/bridge/extractor.go`
- Modify: `internal/bridge/extractor_test.go`

- [ ] **Step 1: Add new input types to types.go**

Append to `internal/bridge/types.go`:

```go
// AskUserQuestionInput is tool_input for AskUserQuestion tool.
type AskUserQuestionInput struct {
	Questions []struct {
		Question string `json:"question"`
	} `json:"questions"`
}

// AgentInput is tool_input for Agent tool.
type AgentInput struct {
	Prompt      string `json:"prompt"`
	Description string `json:"description"`
}
```

- [ ] **Step 2: Write failing tests for new extractions**

Add to `internal/bridge/extractor_test.go`:

```go
func TestExtractContent_AskUserQuestion(t *testing.T) {
	input := `{"questions":[{"question":"你偏好哪种 UI 风格？","header":"UI","options":[],"multiSelect":false}]}`
	_, content := ExtractContent("AskUserQuestion", json.RawMessage(input))
	if !strings.Contains(content, "你偏好哪种 UI 风格") {
		t.Errorf("content = %q, want to contain question text", content)
	}
}

func TestExtractContent_Agent(t *testing.T) {
	input := `{"prompt":"Review the code for security issues","description":"Security review"}`
	_, content := ExtractContent("Agent", json.RawMessage(input))
	if !strings.Contains(content, "Security review") {
		t.Errorf("content = %q, want to contain description", content)
	}
}

func TestExtractContent_AgentFallbackToPrompt(t *testing.T) {
	input := `{"prompt":"Review the code for security issues"}`
	raw, _ := ExtractContent("Agent", json.RawMessage(input))
	if !strings.Contains(raw, "Review the code") {
		t.Errorf("raw = %q, want to contain prompt text", raw)
	}
}
```

- [ ] **Step 3: Run tests to confirm failure**

Run: `cd /data/projects/github.com/sapaude/pager && go test ./internal/bridge/ -run "TestExtractContent_AskUser|TestExtractContent_Agent" -v`
Expected: FAIL

- [ ] **Step 4: Implement enhanced extraction**

In `internal/bridge/extractor.go`, add cases in `extractRaw()` before the MCP prefix check:

```go
	case "AskUserQuestion":
		var in AskUserQuestionInput
		if unmarshal(&in) && len(in.Questions) > 0 && in.Questions[0].Question != "" {
			return in.Questions[0].Question
		}
	case "Agent":
		var in AgentInput
		if unmarshal(&in) {
			if in.Description != "" {
				return "子任务: " + in.Description
			}
			if in.Prompt != "" {
				return "子任务: " + in.Prompt
			}
		}
```

Also update the existing `"Task"` case to try `Prompt` field as fallback:

```go
	case "Task":
		var in TaskInput
		if unmarshal(&in) && in.Description != "" {
			return "子任务: " + in.Description
		}
		// Fallback: try Agent-style prompt
		var agentIn AgentInput
		if unmarshal(&agentIn) && agentIn.Prompt != "" {
			return "子任务: " + agentIn.Prompt
		}
```

- [ ] **Step 5: Run all extractor tests**

Run: `cd /data/projects/github.com/sapaude/pager && go test ./internal/bridge/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/bridge/types.go internal/bridge/extractor.go internal/bridge/extractor_test.go
git commit -m "feat(bridge): enhance content extraction for AskUserQuestion and Agent tools"
```

---

### Task 5: Registry — SessionKey Uses session_id

**Files:**
- Modify: `internal/registry/registry.go`
- Modify: `internal/registry/registry_test.go`

- [ ] **Step 1: Add AttentionLevel to Session struct**

In `internal/registry/registry.go`, add field to `Session` (after `Status`):

```go
type Session struct {
	Key            string                       `json:"Key"`
	Agent          string                       `json:"Agent"`
	Host           string                       `json:"Host"`
	CWD            string                       `json:"CWD"`
	TTY            string                       `json:"TTY"`
	TermProgram    string                       `json:"TermProgram"`
	ITermSessionID string                       `json:"ITermSessionID"`
	Status         string                       `json:"Status"`
	AttentionLevel string                       `json:"AttentionLevel"`  // NEW
	AgentLabel     string                       `json:"AgentLabel"`      // NEW
	SessionID      string                       `json:"SessionID"`       // NEW
	LastEvent      *event.AgentEvent            `json:"LastEvent"`
	PendingTools   map[string]*event.AgentEvent `json:"PendingTools"`
	UpdatedAt      time.Time                    `json:"UpdatedAt"`
}
```

- [ ] **Step 2: Update Apply() to set new fields**

In the `Apply()` method, after creating a new session (line 49-57), add `SessionID` to the new session creation:

```go
	if !exists {
		s = &Session{
			Key:          key,
			Agent:        e.Agent,
			Host:         e.Host,
			CWD:          e.CWD,
			TTY:          e.TTY,
			SessionID:    e.SessionID, // NEW
			PendingTools: make(map[string]*event.AgentEvent),
		}
		r.sessions[key] = s
	}
```

After updating `s.LastEvent = e` (line 68), add:

```go
	s.LastEvent = e
	s.UpdatedAt = e.Timestamp
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}

	// Update attention level and agent label from event
	if e.AttentionLevel != "" {
		s.AttentionLevel = e.AttentionLevel
	}
	if e.AgentLabel != "" {
		s.AgentLabel = e.AgentLabel
	}
```

- [ ] **Step 3: Update test helper and key assertion**

In `internal/registry/registry_test.go`, update `makeEvent` to include `SessionID`:

```go
func makeEvent(eventType, toolName, toolUseID, cwd, tty string) *event.AgentEvent {
	return &event.AgentEvent{
		Agent:     event.AgentClaudeCode,
		Host:      "local",
		SessionID: "test-session-" + cwd, // deterministic for tests
		CWD:       cwd,
		TTY:       tty,
		EventType: eventType,
		ToolName:  toolName,
		ToolUseID: toolUseID,
		Content:   toolName,
		Timestamp: time.Now(),
	}
}
```

Update the key assertion in `TestApply_PreToolUse_CreatesSession`:

```go
	// SessionKey now uses session_id when available
	if s.Key != "test-session-/project" {
		t.Errorf("key = %q, want %q", s.Key, "test-session-/project")
	}
```

- [ ] **Step 4: Add test for session_id based key**

Add new test:

```go
func TestApply_SessionID_Key(t *testing.T) {
	reg := New(func([]*Session) {})

	// Two events from same CWD/TTY but different session_id → two sessions
	e1 := makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/project", "/dev/ttys001")
	e1.SessionID = "session-aaa"
	reg.Apply(e1)

	e2 := makeEvent(event.EventPreToolUse, "Bash", "tu-2", "/project", "/dev/ttys001")
	e2.SessionID = "session-bbb"
	reg.Apply(e2)

	sessions := reg.ListSorted()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (different session_id), got %d", len(sessions))
	}
}

func TestApply_FallbackKey_NoSessionID(t *testing.T) {
	reg := New(func([]*Session) {})

	e := &event.AgentEvent{
		Agent:     event.AgentClaudeCode,
		Host:      "local",
		SessionID: "", // empty — should fallback
		CWD:       "/project",
		TTY:       "/dev/ttys001",
		EventType: event.EventPreToolUse,
		ToolName:  "Bash",
		ToolUseID: "tu-1",
		Content:   "test",
		Timestamp: time.Now(),
	}
	reg.Apply(e)

	sessions := reg.ListSorted()
	if sessions[0].Key != "local:/project:/dev/ttys001" {
		t.Errorf("fallback key = %q, want host:cwd:tty format", sessions[0].Key)
	}
}
```

- [ ] **Step 5: Run all registry tests**

Run: `cd /data/projects/github.com/sapaude/pager && go test ./internal/registry/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat(registry): use session_id as SessionKey, add AttentionLevel + AgentLabel to Session"
```

---

### Task 6: Frontend — Store + Filter State

**Files:**
- Modify: `frontend/src/store/sessions.ts`

- [ ] **Step 1: Update store with filter and new Session fields**

Rewrite `frontend/src/store/sessions.ts`:

```typescript
import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { ListSessions } from '../../bindings/pager/sessionservice.js'

export type AttentionLevel = 'attention' | 'running' | 'done'
export type FilterLevel = AttentionLevel

export interface AgentEvent {
  agent: string
  host: string
  cwd: string
  tty: string
  session_id: string
  term_program: string
  iterm_session_id?: string
  event_type: string
  tool_name: string
  tool_use_id: string
  content: string
  content_raw: string
  attention_level: AttentionLevel
  agent_label: string
  permission_mode?: string
  timestamp: string
}

export interface Session {
  Key: string
  Agent: string
  Host: string
  CWD: string
  TTY: string
  TermProgram: string
  ITermSessionID: string
  Status: string
  AttentionLevel: AttentionLevel
  AgentLabel: string
  SessionID: string
  LastEvent: AgentEvent | null
  PendingTools: Record<string, AgentEvent | null>
  UpdatedAt: string
}

interface SessionStore {
  sessions: Session[]
  filter: FilterLevel
  setSessions: (sessions: Session[]) => void
  setFilter: (filter: FilterLevel) => void
  filteredSessions: () => Session[]
}

export const useSessionStore = create<SessionStore>((set, get) => ({
  sessions: [],
  filter: 'attention' as FilterLevel,

  setSessions: (sessions) =>
    set({
      sessions: [...sessions].sort(
        (a, b) => new Date(b.UpdatedAt).getTime() - new Date(a.UpdatedAt).getTime()
      ),
    }),

  setFilter: (filter) => set({ filter }),

  filteredSessions: () => {
    const { sessions, filter } = get()
    return sessions.filter((s) => s.AttentionLevel === filter)
  },
}))

// Count sessions by attention level
export function useSessionCounts() {
  const sessions = useSessionStore((s) => s.sessions)
  return {
    attention: sessions.filter((s) => s.AttentionLevel === 'attention').length,
    running: sessions.filter((s) => s.AttentionLevel === 'running').length,
    done: sessions.filter((s) => s.AttentionLevel === 'done').length,
  }
}

// Initialize session sync with Wails v3 Events API.
export function initSessionSync() {
  ListSessions()
    .then((sessions: any) => {
      const valid = (sessions ?? []).filter((s: any) => s !== null)
      useSessionStore.getState().setSessions(valid)
    })
    .catch((err: unknown) => {
      console.warn('[pager] ListSessions failed:', err)
    })

  Events.On('sessions-updated', (ev: any) => {
    const sessions = ev?.data ?? ev ?? []
    useSessionStore.getState().setSessions(Array.isArray(sessions) ? sessions : [])
  })
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /data/projects/github.com/sapaude/pager/frontend && npx tsc --noEmit`
Expected: No errors (or only unrelated ones from components not yet updated)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/store/sessions.ts
git commit -m "feat(frontend): add filter state and AttentionLevel to session store"
```

---

### Task 7: Frontend — App Header with 3-Color Filter

**Files:**
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Rewrite App.tsx with filter header**

```tsx
import SessionList from './components/SessionList'
import { useSessionStore, useSessionCounts, type FilterLevel } from './store/sessions'

function App() {
  const filter = useSessionStore((s) => s.filter)
  const setFilter = useSessionStore((s) => s.setFilter)
  const counts = useSessionCounts()

  return (
    <div className="w-full h-screen bg-[--pager-bg] backdrop-blur-2xl text-[--pager-text] overflow-y-auto rounded-xl border border-[--pager-border]">
      {/* Header */}
      <header className="sticky top-0 z-10 bg-[--pager-header-bg] backdrop-blur-lg px-4 py-2.5 border-b border-[--pager-border] flex items-center justify-between">
        <h1 className="text-[13px] font-semibold text-[--pager-text-primary]">Pager</h1>
        <div className="flex gap-0.5 bg-[--pager-filter-bg] rounded-[7px] p-[2px] items-center">
          <FilterDot
            color="red"
            count={counts.attention}
            active={filter === 'attention'}
            onClick={() => setFilter('attention')}
          />
          <FilterDot
            color="green"
            count={counts.running}
            active={filter === 'running'}
            onClick={() => setFilter('running')}
          />
          <FilterDot
            color="gray"
            count={counts.done}
            active={filter === 'done'}
            onClick={() => setFilter('done')}
          />
        </div>
      </header>
      <SessionList />
    </div>
  )
}

function FilterDot({ color, count, active, onClick }: {
  color: 'red' | 'green' | 'gray'
  count: number
  active: boolean
  onClick: () => void
}) {
  const dotColors = {
    red: 'bg-[--pager-red]',
    green: 'bg-[--pager-green]',
    gray: 'bg-[--pager-gray-dot]',
  }
  const textColors = {
    red: 'text-[--pager-red]',
    green: 'text-[--pager-text-muted]',
    gray: 'text-[--pager-text-faint]',
  }

  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-1 px-[7px] py-[3px] rounded-[5px] transition-colors ${
        active ? 'bg-[--pager-filter-active]' : ''
      }`}
    >
      <span className={`w-[7px] h-[7px] rounded-full ${dotColors[color]} ${!active ? 'opacity-60' : ''}`} />
      <span className={`text-[10px] font-medium ${active ? textColors[color] : 'text-[--pager-text-faint]'}`}>
        {count}
      </span>
    </button>
  )
}

export default App
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(frontend): add 3-color filter header (attention/running/done)"
```

---

### Task 8: Frontend — SessionCard v8 Design

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx`

- [ ] **Step 1: Rewrite SessionCard with v8 design**

```tsx
import { useState } from 'react'
import type { Session } from '../store/sessions'
import { JumpToTerminal } from '../../bindings/pager/sessionservice.js'

interface Props {
  session: Session
}

export default function SessionCard({ session }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [jumping, setJumping] = useState(false)

  const content = session.LastEvent?.content ?? session.Status
  const contentRaw = session.LastEvent?.content_raw ?? ''
  const toolName = session.LastEvent?.tool_name ?? ''
  const cwdLast = session.CWD.split('/').filter(Boolean).pop() ?? session.CWD
  const sessionPrefix = (session.SessionID || session.Key || '').slice(0, 8)
  const agentLabel = session.AgentLabel || 'CC'
  const isAttention = session.AttentionLevel === 'attention'

  const handleJump = async (e: React.MouseEvent) => {
    e.stopPropagation()
    setJumping(true)
    try {
      await JumpToTerminal(session.Key)
    } catch (err) {
      console.error('Jump failed:', err)
    } finally {
      setJumping(false)
    }
  }

  const relativeTime = formatRelativeTime(session.UpdatedAt)

  const cardBg = isAttention
    ? 'bg-[--pager-card-attention-bg] border-[--pager-card-attention-border]'
    : session.AttentionLevel === 'running'
    ? 'bg-[--pager-card-running-bg] border-[--pager-card-running-border]'
    : 'bg-[--pager-card-done-bg] border-[--pager-card-done-border] opacity-60'

  return (
    <div
      className={`rounded-lg border cursor-pointer transition-all duration-150 ${cardBg}`}
      onClick={() => setExpanded(!expanded)}
    >
      <div className="px-[10px] py-[9px]">
        {/* Row 1: Agent badge + project + session_id + time */}
        <div className="flex items-center gap-[5px] mb-[5px]">
          <span className="text-[9px] font-semibold px-[5px] py-[1px] rounded-[3px] bg-[--pager-badge-bg] text-[--pager-badge-text] tracking-wide">
            {agentLabel}
          </span>
          <span className="text-[10px] text-[--pager-text-secondary]">{cwdLast}</span>
          <span className="flex-1" />
          <span className="text-[10px] text-[--pager-text-faint]">{sessionPrefix}</span>
          <span className="text-[10px] text-[--pager-text-faint]">·</span>
          <span className="text-[10px] text-[--pager-text-faint]">{relativeTime}</span>
        </div>

        {/* Row 2: Tool tag + content + arrow */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-[6px] flex-1 min-w-0">
            {toolName && (
              <span className="text-[10px] font-mono px-[5px] py-[1px] bg-[--pager-tool-bg] rounded-[3px] text-[--pager-tool-text] uppercase shrink-0 tracking-wide">
                {toolName.length > 10 ? toolName.slice(0, 10) : toolName}
              </span>
            )}
            <span className="text-[12px] text-[--pager-text-primary] whitespace-nowrap overflow-hidden text-ellipsis">
              {content}
            </span>
          </div>
          <button
            onClick={handleJump}
            disabled={jumping}
            title="Jump to terminal"
            className="ml-[6px] p-[3px] rounded text-[--pager-text-faint] hover:text-[--pager-text-secondary] disabled:opacity-30 shrink-0 transition-colors"
          >
            {jumping ? (
              <svg className="w-[14px] h-[14px] animate-spin" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="8" />
              </svg>
            ) : (
              <svg className="w-[14px] h-[14px]" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                <path d="M3 8h10M9 4l4 4-4 4" />
              </svg>
            )}
          </button>
        </div>

        {/* Expanded content */}
        {expanded && (
          <div className="mt-2">
            {contentRaw && (
              <pre className="p-[8px] bg-[--pager-detail-bg] rounded-md text-[11px] leading-relaxed text-[--pager-detail-text] font-mono whitespace-pre-wrap break-all max-h-36 overflow-y-auto border border-[--pager-detail-border] mb-2">
                {contentRaw}
              </pre>
            )}
            <div className="text-[9px] font-mono text-[--pager-text-faint] leading-[1.7]">
              <div><span className="text-[--pager-text-muted]">session:</span> {session.SessionID || session.Key}</div>
              <div><span className="text-[--pager-text-muted]">path:</span> {session.CWD}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function formatRelativeTime(iso: string): string {
  if (!iso) return ''
  const now = Date.now()
  const then = new Date(iso).getTime()
  if (isNaN(then)) return ''
  const diffSec = Math.floor((now - then) / 1000)

  if (diffSec < 5) return 'now'
  if (diffSec < 60) return `${diffSec}s`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/SessionCard.tsx
git commit -m "feat(frontend): implement v8 card design with agent badge, session prefix, expandable detail"
```

---

### Task 9: Frontend — SessionList with Filter

**Files:**
- Modify: `frontend/src/components/SessionList.tsx`
- Delete: `frontend/src/components/StatusIcon.tsx`

- [ ] **Step 1: Update SessionList to use filter**

```tsx
import { useSessionStore } from '../store/sessions'
import SessionCard from './SessionCard'

export default function SessionList() {
  const filteredSessions = useSessionStore((s) => s.filteredSessions())

  if (filteredSessions.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-[--pager-text-faint] gap-3">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z" />
          <path d="M12 6v6l4 2" />
        </svg>
        <span className="text-[13px]">No sessions in this view</span>
      </div>
    )
  }

  return (
    <div className="p-[10px] space-y-[5px]">
      {filteredSessions.map((session) => (
        <SessionCard key={session.Key} session={session} />
      ))}
    </div>
  )
}
```

- [ ] **Step 2: Delete StatusIcon.tsx**

Run: `rm frontend/src/components/StatusIcon.tsx`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/SessionList.tsx
git rm frontend/src/components/StatusIcon.tsx
git commit -m "feat(frontend): filter SessionList by attention level, remove StatusIcon"
```

---

### Task 10: Frontend — CSS Theme Variables (Light/Dark)

**Files:**
- Modify: `frontend/src/index.css`

- [ ] **Step 1: Rewrite index.css with CSS custom properties**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  /* Light mode (default) */
  --pager-bg: rgba(255, 255, 255, 0.85);
  --pager-header-bg: rgba(255, 255, 255, 0.9);
  --pager-border: rgba(0, 0, 0, 0.08);
  --pager-text: #1d1d1f;
  --pager-text-primary: #1d1d1f;
  --pager-text-secondary: rgba(0, 0, 0, 0.45);
  --pager-text-muted: rgba(0, 0, 0, 0.35);
  --pager-text-faint: rgba(0, 0, 0, 0.2);

  --pager-red: #ff3b30;
  --pager-green: #34c759;
  --pager-blue: #007aff;
  --pager-gray-dot: rgba(0, 0, 0, 0.2);

  --pager-filter-bg: rgba(0, 0, 0, 0.04);
  --pager-filter-active: rgba(0, 0, 0, 0.06);

  --pager-badge-bg: rgba(88, 86, 214, 0.1);
  --pager-badge-text: #5856d6;
  --pager-tool-bg: rgba(0, 0, 0, 0.04);
  --pager-tool-text: rgba(0, 0, 0, 0.45);

  --pager-card-attention-bg: rgba(255, 59, 48, 0.04);
  --pager-card-attention-border: rgba(255, 59, 48, 0.1);
  --pager-card-running-bg: rgba(52, 199, 89, 0.04);
  --pager-card-running-border: rgba(52, 199, 89, 0.08);
  --pager-card-done-bg: transparent;
  --pager-card-done-border: rgba(0, 0, 0, 0.04);

  --pager-detail-bg: rgba(0, 0, 0, 0.03);
  --pager-detail-border: rgba(0, 0, 0, 0.05);
  --pager-detail-text: rgba(0, 0, 0, 0.5);
}

@media (prefers-color-scheme: dark) {
  :root {
    --pager-bg: rgba(44, 44, 46, 0.92);
    --pager-header-bg: rgba(44, 44, 46, 0.95);
    --pager-border: rgba(255, 255, 255, 0.08);
    --pager-text: #ffffff;
    --pager-text-primary: rgba(255, 255, 255, 0.88);
    --pager-text-secondary: rgba(255, 255, 255, 0.45);
    --pager-text-muted: rgba(255, 255, 255, 0.35);
    --pager-text-faint: rgba(255, 255, 255, 0.2);

    --pager-red: #ff453a;
    --pager-green: #30d158;
    --pager-blue: #0a84ff;
    --pager-gray-dot: rgba(255, 255, 255, 0.25);

    --pager-filter-bg: rgba(255, 255, 255, 0.05);
    --pager-filter-active: rgba(255, 255, 255, 0.08);

    --pager-badge-bg: rgba(120, 120, 255, 0.12);
    --pager-badge-text: #a78bfa;
    --pager-tool-bg: rgba(255, 255, 255, 0.06);
    --pager-tool-text: rgba(255, 255, 255, 0.45);

    --pager-card-attention-bg: rgba(255, 69, 58, 0.06);
    --pager-card-attention-border: rgba(255, 69, 58, 0.12);
    --pager-card-running-bg: rgba(48, 209, 88, 0.04);
    --pager-card-running-border: rgba(48, 209, 88, 0.08);
    --pager-card-done-bg: transparent;
    --pager-card-done-border: rgba(255, 255, 255, 0.04);

    --pager-detail-bg: rgba(0, 0, 0, 0.3);
    --pager-detail-border: rgba(255, 255, 255, 0.04);
    --pager-detail-text: rgba(255, 255, 255, 0.5);
  }
}

body {
  margin: 0;
  padding: 0;
  overflow: hidden;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', Roboto, sans-serif;
}

::-webkit-scrollbar {
  width: 6px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--pager-text-faint);
  border-radius: 3px;
}

@keyframes pulse-status {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}
.animate-pulse-status {
  animation: pulse-status 2s ease-in-out infinite;
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/index.css
git commit -m "feat(frontend): add CSS custom properties for macOS light/dark theme support"
```

---

### Task 11: Window Behavior — Hide on Blur

**Files:**
- Modify: `main.go` (project root)

- [ ] **Step 1: Add HideOnBlur option to window**

In `main.go`, update the window creation options (around line 89):

```go
	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Pager",
		Name:             "pager-panel",
		Width:            400,
		Height:           600,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		HideOnClose:      true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})
```

Note: Wails v3's `tray.AttachWindow()` already handles hide-on-blur for tray-attached windows. If it doesn't work in alpha.96, add an explicit event handler after `app.Run()` setup — but test first.

- [ ] **Step 2: Build and verify**

Run: `cd /data/projects/github.com/sapaude/pager && go build -buildvcs=false -o bin/Pager .`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "fix(window): ensure panel hides when focus lost"
```

---

### Task 12: Install Script — --agent Parameter

**Files:**
- Modify: `scripts/install-hooks.sh`

- [ ] **Step 1: Update install-hooks.sh to accept --agent**

Replace the full script:

```bash
#!/bin/bash
# Install pager-cc-bridge hook into ~/.claude/settings.json
# Usage: ./install-hooks.sh /path/to/pager-cc-bridge [--agent LABEL]

set -e

BRIDGE_PATH=""
AGENT_LABEL="CC"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --agent)
      AGENT_LABEL="$2"
      shift 2
      ;;
    *)
      if [ -z "$BRIDGE_PATH" ]; then
        BRIDGE_PATH="$1"
      fi
      shift
      ;;
  esac
done

if [ -z "$BRIDGE_PATH" ]; then
  echo "Usage: $0 /path/to/pager-cc-bridge [--agent LABEL]"
  echo "  --agent LABEL   Agent identifier shown in Pager UI (default: CC)"
  echo "  Examples: CC, CC-Int, Codex, MyAgent"
  exit 1
fi

if [ ! -x "$BRIDGE_PATH" ]; then
  echo "Error: $BRIDGE_PATH does not exist or is not executable"
  exit 1
fi

SETTINGS="$HOME/.claude/settings.json"

HOOK_CONFIG=$(cat <<EOF
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH pre_tool_use --agent $AGENT_LABEL",
        "async": true,
        "timeout": 5
      }]
    }],
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH post_tool_use --agent $AGENT_LABEL",
        "async": true,
        "timeout": 5
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH stop --agent $AGENT_LABEL",
        "async": true,
        "timeout": 5
      }]
    }]
  }
}
EOF
)

# Backup existing config
[ -f "$SETTINGS" ] && cp "$SETTINGS" "$SETTINGS.pager-backup"

# Merge or create
if [ -f "$SETTINGS" ]; then
  if ! command -v jq &> /dev/null; then
    echo "Error: jq is required. Install with: brew install jq"
    exit 1
  fi
  jq -s '.[0] * .[1]' "$SETTINGS" <(echo "$HOOK_CONFIG") > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
else
  mkdir -p "$(dirname "$SETTINGS")"
  echo "$HOOK_CONFIG" > "$SETTINGS"
fi

echo "✓ Hook installed to $SETTINGS"
echo "  Agent label: $AGENT_LABEL"
echo "  Restart Claude Code to activate."
```

- [ ] **Step 2: Commit**

```bash
git add scripts/install-hooks.sh
git commit -m "feat(install): support --agent parameter for multi-agent hook identity"
```

---

### Task 13: Build + Regenerate Bindings + Verify

**Files:**
- All (integration verification)

- [ ] **Step 1: Regenerate Wails bindings**

Run: `cd /data/projects/github.com/sapaude/pager && wails3 generate bindings`
Expected: "Processed: ... Services, ... Methods, ... Models"

- [ ] **Step 2: Build frontend**

Run: `cd /data/projects/github.com/sapaude/pager/frontend && npm run build`
Expected: "✓ built in Xms"

- [ ] **Step 3: Build Go binary**

Run: `cd /data/projects/github.com/sapaude/pager && go build -buildvcs=false -o bin/Pager .`
Expected: No errors (ld warnings are fine)

- [ ] **Step 4: Build bridge binary**

Run: `cd /data/projects/github.com/sapaude/pager && go build -buildvcs=false -o bin/pager-cc-bridge ./cmd/bridge/`
Expected: No errors

- [ ] **Step 5: Run all Go tests**

Run: `cd /data/projects/github.com/sapaude/pager && go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" `
Expected: All `ok`

- [ ] **Step 6: Integration smoke test**

```bash
# Restart Pager
pkill -f "bin/Pager" 2>/dev/null; sleep 2

# Send attention-level event
curl -s -X POST http://127.0.0.1:7421/event -H "Content-Type: application/json" -d '{
  "agent":"claude-code","host":"local","cwd":"/Users/test/project","tty":"/dev/ttys005",
  "session_id":"smoke-test-001","term_program":"iTerm.app",
  "event_type":"pre_tool_use","tool_name":"AskUserQuestion","tool_use_id":"st-1",
  "content":"Which UI style?","content_raw":"Which UI style do you prefer?",
  "attention_level":"attention","agent_label":"CC","permission_mode":"default",
  "timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"
}'

# Verify session exists with correct key
curl -s http://127.0.0.1:7421/sessions | python3 -c "
import json, sys
data = json.load(sys.stdin)
for s in data:
    if s.get('SessionID') == 'smoke-test-001':
        assert s['Key'] == 'smoke-test-001', f'Key should be session_id, got {s[\"Key\"]}'
        assert s['AgentLabel'] == 'CC', f'AgentLabel wrong: {s[\"AgentLabel\"]}'
        assert s['AttentionLevel'] == 'attention', f'AttentionLevel wrong: {s[\"AttentionLevel\"]}'
        print('✓ Smoke test passed')
        sys.exit(0)
print('✗ Session not found')
sys.exit(1)
"
```

- [ ] **Step 7: Commit all generated files**

```bash
git add frontend/bindings/
git commit -m "chore: regenerate Wails bindings for v1.1 schema"
```

---

## Execution Checklist

| Task | Component | Est. Time |
|------|-----------|-----------|
| 1 | AgentEvent new fields + SessionKey | 3 min |
| 2 | Attention level logic + tests | 5 min |
| 3 | Bridge --agent parsing | 5 min |
| 4 | Content extraction enhancement | 5 min |
| 5 | Registry session_id key + tests | 5 min |
| 6 | Frontend store + filter | 3 min |
| 7 | App header 3-color filter | 3 min |
| 8 | SessionCard v8 design | 5 min |
| 9 | SessionList filter + cleanup | 2 min |
| 10 | CSS theme variables | 3 min |
| 11 | Window hide-on-blur | 2 min |
| 12 | Install script --agent | 3 min |
| 13 | Build + verify integration | 5 min |
