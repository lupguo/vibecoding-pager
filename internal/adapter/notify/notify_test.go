package notify

import (
	"testing"

	"pager/internal/domain/entity"
)

func TestShouldNotifyByConfig_NilMap(t *testing.T) {
	e := &entity.AgentEvent{AgentLabel: "CC", EventType: "StopFailure"}
	if ShouldNotifyByConfig(e, nil) {
		t.Error("expected false for nil map")
	}
}

func TestShouldNotifyByConfig_LabelNotFound(t *testing.T) {
	e := &entity.AgentEvent{AgentLabel: "OTHER", EventType: "StopFailure"}
	m := map[string][]string{"CC": {"StopFailure"}}
	if ShouldNotifyByConfig(e, m) {
		t.Error("expected false when label not in config")
	}
}

func TestShouldNotifyByConfig_EventNotInList(t *testing.T) {
	e := &entity.AgentEvent{AgentLabel: "CC", EventType: "SomeOtherEvent"}
	m := map[string][]string{"CC": {"StopFailure", "Notification"}}
	if ShouldNotifyByConfig(e, m) {
		t.Error("expected false when event not in list")
	}
}

func TestShouldNotifyByConfig_Match(t *testing.T) {
	e := &entity.AgentEvent{AgentLabel: "CC", EventType: "StopFailure"}
	m := map[string][]string{"CC": {"StopFailure", "Notification"}}
	if !ShouldNotifyByConfig(e, m) {
		t.Error("expected true for matching label and event")
	}
}

func TestShouldNotifyByConfig_CCINTMatch(t *testing.T) {
	e := &entity.AgentEvent{AgentLabel: "CC-INT", EventType: "Notification"}
	m := map[string][]string{
		"CC":     {"StopFailure", "Notification"},
		"CC-INT": {"StopFailure", "Notification"},
	}
	if !ShouldNotifyByConfig(e, m) {
		t.Error("expected true for CC-INT Notification match")
	}
}

func TestShouldNotify_AttentionOnly(t *testing.T) {
	// StopFailure -> StatusError -> notify in attention_only.
	e := &entity.AgentEvent{EventType: "StopFailure"}
	if !ShouldNotify(e, "attention_only") {
		t.Error("StopFailure should notify in attention_only")
	}

	// Stop (clean) -> StatusDone -> should NOT notify in attention_only.
	e2 := &entity.AgentEvent{EventType: "Stop"}
	if ShouldNotify(e2, "attention_only") {
		t.Error("Stop (clean) should NOT notify in attention_only")
	}

	// Notification -> StatusWaiting -> notify in attention_only.
	e3 := &entity.AgentEvent{EventType: "Notification"}
	if !ShouldNotify(e3, "attention_only") {
		t.Error("Notification should notify in attention_only")
	}

	// PostToolUse -> StatusWorking -> should NOT notify.
	e4 := &entity.AgentEvent{EventType: "PostToolUse"}
	if ShouldNotify(e4, "attention_only") {
		t.Error("PostToolUse should NOT notify in attention_only")
	}
}

func TestShouldNotify_All(t *testing.T) {
	// Stop (clean) -> StatusDone -> notify in "all".
	e := &entity.AgentEvent{EventType: "Stop"}
	if !ShouldNotify(e, "all") {
		t.Error("Stop should notify in all")
	}

	// StopFailure -> StatusError -> notify in "all".
	e2 := &entity.AgentEvent{EventType: "StopFailure"}
	if !ShouldNotify(e2, "all") {
		t.Error("StopFailure should notify in all")
	}

	// Notification -> StatusWaiting -> notify in "all".
	e3 := &entity.AgentEvent{EventType: "Notification"}
	if !ShouldNotify(e3, "all") {
		t.Error("Notification should notify in all")
	}

	// PostToolUse -> StatusWorking -> should NOT notify in "all".
	e4 := &entity.AgentEvent{EventType: "PostToolUse"}
	if ShouldNotify(e4, "all") {
		t.Error("PostToolUse should NOT notify in all")
	}
}

func TestShouldNotify_UnknownLevel(t *testing.T) {
	e := &entity.AgentEvent{EventType: "StopFailure"}
	if ShouldNotify(e, "off") {
		t.Error("unknown level should not notify")
	}
	if ShouldNotify(e, "") {
		t.Error("empty level should not notify")
	}
}
