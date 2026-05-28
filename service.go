package main

import (
	"fmt"

	"pager/internal/registry"
	"pager/internal/terminal"
)

// SessionService exposes session state and actions to the React frontend via Wails bindings.
//
// Wails v3 auto-generates TypeScript stubs in frontend/src/bindings/ from exported methods.
// Rules for binding generation:
//   - The struct and all bound methods must be exported.
//   - Method parameters and return values must be JSON-serialisable (or error).
//   - Methods named ServiceStartup / ServiceShutdown are treated as lifecycle hooks, not bindings.
type SessionService struct {
	reg *registry.Registry
}

// ListSessions returns all sessions sorted by UpdatedAt descending (newest first).
//
// Used by the frontend for:
//   - Initial data load on panel open.
//   - Manual refresh (pull-to-refresh).
//
// Real-time updates are pushed via the "sessions-updated" Wails event; polling is not needed.
func (s *SessionService) ListSessions() []*registry.Session {
	if s.reg == nil {
		return nil
	}
	return s.reg.ListSorted()
}

// JumpToTerminal brings the terminal window/tab that owns sessionKey to the foreground.
//
// Looks up the session by key, then delegates to terminal.Jump which handles
// iTerm2 (via ITermSessionID or TTY) and Terminal.app (via TTY) via osascript.
//
// Returns an error string visible in the frontend Toast on failure (e.g. permission denied
// on first run — macOS will prompt the user for Automation access).
func (s *SessionService) JumpToTerminal(sessionKey string) error {
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

// DismissSession removes a session from the registry (e.g. user manually closes a card).
//
// The registry's onChange callback fires after removal, so the frontend receives an updated
// "sessions-updated" event automatically — no additional round-trip needed.
func (s *SessionService) DismissSession(sessionKey string) {
	if s.reg == nil {
		return
	}
	s.reg.Remove(sessionKey)
}
