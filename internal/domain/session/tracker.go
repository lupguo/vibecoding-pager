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
		// If AskUserQuestion is pending (awaiting user response), keep attention state.
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
