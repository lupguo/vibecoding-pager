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
