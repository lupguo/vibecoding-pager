# SQLite Event Store Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SQLite persistence layer for hook events with buffered async writes, full 8 hook type support, startup replay, and user-facing data management.

**Architecture:** Event Sourcing — SessionTracker (in-memory state machine for real-time UI) + EventStore (SQLite for persistence/analytics). Dual-write on every incoming event. On startup, replay recent events from SQLite to rebuild tracker state.

**Tech Stack:** Go 1.25, jmoiron/sqlx, modernc.org/sqlite, Wails v3, React/TypeScript/Zustand

---

## Task 1: Rename Registry → SessionTracker

Rename the existing Registry to Tracker with updated method names. No logic changes, just naming.

**Files:**
- Rename: `internal/domain/session/registry.go` → `internal/domain/session/tracker.go`
- Rename: `internal/domain/session/registry_test.go` → `internal/domain/session/tracker_test.go`
- Modify: `internal/adapter/httpapi/server.go`
- Modify: `internal/adapter/httpapi/server_test.go`
- Modify: `internal/wails/app.go`
- Modify: `internal/wails/session_svc.go`

- [ ] **Step 1: Rename files**

```bash
cd /private/data/projects/github.com/sapaude/pager
mv internal/domain/session/registry.go internal/domain/session/tracker.go
mv internal/domain/session/registry_test.go internal/domain/session/tracker_test.go
```

- [ ] **Step 2: Refactor tracker.go — rename struct and methods**

Replace the full content of `internal/domain/session/tracker.go`:

```go
package session

import (
	"sort"
	"sync"
	"time"

	"pager/internal/domain/entity"
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
	AttentionLevel string                       `json:"AttentionLevel"`
	AgentLabel     string                       `json:"AgentLabel"`
	SessionID      string                       `json:"SessionID"`
	LastEvent      *entity.AgentEvent            `json:"LastEvent"`
	PendingTools   map[string]*entity.AgentEvent `json:"PendingTools"`
	UpdatedAt      time.Time                    `json:"UpdatedAt"`
}

// Tracker maintains real-time session state from an event stream.
type Tracker struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	onUpdate func(sessions []*Session)
}

// NewTracker creates a Tracker with an onUpdate callback.
func NewTracker(onUpdate func([]*Session)) *Tracker {
	return &Tracker{
		sessions: make(map[string]*Session),
		onUpdate: onUpdate,
	}
}

// TrackEvent processes an AgentEvent and updates session state.
func (t *Tracker) TrackEvent(e *entity.AgentEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := e.SessionKey()
	s, exists := t.sessions[key]
	if !exists {
		s = &Session{
			Key:          key,
			Agent:        e.Agent,
			Host:         e.Host,
			CWD:          e.CWD,
			TTY:          e.TTY,
			SessionID:    e.SessionID,
			PendingTools: make(map[string]*entity.AgentEvent),
		}
		t.sessions[key] = s
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

	// Update attention level and agent label from event
	if e.AttentionLevel != "" {
		s.AttentionLevel = e.AttentionLevel
	}
	if e.AgentLabel != "" {
		s.AgentLabel = e.AgentLabel
	}

	switch e.EventType {
	case entity.EventPreToolUse:
		s.Status = entity.StatusWaiting
		if e.ToolUseID != "" {
			s.PendingTools[e.ToolUseID] = e
		}
	case entity.EventPostToolUse:
		if e.ToolUseID != "" {
			delete(s.PendingTools, e.ToolUseID)
		}
		if len(s.PendingTools) == 0 {
			s.Status = entity.StatusActive
		}
	case entity.EventStop:
		if hasAskUserPending(s.PendingTools) {
			s.Status = entity.StatusWaiting
		} else {
			s.Status = entity.StatusFinished
			s.PendingTools = make(map[string]*entity.AgentEvent)
		}
	case entity.EventError:
		s.Status = entity.StatusError
	case entity.EventSessionStart:
		s.Status = entity.StatusActive
	}

	if t.onUpdate != nil {
		t.onUpdate(t.snapshotLocked())
	}
}

// Replay rebuilds tracker state from a slice of historical events.
// Used on app startup to restore state from SQLite.
func (t *Tracker) Replay(events []*entity.AgentEvent) {
	for _, e := range events {
		t.TrackEvent(e)
	}
}

// ListByRecent returns sessions ordered by UpdatedAt descending (newest first).
func (t *Tracker) ListByRecent() []*Session {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshotLocked()
}

func (t *Tracker) snapshotLocked() []*Session {
	result := make([]*Session, 0, len(t.sessions))
	for _, s := range t.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// Session finds a session by its key.
func (t *Tracker) Session(key string) (*Session, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.sessions[key]
	return s, ok
}

// SessionByTTY finds a session by TTY.
func (t *Tracker) SessionByTTY(tty string) (*Session, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, s := range t.sessions {
		if s.TTY == tty {
			return s, true
		}
	}
	return nil, false
}

// Dismiss removes a session from the tracker.
func (t *Tracker) Dismiss(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sessions, key)
	if t.onUpdate != nil {
		t.onUpdate(t.snapshotLocked())
	}
}

// hasAskUserPending checks if any pending tool is AskUserQuestion.
func hasAskUserPending(pending map[string]*entity.AgentEvent) bool {
	for _, e := range pending {
		if e != nil && e.ToolName == "AskUserQuestion" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Add EventSessionStart constant to entity/event.go**

Add to the constants in `internal/domain/entity/event.go`:

```go
const (
	EventPreToolUse       = "pre_tool_use"
	EventPostToolUse      = "post_tool_use"
	EventStop             = "stop"
	EventError            = "error"
	EventNotification     = "notification"
	EventSessionStart     = "session_start"
	EventUserPromptSubmit = "user_prompt_submit"
	EventSubagentStop     = "subagent_stop"
	EventPreCompact       = "pre_compact"
)
```

- [ ] **Step 4: Update tracker_test.go — rename constructor calls**

Replace all `New(` with `NewTracker(` and `reg.Apply(` with `reg.TrackEvent(` and `reg.ListSorted()` with `reg.ListByRecent()` and `reg.GetByTTY(` with `reg.SessionByTTY(`:

```go
package session

import (
	"testing"
	"time"

	"pager/internal/domain/entity"
)

func makeEvent(eventType, toolName, toolUseID, cwd, tty string) *entity.AgentEvent {
	return &entity.AgentEvent{
		Agent:     entity.AgentClaudeCode,
		Host:      "local",
		SessionID: "test-session-" + cwd,
		CWD:       cwd,
		TTY:       tty,
		EventType: eventType,
		ToolName:  toolName,
		ToolUseID: toolUseID,
		Content:   toolName,
		Timestamp: time.Now(),
	}
}

func TestTrackEvent_PreToolUse_CreatesSession(t *testing.T) {
	var called int
	tr := NewTracker(func(sessions []*Session) { called++ })

	e := makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/project", "/dev/ttys001")
	tr.TrackEvent(e)

	sessions := tr.ListByRecent()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.Status != entity.StatusWaiting {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusWaiting)
	}
	if s.Key != "test-session-/project" {
		t.Errorf("key = %q, want %q", s.Key, "test-session-/project")
	}
	if _, ok := s.PendingTools["tu-1"]; !ok {
		t.Error("tu-1 should be in PendingTools")
	}
	if called != 1 {
		t.Errorf("onUpdate called %d times, want 1", called)
	}
}

func TestTrackEvent_PostToolUse_ClearsPending(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent(entity.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))

	if len(lastSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(lastSessions))
	}
	s := lastSessions[0]
	if s.Status != entity.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusActive)
	}
	if len(s.PendingTools) != 0 {
		t.Errorf("PendingTools should be empty, has %d", len(s.PendingTools))
	}
}

func TestTrackEvent_PostToolUse_MultiplePending(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Edit", "tu-2", "/p", "/dev/ttys001"))

	tr.TrackEvent(makeEvent(entity.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	s := lastSessions[0]
	if s.Status != entity.StatusWaiting {
		t.Errorf("status = %q, want %q (still has pending)", s.Status, entity.StatusWaiting)
	}

	tr.TrackEvent(makeEvent(entity.EventPostToolUse, "Edit", "tu-2", "/p", "/dev/ttys001"))
	s = lastSessions[0]
	if s.Status != entity.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusActive)
	}
}

func TestTrackEvent_Stop(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent(entity.EventStop, "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusFinished {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusFinished)
	}
}

func TestTrackEvent_Error(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent(entity.EventError, "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusError {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusError)
	}
}

func TestTrackEvent_SessionStart(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	e := makeEvent(entity.EventSessionStart, "", "", "/p", "/dev/ttys001")
	tr.TrackEvent(e)

	s := lastSessions[0]
	if s.Status != entity.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusActive)
	}
}

func TestListByRecent_OrderByUpdatedAt(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	e1 := makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/project-a", "/dev/ttys001")
	e1.Timestamp = time.Now().Add(-10 * time.Second)
	tr.TrackEvent(e1)

	e2 := makeEvent(entity.EventPreToolUse, "Bash", "tu-2", "/project-b", "/dev/ttys002")
	e2.Timestamp = time.Now()
	tr.TrackEvent(e2)

	sessions := tr.ListByRecent()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].CWD != "/project-b" {
		t.Errorf("first session CWD = %q, want /project-b", sessions[0].CWD)
	}
}

func TestSessionByTTY(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys005"))

	s, ok := tr.SessionByTTY("/dev/ttys005")
	if !ok {
		t.Fatal("expected to find session by TTY")
	}
	if s.TTY != "/dev/ttys005" {
		t.Errorf("TTY = %q, want /dev/ttys005", s.TTY)
	}

	_, ok = tr.SessionByTTY("/dev/ttys999")
	if ok {
		t.Error("should not find non-existent TTY")
	}
}

func TestSession_ByKey(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	e1 := makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/project", "/dev/ttys001")
	e1.SessionID = "session-aaa"
	tr.TrackEvent(e1)

	_, ok := tr.Session("session-aaa")
	if !ok {
		t.Fatal("expected to find session by key")
	}

	_, ok = tr.Session("nonexistent")
	if ok {
		t.Error("should not find non-existent key")
	}
}

func TestDismiss(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	if len(lastSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(lastSessions))
	}

	tr.Dismiss("test-session-/p")
	if len(lastSessions) != 0 {
		t.Fatalf("expected 0 sessions after dismiss, got %d", len(lastSessions))
	}
}

func TestReplay(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	events := []*entity.AgentEvent{
		makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"),
		makeEvent(entity.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"),
	}
	tr.Replay(events)

	sessions := tr.ListByRecent()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after replay, got %d", len(sessions))
	}
	if sessions[0].Status != entity.StatusActive {
		t.Errorf("status = %q, want %q", sessions[0].Status, entity.StatusActive)
	}
}
```

- [ ] **Step 5: Update httpapi/server.go — use Tracker**

Replace `*session.Registry` with `*session.Tracker` and update method calls:

```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

const ListenAddr = "127.0.0.1:7421"

// Server handles HTTP requests from bridge processes.
type Server struct {
	tracker *session.Tracker
}

// New creates a Server with the given tracker.
func New(tracker *session.Tracker) *Server {
	return &Server{tracker: tracker}
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/event", s.HandleEvent)
	mux.HandleFunc("/sessions", s.HandleSessions)
	mux.HandleFunc("/debug-log", s.HandleDebugLog)
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
	var e entity.AgentEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.tracker.TrackEvent(&e)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// HandleSessions returns all sessions as JSON (debug endpoint).
func (s *Server) HandleSessions(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.tracker.ListByRecent())
}

// HandleDebugLog receives debug messages from the frontend and logs them.
func (s *Server) HandleDebugLog(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(req.Body)
	log.Printf("[frontend-debug] %s", string(body))
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 6: Update httpapi/server_test.go — use NewTracker**

Replace `session.New(` with `session.NewTracker(` and `reg.ListSorted()` with `tracker.ListByRecent()`:

```go
package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

func TestHandleEvent_ValidPost(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	e := entity.AgentEvent{
		Agent:     entity.AgentClaudeCode,
		Host:      "local",
		CWD:       "/project",
		TTY:       "/dev/ttys001",
		EventType: entity.EventPreToolUse,
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

	sessions := tracker.ListByRecent()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
}

func TestHandleEvent_RejectsGet(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	req := httptest.NewRequest(http.MethodGet, "/event", nil)
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleEvent_BadJSON(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSessions(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

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

- [ ] **Step 7: Update wails/app.go — use Tracker**

Replace `session.Registry` with `session.Tracker` and `session.New(` with `session.NewTracker(` and `p.reg` with `p.tracker`:

In `internal/wails/app.go`, change the struct field and all references:
- `reg  *session.Registry` → `tracker *session.Tracker`
- `p.reg = session.New(func(sessions []*session.Session) {` → `p.tracker = session.NewTracker(func(sessions []*session.Session) {`
- `p.srv = httpapi.New(p.reg)` → `p.srv = httpapi.New(p.tracker)`
- `sessionBinding := &SessionBinding{reg: p.reg}` → `sessionBinding := &SessionBinding{tracker: p.tracker}`

- [ ] **Step 8: Update wails/session_svc.go — use Tracker**

```go
package wails

import (
	"fmt"

	"pager/internal/adapter/terminal"
	"pager/internal/domain/session"
)

// SessionBinding exposes session state and actions to the React frontend via Wails bindings.
type SessionBinding struct {
	tracker *session.Tracker
}

// ListSessions returns all sessions sorted by UpdatedAt descending.
func (s *SessionBinding) ListSessions() []*session.Session {
	if s.tracker == nil {
		return nil
	}
	return s.tracker.ListByRecent()
}

// JumpToTerminal brings the terminal window/tab to the foreground.
func (s *SessionBinding) JumpToTerminal(sessionKey string) error {
	if s.tracker == nil {
		return fmt.Errorf("tracker not initialised")
	}

	sess, ok := s.tracker.Session(sessionKey)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionKey)
	}

	req := terminal.JumpRequest{
		TTY:            sess.TTY,
		TermProgram:    sess.TermProgram,
		ITermSessionID: sess.ITermSessionID,
	}

	return terminal.Jump(req)
}

// DismissSession removes a session from the tracker.
func (s *SessionBinding) DismissSession(sessionKey string) {
	if s.tracker == nil {
		return
	}
	s.tracker.Dismiss(sessionKey)
}
```

- [ ] **Step 9: Run tests to verify refactoring**

Run: `go test ./internal/...`
Expected: All tests PASS

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "refactor: rename Registry → SessionTracker with descriptive method names

- Registry → Tracker, Apply → TrackEvent, ListSorted → ListByRecent
- GetByKey → Session, GetByTTY → SessionByTTY, Remove → Dismiss
- Add Replay method for startup state recovery
- Add EventSessionStart and other new event type constants
- Update all consumers: httpapi, wails bindings, tests"
```

---

## Task 2: Add sqlx + modernc dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add dependencies**

```bash
cd /private/data/projects/github.com/sapaude/pager
go get github.com/jmoiron/sqlx
go get modernc.org/sqlite
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add jmoiron/sqlx and modernc.org/sqlite"
```

---

## Task 3: EventStore Interface + Schema

Define the interface contract and DDL before implementing.

**Files:**
- Rewrite: `internal/infra/store/store.go`
- Create: `internal/infra/store/schema.sql`

- [ ] **Step 1: Rewrite store.go with EventStore interface**

```go
package store

import "pager/internal/domain/entity"

// EventStore defines the contract for persistent event storage.
type EventStore interface {
	// Record asynchronously writes an event to storage.
	// Non-blocking: uses an internal buffer channel.
	// Drops the event with a warning log if the buffer is full.
	Record(e *entity.AgentEvent)

	// LoadRecentSessions loads events for sessions updated within the last N hours
	// (excluding soft-deleted sessions). Returns events in chronological order
	// for replay into SessionTracker.
	LoadRecentSessions(hours int) ([]*entity.AgentEvent, error)

	// SessionEvents returns all events for a given session key, ordered by timestamp ASC.
	SessionEvents(sessionKey string) ([]*entity.AgentEvent, error)

	// DismissSession soft-deletes a session (sets deleted_at).
	DismissSession(sessionKey string) error

	// PurgeOlderThan physically deletes events and sessions older than N days.
	// Returns total number of rows deleted across both tables.
	PurgeOlderThan(days int) (affected int64, err error)

	// PurgeAll physically deletes all events and sessions.
	PurgeAll() (affected int64, err error)

	// Stats returns database file size in bytes and total event count.
	Stats() (dbSizeBytes int64, eventCount int64, err error)

	// Close flushes remaining buffered events and closes the database connection.
	Close() error
}
```

- [ ] **Step 2: Create schema.sql**

```sql
-- Pager SQLite Schema
-- All tables use t_ prefix and id auto-increment primary key.

CREATE TABLE IF NOT EXISTS t_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    session_key      TEXT    NOT NULL UNIQUE,            -- 会话唯一标识 (session_id 或 host:cwd:tty)
    session_id       TEXT    NOT NULL DEFAULT '',        -- CC 原生 session_id
    agent            TEXT    NOT NULL DEFAULT 'claude-code', -- 代理类型: claude-code / codex
    host             TEXT    NOT NULL DEFAULT '',        -- 主机名
    cwd              TEXT    NOT NULL DEFAULT '',        -- 工作目录
    project_name     TEXT    NOT NULL DEFAULT '',        -- 项目名 (CWD 末段，用于分组排序)
    tty              TEXT    NOT NULL DEFAULT '',        -- 终端 TTY 路径
    term_program     TEXT    NOT NULL DEFAULT '',        -- 终端程序: iTerm2 / Terminal.app
    iterm_session_id TEXT    NOT NULL DEFAULT '',        -- iTerm2 会话ID (跳转用)
    status           TEXT    NOT NULL DEFAULT 'active',  -- 当前状态: waiting/active/finished/error
    attention_level  TEXT    NOT NULL DEFAULT 'running', -- 注意力级别: attention/running/done
    agent_label      TEXT    NOT NULL DEFAULT 'CC',      -- 代理标签
    created_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 首次事件时间
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 最后事件时间
    deleted_at       TEXT             DEFAULT NULL       -- 软删除标记 (NULL=未删除)
);

CREATE INDEX IF NOT EXISTS idx_sessions_project ON t_sessions(project_name, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON t_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted ON t_sessions(deleted_at);

CREATE TABLE IF NOT EXISTS t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    session_key     TEXT    NOT NULL,                   -- 所属会话标识 (FK → t_sessions.session_key)
    event_type      TEXT    NOT NULL,                   -- 事件类型: pre_tool_use/post_tool_use/stop/error/notification/session_start/user_prompt_submit/subagent_stop/pre_compact
    tool_name       TEXT    NOT NULL DEFAULT '',        -- 工具名称: Bash/Edit/Read/Write/...
    tool_use_id     TEXT    NOT NULL DEFAULT '',        -- 工具调用唯一ID
    content         TEXT    NOT NULL DEFAULT '',        -- 截断显示内容 (≤60 rune)
    content_raw     TEXT    NOT NULL DEFAULT '',        -- 完整内容摘要
    attention_level TEXT    NOT NULL DEFAULT '',        -- 事件级别
    permission_mode TEXT    NOT NULL DEFAULT '',        -- 权限模式
    raw_payload     BLOB             DEFAULT NULL,     -- 完整 stdin JSON (原始字节，全量存储)
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 事件发生时间

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);

CREATE INDEX IF NOT EXISTS idx_events_session_ts ON t_events(session_key, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_type       ON t_events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp  ON t_events(timestamp DESC);
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/infra/store/...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/infra/store/store.go internal/infra/store/schema.sql
git commit -m "feat(store): define EventStore interface and SQLite schema DDL"
```

---

## Task 4: SQLite Implementation — Core

Implement the SQLite store with buffered writes.

**Files:**
- Create: `internal/infra/store/sqlite.go`
- Create: `internal/infra/store/sqlite_test.go`

- [ ] **Step 1: Write failing test — Record and LoadRecentSessions**

Create `internal/infra/store/sqlite_test.go`:

```go
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pager/internal/domain/entity"
)

func tempDBPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.db")
}

func makeTestEvent(sessionKey, eventType, toolName string) *entity.AgentEvent {
	return &entity.AgentEvent{
		Agent:      entity.AgentClaudeCode,
		Host:       "local",
		SessionID:  sessionKey,
		CWD:        "/projects/myapp",
		TTY:        "/dev/ttys001",
		EventType:  eventType,
		ToolName:   toolName,
		ToolUseID:  "tu-" + toolName,
		Content:    toolName + " summary",
		ContentRaw: toolName + " full content",
		RawPayload: json.RawMessage(`{"tool_name":"` + toolName + `"}`),
		Timestamp:  time.Now(),
	}
}

func TestSQLiteStore_RecordAndLoad(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	// Record events
	e1 := makeTestEvent("sess-1", entity.EventPreToolUse, "Bash")
	e2 := makeTestEvent("sess-1", entity.EventPostToolUse, "Bash")
	s.Record(e1)
	s.Record(e2)

	// Flush by closing and reopening
	s.Close()

	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	events, err := s2.LoadRecentSessions(24)
	if err != nil {
		t.Fatalf("LoadRecentSessions: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].EventType != entity.EventPreToolUse {
		t.Errorf("event[0].EventType = %q, want pre_tool_use", events[0].EventType)
	}
	if events[0].SessionID != "sess-1" {
		t.Errorf("event[0].SessionID = %q, want sess-1", events[0].SessionID)
	}
}

func TestSQLiteStore_DismissSession(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	e := makeTestEvent("sess-dismiss", entity.EventPreToolUse, "Bash")
	s.Record(e)
	s.Close()

	s2, _ := NewSQLiteStore(dbPath)
	defer s2.Close()

	if err := s2.DismissSession("sess-dismiss"); err != nil {
		t.Fatalf("DismissSession: %v", err)
	}

	events, _ := s2.LoadRecentSessions(24)
	if len(events) != 0 {
		t.Errorf("expected 0 events after dismiss, got %d", len(events))
	}
}

func TestSQLiteStore_PurgeOlderThan(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	// Create event with old timestamp
	e := makeTestEvent("sess-old", entity.EventPreToolUse, "Bash")
	e.Timestamp = time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	s.Record(e)
	s.Close()

	s2, _ := NewSQLiteStore(dbPath)
	defer s2.Close()

	affected, err := s2.PurgeOlderThan(7)
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if affected == 0 {
		t.Error("expected rows to be purged")
	}
}

func TestSQLiteStore_PurgeAll(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	s.Record(makeTestEvent("sess-a", entity.EventPreToolUse, "Bash"))
	s.Record(makeTestEvent("sess-b", entity.EventStop, ""))
	s.Close()

	s2, _ := NewSQLiteStore(dbPath)
	defer s2.Close()

	affected, err := s2.PurgeAll()
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}
	if affected == 0 {
		t.Error("expected rows to be purged")
	}

	events, _ := s2.LoadRecentSessions(9999)
	if len(events) != 0 {
		t.Errorf("expected 0 events after purge all, got %d", len(events))
	}
}

func TestSQLiteStore_Stats(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	s.Record(makeTestEvent("sess-stats", entity.EventPreToolUse, "Bash"))
	s.Close()

	s2, _ := NewSQLiteStore(dbPath)
	defer s2.Close()

	size, count, err := s2.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if size == 0 {
		t.Error("expected non-zero db size")
	}
	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestSQLiteStore_BufferFullDropsEvent(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	// Fill buffer (cap=512) — should not panic or block
	for i := 0; i < 600; i++ {
		s.Record(makeTestEvent("sess-flood", entity.EventPreToolUse, "Bash"))
	}
	// If we get here without blocking, the drop logic works
}

func TestSQLiteStore_DBFileCreated(t *testing.T) {
	dbPath := tempDBPath(t)
	_, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected DB file to be created")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `go test ./internal/infra/store/ -v`
Expected: FAIL — `NewSQLiteStore` undefined

- [ ] **Step 3: Implement sqlite.go**

Create `internal/infra/store/sqlite.go`:

```go
package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"pager/internal/domain/entity"
)

//go:embed schema.sql
var schemaDDL string

const (
	bufferCap  = 512
	batchSize  = 50
	flushInterval = 2 * time.Second
)

// sqliteStore implements EventStore using SQLite via sqlx.
type sqliteStore struct {
	db     *sqlx.DB
	buf    chan *entity.AgentEvent
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed chan struct{}
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath and initializes the schema.
func NewSQLiteStore(dbPath string) (*sqliteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL", dbPath)
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Set connection pool (SQLite single-writer)
	db.SetMaxOpenConns(1)

	// Execute schema DDL
	if _, err := db.Exec(schemaDDL); err != nil {
		db.Close()
		return nil, fmt.Errorf("exec schema: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &sqliteStore{
		db:     db,
		buf:    make(chan *entity.AgentEvent, bufferCap),
		ctx:    ctx,
		cancel: cancel,
		closed: make(chan struct{}),
	}

	s.wg.Add(1)
	go s.flushLoop()

	return s, nil
}

// Record enqueues an event for async batch writing.
// Non-blocking: drops the event if buffer is full.
func (s *sqliteStore) Record(e *entity.AgentEvent) {
	select {
	case s.buf <- e:
	default:
		slog.Warn("event buffer full, dropping event",
			"session_key", e.SessionKey(),
			"event_type", e.EventType,
		)
	}
}

// LoadRecentSessions loads events for non-deleted sessions updated within N hours.
func (s *sqliteStore) LoadRecentSessions(hours int) ([]*entity.AgentEvent, error) {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05")

	query := `
		SELECT e.session_key, e.event_type, e.tool_name, e.tool_use_id,
		       e.content, e.content_raw, e.attention_level, e.permission_mode,
		       e.raw_payload, e.timestamp,
		       s.session_id, s.agent, s.host, s.cwd, s.tty,
		       s.term_program, s.iterm_session_id, s.agent_label
		FROM t_events e
		JOIN t_sessions s ON e.session_key = s.session_key
		WHERE s.deleted_at IS NULL
		  AND s.updated_at >= ?
		ORDER BY e.timestamp ASC
	`

	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("load recent sessions: %w", err)
	}
	defer rows.Close()

	var events []*entity.AgentEvent
	for rows.Next() {
		var (
			sessionKey, eventType, toolName, toolUseID      string
			content, contentRaw, attentionLevel, permMode   string
			rawPayload                                      []byte
			tsStr                                           string
			sessionID, agent, host, cwd, tty                string
			termProgram, itermSessionID, agentLabel         string
		)

		if err := rows.Scan(
			&sessionKey, &eventType, &toolName, &toolUseID,
			&content, &contentRaw, &attentionLevel, &permMode,
			&rawPayload, &tsStr,
			&sessionID, &agent, &host, &cwd, &tty,
			&termProgram, &itermSessionID, &agentLabel,
		); err != nil {
			return nil, fmt.Errorf("scan event row: %w", err)
		}

		ts, _ := time.ParseInLocation("2006-01-02 15:04:05", tsStr, time.Local)

		events = append(events, &entity.AgentEvent{
			Agent:          agent,
			Host:           host,
			CWD:            cwd,
			TTY:            tty,
			SessionID:      sessionID,
			TermProgram:    termProgram,
			ITermSessionID: itermSessionID,
			EventType:      eventType,
			ToolName:       toolName,
			ToolUseID:      toolUseID,
			Content:        content,
			ContentRaw:     contentRaw,
			AttentionLevel: attentionLevel,
			AgentLabel:     agentLabel,
			PermissionMode: permMode,
			RawPayload:     json.RawMessage(rawPayload),
			Timestamp:      ts,
		})
	}

	return events, rows.Err()
}

// SessionEvents returns all events for a session, ordered by timestamp.
func (s *sqliteStore) SessionEvents(sessionKey string) ([]*entity.AgentEvent, error) {
	query := `
		SELECT event_type, tool_name, tool_use_id, content, content_raw,
		       attention_level, permission_mode, raw_payload, timestamp
		FROM t_events
		WHERE session_key = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.Query(query, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("session events: %w", err)
	}
	defer rows.Close()

	var events []*entity.AgentEvent
	for rows.Next() {
		var (
			eventType, toolName, toolUseID                  string
			content, contentRaw, attentionLevel, permMode   string
			rawPayload                                      []byte
			tsStr                                           string
		)

		if err := rows.Scan(
			&eventType, &toolName, &toolUseID, &content, &contentRaw,
			&attentionLevel, &permMode, &rawPayload, &tsStr,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		ts, _ := time.ParseInLocation("2006-01-02 15:04:05", tsStr, time.Local)

		events = append(events, &entity.AgentEvent{
			SessionID:      sessionKey,
			EventType:      eventType,
			ToolName:       toolName,
			ToolUseID:      toolUseID,
			Content:        content,
			ContentRaw:     contentRaw,
			AttentionLevel: attentionLevel,
			PermissionMode: permMode,
			RawPayload:     json.RawMessage(rawPayload),
			Timestamp:      ts,
		})
	}

	return events, rows.Err()
}

// DismissSession soft-deletes a session.
func (s *sqliteStore) DismissSession(sessionKey string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(`UPDATE t_sessions SET deleted_at = ? WHERE session_key = ?`, now, sessionKey)
	return err
}

// PurgeOlderThan physically deletes data older than N days.
func (s *sqliteStore) PurgeOlderThan(days int) (int64, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02 15:04:05")

	var total int64

	res, err := s.db.Exec(`DELETE FROM t_events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	total += n

	res, err = s.db.Exec(`DELETE FROM t_sessions WHERE updated_at < ?`, cutoff)
	if err != nil {
		return total, err
	}
	n, _ = res.RowsAffected()
	total += n

	return total, nil
}

// PurgeAll physically deletes all data.
func (s *sqliteStore) PurgeAll() (int64, error) {
	var total int64

	res, err := s.db.Exec(`DELETE FROM t_events`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	total += n

	res, err = s.db.Exec(`DELETE FROM t_sessions`)
	if err != nil {
		return total, err
	}
	n, _ = res.RowsAffected()
	total += n

	return total, nil
}

// Stats returns DB file size and event count.
func (s *sqliteStore) Stats() (int64, int64, error) {
	dbPath := s.dbPath()
	info, err := os.Stat(dbPath)
	var size int64
	if err == nil {
		size = info.Size()
	}

	var count int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM t_events`).Scan(&count); err != nil {
		return size, 0, err
	}

	return size, count, nil
}

// Close flushes remaining events and closes the DB.
func (s *sqliteStore) Close() error {
	s.cancel()
	s.wg.Wait()
	return s.db.Close()
}

// flushLoop runs in a background goroutine, batching channel events into DB writes.
func (s *sqliteStore) flushLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]*entity.AgentEvent, 0, batchSize)

	for {
		select {
		case e, ok := <-s.buf:
			if !ok {
				// Channel closed during shutdown — flush remaining
				if len(batch) > 0 {
					s.writeBatch(batch)
				}
				return
			}
			batch = append(batch, e)
			if len(batch) >= batchSize {
				s.writeBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.writeBatch(batch)
				batch = batch[:0]
			}
		case <-s.ctx.Done():
			// Drain remaining events from buffer
			for {
				select {
				case e, ok := <-s.buf:
					if !ok {
						break
					}
					batch = append(batch, e)
				default:
					goto done
				}
			}
		done:
			if len(batch) > 0 {
				s.writeBatch(batch)
			}
			return
		}
	}
}

// writeBatch writes a slice of events to SQLite in a single transaction.
func (s *sqliteStore) writeBatch(events []*entity.AgentEvent) {
	tx, err := s.db.Begin()
	if err != nil {
		slog.Error("begin tx", "error", err)
		return
	}

	for _, e := range events {
		sessionKey := e.SessionKey()
		projectName := projectFromCWD(e.CWD)
		ts := e.Timestamp.Format("2006-01-02 15:04:05")

		// UPSERT session
		_, err := tx.Exec(`
			INSERT INTO t_sessions (session_key, session_id, agent, host, cwd, project_name, tty, term_program, iterm_session_id, status, attention_level, agent_label, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(session_key) DO UPDATE SET
				status = excluded.status,
				attention_level = excluded.attention_level,
				agent_label = CASE WHEN excluded.agent_label != '' THEN excluded.agent_label ELSE t_sessions.agent_label END,
				term_program = CASE WHEN excluded.term_program != '' THEN excluded.term_program ELSE t_sessions.term_program END,
				iterm_session_id = CASE WHEN excluded.iterm_session_id != '' THEN excluded.iterm_session_id ELSE t_sessions.iterm_session_id END,
				updated_at = excluded.updated_at
		`,
			sessionKey, e.SessionID, e.Agent, e.Host, e.CWD, projectName,
			e.TTY, e.TermProgram, e.ITermSessionID,
			statusFromEvent(e), e.AttentionLevel, e.AgentLabel, ts, ts,
		)
		if err != nil {
			slog.Error("upsert session", "error", err, "session_key", sessionKey)
			continue
		}

		// INSERT event
		_, err = tx.Exec(`
			INSERT INTO t_events (session_key, event_type, tool_name, tool_use_id, content, content_raw, attention_level, permission_mode, raw_payload, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			sessionKey, e.EventType, e.ToolName, e.ToolUseID,
			e.Content, e.ContentRaw, e.AttentionLevel, e.PermissionMode,
			[]byte(e.RawPayload), ts,
		)
		if err != nil {
			slog.Error("insert event", "error", err, "session_key", sessionKey)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit batch", "error", err)
	}
}

// statusFromEvent derives the session status from an event type.
func statusFromEvent(e *entity.AgentEvent) string {
	switch e.EventType {
	case entity.EventPreToolUse:
		return entity.StatusWaiting
	case entity.EventPostToolUse:
		return entity.StatusActive
	case entity.EventStop:
		return entity.StatusFinished
	case entity.EventError:
		return entity.StatusError
	case entity.EventSessionStart:
		return entity.StatusActive
	default:
		return entity.StatusActive
	}
}

// projectFromCWD extracts the last path segment as project name.
func projectFromCWD(cwd string) string {
	segments := strings.Split(cwd, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i] != "" {
			return segments[i]
		}
	}
	return cwd
}

// dbPath extracts the file path from the sqlx DB DSN.
func (s *sqliteStore) dbPath() string {
	var path string
	s.db.QueryRow("PRAGMA database_list").Scan(nil, nil, &path)
	return path
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/infra/store/ -v -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infra/store/sqlite.go internal/infra/store/sqlite_test.go
git commit -m "feat(store): implement SQLite EventStore with buffered batch writes

- WAL mode, single-writer goroutine
- Channel buffer cap=512, flush at 50 events or 2s
- Drop + warn on buffer overflow
- UPSERT sessions + INSERT events per batch
- Purge, dismiss, stats operations"
```

---

## Task 5: Bridge Expansion — New Hook Event Types

Add support for `session_start`, `user_prompt_submit`, `subagent_stop`, `pre_compact`.

**Files:**
- Modify: `internal/adapter/bridge/types.go`
- Modify: `internal/adapter/bridge/extractor.go`
- Modify: `internal/adapter/bridge/attention.go`
- Modify: `internal/adapter/bridge/extractor_test.go`
- Modify: `cmd/bridge/main.go`

- [ ] **Step 1: Expand CCHookInput in types.go**

Replace `internal/adapter/bridge/types.go`:

```go
package bridge

import "encoding/json"

// CCHookInput is the JSON structure CC passes via stdin to hooks.
type CCHookInput struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolUseID      string          `json:"tool_use_id"`
	PermissionMode string          `json:"permission_mode"`
	// SessionStart fields
	Source string `json:"source"` // startup | resume | clear
	// UserPromptSubmit fields
	Prompt string `json:"prompt"`
	// Notification fields
	Message string `json:"message"`
	// PreCompact fields
	Trigger string `json:"trigger"` // manual | auto
	// Stop/SubagentStop fields
	StopHookActive bool `json:"stop_hook_active"`
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

- [ ] **Step 2: Expand ExtractContent for new event types**

Add to `internal/adapter/bridge/extractor.go` a new function for event-level content extraction:

```go
// ExtractEventContent extracts content from event-level hooks (not tool-based).
// Returns (contentRaw, content).
func ExtractEventContent(eventType string, in *CCHookInput) (contentRaw, content string) {
	switch eventType {
	case "session_start":
		raw := "会话启动: " + in.Source
		return raw, truncateRunes(raw, contentMaxRunes)
	case "user_prompt_submit":
		raw := in.Prompt
		if raw == "" {
			raw = "用户输入"
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	case "subagent_stop":
		raw := "子Agent完成"
		return raw, raw
	case "pre_compact":
		raw := "对话压缩: " + in.Trigger
		return raw, truncateRunes(raw, contentMaxRunes)
	case "notification":
		raw := in.Message
		if raw == "" {
			raw = "通知"
		}
		return raw, truncateRunes(raw, contentMaxRunes)
	default:
		return eventType, eventType
	}
}
```

- [ ] **Step 3: Update DetermineAttentionLevel for new events**

In `internal/adapter/bridge/attention.go`, add cases for new event types:

```go
func DetermineAttentionLevel(eventType, permissionMode, toolName string) string {
	switch eventType {
	case entity.EventStop, entity.EventError, entity.EventSubagentStop:
		return entity.AttentionDone
	case entity.EventPostToolUse:
		return entity.AttentionRunning
	case entity.EventPreToolUse:
		if attentionTools[toolName] {
			return entity.AttentionAttention
		}
		if permissionMode == "bypassPermissions" {
			return entity.AttentionRunning
		}
		return entity.AttentionAttention
	case entity.EventSessionStart, entity.EventUserPromptSubmit, entity.EventPreCompact, entity.EventNotification:
		return entity.AttentionRunning
	default:
		return entity.AttentionRunning
	}
}
```

- [ ] **Step 4: Update cmd/bridge/main.go for new event types**

Replace `cmd/bridge/main.go`:

```go
package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"pager/internal/adapter/bridge"
	"pager/internal/domain/entity"
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

	// Extract content based on event type
	var contentRaw, content string
	if isToolEvent(eventType) {
		contentRaw, content = bridge.ExtractContent(in.ToolName, in.ToolInput)
	} else {
		contentRaw, content = bridge.ExtractEventContent(eventType, &in)
	}

	attentionLevel := bridge.DetermineAttentionLevel(eventType, in.PermissionMode, in.ToolName)

	e := entity.AgentEvent{
		Agent:          entity.AgentClaudeCode,
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

// isToolEvent returns true if the event type involves tool use.
func isToolEvent(eventType string) bool {
	return eventType == entity.EventPreToolUse || eventType == entity.EventPostToolUse
}

// parseArgs extracts event type and --agent flag from command-line args.
func parseArgs(args []string) (eventType, agentLabel string) {
	eventType = "unknown"
	agentLabel = "CC"

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--agent" && i+1 < len(args):
			agentLabel = args[i+1]
			i++
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

- [ ] **Step 5: Add tests for new event content extraction**

Add to `internal/adapter/bridge/extractor_test.go`:

```go
func TestExtractEventContent_SessionStart(t *testing.T) {
	in := &bridge.CCHookInput{Source: "startup"}
	raw, content := bridge.ExtractEventContent("session_start", in)
	if raw != "会话启动: startup" {
		t.Errorf("raw = %q", raw)
	}
	if content != "会话启动: startup" {
		t.Errorf("content = %q", content)
	}
}

func TestExtractEventContent_UserPromptSubmit(t *testing.T) {
	in := &bridge.CCHookInput{Prompt: "帮我修复这个 bug，详细错误信息是 panic: nil pointer dereference"}
	raw, content := bridge.ExtractEventContent("user_prompt_submit", in)
	if raw != in.Prompt {
		t.Errorf("raw = %q, want full prompt", raw)
	}
	if len([]rune(content)) > 61 { // 60 + "…"
		t.Errorf("content not truncated: %q", content)
	}
}

func TestExtractEventContent_SubagentStop(t *testing.T) {
	in := &bridge.CCHookInput{}
	raw, content := bridge.ExtractEventContent("subagent_stop", in)
	if raw != "子Agent完成" {
		t.Errorf("raw = %q", raw)
	}
	if content != "子Agent完成" {
		t.Errorf("content = %q", content)
	}
}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/adapter/bridge/ -v`
Expected: All PASS

Run: `go test ./... 2>&1 | tail -20`
Expected: All packages PASS

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/bridge/ cmd/bridge/main.go internal/domain/entity/event.go
git commit -m "feat(bridge): support all 8 CC hook event types

- Expand CCHookInput with session_start, user_prompt_submit, subagent_stop, pre_compact fields
- Add ExtractEventContent for non-tool event content extraction
- Update DetermineAttentionLevel for new event types
- Bridge main.go dispatches tool vs event-level content extraction"
```

---

## Task 6: Wire EventStore into HTTP Server

Dual-write: tracker (memory) + store (SQLite).

**Files:**
- Modify: `internal/adapter/httpapi/server.go`
- Modify: `internal/adapter/httpapi/server_test.go`

- [ ] **Step 1: Add EventStore to Server**

Update `internal/adapter/httpapi/server.go`:

```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/store"
)

const ListenAddr = "127.0.0.1:7421"

// Server handles HTTP requests from bridge processes.
type Server struct {
	tracker *session.Tracker
	store   store.EventStore
}

// New creates a Server with the given tracker and optional event store.
func New(tracker *session.Tracker, opts ...ServerOption) *Server {
	s := &Server{tracker: tracker}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServerOption configures the Server.
type ServerOption func(*Server)

// WithStore sets the EventStore for persistent event recording.
func WithStore(es store.EventStore) ServerOption {
	return func(s *Server) { s.store = es }
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/event", s.HandleEvent)
	mux.HandleFunc("/sessions", s.HandleSessions)
	mux.HandleFunc("/debug-log", s.HandleDebugLog)
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
	var e entity.AgentEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.tracker.TrackEvent(&e)

	if s.store != nil {
		s.store.Record(&e)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// HandleSessions returns all sessions as JSON (debug endpoint).
func (s *Server) HandleSessions(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.tracker.ListByRecent())
}

// HandleDebugLog receives debug messages from the frontend and logs them.
func (s *Server) HandleDebugLog(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(req.Body)
	log.Printf("[frontend-debug] %s", string(body))
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 2: Update server_test.go — tests still pass with no store**

Run: `go test ./internal/adapter/httpapi/ -v`
Expected: All PASS (store is nil, so Record is skipped)

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/httpapi/server.go
git commit -m "feat(httpapi): dual-write events to tracker + optional EventStore"
```

---

## Task 7: Config Extension + Settings Binding

Add `SessionLoadHours` config field and data management bindings.

**Files:**
- Modify: `internal/infra/config/config.go`
- Modify: `internal/wails/settings_svc.go`

- [ ] **Step 1: Add SessionLoadHours to Settings**

In `internal/infra/config/config.go`, add the field:

```go
type Settings struct {
	Language          string `json:"language"`
	Theme             string `json:"theme"`
	Opacity           int    `json:"opacity"`
	HotkeyToggle      string `json:"hotkey_toggle"`
	NotificationLevel string `json:"notification_level"`
	PopupWidth        int    `json:"popup_width"`
	PopupPinned       bool   `json:"popup_pinned"`
	SessionLoadHours  int    `json:"session_load_hours"`
}

func Defaults() Settings {
	return Settings{
		Language:          "zh",
		Theme:             "system",
		Opacity:           75,
		HotkeyToggle:     "Alt+E",
		NotificationLevel: "attention_only",
		PopupWidth:        380,
		PopupPinned:       false,
		SessionLoadHours:  24,
	}
}
```

- [ ] **Step 2: Add data management methods to SettingsBinding**

Update `internal/wails/settings_svc.go`:

```go
package wails

import (
	"pager/internal/infra/config"
	"pager/internal/infra/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsBinding exposes user settings to the React frontend via Wails bindings.
type SettingsBinding struct {
	path     string
	onChange func(config.Settings)
	store    store.EventStore
}

// NewSettingsBinding creates a SettingsBinding.
func NewSettingsBinding(onChange func(config.Settings), eventStore store.EventStore) *SettingsBinding {
	return &SettingsBinding{
		path:     config.DefaultPath(),
		onChange: onChange,
		store:    eventStore,
	}
}

// GetSettings returns the current settings.
func (s *SettingsBinding) GetSettings() config.Settings {
	cfg, _ := config.LoadFrom(s.path)
	return cfg
}

// UpdateSettings saves new settings, broadcasts change event, and triggers onChange.
func (s *SettingsBinding) UpdateSettings(cfg config.Settings) error {
	if err := config.SaveTo(s.path, cfg); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("settings-changed", cfg)
	}
	if s.onChange != nil {
		s.onChange(cfg)
	}
	return nil
}

// DataStats returns database statistics for display in settings UI.
type DataStats struct {
	DBSizeBytes int64 `json:"db_size_bytes"`
	EventCount  int64 `json:"event_count"`
}

// GetDataStats returns current database size and event count.
func (s *SettingsBinding) GetDataStats() DataStats {
	if s.store == nil {
		return DataStats{}
	}
	size, count, _ := s.store.Stats()
	return DataStats{DBSizeBytes: size, EventCount: count}
}

// PurgeData physically deletes events and sessions older than N days.
// Pass 0 to delete all data.
func (s *SettingsBinding) PurgeData(days int) (int64, error) {
	if s.store == nil {
		return 0, nil
	}
	if days == 0 {
		return s.store.PurgeAll()
	}
	return s.store.PurgeOlderThan(days)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infra/config/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infra/config/config.go internal/wails/settings_svc.go
git commit -m "feat(settings): add SessionLoadHours config and data management bindings

- SessionLoadHours defaults to 24 hours
- GetDataStats returns DB size and event count
- PurgeData(days) for manual cleanup (0 = all)"
```

---

## Task 8: App Startup — DB Init + Replay

Wire everything together in the app lifecycle.

**Files:**
- Modify: `internal/wails/app.go`

- [ ] **Step 1: Update app.go with SQLite initialization and replay**

Key changes to `internal/wails/app.go`:

1. Add imports for `store` package
2. Add `store store.EventStore` field to `PagerApp`
3. Initialize SQLite store before tracker
4. Load recent events and replay into tracker
5. Pass store to `httpapi.New` and `NewSettingsBinding`
6. Close store in `ServiceShutdown`

```go
package wails

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"pager/internal/adapter/httpapi"
	"pager/internal/adapter/notify"
	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/config"
	"pager/internal/infra/store"
)

//go:embed assets/tray-icon@2x.png
var trayIconData []byte

// PagerApp orchestrates the full Wails application lifecycle.
type PagerApp struct {
	tracker *session.Tracker
	srv     *httpapi.Server
	store   store.EventStore
	tray    *application.SystemTray
}

// defaultDBPath returns the SQLite database file path.
func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "pager.db")
}

// NewPagerApp assembles and returns a runnable Wails application.
func NewPagerApp(assets embed.FS) *application.App {
	logger := slog.Default().With("module", "wails")

	p := &PagerApp{}

	// ── Load config ─────────────────────────────────────────────────────────
	initialCfg, _ := config.LoadFrom(config.DefaultPath())

	// ── SQLite EventStore ───────────────────────────────────────────────────
	eventStore, err := store.NewSQLiteStore(defaultDBPath())
	if err != nil {
		slog.Error("failed to open event store", "error", err)
		// Continue without persistence — graceful degradation
	}
	p.store = eventStore

	// ── SessionTracker ──────────────────────────────────────────────────────
	p.tracker = session.NewTracker(func(sessions []*session.Session) {
		wailsApp := application.Get()
		if wailsApp == nil {
			return
		}
		wailsApp.Event.Emit("sessions-updated", sessions)

		if len(sessions) > 0 && sessions[0].LastEvent != nil {
			cfg, _ := config.LoadFrom(config.DefaultPath())
			notify.ShowFull(sessions[0].LastEvent, cfg.NotificationLevel, cfg.Language)
		}

		if p.tray != nil {
			p.updateTrayIcon(sessions)
		}
	})

	// ── Replay from SQLite ──────────────────────────────────────────────────
	if eventStore != nil {
		replayEvents, err := eventStore.LoadRecentSessions(initialCfg.SessionLoadHours)
		if err != nil {
			slog.Error("failed to load recent sessions", "error", err)
		} else if len(replayEvents) > 0 {
			logger.Info("replaying events from SQLite", "count", len(replayEvents))
			p.tracker.Replay(replayEvents)
		}
	}

	// ── HTTP Server ─────────────────────────────────────────────────────────
	var serverOpts []httpapi.ServerOption
	if eventStore != nil {
		serverOpts = append(serverOpts, httpapi.WithStore(eventStore))
	}
	p.srv = httpapi.New(p.tracker, serverOpts...)

	// ── Services ────────────────────────────────────────────────────────────
	sessionBinding := &SessionBinding{tracker: p.tracker}

	var popupWindow *application.WebviewWindow
	var settingsWindow *application.WebviewWindow

	settingsBinding := NewSettingsBinding(func(cfg config.Settings) {
		RegisterHotkey(popupWindow, cfg.HotkeyToggle)
	}, eventStore)

	windowBinding := &WindowBinding{configPath: config.DefaultPath()}

	// ── Wails app ───────────────────────────────────────────────────────────
	wailsApp := application.New(application.Options{
		Name:        "Pager",
		Description: "AI coding agents 状态感知层",
		Services: []application.Service{
			application.NewService(p),
			application.NewService(sessionBinding),
			application.NewService(settingsBinding),
			application.NewService(windowBinding),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// ── System tray ─────────────────────────────────────────────────────────
	tray := wailsApp.SystemTray.New()
	tray.SetTemplateIcon(trayIconData)
	tray.SetTooltip("Pager — AI agent monitor")
	p.tray = tray

	// ── Popup window ────────────────────────────────────────────────────────
	popupWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Pager",
		Name:             "pager-panel",
		Width:            initialCfg.PopupWidth,
		Height:           520,
		MinWidth:         300,
		MaxWidth:         600,
		MinHeight:        200,
		MaxHeight:        800,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      initialCfg.PopupPinned,
		DisableResize:    false,
		HideOnFocusLost:  false,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTransparent,
		},
	})

	popupWindow.RegisterHook(events.Common.WindowLostFocus, func(e *application.WindowEvent) {
		cfg, _ := config.LoadFrom(config.DefaultPath())
		if !cfg.PopupPinned {
			popupWindow.Hide()
		}
	})

	// ── Settings window ─────────────────────────────────────────────────────
	settingsWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "Pager 设置",
		Name:          "pager-settings",
		Width:         720,
		Height:        520,
		Hidden:        true,
		Frameless:     true,
		DisableResize: true,
		URL:           "#/settings",
	})

	settingsWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		settingsWindow.Hide()
	})

	windowBinding.popup = popupWindow
	windowBinding.settings = settingsWindow

	// ── Tray menu ───────────────────────────────────────────────────────────
	trayMenu := wailsApp.NewMenu()
	trayMenu.Add("偏好设置...").
		OnClick(func(_ *application.Context) {
			settingsWindow.Show()
			settingsWindow.Focus()
		})
	trayMenu.AddSeparator()
	trayMenu.Add("退出 Pager").
		SetAccelerator("CmdOrCtrl+Q").
		OnClick(func(_ *application.Context) {
			wailsApp.Quit()
		})
	tray.SetMenu(trayMenu)

	tray.AttachWindow(popupWindow).WindowOffset(5)

	// ── Global hotkey ───────────────────────────────────────────────────────
	go func() {
		time.Sleep(500 * time.Millisecond)
		RegisterHotkey(popupWindow, initialCfg.HotkeyToggle)
	}()

	logger.Info("app assembled")
	return wailsApp
}

// ServiceStartup implements application.ServiceStartup.
func (p *PagerApp) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	slog.Info("starting HTTP server", "module", "wails", "addr", httpapi.ListenAddr)
	p.srv.Start()
	return nil
}

// ServiceShutdown implements application.ServiceShutdown.
func (p *PagerApp) ServiceShutdown() error {
	if p.store != nil {
		slog.Info("closing event store", "module", "wails")
		return p.store.Close()
	}
	return nil
}

func (p *PagerApp) updateTrayIcon(sessions []*session.Session) {
	hasWaiting := false
	hasActive := false

	for _, s := range sessions {
		switch s.Status {
		case entity.StatusWaiting:
			hasWaiting = true
		case entity.StatusActive:
			hasActive = true
		}
	}

	switch {
	case hasWaiting:
		p.tray.SetLabel("●")
	case hasActive:
		p.tray.SetLabel("")
	default:
		p.tray.SetLabel("")
	}
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/wails/app.go
git commit -m "feat(app): wire SQLite EventStore into app lifecycle

- Initialize SQLite on startup, graceful degradation if fails
- Replay recent events into SessionTracker on boot
- Pass store to HTTP server and settings binding
- Close store on shutdown (flush remaining buffer)"
```

---

## Task 9: DismissSession with Store Integration

Wire the dismiss action to also soft-delete in SQLite.

**Files:**
- Modify: `internal/wails/session_svc.go`

- [ ] **Step 1: Add store to SessionBinding and update DismissSession**

```go
package wails

import (
	"fmt"

	"pager/internal/adapter/terminal"
	"pager/internal/domain/session"
	"pager/internal/infra/store"
)

// SessionBinding exposes session state and actions to the React frontend via Wails bindings.
type SessionBinding struct {
	tracker *session.Tracker
	store   store.EventStore
}

// ListSessions returns all sessions sorted by UpdatedAt descending.
func (s *SessionBinding) ListSessions() []*session.Session {
	if s.tracker == nil {
		return nil
	}
	return s.tracker.ListByRecent()
}

// JumpToTerminal brings the terminal window/tab to the foreground.
func (s *SessionBinding) JumpToTerminal(sessionKey string) error {
	if s.tracker == nil {
		return fmt.Errorf("tracker not initialised")
	}

	sess, ok := s.tracker.Session(sessionKey)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionKey)
	}

	req := terminal.JumpRequest{
		TTY:            sess.TTY,
		TermProgram:    sess.TermProgram,
		ITermSessionID: sess.ITermSessionID,
	}

	return terminal.Jump(req)
}

// DismissSession removes a session from the tracker and soft-deletes in store.
func (s *SessionBinding) DismissSession(sessionKey string) {
	if s.tracker == nil {
		return
	}
	s.tracker.Dismiss(sessionKey)

	if s.store != nil {
		_ = s.store.DismissSession(sessionKey)
	}
}
```

- [ ] **Step 2: Update app.go to pass store to SessionBinding**

In `internal/wails/app.go`, change:

```go
sessionBinding := &SessionBinding{tracker: p.tracker, store: eventStore}
```

- [ ] **Step 3: Run build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/wails/session_svc.go internal/wails/app.go
git commit -m "feat(session): dismiss also soft-deletes in SQLite store"
```

---

## Task 10: Frontend — Sort Logic Change

Change project grouping sort from recent-activity to alphabetical.

**Files:**
- Modify: `frontend/src/store/sessions.ts`

- [ ] **Step 1: Update useProjectGroups sort**

In `frontend/src/store/sessions.ts`, change the `groups.sort` in `useProjectGroups()`:

Replace:
```typescript
groups.sort((a, b) => {
    const aTime = new Date(a.sessions[0]?.UpdatedAt || 0).getTime()
    const bTime = new Date(b.sessions[0]?.UpdatedAt || 0).getTime()
    return bTime - aTime
  })
```

With:
```typescript
  // Sort projects alphabetically by name
  groups.sort((a, b) => a.project.localeCompare(b.project))
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add frontend/src/store/sessions.ts
git commit -m "feat(ui): sort project groups alphabetically instead of by recent activity"
```

---

## Task 11: Frontend — Settings Data Management UI

Add data management section to settings panel.

**Files:**
- Modify: `frontend/src/pages/SettingsPanel.tsx` (or relevant settings component)

- [ ] **Step 1: Add data management section**

Add a new section to the settings panel with:
- Session load hours input (numeric, bound to `session_load_hours` setting)
- DB stats display (size + event count)
- Purge buttons: 7 days / 30 days / All (with confirmation dialog)

The exact implementation depends on the current SettingsPanel structure. Key bindings needed:

```typescript
// Import from generated Wails bindings
import { GetDataStats, PurgeData } from '../../bindings/pager/internal/wails/settingsbinding.js'

// Component logic
const [stats, setStats] = useState({ db_size_bytes: 0, event_count: 0 })

useEffect(() => {
  GetDataStats().then(setStats)
}, [])

const handlePurge = async (days: number) => {
  if (!confirm(days === 0 ? '确定清理全部数据？' : `确定清理 ${days} 天前的数据？`)) return
  await PurgeData(days)
  const newStats = await GetDataStats()
  setStats(newStats)
}
```

- [ ] **Step 2: Regenerate Wails bindings**

Run: `make bindings`
Expected: New binding files generated for `GetDataStats` and `PurgeData`

- [ ] **Step 3: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add frontend/
git commit -m "feat(ui): add data management section to settings panel

- Session load hours configuration
- DB size and event count display
- Purge buttons with confirmation: 7d / 30d / all"
```

---

## Task 12: Integration Test — Full Flow

End-to-end verification that events flow from bridge → server → tracker + SQLite → reload.

**Files:**
- None (manual testing + existing test run)

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: All PASS

- [ ] **Step 2: Run lint**

Run: `make lint`
Expected: No errors

- [ ] **Step 3: Build production app**

Run: `make build`
Expected: `.app` bundle created successfully

- [ ] **Step 4: Manual smoke test**

1. Start app: `make dev`
2. Send test event: `echo '{"session_id":"test-123","cwd":"/tmp/test","tool_name":"Bash","tool_input":{"command":"echo hi"},"tool_use_id":"tu-1"}' | go run ./cmd/bridge pre_tool_use`
3. Verify event appears in UI
4. Quit and restart app
5. Verify session persists after restart (loaded from SQLite)
6. Dismiss session in UI
7. Verify session disappears and stays gone after restart

- [ ] **Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: integration test fixes"
```
