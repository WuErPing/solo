package agent

import (
	"github.com/WuErPing/solo/protocol"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

func TestAgentStreamEventValueImplementsCriticalEventInterface(t *testing.T) {
	evt := AgentStreamEvent{
		Event: protocol.TurnCompletedStreamEvent{},
		Timestamp: time.Now(),
	}

	if !evt.IsCriticalEvent() {
		t.Fatal("expected turn_completed to be critical")
	}
	if _, ok := interface{}(evt).(base.CriticalEvent); !ok {
		t.Fatal("AgentStreamEvent value must implement base.CriticalEvent for dispatcher priority checks")
	}
}

func TestAgentStreamEventTimelineReasoningIsSemiCritical(t *testing.T) {
	cases := []struct {
		name string
		item interface{}
	}{
		{
			name: "timeline item struct",
			item: TimelineItem{Type: "reasoning", Text: "thinking"},
		},
		{
			name: "timeline item map",
			item: map[string]interface{}{"type": "reasoning", "text": "thinking"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := AgentStreamEvent{
				Event:     protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "reasoning", Text: "thinking"}},
				Timestamp: time.Now(),
			}

			if !evt.IsSemiCriticalEvent() {
				t.Fatal("expected reasoning timeline event to be semi-critical")
			}
			if _, ok := interface{}(evt).(base.SemiCriticalEvent); !ok {
				t.Fatal("AgentStreamEvent value must implement base.SemiCriticalEvent for dispatcher priority checks")
			}
		})
	}
}
