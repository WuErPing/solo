package agent

import (
	"testing"
	"time"
)

func TestIsCriticalEvent_AllTerminalTypes(t *testing.T) {
	cases := []struct {
		eventType string
		expected  bool
	}{
		{"turn_completed", true},
		{"turn_failed", true},
		{"turn_canceled", true},
		{"user_message", false},
		{"assistant_message", false},
		{"tool_use", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event: map[string]interface{}{
					"type": tc.eventType,
				},
				Timestamp: time.Now(),
			}

			if got := evt.IsCriticalEvent(); got != tc.expected {
				t.Errorf("IsCriticalEvent() for type %q = %v, want %v", tc.eventType, got, tc.expected)
			}
		})
	}
}

func TestIsCriticalEvent_NonMapEvent(t *testing.T) {
	evt := AgentStreamEvent{
		Event:     "not a map",
		Timestamp: time.Now(),
	}

	if evt.IsCriticalEvent() {
		t.Error("expected false for non-map event")
	}
}

func TestIsCriticalEvent_MapWithoutType(t *testing.T) {
	evt := AgentStreamEvent{
		Event: map[string]interface{}{
			"data": "no type key",
		},
		Timestamp: time.Now(),
	}

	if evt.IsCriticalEvent() {
		t.Error("expected false for map without type key")
	}
}

func TestIsCriticalEvent_TypeNotString(t *testing.T) {
	evt := AgentStreamEvent{
		Event: map[string]interface{}{
			"type": 123,
		},
		Timestamp: time.Now(),
	}

	if evt.IsCriticalEvent() {
		t.Error("expected false when type is not string")
	}
}

func TestIsSemiCriticalEvent_ReasoningType(t *testing.T) {
	evt := AgentStreamEvent{
		Event: map[string]interface{}{
			"type": "reasoning",
		},
		Timestamp: time.Now(),
	}

	if !evt.IsSemiCriticalEvent() {
		t.Error("expected true for reasoning type")
	}
}

func TestIsSemiCriticalEvent_TimelineWithReasoningItem(t *testing.T) {
	cases := []struct {
		name     string
		item     interface{}
		expected bool
	}{
		{
			name:     "reasoning struct",
			item:     TimelineItem{Type: "reasoning"},
			expected: true,
		},
		{
			name:     "reasoning map",
			item:     map[string]interface{}{"type": "reasoning"},
			expected: true,
		},
		{
			name:     "text struct",
			item:     TimelineItem{Type: "text"},
			expected: false,
		},
		{
			name:     "text map",
			item:     map[string]interface{}{"type": "text"},
			expected: false,
		},
		{
			name:     "nil item",
			item:     nil,
			expected: false,
		},
		{
			name:     "string item",
			item:     "not a valid item",
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event: map[string]interface{}{
					"type": "timeline",
					"item": tc.item,
				},
				Timestamp: time.Now(),
			}

			if got := evt.IsSemiCriticalEvent(); got != tc.expected {
				t.Errorf("IsSemiCriticalEvent() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestIsSemiCriticalEvent_NonTimelineNonReasoning(t *testing.T) {
	cases := []string{
		"user_message",
		"assistant_message",
		"tool_use",
		"tool_result",
		"",
	}

	for _, eventType := range cases {
		t.Run(eventType, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event: map[string]interface{}{
					"type": eventType,
				},
				Timestamp: time.Now(),
			}

			if evt.IsSemiCriticalEvent() {
				t.Errorf("expected false for type %q", eventType)
			}
		})
	}
}

func TestIsSemiCriticalEvent_NonMapEvent(t *testing.T) {
	evt := AgentStreamEvent{
		Event:     "not a map",
		Timestamp: time.Now(),
	}

	if evt.IsSemiCriticalEvent() {
		t.Error("expected false for non-map event")
	}
}

func TestIsSemiCriticalEvent_MapWithoutType(t *testing.T) {
	evt := AgentStreamEvent{
		Event: map[string]interface{}{
			"data": "no type key",
		},
		Timestamp: time.Now(),
	}

	if evt.IsSemiCriticalEvent() {
		t.Error("expected false for map without type key")
	}
}
