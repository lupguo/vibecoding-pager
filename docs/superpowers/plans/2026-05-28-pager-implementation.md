# Pager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Pager — a macOS MenuBar app that receives Claude Code hook events, displays session status, sends notifications, and enables one-click terminal jump-back.

**Architecture:** Bridge binary (stdin → HTTP POST) pushes AgentEvent to an HTTP server inside a Wails v3 single-process app. Server feeds a Registry (in-memory state machine) which emits events to the React frontend via Wails Events and triggers macOS notifications via osascript.

**Tech Stack:** Go 1.24, Wails v3, React 18, TypeScript, Tailwind CSS, zustand

---

## File Structure

```
pager/
├── go.mod
├── go.sum
├── main.go                        # Wails v3 entry point
├── app.go                         # App struct, OnStartup/OnShutdown
├── service.go                     # SessionService (frontend bindings)
├── cmd/
│   └── bridge/
│       └── main.go                # pager-cc-bridge executable
├── internal/
│   ├── event/
│   │   └── types.go              # AgentEvent + constants
│   ├── bridge/
│   │   ├── types.go              # CCHookInput + tool input structs
│   │   ├── extractor.go          # Content extraction logic
│   │   ├── extractor_test.go     # Tests for extractor
│   │   └── poster.go             # HTTP POST client
│   ├── registry/
│   │   ├── registry.go           # Registry + Session + state machine
│   │   └── registry_test.go      # Tests for Apply logic
│   ├── server/
│   │   ├── server.go             # HTTP server (:7421)
│   │   └── server_test.go        # Integration test
│   ├── notify/
│   │   └── notify.go             # osascript notifications
│   └── terminal/
│       └── jump.go               # osascript terminal jump
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── postcss.config.js
│   ├── tailwind.config.js
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── store/
│       │   └── sessions.ts       # zustand store
│       └── components/
│           ├── SessionList.tsx
│           ├── SessionCard.tsx
│           └── StatusIcon.tsx
├── scripts/
│   ├── install-hooks.sh
│   └── install-launchd.sh
└── wails.json
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: directory structure

- [ ] **Step 1: Initialize Go module**

```bash
cd /private/data/projects/github.com/sapaude/pager
go mod init pager
```

Then edit `go.mod` to set Go 1.24:

```go
module pager

go 1.24
```

- [ ] **Step 2: Create directory structure**

```bash
mkdir -p cmd/bridge
mkdir -p internal/event
mkdir -p internal/bridge
mkdir -p internal/registry
mkdir -p internal/server
mkdir -p internal/notify
mkdir -p internal/terminal
mkdir -p frontend/src/components
mkdir -p frontend/src/store
mkdir -p scripts
```

- [ ] **Step 3: Commit**

```bash
git init
git add go.mod CLAUDE.md docs/
git commit -m "feat: initialize pager project with go.mod and spec docs"
```

---

## Task 2: Event Types

**Files:**
- Create: `internal/event/types.go`

- [ ] **Step 1: Write event types**

```go
package event

import (
	"encoding/json"
	"time"
)

// EventType constants
const (
	EventPreToolUse  = "pre_tool_use"
	EventPostToolUse = "post_tool_use"
	EventStop        = "stop"
	EventError       = "error"
	EventNotification = "notification"
)

// Agent constants
const (
	AgentClaudeCode = "claude-code"
	AgentCodex      = "codex"
)

// SessionStatus constants
const (
	StatusWaiting  = "waiting"
	StatusActive   = "active"
	StatusFinished = "finished"
	StatusError    = "error"
)

// AgentEvent is the single cross-layer data structure.
// Bridge fills all fields; server and UI are read-only consumers.
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
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// SessionKey returns the unique session identifier (host:cwd:tty triple).
func (e *AgentEvent) SessionKey() string {
	return e.Host + ":" + e.CWD + ":" + e.TTY
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/event/
```

Expected: no output (success)

- [ ] **Step 3: Commit**

```bash
git add internal/event/types.go
git commit -m "feat: add AgentEvent type and constants"
```

---

## Task 3: Bridge Types

**Files:**
- Create: `internal/bridge/types.go`

- [ ] **Step 1: Write bridge input types**

```go
package bridge

import "encoding/json"

// CCHookInput is the JSON structure CC passes via stdin to hooks.
type CCHookInput struct {
	SessionID string          `json:"session_id"`
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	ToolUseID string          `json:"tool_use_id"`
}

// BashInput is tool_input for Bash tool.
type BashInput struct {
	Command string `json:"command"`
}

// FileInput is tool_input for Edit/Write/Read tools.
type FileInput struct {
	FilePath string `json:"file_path"`
}

// GlobInput is tool_input for Glob tool.
type GlobInput struct {
	Pattern string `json:"pattern"`
}

// GrepInput is tool_input for Grep tool.
type GrepInput struct {
	Pattern string `json:"pattern"`
}

// WebFetchInput is tool_input for WebFetch tool.
type WebFetchInput struct {
	URL string `json:"url"`
}

// WebSearchInput is tool_input for WebSearch tool.
type WebSearchInput struct {
	Query string `json:"query"`
}

// TaskInput is tool_input for Task (subagent) tool.
type TaskInput struct {
	Description string `json:"description"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/bridge/
```

Expected: no output (success)

- [ ] **Step 3: Commit**

```bash
git add internal/bridge/types.go
git commit -m "feat: add CC hook input types for bridge"
```

---

## Task 4: Bridge Extractor with Tests

**Files:**
- Create: `internal/bridge/extractor.go`
- Create: `internal/bridge/extractor_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package bridge

import (
	"encoding/json"
	"testing"
)

func TestExtractContent_Bash(t *testing.T) {
	input := json.RawMessage(`{"command":"go build ./..."}`)
	raw, content := ExtractContent("Bash", input)
	if raw != "go build ./..." {
		t.Errorf("raw = %q, want %q", raw, "go build ./...")
	}
	if content != "go build ./..." {
		t.Errorf("content = %q, want %q", content, "go build ./...")
	}
}

func TestExtractContent_Edit(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/src/main.go"}`)
	raw, content := ExtractContent("Edit", input)
	if raw != "编辑 /src/main.go" {
		t.Errorf("raw = %q, want %q", raw, "编辑 /src/main.go")
	}
	if content != "编辑 /src/main.go" {
		t.Errorf("content = %q, want %q", content, "编辑 /src/main.go")
	}
}

func TestExtractContent_Write(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/out.txt"}`)
	raw, content := ExtractContent("Write", input)
	if raw != "写入 /tmp/out.txt" {
		t.Errorf("raw = %q, want %q", raw, "写入 /tmp/out.txt")
	}
	if content != "写入 /tmp/out.txt" {
		t.Errorf("content = %q, want %q", content, "写入 /tmp/out.txt")
	}
}

func TestExtractContent_Read(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/etc/hosts"}`)
	raw, content := ExtractContent("Read", input)
	if raw != "读取 /etc/hosts" {
		t.Errorf("raw = %q, want %q", raw, "读取 /etc/hosts")
	}
	if content != "读取 /etc/hosts" {
		t.Errorf("content = %q, want %q", content, "读取 /etc/hosts")
	}
}

func TestExtractContent_Glob(t *testing.T) {
	input := json.RawMessage(`{"pattern":"**/*.go"}`)
	raw, _ := ExtractContent("Glob", input)
	if raw != "查找 **/*.go" {
		t.Errorf("raw = %q, want %q", raw, "查找 **/*.go")
	}
}

func TestExtractContent_Grep(t *testing.T) {
	input := json.RawMessage(`{"pattern":"TODO"}`)
	raw, _ := ExtractContent("Grep", input)
	if raw != "搜索 TODO" {
		t.Errorf("raw = %q, want %q", raw, "搜索 TODO")
	}
}

func TestExtractContent_WebFetch(t *testing.T) {
	input := json.RawMessage(`{"url":"https://example.com"}`)
	raw, _ := ExtractContent("WebFetch", input)
	if raw != "抓取 https://example.com" {
		t.Errorf("raw = %q, want %q", raw, "抓取 https://example.com")
	}
}

func TestExtractContent_WebSearch(t *testing.T) {
	input := json.RawMessage(`{"query":"golang wails v3"}`)
	raw, _ := ExtractContent("WebSearch", input)
	if raw != "搜索 golang wails v3" {
		t.Errorf("raw = %q, want %q", raw, "搜索 golang wails v3")
	}
}

func TestExtractContent_Task(t *testing.T) {
	input := json.RawMessage(`{"description":"Run linter"}`)
	raw, _ := ExtractContent("Task", input)
	if raw != "子任务: Run linter" {
		t.Errorf("raw = %q, want %q", raw, "子任务: Run linter")
	}
}

func TestExtractContent_MCP(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("mcp__github__create_pr", input)
	if raw != "MCP: create_pr" {
		t.Errorf("raw = %q, want %q", raw, "MCP: create_pr")
	}
}

func TestExtractContent_Unknown(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("SomeNewTool", input)
	if raw != "SomeNewTool" {
		t.Errorf("raw = %q, want %q", raw, "SomeNewTool")
	}
}

func TestExtractContent_Truncation(t *testing.T) {
	// Build a command longer than 60 runes
	longCmd := "echo 'this is a very long command that definitely exceeds sixty characters limit for display'"
	input, _ := json.Marshal(BashInput{Command: longCmd})
	raw, content := ExtractContent("Bash", json.RawMessage(input))
	if raw != longCmd {
		t.Errorf("raw should be untruncated")
	}
	runes := []rune(content)
	if len(runes) != 61 { // 60 + "…"
		t.Errorf("content rune len = %d, want 61 (60 + ellipsis)", len(runes))
	}
	if string(runes[60]) != "…" {
		t.Errorf("last rune should be ellipsis, got %q", string(runes[60]))
	}
}

func TestExtractContent_EmptyInput(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("Bash", input)
	// Empty command falls through to toolName fallback
	if raw != "Bash" {
		t.Errorf("raw = %q, want %q", raw, "Bash")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/bridge/ -v
```

Expected: compilation error — `ExtractContent` not defined.

- [ ] **Step 3: Write the extractor implementation**

```go
package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
)

const contentMaxRunes = 60

// ExtractContent extracts a human-readable summary from tool name + input.
// Returns (contentRaw, content) where content is truncated to contentMaxRunes.
func ExtractContent(toolName string, toolInput json.RawMessage) (contentRaw, content string) {
	raw := extractRaw(toolName, toolInput)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = toolName
	}
	return raw, truncateRunes(raw, contentMaxRunes)
}

func extractRaw(toolName string, toolInput json.RawMessage) string {
	unmarshal := func(v interface{}) bool {
		return json.Unmarshal(toolInput, v) == nil
	}

	switch toolName {
	case "Bash":
		var in BashInput
		if unmarshal(&in) && in.Command != "" {
			return in.Command
		}
	case "Edit":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "编辑 " + in.FilePath
		}
	case "Write":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "写入 " + in.FilePath
		}
	case "Read":
		var in FileInput
		if unmarshal(&in) && in.FilePath != "" {
			return "读取 " + in.FilePath
		}
	case "Glob":
		var in GlobInput
		if unmarshal(&in) && in.Pattern != "" {
			return "查找 " + in.Pattern
		}
	case "Grep":
		var in GrepInput
		if unmarshal(&in) && in.Pattern != "" {
			return "搜索 " + in.Pattern
		}
	case "WebFetch":
		var in WebFetchInput
		if unmarshal(&in) && in.URL != "" {
			return "抓取 " + in.URL
		}
	case "WebSearch":
		var in WebSearchInput
		if unmarshal(&in) && in.Query != "" {
			return "搜索 " + in.Query
		}
	case "Task":
		var in TaskInput
		if unmarshal(&in) && in.Description != "" {
			return "子任务: " + in.Description
		}
	}

	// MCP tools: mcp__github__create_pr etc.
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(toolName, "__")
		return fmt.Sprintf("MCP: %s", parts[len(parts)-1])
	}

	return toolName
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/bridge/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/extractor.go internal/bridge/extractor_test.go
git commit -m "feat: implement content extractor with tests"
```

---

## Task 5: Bridge Poster

**Files:**
- Create: `internal/bridge/poster.go`

- [ ] **Step 1: Write poster implementation**

```go
package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"pager/internal/event"
)

const serverURL = "http://127.0.0.1:7421/event"

// PostEvent sends an AgentEvent to the Pager server.
// Timeout is 1 second. Failures are silent (stderr warning only).
func PostEvent(e event.AgentEvent) {
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[pager-bridge] warn: %v\n", err)
		return
	}
	defer resp.Body.Close()
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/bridge/
```

Expected: no output (success)

- [ ] **Step 3: Commit**

```bash
git add internal/bridge/poster.go
git commit -m "feat: add HTTP poster for bridge events"
```

---

## Task 6: Registry with Tests

**Files:**
- Create: `internal/registry/registry.go`
- Create: `internal/registry/registry_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package registry

import (
	"testing"
	"time"

	"pager/internal/event"
)

func makeEvent(eventType, toolName, toolUseID, cwd, tty string) *event.AgentEvent {
	return &event.AgentEvent{
		Agent:     event.AgentClaudeCode,
		Host:      "local",
		CWD:       cwd,
		TTY:       tty,
		EventType: eventType,
		ToolName:  toolName,
		ToolUseID: toolUseID,
		Content:   toolName,
		Timestamp: time.Now(),
	}
}

func TestApply_PreToolUse_CreatesSession(t *testing.T) {
	var called int
	reg := New(func(sessions []*Session) { called++ })

	e := makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/project", "/dev/ttys001")
	reg.Apply(e)

	sessions := reg.ListSorted()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.Status != event.StatusWaiting {
		t.Errorf("status = %q, want %q", s.Status, event.StatusWaiting)
	}
	if s.Key != "local:/project:/dev/ttys001" {
		t.Errorf("key = %q, want %q", s.Key, "local:/project:/dev/ttys001")
	}
	if _, ok := s.PendingTools["tu-1"]; !ok {
		t.Error("tu-1 should be in PendingTools")
	}
	if called != 1 {
		t.Errorf("onChange called %d times, want 1", called)
	}
}

func TestApply_PostToolUse_ClearsPending(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	// pre creates pending
	reg.Apply(makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	// post clears it
	reg.Apply(makeEvent(event.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))

	if len(lastSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(lastSessions))
	}
	s := lastSessions[0]
	if s.Status != event.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, event.StatusActive)
	}
	if len(s.PendingTools) != 0 {
		t.Errorf("PendingTools should be empty, has %d", len(s.PendingTools))
	}
}

func TestApply_PostToolUse_MultiplePending(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	// Two pre_tool_use events
	reg.Apply(makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(event.EventPreToolUse, "Edit", "tu-2", "/p", "/dev/ttys001"))

	// Clear one - should still be waiting
	reg.Apply(makeEvent(event.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	s := lastSessions[0]
	if s.Status != event.StatusWaiting {
		t.Errorf("status = %q, want %q (still has pending)", s.Status, event.StatusWaiting)
	}

	// Clear the other - now active
	reg.Apply(makeEvent(event.EventPostToolUse, "Edit", "tu-2", "/p", "/dev/ttys001"))
	s = lastSessions[0]
	if s.Status != event.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, event.StatusActive)
	}
}

func TestApply_Stop(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	reg.Apply(makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(event.EventStop, "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != event.StatusFinished {
		t.Errorf("status = %q, want %q", s.Status, event.StatusFinished)
	}
}

func TestApply_Error(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	reg.Apply(makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(event.EventError, "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != event.StatusError {
		t.Errorf("status = %q, want %q", s.Status, event.StatusError)
	}
}

func TestListSorted_OrderByUpdatedAt(t *testing.T) {
	reg := New(func([]*Session) {})

	// Create two sessions with different times
	e1 := makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/project-a", "/dev/ttys001")
	e1.Timestamp = time.Now().Add(-10 * time.Second)
	reg.Apply(e1)

	e2 := makeEvent(event.EventPreToolUse, "Bash", "tu-2", "/project-b", "/dev/ttys002")
	e2.Timestamp = time.Now()
	reg.Apply(e2)

	sessions := reg.ListSorted()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Most recent first
	if sessions[0].CWD != "/project-b" {
		t.Errorf("first session CWD = %q, want /project-b", sessions[0].CWD)
	}
}

func TestGetByTTY(t *testing.T) {
	reg := New(func([]*Session) {})

	reg.Apply(makeEvent(event.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys005"))

	s, ok := reg.GetByTTY("/dev/ttys005")
	if !ok {
		t.Fatal("expected to find session by TTY")
	}
	if s.TTY != "/dev/ttys005" {
		t.Errorf("TTY = %q, want /dev/ttys005", s.TTY)
	}

	_, ok = reg.GetByTTY("/dev/ttys999")
	if ok {
		t.Error("should not find non-existent TTY")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/registry/ -v
```

Expected: compilation error — package not yet implemented.

- [ ] **Step 3: Write the Registry implementation**

```go
package registry

import (
	"sort"
	"sync"
	"time"

	"pager/internal/event"
)

// Session represents an active agent session.
type Session struct {
	Key            string                       `json:"Key"`
	Agent          string                       `json:"Agent"`
	Host           string                       `json:"Host"`
	CWD            string                       `json:"CWD"`
	TTY            string                       `json:"TTY"`
	TermProgram    string                       `json:"TermProgram"`
	ITermSessionID string                       `json:"ITermSessionID"`
	Status         string                       `json:"Status"`
	LastEvent      *event.AgentEvent            `json:"LastEvent"`
	PendingTools   map[string]*event.AgentEvent `json:"PendingTools"`
	UpdatedAt      time.Time                    `json:"UpdatedAt"`
}

// Registry is a thread-safe session registry.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	onChange func(sessions []*Session)
}

// New creates a Registry with an onChange callback.
func New(onChange func([]*Session)) *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
		onChange: onChange,
	}
}

// Apply processes an AgentEvent and updates the Registry state.
func (r *Registry) Apply(e *event.AgentEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := e.SessionKey()
	s, exists := r.sessions[key]
	if !exists {
		s = &Session{
			Key:          key,
			Agent:        e.Agent,
			Host:         e.Host,
			CWD:          e.CWD,
			TTY:          e.TTY,
			PendingTools: make(map[string]*event.AgentEvent),
		}
		r.sessions[key] = s
	}

	// Update terminal info if provided
	if e.TermProgram != "" {
		s.TermProgram = e.TermProgram
	}
	if e.ITermSessionID != "" {
		s.ITermSessionID = e.ITermSessionID
	}

	s.LastEvent = e
	s.UpdatedAt = e.Timestamp
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}

	switch e.EventType {
	case event.EventPreToolUse:
		s.Status = event.StatusWaiting
		if e.ToolUseID != "" {
			s.PendingTools[e.ToolUseID] = e
		}
	case event.EventPostToolUse:
		if e.ToolUseID != "" {
			delete(s.PendingTools, e.ToolUseID)
		}
		if len(s.PendingTools) == 0 {
			s.Status = event.StatusActive
		}
		// else still waiting for other pending tools
	case event.EventStop:
		s.Status = event.StatusFinished
		s.PendingTools = make(map[string]*event.AgentEvent)
	case event.EventError:
		s.Status = event.StatusError
	}

	if r.onChange != nil {
		r.onChange(r.listSortedLocked())
	}
}

// ListSorted returns sessions ordered by UpdatedAt descending (newest first).
func (r *Registry) ListSorted() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listSortedLocked()
}

func (r *Registry) listSortedLocked() []*Session {
	result := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// GetByTTY finds a session by TTY.
func (r *Registry) GetByTTY(tty string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sessions {
		if s.TTY == tty {
			return s, true
		}
	}
	return nil, false
}

// GetByKey finds a session by its key.
func (r *Registry) GetByKey(key string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[key]
	return s, ok
}

// Remove deletes a session by key.
func (r *Registry) Remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, key)
	if r.onChange != nil {
		r.onChange(r.listSortedLocked())
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/registry/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat: implement session registry with state machine"
```

---

## Task 7: HTTP Server

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Write the server test**

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pager/internal/event"
	"pager/internal/registry"
)

func TestHandleEvent_ValidPost(t *testing.T) {
	var appliedEvent *event.AgentEvent
	reg := registry.New(func([]*registry.Session) {})
	// We'll test through the server's handler directly
	srv := New(reg)

	e := event.AgentEvent{
		Agent:     event.AgentClaudeCode,
		Host:      "local",
		CWD:       "/project",
		TTY:       "/dev/ttys001",
		EventType: event.EventPreToolUse,
		ToolName:  "Bash",
		ToolUseID: "tu-1",
		Content:   "go build",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(e)

	req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	sessions := reg.ListSorted()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	_ = appliedEvent // used for potential future assertions
}

func TestHandleEvent_RejectsGet(t *testing.T) {
	reg := registry.New(func([]*registry.Session) {})
	srv := New(reg)

	req := httptest.NewRequest(http.MethodGet, "/event", nil)
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleEvent_BadJSON(t *testing.T) {
	reg := registry.New(func([]*registry.Session) {})
	srv := New(reg)

	req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSessions(t *testing.T) {
	reg := registry.New(func([]*registry.Session) {})
	srv := New(reg)

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	w := httptest.NewRecorder()

	srv.HandleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/server/ -v
```

Expected: compilation error.

- [ ] **Step 3: Write server implementation**

```go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"pager/internal/event"
	"pager/internal/registry"
)

const ListenAddr = "127.0.0.1:7421"

// Server handles HTTP requests from bridge processes.
type Server struct {
	reg *registry.Registry
}

// New creates a Server with the given registry.
func New(reg *registry.Registry) *Server {
	return &Server{reg: reg}
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/event", s.HandleEvent)
	mux.HandleFunc("/sessions", s.HandleSessions)
	go func() {
		log.Printf("[pager-server] listening on %s", ListenAddr)
		if err := http.ListenAndServe(ListenAddr, mux); err != nil {
			log.Printf("[pager-server] error: %v", err)
		}
	}()
}

// HandleEvent processes incoming AgentEvent POST requests.
func (s *Server) HandleEvent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var e event.AgentEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.reg.Apply(&e)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// HandleSessions returns all sessions as JSON (debug endpoint).
func (s *Server) HandleSessions(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.reg.ListSorted())
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/server/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat: implement HTTP server for event ingestion"
```

---

## Task 8: Notify Module

**Files:**
- Create: `internal/notify/notify.go`

- [ ] **Step 1: Write notification implementation**

```go
package notify

import (
	"fmt"
	"os/exec"
	"strings"

	"pager/internal/event"
)

// Show triggers a macOS system notification based on event type.
// Uses osascript. Only fires for pre_tool_use, stop, and error events.
func Show(e *event.AgentEvent) {
	var title, body string
	switch e.EventType {
	case event.EventPreToolUse:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
		if body == "" {
			body = "等待确认: " + e.ToolName
		}
	case event.EventStop:
		title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
		body = "任务完成"
	case event.EventError:
		title = fmt.Sprintf("%s · %s [错误]", agentLabel(e.Agent), lastPath(e.CWD))
		body = e.Content
	default:
		return
	}
	showOsascript(title, body)
}

func showOsascript(title, body string) {
	title = sanitize(title)
	body = sanitize(body)
	script := fmt.Sprintf(
		`display notification "%s" with title "Pager" subtitle "%s" sound name "Tink"`,
		body, title,
	)
	_ = exec.Command("osascript", "-e", script).Run()
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	r := []rune(s)
	if len(r) > 100 {
		s = string(r[:100]) + "…"
	}
	return s
}

func agentLabel(agent string) string {
	switch agent {
	case event.AgentClaudeCode:
		return "Claude Code"
	case event.AgentCodex:
		return "Codex"
	default:
		return agent
	}
}

func lastPath(p string) string {
	parts := strings.Split(strings.TrimRight(p, "/"), "/")
	if len(parts) == 0 || p == "" {
		return p
	}
	return parts[len(parts)-1]
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/notify/
```

Expected: no output (success)

- [ ] **Step 3: Commit**

```bash
git add internal/notify/notify.go
git commit -m "feat: add macOS notification via osascript"
```

---

## Task 9: Terminal Jump Module

**Files:**
- Create: `internal/terminal/jump.go`

- [ ] **Step 1: Write terminal jump implementation**

```go
package terminal

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// JumpRequest contains the info needed to activate a terminal tab.
type JumpRequest struct {
	TTY            string
	TermProgram    string
	ITermSessionID string
}

// Jump activates the terminal tab matching the given request.
func Jump(req JumpRequest) error {
	switch {
	case strings.Contains(req.TermProgram, "iTerm"):
		return jumpITerm(req)
	case req.TermProgram == "Apple_Terminal":
		return jumpAppleTerminal(req.TTY)
	case strings.Contains(req.TermProgram, "WezTerm"):
		return jumpWezTerm(req.TTY)
	default:
		return activateApp(req.TermProgram)
	}
}

func jumpITerm(req JumpRequest) error {
	var script string
	if req.ITermSessionID != "" {
		script = fmt.Sprintf(`
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (unique id of s) contains "%s" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell`, req.ITermSessionID)
	} else {
		script = fmt.Sprintf(`
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (tty of s) is "%s" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell`, req.TTY)
	}
	return runOsa(script)
}

func jumpAppleTerminal(tty string) error {
	script := fmt.Sprintf(`
tell application "Terminal"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      if (tty of t) is "%s" then
        set selected of t to true
        set index of w to 1
        return
      end if
    end repeat
  end repeat
end tell`, tty)
	return runOsa(script)
}

func jumpWezTerm(tty string) error {
	return activateApp("WezTerm")
}

func activateApp(termProgram string) error {
	app := strings.TrimSuffix(termProgram, ".app")
	if app == "" {
		app = "Terminal"
	}
	return runOsa(fmt.Sprintf(`tell application "%s" to activate`, app))
}

func runOsa(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[terminal] osascript error: %v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return err
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/terminal/
```

Expected: no output (success)

- [ ] **Step 3: Commit**

```bash
git add internal/terminal/jump.go
git commit -m "feat: add terminal jump via osascript"
```

---

## Task 10: Bridge Main Program

**Files:**
- Create: `cmd/bridge/main.go`

- [ ] **Step 1: Write bridge main**

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

	eventType := "unknown"
	if len(os.Args) > 1 {
		eventType = os.Args[1]
	}

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
		RawPayload:     raw,
		Timestamp:      time.Now(),
	}

	bridge.PostEvent(e)
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

- [ ] **Step 2: Build the bridge binary**

```bash
go build -o /tmp/pager-cc-bridge ./cmd/bridge/
```

Expected: binary created at `/tmp/pager-cc-bridge`

- [ ] **Step 3: Commit**

```bash
git add cmd/bridge/main.go
git commit -m "feat: implement pager-cc-bridge main program"
```

---

## Task 11: Layer 1 Full Verification

- [ ] **Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS

- [ ] **Step 2: Build all packages**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 3: Build bridge binary**

```bash
go build -o /tmp/pager-cc-bridge ./cmd/bridge/
ls -la /tmp/pager-cc-bridge
```

Expected: executable file exists

---

## Task 12: Wails App Core (app.go)

**Files:**
- Create: `app.go`

- [ ] **Step 1: Write app.go**

```go
package main

import (
	"context"
	"log"

	"pager/internal/event"
	"pager/internal/notify"
	"pager/internal/registry"
	"pager/internal/server"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App holds the Wails application state.
type App struct {
	wailsApp *application.App
	reg      *registry.Registry
	srv      *server.Server
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// SetWailsApp injects the Wails app reference (called after app creation).
func (a *App) SetWailsApp(app *application.App) {
	a.wailsApp = app
}

// OnStartup is called by Wails when the app starts.
func (a *App) OnStartup(ctx context.Context, options application.ServiceOptions) error {
	a.reg = registry.New(func(sessions []*registry.Session) {
		// Push to frontend
		if a.wailsApp != nil {
			a.wailsApp.EmitEvent("sessions-updated", sessions)
		}

		// Trigger notification for the latest waiting/stop/error session
		if len(sessions) > 0 {
			latest := sessions[0]
			if latest.LastEvent != nil {
				switch latest.LastEvent.EventType {
				case event.EventPreToolUse, event.EventStop, event.EventError:
					notify.Show(latest.LastEvent)
				}
			}
		}
	})

	a.srv = server.New(a.reg)
	a.srv.Start()
	log.Println("[pager] app started")
	return nil
}

// OnShutdown is called by Wails when the app is closing.
func (a *App) OnShutdown(ctx context.Context) {
	log.Println("[pager] app shutting down")
}

// Registry returns the registry (for SessionService).
func (a *App) Registry() *registry.Registry {
	return a.reg
}
```

- [ ] **Step 2: Add Wails v3 dependency**

```bash
go get github.com/wailsapp/wails/v3@latest
```

- [ ] **Step 3: Verify compilation (may need go mod tidy)**

```bash
go mod tidy
go build ./...
```

Note: If Wails v3 is not yet available as a stable module, use the alpha/beta tag or commit hash. Check `go list -m -versions github.com/wailsapp/wails/v3` for available versions.

- [ ] **Step 4: Commit**

```bash
git add app.go go.mod go.sum
git commit -m "feat: add Wails app with registry and notification integration"
```

---

## Task 13: Session Service (service.go)

**Files:**
- Create: `service.go`

- [ ] **Step 1: Write service.go**

```go
package main

import (
	"fmt"

	"pager/internal/registry"
	"pager/internal/terminal"
)

// SessionService exposes methods to the React frontend via Wails bindings.
type SessionService struct {
	reg *registry.Registry
}

// NewSessionService creates a SessionService with the given registry.
func NewSessionService(reg *registry.Registry) *SessionService {
	return &SessionService{reg: reg}
}

// ListSessions returns all sessions sorted by UpdatedAt descending.
func (s *SessionService) ListSessions() []*registry.Session {
	if s.reg == nil {
		return nil
	}
	return s.reg.ListSorted()
}

// JumpToTerminal activates the terminal tab for the given session.
func (s *SessionService) JumpToTerminal(sessionKey string) error {
	if s.reg == nil {
		return fmt.Errorf("registry not initialized")
	}
	session, ok := s.reg.GetByKey(sessionKey)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionKey)
	}
	return terminal.Jump(terminal.JumpRequest{
		TTY:            session.TTY,
		TermProgram:    session.TermProgram,
		ITermSessionID: session.ITermSessionID,
	})
}

// DismissSession removes a session from the registry.
func (s *SessionService) DismissSession(sessionKey string) {
	if s.reg != nil {
		s.reg.Remove(sessionKey)
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add service.go
git commit -m "feat: add SessionService for frontend bindings"
```

---

## Task 14: Wails Main Entry (main.go)

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write main.go**

```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	pagerApp := NewApp()

	app := application.New(application.Options{
		Name:        "Pager",
		Description: "AI coding agents 状态感知层",
		Services: []application.Service{
			application.NewService(pagerApp),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	pagerApp.SetWailsApp(app)

	// System tray
	tray := app.NewSystemTray()
	_ = tray // Icon setup done after build with embedded resources

	// Popup window (attached to tray)
	window := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:            "Pager",
		Width:            400,
		Height:           600,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})
	_ = window

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go mod tidy
go build .
```

Note: This will fail until `frontend/dist` exists. For now, create a placeholder:

```bash
mkdir -p frontend/dist
echo "<html><body>Pager</body></html>" > frontend/dist/index.html
```

Then retry:

```bash
go build .
```

- [ ] **Step 3: Commit**

```bash
git add main.go frontend/dist/index.html
git commit -m "feat: add Wails v3 main entry point with tray and window"
```

---

## Task 15: Frontend Scaffolding

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/tsconfig.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tailwind.config.js`
- Create: `frontend/postcss.config.js`
- Create: `frontend/index.html`
- Create: `frontend/src/main.tsx`

- [ ] **Step 1: Write package.json**

```json
{
  "name": "pager-frontend",
  "private": true,
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "zustand": "^5.0.0",
    "@wailsio/runtime": "^3.0.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.0",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.4.47",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.6.0",
    "vite": "^6.0.0"
  }
}
```

- [ ] **Step 2: Write tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src"]
}
```

- [ ] **Step 3: Write vite.config.ts**

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
  },
})
```

- [ ] **Step 4: Write tailwind.config.js**

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
```

- [ ] **Step 5: Write postcss.config.js**

```javascript
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

- [ ] **Step 6: Write index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Pager</title>
  </head>
  <body class="bg-transparent">
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 7: Write src/main.tsx**

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'
import { initSessionSync } from './store/sessions'

initSessionSync()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```

- [ ] **Step 8: Write src/index.css**

Create `frontend/src/index.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

body {
  margin: 0;
  padding: 0;
  overflow: hidden;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
```

- [ ] **Step 9: Commit**

```bash
git add frontend/package.json frontend/tsconfig.json frontend/vite.config.ts \
  frontend/tailwind.config.js frontend/postcss.config.js frontend/index.html \
  frontend/src/main.tsx frontend/src/index.css
git commit -m "feat: scaffold frontend with React, Vite, Tailwind"
```

---

## Task 16: Frontend Store

**Files:**
- Create: `frontend/src/store/sessions.ts`

- [ ] **Step 1: Write zustand store**

```typescript
import { create } from 'zustand'

export type SessionStatus = 'waiting' | 'active' | 'finished' | 'error'

export interface Session {
  Key: string
  Agent: string
  Host: string
  CWD: string
  TTY: string
  TermProgram: string
  ITermSessionID: string
  Status: SessionStatus
  LastEvent: {
    Content: string
    ContentRaw: string
    ToolName: string
    EventType: string
  } | null
  UpdatedAt: string
}

interface SessionStore {
  sessions: Session[]
  setSessions: (sessions: Session[]) => void
}

export const useSessionStore = create<SessionStore>((set) => ({
  sessions: [],
  setSessions: (sessions) =>
    set({
      sessions: [...sessions].sort(
        (a, b) => new Date(b.UpdatedAt).getTime() - new Date(a.UpdatedAt).getTime()
      ),
    }),
}))

// Initialize session sync with Wails events.
// Called once at app startup.
export function initSessionSync() {
  // Dynamic import to avoid build errors when @wailsio/runtime is not available
  import('@wailsio/runtime').then(({ Events }) => {
    Events.On('sessions-updated', (event: { data: Session[] }) => {
      useSessionStore.getState().setSessions(event.data ?? [])
    })
  }).catch(() => {
    console.warn('[pager] Wails runtime not available, running in dev mode')
  })

  // Initial load via binding
  import('../bindings/main/App').then(({ ListSessions }) => {
    ListSessions().then((sessions: Session[]) => {
      useSessionStore.getState().setSessions(sessions ?? [])
    })
  }).catch(() => {
    console.warn('[pager] Bindings not available')
  })
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/store/sessions.ts
git commit -m "feat: add zustand session store with Wails event sync"
```

---

## Task 17: Frontend Components

**Files:**
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/components/SessionList.tsx`
- Create: `frontend/src/components/SessionCard.tsx`
- Create: `frontend/src/components/StatusIcon.tsx`

- [ ] **Step 1: Write App.tsx**

```tsx
import SessionList from './components/SessionList'

function App() {
  return (
    <div className="w-full h-screen bg-gray-900/95 backdrop-blur-xl text-white overflow-y-auto rounded-xl border border-gray-700/50">
      <header className="sticky top-0 z-10 bg-gray-900/90 backdrop-blur px-4 py-3 border-b border-gray-700/50">
        <h1 className="text-sm font-semibold text-gray-300">Pager</h1>
      </header>
      <SessionList />
    </div>
  )
}

export default App
```

- [ ] **Step 2: Write SessionList.tsx**

```tsx
import { useSessionStore } from '../store/sessions'
import SessionCard from './SessionCard'

export default function SessionList() {
  const sessions = useSessionStore((s) => s.sessions)

  if (sessions.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500 text-sm">
        暂无活跃会话
      </div>
    )
  }

  return (
    <div className="p-2 space-y-2">
      {sessions.map((session) => (
        <SessionCard key={session.Key} session={session} />
      ))}
    </div>
  )
}
```

- [ ] **Step 3: Write SessionCard.tsx**

```tsx
import { useState } from 'react'
import type { Session } from '../store/sessions'
import StatusIcon from './StatusIcon'

interface Props {
  session: Session
}

export default function SessionCard({ session }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [jumping, setJumping] = useState(false)

  const content = session.LastEvent?.Content ?? session.Status
  const contentRaw = session.LastEvent?.ContentRaw ?? ''
  const cwdLast = session.CWD.split('/').filter(Boolean).pop() ?? session.CWD

  const handleJump = async () => {
    setJumping(true)
    try {
      const { JumpToTerminal } = await import('../bindings/main/SessionService')
      await JumpToTerminal(session.Key)
    } catch {
      console.error('Jump failed')
    } finally {
      setJumping(false)
    }
  }

  const relativeTime = formatRelativeTime(session.UpdatedAt)

  return (
    <div className="bg-gray-800/80 rounded-lg p-3 border border-gray-700/50 hover:border-gray-600/50 transition-colors">
      {/* Header row */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2 min-w-0">
          <StatusIcon agent={session.Agent} />
          <span className="text-xs text-gray-400 truncate">
            {agentLabel(session.Agent)} · {cwdLast}
          </span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <StatusBadge status={session.Status} updatedAt={session.UpdatedAt} />
          <span className="text-xs text-gray-500">{relativeTime}</span>
        </div>
      </div>

      {/* Content row */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => setExpanded(!expanded)}
          className="text-sm text-gray-200 truncate text-left flex-1 hover:text-white transition-colors"
        >
          {content}
        </button>
        <button
          onClick={handleJump}
          disabled={jumping}
          className="ml-2 px-2 py-1 text-xs bg-blue-600/80 hover:bg-blue-500 rounded text-white disabled:opacity-50 shrink-0 transition-colors"
        >
          {jumping ? '...' : '跳转'}
        </button>
      </div>

      {/* Expanded raw content */}
      {expanded && contentRaw && (
        <pre className="mt-2 p-2 bg-gray-900/80 rounded text-xs text-gray-300 font-mono whitespace-pre-wrap break-all max-h-40 overflow-y-auto">
          {contentRaw}
        </pre>
      )}
    </div>
  )
}

function StatusBadge({ status, updatedAt }: { status: string; updatedAt: string }) {
  const colors: Record<string, string> = {
    waiting: 'bg-red-500/80 text-white',
    active: 'bg-blue-500/80 text-white',
    finished: 'bg-gray-600/80 text-gray-300',
    error: 'bg-orange-500/80 text-white',
  }
  const labels: Record<string, string> = {
    waiting: '等待中',
    active: '执行中',
    finished: '已完成',
    error: '错误',
  }

  return (
    <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${colors[status] ?? colors.finished}`}>
      {labels[status] ?? status}
    </span>
  )
}

function agentLabel(agent: string): string {
  switch (agent) {
    case 'claude-code': return 'Claude Code'
    case 'codex': return 'Codex'
    default: return agent
  }
}

function formatRelativeTime(iso: string): string {
  const now = Date.now()
  const then = new Date(iso).getTime()
  const diffSec = Math.floor((now - then) / 1000)

  if (diffSec < 10) return '刚刚'
  if (diffSec < 60) return `${diffSec}秒前`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}分钟前`
  // Show time for older events
  const d = new Date(iso)
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}
```

- [ ] **Step 4: Write StatusIcon.tsx**

```tsx
interface Props {
  agent: string
}

export default function StatusIcon({ agent }: Props) {
  const colors: Record<string, string> = {
    'claude-code': 'bg-[#5B9BD5]',
    'codex': 'bg-[#4CAF50]',
  }
  const color = colors[agent] ?? 'bg-gray-400'

  return (
    <span className={`inline-block w-2 h-2 rounded-full ${color}`} />
  )
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/
git commit -m "feat: add session list UI components"
```

---

## Task 18: Frontend Build Verification

- [ ] **Step 1: Install dependencies**

```bash
cd frontend && npm install
```

- [ ] **Step 2: Create stub bindings for build**

Create `frontend/src/bindings/main/App.ts`:

```typescript
export async function ListSessions(): Promise<any[]> {
  return []
}
```

Create `frontend/src/bindings/main/SessionService.ts`:

```typescript
export async function JumpToTerminal(_sessionKey: string): Promise<void> {}
export async function DismissSession(_sessionKey: string): Promise<void> {}
```

Note: These stubs will be replaced by Wails auto-generated bindings after `wails3 generate bindings`.

- [ ] **Step 3: Build frontend**

```bash
cd frontend && npm run build
```

Expected: build succeeds, `frontend/dist/` created

- [ ] **Step 4: Commit**

```bash
git add frontend/src/bindings/
git commit -m "feat: add stub bindings for standalone frontend build"
```

---

## Task 19: Scripts

**Files:**
- Create: `scripts/install-hooks.sh`
- Create: `scripts/install-launchd.sh`

- [ ] **Step 1: Write install-hooks.sh**

```bash
#!/bin/bash
# Install pager-cc-bridge hook into ~/.claude/settings.json
# Usage: ./install-hooks.sh /path/to/pager-cc-bridge

set -e

BRIDGE_PATH="${1:?Usage: $0 /path/to/pager-cc-bridge}"

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
        "command": "$BRIDGE_PATH pre_tool_use",
        "async": true,
        "timeout": 5
      }]
    }],
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH post_tool_use",
        "async": true,
        "timeout": 5
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH stop",
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

echo "Hook installed to $SETTINGS"
echo "Restart Claude Code to activate."
```

- [ ] **Step 2: Write install-launchd.sh**

```bash
#!/bin/bash
# Register Pager as a launchd LaunchAgent for auto-start and crash recovery.

APP_PATH="${1:?Usage: $0 /Applications/Pager.app}"
PLIST="$HOME/Library/LaunchAgents/com.sapaude.pager.plist"

mkdir -p "$HOME/.pager"

cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.sapaude.pager</string>
    <key>ProgramArguments</key>
    <array>
        <string>$APP_PATH/Contents/MacOS/Pager</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.pager/pager.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.pager/pager-error.log</string>
</dict>
</plist>
EOF

launchctl load "$PLIST"
echo "Pager LaunchAgent registered."
```

- [ ] **Step 3: Make scripts executable and commit**

```bash
chmod +x scripts/install-hooks.sh scripts/install-launchd.sh
git add scripts/
git commit -m "feat: add hook and launchd installation scripts"
```

---

## Task 20: Wails Configuration

**Files:**
- Create: `wails.json`

- [ ] **Step 1: Write wails.json**

```json
{
  "$schema": "https://wails.io/schemas/config.v3.json",
  "name": "Pager",
  "outputfilename": "Pager",
  "frontend:install": "cd frontend && npm install",
  "frontend:build": "cd frontend && npm run build",
  "frontend:dev:watcher": "cd frontend && npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "sapaude"
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add wails.json
git commit -m "feat: add Wails v3 configuration"
```

---

## Task 21: Final Integration Check

- [ ] **Step 1: Full Go build**

```bash
go mod tidy
go build ./...
```

- [ ] **Step 2: Run all Go tests**

```bash
go test ./... -v -count=1
```

Expected: all PASS

- [ ] **Step 3: Frontend build**

```bash
cd frontend && npm install && npm run build
```

Expected: `frontend/dist/` populated

- [ ] **Step 4: Build bridge binary**

```bash
go build -o bin/pager-cc-bridge ./cmd/bridge/
```

Expected: binary at `bin/pager-cc-bridge`

- [ ] **Step 5: Final commit with .gitignore**

Create `.gitignore`:

```
bin/
frontend/node_modules/
frontend/dist/
*.exe
*.app
.DS_Store
```

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```
