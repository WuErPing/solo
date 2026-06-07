package agent

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestIsCriticalEvent_AllTerminalTypes(t *testing.T) {
	cases := []struct {
		name     string
		event    interface{}
		expected bool
	}{
		{"turn_completed", protocol.TurnCompletedStreamEvent{}, true},
		{"turn_failed", protocol.TurnFailedStreamEvent{Error: "fail"}, true},
		{"turn_canceled", protocol.TurnCanceledStreamEvent{}, true},
		{"timeline", protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "text"}}, false},
		{"thread_started", protocol.ThreadStartedStreamEvent{}, false},
		{"usage_updated", protocol.UsageUpdatedStreamEvent{}, false},
		{"permission_requested", protocol.PermissionRequestedStreamEvent{}, false},
		{"string_event", "not a stream event", false},
		{"nil_event", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event:     tc.event,
				Timestamp: time.Now(),
			}

			if got := evt.IsCriticalEvent(); got != tc.expected {
				t.Errorf("IsCriticalEvent() for %q = %v, want %v", tc.name, got, tc.expected)
			}
		})
	}
}

func TestIsSemiCriticalEvent_ReasoningTimeline(t *testing.T) {
	cases := []struct {
		name     string
		itemType string
		expected bool
	}{
		{"reasoning", "reasoning", true},
		{"text", "text", false},
		{"assistant_message", "assistant_message", false},
		{"tool_call", "tool_call", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event: protocol.TimelineStreamEvent{
					Item: protocol.TimelineItem{Type: tc.itemType},
				},
				Timestamp: time.Now(),
			}

			if got := evt.IsSemiCriticalEvent(); got != tc.expected {
				t.Errorf("IsSemiCriticalEvent() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestIsSemiCriticalEvent_NonTimelineEvents(t *testing.T) {
	cases := []struct {
		name  string
		event interface{}
	}{
		{"turn_completed", protocol.TurnCompletedStreamEvent{}},
		{"turn_failed", protocol.TurnFailedStreamEvent{}},
		{"thread_started", protocol.ThreadStartedStreamEvent{}},
		{"usage_updated", protocol.UsageUpdatedStreamEvent{}},
		{"string", "not a stream event"},
		{"nil", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event:     tc.event,
				Timestamp: time.Now(),
			}

			if evt.IsSemiCriticalEvent() {
				t.Errorf("expected false for %q", tc.name)
			}
		})
	}
}
