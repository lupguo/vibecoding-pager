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
	bufferCap     = 512
	batchSize     = 50
	flushInterval = 2 * time.Second
)

// sqliteStore implements EventStore using SQLite via sqlx.
type sqliteStore struct {
	db     *sqlx.DB
	dbPath string
	buf    chan *entity.AgentEvent
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath and initializes the schema.
func NewSQLiteStore(dbPath string) (*sqliteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", dbPath)
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
		dbPath: dbPath,
		buf:    make(chan *entity.AgentEvent, bufferCap),
		ctx:    ctx,
		cancel: cancel,
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
		SELECT e.session_key, e.agent_label, e.event_type, e.tool_name, e.tool_use_id,
		       e.content, e.content_raw, e.attention_level, e.permission_mode,
		       e.raw_payload, e.timestamp,
		       s.session_id, s.agent, s.host, s.cwd, s.tty,
		       s.term_program, s.iterm_session_id
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
			sessionKey, agentLabel, eventType, toolName, toolUseID string
			content, contentRaw, attentionLevel, permMode          string
			rawPayload                                              []byte
			tsStr                                                   string
			sessionID, agent, host, cwd, tty                       string
			termProgram, itermSessionID                            string
		)

		if err := rows.Scan(
			&sessionKey, &agentLabel, &eventType, &toolName, &toolUseID,
			&content, &contentRaw, &attentionLevel, &permMode,
			&rawPayload, &tsStr,
			&sessionID, &agent, &host, &cwd, &tty,
			&termProgram, &itermSessionID,
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
		SELECT agent_label, event_type, tool_name, tool_use_id, content, content_raw,
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
			agentLabel, eventType, toolName, toolUseID            string
			content, contentRaw, attentionLevel, permMode string
			rawPayload                                    []byte
			tsStr                                         string
		)

		if err := rows.Scan(
			&agentLabel, &eventType, &toolName, &toolUseID, &content, &contentRaw,
			&attentionLevel, &permMode, &rawPayload, &tsStr,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		ts, _ := time.ParseInLocation("2006-01-02 15:04:05", tsStr, time.Local)

		events = append(events, &entity.AgentEvent{
			SessionID:      sessionKey,
			AgentLabel:     agentLabel,
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
	info, err := os.Stat(s.dbPath)
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
		case e := <-s.buf:
			if e == nil {
				// Channel was closed or nil received after context cancel
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
			// Drain remaining events from buffer (non-blocking)
			for {
				select {
				case e := <-s.buf:
					if e == nil {
						goto flush
					}
					batch = append(batch, e)
				default:
					goto flush
				}
			}
		flush:
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
			INSERT INTO t_events (session_key, agent_label, event_type, tool_name, tool_use_id, content, content_raw, attention_level, permission_mode, raw_payload, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			sessionKey, e.AgentLabel, e.EventType, e.ToolName, e.ToolUseID,
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
