package notify

import (
	"testing"

	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
)

func TestShouldNotify_AttentionOnly(t *testing.T) {
	cases := []struct {
		ev   *entity.AgentEvent
		want bool
	}{
		{&entity.AgentEvent{EventType: "StopFailure"}, true},
		{&entity.AgentEvent{EventType: "PermissionRequest"}, true},
		{&entity.AgentEvent{EventType: "Stop"}, false},
		{&entity.AgentEvent{EventType: "PostToolUse", ToolName: "Edit", PermissionMode: "default"}, false},
		{&entity.AgentEvent{EventType: "PreToolUse", ToolName: "Edit", PermissionMode: "default"}, true},
	}
	for _, tc := range cases {
		if got := ShouldNotify(tc.ev, "attention_only"); got != tc.want {
			t.Errorf("ShouldNotify(%q) = %v; want %v", tc.ev.EventType, got, tc.want)
		}
	}
}

func TestShouldNotifyByConfig(t *testing.T) {
	cfg := map[string][]string{
		"CC": {"PreToolUse", "Stop"},
	}
	if !ShouldNotifyByConfig(&entity.AgentEvent{AgentLabel: "CC", EventType: "Stop"}, cfg) {
		t.Error("CC + Stop should notify")
	}
	if ShouldNotifyByConfig(&entity.AgentEvent{AgentLabel: "CC", EventType: "Notification"}, cfg) {
		t.Error("CC + Notification should NOT notify")
	}
}

// TestBuildOsascriptCmd verifies the AppleScript fragment built for the
// `osascript -e ...` fallback used when the binary runs unbundled. The
// nasty case is escaping: AppleScript treats backslash as escape and
// double-quote as string terminator, so titles/bodies that contain them
// must be quoted or the script will throw a syntax error AND the
// notification will silently never appear.
func TestBuildOsascriptCmd(t *testing.T) {
	cases := []struct {
		name     string
		title    string
		body     string
		wantArgs []string
	}{
		{
			name:  "plain text",
			title: "Pager · projA",
			body:  "Stop",
			wantArgs: []string{
				"-e",
				`display notification "Stop" with title "VibeCoding Pager" subtitle "Pager · projA" sound name "Tink"`,
			},
		},
		{
			name:  "double quotes get escaped",
			title: `agent says "hi"`,
			body:  `said "hello"`,
			wantArgs: []string{
				"-e",
				`display notification "said \"hello\"" with title "VibeCoding Pager" subtitle "agent says \"hi\"" sound name "Tink"`,
			},
		},
		{
			name:  "backslash gets escaped before quote-escape",
			title: `path C:\foo`,
			body:  `\n is literal`,
			wantArgs: []string{
				"-e",
				`display notification "\\n is literal" with title "VibeCoding Pager" subtitle "path C:\\foo" sound name "Tink"`,
			},
		},
		{
			name:  "newline replaced with space (single-line script)",
			title: "line1\nline2",
			body:  "first\nsecond",
			wantArgs: []string{
				"-e",
				`display notification "first second" with title "VibeCoding Pager" subtitle "line1 line2" sound name "Tink"`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildOsascriptCmd(tc.title, tc.body)
			if len(got) != len(tc.wantArgs) {
				t.Fatalf("arg count = %d; want %d (got=%v)", len(got), len(tc.wantArgs), got)
			}
			for i := range got {
				if got[i] != tc.wantArgs[i] {
					t.Errorf("arg[%d] = %q\n  want %q", i, got[i], tc.wantArgs[i])
				}
			}
		})
	}
}

// TestShowEvent_NilSvc_FallsBackToOsascript verifies that when the Wails
// NotificationService is nil (unbundled-mode runs where service registration
// was skipped to avoid the macOS bundle-id panic), ShowEvent does NOT silently
// drop the notification — it routes through the osascript fallback path
// instead. We can't actually run osascript in CI, but we can prove the path
// is reached by injecting a custom runner.
func TestShowEvent_NilSvc_FallsBackToOsascript(t *testing.T) {
	called := false
	var capturedArgs []string
	prev := osascriptRunner
	osascriptRunner = func(args []string) error {
		called = true
		capturedArgs = args
		return nil
	}
	defer func() { osascriptRunner = prev }()

	ShowEvent(nil, &entity.AgentEvent{
		Agent: entity.AgentClaudeCode, AgentLabel: "CC", CWD: "/p/projA",
		EventType: "Stop", Content: "task complete",
	}, "zh")

	if !called {
		t.Fatal("expected osascript runner to be invoked when svc is nil")
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "-e" {
		t.Fatalf("expected osascript -e <script>, got %v", capturedArgs)
	}
}
