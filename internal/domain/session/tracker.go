package session

import (
	"sort"
	"sync"
	"time"

	"pager/internal/domain/entity"
)

// Session represents an active agent session.
type Session struct {
	Key            string                        `json:"Key"`
	Agent          string                        `json:"Agent"`
	Host           string                        `json:"Host"`
	CWD            string                        `json:"CWD"`
	TTY            string                        `json:"TTY"`
	TermProgram    string                        `json:"TermProgram"`
	ITermSessionID string                        `json:"ITermSessionID"`
	Status         entity.SessionStatus          `json:"Status"`
	AgentLabel     string                        `json:"AgentLabel"`
	SessionID      string                        `json:"SessionID"`
	LastEvent      *entity.AgentEvent            `json:"LastEvent"`
	PendingTools   map[string]*entity.AgentEvent `json:"PendingTools"`
	UpdatedAt      time.Time                     `json:"UpdatedAt"`
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

	// Update agent label from event
	if e.AgentLabel != "" {
		s.AgentLabel = e.AgentLabel
	}

	// Maintain pending-tool bookkeeping (must run before DeriveStatus so the
	// hasPendingAskUser flag is accurate).
	switch e.EventType {
	case "PreToolUse":
		if e.ToolUseID != "" {
			s.PendingTools[e.ToolUseID] = e
		}
	case "PostToolUse":
		if e.ToolUseID != "" {
			delete(s.PendingTools, e.ToolUseID)
		}
	case "Stop", "SessionEnd":
		// Clean termination clears pending bookkeeping unless AskUserQuestion is in flight.
		if !hasAskUserPending(s.PendingTools) {
			s.PendingTools = make(map[string]*entity.AgentEvent)
		}
	}

	s.Status = DeriveStatus(e.EventType, e.ToolName, e.PermissionMode, hasAskUserPending(s.PendingTools))

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

// DismissByProject removes all sessions whose CWD's last segment equals project.
// Returns the dismissed session keys for caller-side store sync.
func (t *Tracker) DismissByProject(project string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var keys []string
	for k, s := range t.sessions {
		if ProjectFromCWD(s.CWD) == project {
			keys = append(keys, k)
			delete(t.sessions, k)
		}
	}
	if len(keys) > 0 && t.onUpdate != nil {
		t.onUpdate(t.snapshotLocked())
	}
	return keys
}

// ProjectFromCWD returns the last non-empty path segment of cwd.
// Mirrors the frontend's projectFromCWD logic to keep grouping consistent.
func ProjectFromCWD(cwd string) string {
	for i := len(cwd) - 1; i >= 0; i-- {
		if cwd[i] == '/' {
			if i == len(cwd)-1 {
				continue
			}
			return cwd[i+1:]
		}
	}
	return cwd
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
