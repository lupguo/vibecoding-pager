package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
)

func TestHandleEvent_ValidPost(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	e := entity.AgentEvent{
		Agent:     entity.AgentClaudeCode,
		Host:      "local",
		CWD:       "/project",
		TTY:       "/dev/ttys001",
		EventType: entity.EventPreToolUse,
		ToolName:  "Bash",
		ToolUseID: "tu-1",
		Content:   "go build",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(e)

	req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	sessions := tracker.ListByRecent()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
}

func TestHandleEvent_RejectsGet(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	req := httptest.NewRequest(http.MethodGet, "/event", nil)
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleEvent_BadJSON(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	srv.HandleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSessions(t *testing.T) {
	tracker := session.NewTracker(func([]*session.Session) {})
	srv := New(tracker)

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	w := httptest.NewRecorder()

	srv.HandleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
