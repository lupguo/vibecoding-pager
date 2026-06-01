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
