package session

import (
	"testing"
	"time"

	"pager/internal/domain/entity"
)

func makeEvent(eventType, toolName, toolUseID, cwd, tty string) *entity.AgentEvent {
	return &entity.AgentEvent{
		Agent:     entity.AgentClaudeCode,
		Host:      "local",
		SessionID: "test-session-" + cwd,
		CWD:       cwd,
		TTY:       tty,
		EventType: eventType,
		ToolName:  toolName,
		ToolUseID: toolUseID,
		Content:   toolName,
		Timestamp: time.Now(),
	}
}

func TestTrackEvent_PreToolUse_CreatesSession(t *testing.T) {
	var called int
	tr := NewTracker(func(sessions []*Session) { called++ })

	e := makeEvent("PreToolUse", "Bash", "tu-1", "/project", "/dev/ttys001")
	tr.TrackEvent(e)

	sessions := tr.ListByRecent()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.Status != entity.StatusWaiting {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusWaiting)
	}
	if s.Key != "test-session-/project" {
		t.Errorf("key = %q, want %q", s.Key, "test-session-/project")
	}
	if _, ok := s.PendingTools["tu-1"]; !ok {
		t.Error("tu-1 should be in PendingTools")
	}
	if called != 1 {
		t.Errorf("onUpdate called %d times, want 1", called)
	}
}

func TestTrackEvent_PostToolUse_ClearsPending(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent("PostToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))

	if len(lastSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(lastSessions))
	}
	s := lastSessions[0]
	if s.Status != entity.StatusWorking {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusWorking)
	}
	if len(s.PendingTools) != 0 {
		t.Errorf("PendingTools should be empty, has %d", len(s.PendingTools))
	}
}

func TestTrackEvent_PendingBookkeeping_MultipleTools(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent("PreToolUse", "Edit", "tu-2", "/p", "/dev/ttys001"))

	tr.TrackEvent(makeEvent("PostToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))
	s := lastSessions[0]
	if _, ok := s.PendingTools["tu-1"]; ok {
		t.Error("tu-1 should be cleared from PendingTools after PostToolUse")
	}
	if _, ok := s.PendingTools["tu-2"]; !ok {
		t.Error("tu-2 should still be in PendingTools")
	}

	tr.TrackEvent(makeEvent("PostToolUse", "Edit", "tu-2", "/p", "/dev/ttys001"))
	s = lastSessions[0]
	if len(s.PendingTools) != 0 {
		t.Errorf("PendingTools should be empty, has %d", len(s.PendingTools))
	}
	if s.Status != entity.StatusWorking {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusWorking)
	}
}

func TestTrackEvent_Stop(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent("Stop", "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusDone {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusDone)
	}
	if len(s.PendingTools) != 0 {
		t.Errorf("PendingTools should be cleared on Stop, has %d", len(s.PendingTools))
	}
}

func TestTrackEvent_Stop_AskUserQuestionPending(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent("PreToolUse", "AskUserQuestion", "tu-ask", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent("Stop", "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusWaiting {
		t.Errorf("status = %q, want %q (AskUserQuestion still pending)", s.Status, entity.StatusWaiting)
	}
	if _, ok := s.PendingTools["tu-ask"]; !ok {
		t.Error("AskUserQuestion pending should NOT be cleared by Stop")
	}
}

func TestTrackEvent_Error(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))
	tr.TrackEvent(makeEvent("Error", "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusError {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusError)
	}
}

func TestTrackEvent_SessionStart(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	e := makeEvent("SessionStart", "", "", "/p", "/dev/ttys001")
	tr.TrackEvent(e)

	s := lastSessions[0]
	if s.Status != entity.StatusWorking {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusWorking)
	}
}

func TestListByRecent_OrderByUpdatedAt(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	e1 := makeEvent("PreToolUse", "Bash", "tu-1", "/project-a", "/dev/ttys001")
	e1.Timestamp = time.Now().Add(-10 * time.Second)
	tr.TrackEvent(e1)

	e2 := makeEvent("PreToolUse", "Bash", "tu-2", "/project-b", "/dev/ttys002")
	e2.Timestamp = time.Now()
	tr.TrackEvent(e2)

	sessions := tr.ListByRecent()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].CWD != "/project-b" {
		t.Errorf("first session CWD = %q, want /project-b", sessions[0].CWD)
	}
}

func TestSessionByTTY(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	tr.TrackEvent(makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys005"))

	s, ok := tr.SessionByTTY("/dev/ttys005")
	if !ok {
		t.Fatal("expected to find session by TTY")
	}
	if s.TTY != "/dev/ttys005" {
		t.Errorf("TTY = %q, want /dev/ttys005", s.TTY)
	}

	_, ok = tr.SessionByTTY("/dev/ttys999")
	if ok {
		t.Error("should not find non-existent TTY")
	}
}

func TestSession_ByKey(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	e1 := makeEvent("PreToolUse", "Bash", "tu-1", "/project", "/dev/ttys001")
	e1.SessionID = "session-aaa"
	tr.TrackEvent(e1)

	_, ok := tr.Session("session-aaa")
	if !ok {
		t.Fatal("expected to find session by key")
	}

	_, ok = tr.Session("nonexistent")
	if ok {
		t.Error("should not find non-existent key")
	}
}

func TestDismiss(t *testing.T) {
	var lastSessions []*Session
	tr := NewTracker(func(sessions []*Session) { lastSessions = sessions })

	tr.TrackEvent(makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"))
	if len(lastSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(lastSessions))
	}

	tr.Dismiss("test-session-/p")
	if len(lastSessions) != 0 {
		t.Fatalf("expected 0 sessions after dismiss, got %d", len(lastSessions))
	}
}

func TestReplay(t *testing.T) {
	tr := NewTracker(func([]*Session) {})

	events := []*entity.AgentEvent{
		makeEvent("PreToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"),
		makeEvent("PostToolUse", "Bash", "tu-1", "/p", "/dev/ttys001"),
	}
	tr.Replay(events)

	sessions := tr.ListByRecent()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after replay, got %d", len(sessions))
	}
	if sessions[0].Status != entity.StatusWorking {
		t.Errorf("status = %q, want %q", sessions[0].Status, entity.StatusWorking)
	}
}
