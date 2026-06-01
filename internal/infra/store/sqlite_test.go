package store

import (
	"encoding/json"
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

	// Record events with distinct timestamps
	e1 := makeTestEvent("sess-1", entity.EventPreToolUse, "Bash")
	e1.Timestamp = time.Now().Add(-1 * time.Second)
	e2 := makeTestEvent("sess-1", entity.EventPostToolUse, "Bash")
	e2.Timestamp = time.Now()
	s.Record(e1)
	s.Record(e2)

	// Flush by closing and reopening
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

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

func TestSQLiteStore_SessionEvents(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	s.Record(makeTestEvent("sess-query", entity.EventPreToolUse, "Bash"))
	s.Record(makeTestEvent("sess-query", entity.EventPostToolUse, "Bash"))
	s.Record(makeTestEvent("sess-other", entity.EventPreToolUse, "Edit"))
	s.Close()

	s2, _ := NewSQLiteStore(dbPath)
	defer s2.Close()

	events, err := s2.SessionEvents("sess-query")
	if err != nil {
		t.Fatalf("SessionEvents: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events for sess-query, got %d", len(events))
	}
}
