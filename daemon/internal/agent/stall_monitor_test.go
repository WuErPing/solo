package agent

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestStallMonitor(t *testing.T) (*StallMonitor, chan string) {
	t.Helper()
	interrupted := make(chan string, 1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	m := NewStallMonitor(logger, func(id string) error {
		interrupted <- id
		return nil
	},
		WithCheckInterval(50*time.Millisecond),
		WithInactivityThreshold(150*time.Millisecond),
		WithRepetitionThreshold(5, 3),
	)
	m.Start()
	t.Cleanup(m.Stop)
	return m, interrupted
}

func TestStallMonitor_Inactivity(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-a")

	// No events → should stall after inactivity threshold + one check interval.
	select {
	case id := <-interrupted:
		if id != "agent-a" {
			t.Fatalf("expected agent-a, got %s", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("expected stall interrupt due to inactivity")
	}
}

func TestStallMonitor_ActivityResetsInactivity(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-b")

	// Send periodic varied events to keep agent alive.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 8; i++ {
			m.RecordEvent("agent-b", AgentStreamEvent{
				Event: map[string]interface{}{
					"type": "timeline",
					"item": TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("msg %d", i)},
				},
			})
			time.Sleep(80 * time.Millisecond)
		}
	}()

	select {
	case id := <-interrupted:
		t.Fatalf("agent should not stall while receiving varied events, got interrupt for %s", id)
	case <-done:
		// Agent survived 640ms without stalling (threshold is 150ms).
	}
}

func TestStallMonitor_Repetition(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-c")

	// Send identical messages repeatedly.
	for i := 0; i < 5; i++ {
		m.RecordEvent("agent-c", AgentStreamEvent{
			Event: map[string]interface{}{
				"type": "timeline",
				"item": TimelineItem{Type: "assistant_message", Text: "There are 5 agent.py files in this project:"},
			},
		})
	}

	select {
	case id := <-interrupted:
		if id != "agent-c" {
			t.Fatalf("expected agent-c, got %s", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("expected stall interrupt due to repetition")
	}
}

func TestStallMonitor_NoFalsePositiveForVariedOutput(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-d")

	// Send varied messages — no stall expected.
	for i := 0; i < 10; i++ {
		m.RecordEvent("agent-d", AgentStreamEvent{
			Event: map[string]interface{}{
				"type": "timeline",
				"item": TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("message %d", i)},
			},
		})
	}

	// Wait less than inactivity threshold to avoid false positive.
	time.Sleep(80 * time.Millisecond)

	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for %s", id)
	default:
	}
}

func TestStallMonitor_UnregisterStopsTracking(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-e")
	m.UnregisterAgent("agent-e")

	// Even if no events arrive, unregistered agent should not be interrupted.
	time.Sleep(300 * time.Millisecond)

	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for unregistered agent %s", id)
	default:
	}
}

func TestStallMonitor_HasRecentProgress(t *testing.T) {
	m, _ := newTestStallMonitor(t)
	m.RegisterAgent("agent-f")

	if !m.HasRecentProgress("agent-f") {
		t.Fatal("expected recent progress right after registration")
	}

	// Wait past inactivity threshold.
	time.Sleep(200 * time.Millisecond)

	if m.HasRecentProgress("agent-f") {
		t.Fatal("expected no recent progress after inactivity threshold")
	}
}

func TestStallMonitor_RecordEventCreatesState(t *testing.T) {
	m, _ := newTestStallMonitor(t)
	// No explicit RegisterAgent — RecordEvent should lazily create state.
	m.RecordEvent("agent-g", AgentStreamEvent{
		Event: map[string]interface{}{
			"type": "timeline",
			"item": TimelineItem{Type: "assistant_message", Text: "hello"},
		},
	})

	if !m.HasRecentProgress("agent-g") {
		t.Fatal("expected recent progress after RecordEvent")
	}
}

func TestStallMonitor_ReasoningEventsCountAsActivity(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-h")

	// Send only reasoning events — these should count as activity.
	for i := 0; i < 5; i++ {
		m.RecordEvent("agent-h", AgentStreamEvent{
			Event: map[string]interface{}{
				"type": "timeline",
				"item": TimelineItem{Type: "reasoning", Text: "thinking..."},
			},
		})
		time.Sleep(80 * time.Millisecond)
	}

	// Wait less than threshold after last event.
	time.Sleep(80 * time.Millisecond)

	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for %s", id)
	default:
	}
}

func TestStallMonitor_RepetitionIgnoresWhitespace(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-i")

	// Same text with different whitespace/casing — only 2 messages, below threshold.
	m.RecordEvent("agent-i", AgentStreamEvent{
		Event: map[string]interface{}{
			"type": "timeline",
			"item": TimelineItem{Type: "assistant_message", Text: "  Hello World  "},
		},
	})
	m.RecordEvent("agent-i", AgentStreamEvent{
		Event: map[string]interface{}{
			"type": "timeline",
			"item": TimelineItem{Type: "assistant_message", Text: "hello world"},
		},
	})

	// Keep sending varied events in background to avoid inactivity stall.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 5; i++ {
			m.RecordEvent("agent-i", AgentStreamEvent{
				Event: map[string]interface{}{
					"type": "timeline",
					"item": TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("keep-alive %d", i)},
				},
			})
			time.Sleep(80 * time.Millisecond)
		}
	}()

	// Not enough for threshold (need 3 identical in window of 5).
	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for %s", id)
	case <-done:
	}
}

func TestStallMonitor_RepetitionWithMapItem(t *testing.T) {
	m, interrupted := newTestStallMonitor(t)
	m.RegisterAgent("agent-j")

	for i := 0; i < 5; i++ {
		m.RecordEvent("agent-j", AgentStreamEvent{
			Event: map[string]interface{}{
				"type": "timeline",
				"item": map[string]interface{}{
					"type": "assistant_message",
					"text": "loop",
				},
			},
		})
	}

	select {
	case id := <-interrupted:
		if id != "agent-j" {
			t.Fatalf("expected agent-j, got %s", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("expected stall interrupt for map-shaped timeline item")
	}
}
