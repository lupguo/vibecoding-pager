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
	AttentionLevel string                       `json:"AttentionLevel"`
	AgentLabel     string                       `json:"AgentLabel"`
	SessionID      string                       `json:"SessionID"`
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
			SessionID:    e.SessionID,
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

	// Update attention level and agent label from event
	if e.AttentionLevel != "" {
		s.AttentionLevel = e.AttentionLevel
	}
	if e.AgentLabel != "" {
		s.AgentLabel = e.AgentLabel
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
