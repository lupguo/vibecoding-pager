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

func TestApply_PreToolUse_CreatesSession(t *testing.T) {
	var called int
	reg := New(func(sessions []*Session) { called++ })

	e := makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/project", "/dev/ttys001")
	reg.Apply(e)

	sessions := reg.ListSorted()
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
		t.Errorf("onChange called %d times, want 1", called)
	}
}

func TestApply_PostToolUse_ClearsPending(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	reg.Apply(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(entity.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))

	if len(lastSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(lastSessions))
	}
	s := lastSessions[0]
	if s.Status != entity.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusActive)
	}
	if len(s.PendingTools) != 0 {
		t.Errorf("PendingTools should be empty, has %d", len(s.PendingTools))
	}
}

func TestApply_PostToolUse_MultiplePending(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	reg.Apply(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(entity.EventPreToolUse, "Edit", "tu-2", "/p", "/dev/ttys001"))

	reg.Apply(makeEvent(entity.EventPostToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	s := lastSessions[0]
	if s.Status != entity.StatusWaiting {
		t.Errorf("status = %q, want %q (still has pending)", s.Status, entity.StatusWaiting)
	}

	reg.Apply(makeEvent(entity.EventPostToolUse, "Edit", "tu-2", "/p", "/dev/ttys001"))
	s = lastSessions[0]
	if s.Status != entity.StatusActive {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusActive)
	}
}

func TestApply_Stop(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	reg.Apply(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(entity.EventStop, "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusFinished {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusFinished)
	}
}

func TestApply_Error(t *testing.T) {
	var lastSessions []*Session
	reg := New(func(sessions []*Session) { lastSessions = sessions })

	reg.Apply(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys001"))
	reg.Apply(makeEvent(entity.EventError, "", "", "/p", "/dev/ttys001"))

	s := lastSessions[0]
	if s.Status != entity.StatusError {
		t.Errorf("status = %q, want %q", s.Status, entity.StatusError)
	}
}

func TestListSorted_OrderByUpdatedAt(t *testing.T) {
	reg := New(func([]*Session) {})

	e1 := makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/project-a", "/dev/ttys001")
	e1.Timestamp = time.Now().Add(-10 * time.Second)
	reg.Apply(e1)

	e2 := makeEvent(entity.EventPreToolUse, "Bash", "tu-2", "/project-b", "/dev/ttys002")
	e2.Timestamp = time.Now()
	reg.Apply(e2)

	sessions := reg.ListSorted()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].CWD != "/project-b" {
		t.Errorf("first session CWD = %q, want /project-b", sessions[0].CWD)
	}
}

func TestGetByTTY(t *testing.T) {
	reg := New(func([]*Session) {})

	reg.Apply(makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/p", "/dev/ttys005"))

	s, ok := reg.GetByTTY("/dev/ttys005")
	if !ok {
		t.Fatal("expected to find session by TTY")
	}
	if s.TTY != "/dev/ttys005" {
		t.Errorf("TTY = %q, want /dev/ttys005", s.TTY)
	}

	_, ok = reg.GetByTTY("/dev/ttys999")
	if ok {
		t.Error("should not find non-existent TTY")
	}
}

func TestApply_SessionID_Key(t *testing.T) {
	reg := New(func([]*Session) {})

	e1 := makeEvent(entity.EventPreToolUse, "Bash", "tu-1", "/project", "/dev/ttys001")
	e1.SessionID = "session-aaa"
	reg.Apply(e1)

	e2 := makeEvent(entity.EventPreToolUse, "Bash", "tu-2", "/project", "/dev/ttys001")
	e2.SessionID = "session-bbb"
	reg.Apply(e2)

	sessions := reg.ListSorted()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (different session_id), got %d", len(sessions))
	}
}

func TestApply_FallbackKey_NoSessionID(t *testing.T) {
	reg := New(func([]*Session) {})

	e := &entity.AgentEvent{
		Agent:     entity.AgentClaudeCode,
		Host:      "local",
		SessionID: "",
		CWD:       "/project",
		TTY:       "/dev/ttys001",
		EventType: entity.EventPreToolUse,
		ToolName:  "Bash",
		ToolUseID: "tu-1",
		Content:   "test",
		Timestamp: time.Now(),
	}
	reg.Apply(e)

	sessions := reg.ListSorted()
	if sessions[0].Key != "local:/project:/dev/ttys001" {
		t.Errorf("fallback key = %q, want host:cwd:tty format", sessions[0].Key)
	}
}
