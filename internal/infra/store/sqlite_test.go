package store

import (
	"database/sql"
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

func TestSQLiteStore_InsertEvent_PopulatesCwdAndProjectName(t *testing.T) {
	dbPath := tempDBPath(t)
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	e := makeTestEvent("sess-cwd", entity.EventPreToolUse, "Bash")
	e.CWD = "/Users/dev/projects/myapp"
	s.Record(e)

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen with raw SQL to inspect the new columns
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer rawDB.Close()

	var cwd, project string
	err = rawDB.QueryRow(
		`SELECT cwd, project_name FROM t_events WHERE session_key = ? ORDER BY id DESC LIMIT 1`,
		"sess-cwd",
	).Scan(&cwd, &project)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if cwd != "/Users/dev/projects/myapp" {
		t.Errorf("cwd = %q, want %q", cwd, "/Users/dev/projects/myapp")
	}
	if project != "myapp" {
		t.Errorf("project_name = %q, want %q", project, "myapp")
	}
}

func TestSQLiteStore_Migrate_BackfillsCwdAndProjectName(t *testing.T) {
	dbPath := tempDBPath(t)

	// First open: NewSQLiteStore creates schema + runs migrate (no-op for new DB)
	// + insert a session and an event with the new columns auto-populated.
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	e := makeTestEvent("sess-bf", entity.EventPreToolUse, "Bash")
	e.CWD = "/projects/legacy"
	s.Record(e)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate "old data": clear the new columns to '' on the existing row,
	// then re-open (which runs migrate -> backfill).
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := rawDB.Exec(`UPDATE t_events SET cwd='', project_name='' WHERE session_key='sess-bf'`); err != nil {
		t.Fatalf("clear cols: %v", err)
	}
	rawDB.Close()

	// Reopen — migrate() should backfill from t_sessions.
	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	rawDB2, _ := sql.Open("sqlite", dbPath)
	defer rawDB2.Close()
	var cwd, project string
	if err := rawDB2.QueryRow(
		`SELECT cwd, project_name FROM t_events WHERE session_key='sess-bf' LIMIT 1`,
	).Scan(&cwd, &project); err != nil {
		t.Fatalf("query: %v", err)
	}
	if cwd == "" || project == "" {
		t.Errorf("expected backfill, got cwd=%q project=%q", cwd, project)
	}
	if project != "legacy" {
		t.Errorf("project_name = %q, want %q", project, "legacy")
	}
}

func TestMigrationV1ToV2(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v1.db")

	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	v1Schema := `
CREATE TABLE t_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key TEXT NOT NULL UNIQUE,
    session_id TEXT NOT NULL DEFAULT '',
    agent TEXT NOT NULL DEFAULT 'claude-code',
    host TEXT NOT NULL DEFAULT '',
    cwd TEXT NOT NULL DEFAULT '',
    project_name TEXT NOT NULL DEFAULT '',
    tty TEXT NOT NULL DEFAULT '',
    term_program TEXT NOT NULL DEFAULT '',
    iterm_session_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    attention_level TEXT NOT NULL DEFAULT 'running',
    agent_label TEXT NOT NULL DEFAULT 'CC',
    created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    deleted_at TEXT DEFAULT NULL
);
CREATE TABLE t_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    tool_name TEXT NOT NULL DEFAULT '',
    tool_use_id TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    content_raw TEXT NOT NULL DEFAULT '',
    attention_level TEXT NOT NULL DEFAULT '',
    permission_mode TEXT NOT NULL DEFAULT '',
    raw_payload BLOB DEFAULT NULL,
    timestamp TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);
INSERT INTO t_sessions (session_key, status) VALUES ('s-active', 'active');
INSERT INTO t_sessions (session_key, status) VALUES ('s-finished', 'finished');
INSERT INTO t_sessions (session_key, status) VALUES ('s-error', 'error');
`
	if _, err := raw.Exec(v1Schema); err != nil {
		t.Fatalf("seed v1 schema: %v", err)
	}
	_ = raw.Close()

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	rows, err := store.db.Queryx(`SELECT session_key, status FROM t_sessions ORDER BY session_key`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for rows.Next() {
		var k, s string
		if err := rows.Scan(&k, &s); err != nil {
			t.Fatal(err)
		}
		got[k] = s
	}
	rows.Close()

	want := map[string]string{
		"s-active":   "working",
		"s-finished": "done",
		"s-error":    "error",
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("status[%s] = %q; want %q", k, got[k], w)
		}
	}

	cols, _ := store.db.Queryx(`PRAGMA table_info(t_sessions)`)
	for cols.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		_ = cols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == "attention_level" {
			t.Errorf("t_sessions.attention_level still present after migration")
		}
	}
	cols.Close()
}
