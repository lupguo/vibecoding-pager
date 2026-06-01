//go:build integration

package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"pager/internal/adapter/httpapi"
	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/store"
)

// TestE2E_StatusModelDataPath drives the full bridge → HTTP → tracker → SQLite path
// for every status-relevant event combination and asserts the resulting Session.Status.
// It does NOT cover UI, hotkey, or notification surfaces — see the manual checklist
// at docs/regression/2026-06-01-ui-state-regression.md for those.
func TestE2E_StatusModelDataPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(filepath.Join(dir, "e2e.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	tr := session.NewTracker(nil)
	srv := httpapi.New(tr, httpapi.WithStore(st))
	srv.Start()
	defer srv.Stop()

	// Wait briefly for listener
	time.Sleep(150 * time.Millisecond)

	type expect struct {
		name      string
		event     entity.AgentEvent
		preceding []entity.AgentEvent
		want      entity.SessionStatus
	}
	cases := []expect{
		{
			name: "PreToolUse AskUserQuestion → waiting",
			event: entity.AgentEvent{
				SessionID: "s-ask", CWD: "/p/a", EventType: entity.EventPreToolUse,
				ToolName: "AskUserQuestion", Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
		{
			name: "PreToolUse Edit + bypass → working",
			event: entity.AgentEvent{
				SessionID: "s-bypass", CWD: "/p/b", EventType: entity.EventPreToolUse,
				ToolName: "Edit", PermissionMode: "bypassPermissions", Timestamp: time.Now(),
			},
			want: entity.StatusWorking,
		},
		{
			name: "PreToolUse Edit + default → waiting",
			event: entity.AgentEvent{
				SessionID: "s-edit", CWD: "/p/c", EventType: entity.EventPreToolUse,
				ToolName: "Edit", PermissionMode: "default", Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
		{
			name: "PostToolUse → working",
			event: entity.AgentEvent{
				SessionID: "s-post", CWD: "/p/d", EventType: entity.EventPostToolUse,
				ToolName: "Bash", Timestamp: time.Now(),
			},
			want: entity.StatusWorking,
		},
		{
			name: "Stop (no pending) → done",
			event: entity.AgentEvent{
				SessionID: "s-stop", CWD: "/p/e", EventType: entity.EventStop, Timestamp: time.Now(),
			},
			want: entity.StatusDone,
		},
		{
			name: "Stop (AskUser pending) → waiting",
			preceding: []entity.AgentEvent{
				{SessionID: "s-pending-ask", CWD: "/p/f", EventType: entity.EventPreToolUse,
					ToolName: "AskUserQuestion", ToolUseID: "tu-1", Timestamp: time.Now()},
			},
			event: entity.AgentEvent{
				SessionID: "s-pending-ask", CWD: "/p/f", EventType: entity.EventStop,
				Timestamp: time.Now().Add(time.Second),
			},
			want: entity.StatusWaiting,
		},
		{
			name: "StopFailure → error",
			event: entity.AgentEvent{
				SessionID: "s-fail", CWD: "/p/g", EventType: entity.EventStopFailure, Timestamp: time.Now(),
			},
			want: entity.StatusError,
		},
		{
			name: "PermissionRequest → waiting",
			event: entity.AgentEvent{
				SessionID: "s-perm", CWD: "/p/h", EventType: entity.EventPermissionRequest,
				Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
		{
			name: "Notification → waiting",
			event: entity.AgentEvent{
				SessionID: "s-notif", CWD: "/p/i", EventType: entity.EventNotification,
				Timestamp: time.Now(),
			},
			want: entity.StatusWaiting,
		},
	}

	postEvent := func(t *testing.T, e entity.AgentEvent) {
		t.Helper()
		body, _ := json.Marshal(e)
		resp, err := http.Post(
			fmt.Sprintf("http://%s/event", httpapi.ListenAddr),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST status: %d", resp.StatusCode)
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, pre := range tc.preceding {
				postEvent(t, pre)
			}
			postEvent(t, tc.event)

			time.Sleep(50 * time.Millisecond)

			s, ok := tr.Session(tc.event.SessionID)
			if !ok {
				t.Fatalf("session %q not found", tc.event.SessionID)
			}
			if s.Status != tc.want {
				t.Errorf("session.Status = %q; want %q", s.Status, tc.want)
			}
		})
	}
}

// TestE2E_DismissByProjectFlow asserts the per-project bulk clear chain
// (tracker.DismissByProject correctly removes only the matching project's sessions).
func TestE2E_DismissByProjectFlow(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(filepath.Join(dir, "e2e2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tr := session.NewTracker(nil)
	mk := func(key, cwd string) *entity.AgentEvent {
		return &entity.AgentEvent{
			SessionID: key, CWD: cwd, EventType: entity.EventPreToolUse, ToolName: "Edit",
			PermissionMode: "bypassPermissions", Timestamp: time.Now(),
		}
	}
	tr.TrackEvent(mk("s1", "/x/projA"))
	tr.TrackEvent(mk("s2", "/x/projA"))
	tr.TrackEvent(mk("s3", "/x/projB"))

	keys := tr.DismissByProject("projA")
	if len(keys) != 2 {
		t.Errorf("dismissed = %d; want 2", len(keys))
	}
	left := tr.ListByRecent()
	if len(left) != 1 || left[0].SessionID != "s3" {
		t.Errorf("remaining = %+v; want only s3", left)
	}
}
