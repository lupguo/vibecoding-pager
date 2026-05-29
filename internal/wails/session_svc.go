package wails

import (
	"fmt"

	"pager/internal/adapter/terminal"
	"pager/internal/domain/session"
)

// SessionBinding exposes session state and actions to the React frontend via Wails bindings.
type SessionBinding struct {
	reg *session.Registry
}

// ListSessions returns all sessions sorted by UpdatedAt descending.
func (s *SessionBinding) ListSessions() []*session.Session {
	if s.reg == nil {
		return nil
	}
	return s.reg.ListSorted()
}

// JumpToTerminal brings the terminal window/tab to the foreground.
func (s *SessionBinding) JumpToTerminal(sessionKey string) error {
	if s.reg == nil {
		return fmt.Errorf("registry not initialised")
	}

	sess, ok := s.reg.GetByKey(sessionKey)
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

// DismissSession removes a session from the registry.
func (s *SessionBinding) DismissSession(sessionKey string) {
	if s.reg == nil {
		return
	}
	s.reg.Remove(sessionKey)
}
