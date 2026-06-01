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
