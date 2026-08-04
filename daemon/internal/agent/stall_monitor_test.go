package agent

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// syncBuffer is a goroutine-safe buffer for capturing slog output written by
// the stall monitor's own goroutine. The interrupt channel only carries the
// agentID, so the stall reason is observed through the monitor's log.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// timelineAssistantEvent builds a typed timeline stream event carrying an
// assistant message. extractAssistantTextFromStreamEvent only matches typed
// protocol.TimelineStreamEvent values, so repetition tests MUST use typed
// events — map-shaped events never reach the repetition tracker.
func timelineAssistantEvent(text string) AgentStreamEvent {
	return AgentStreamEvent{
		Event: protocol.TimelineStreamEvent{
			Item: protocol.TimelineItem{Type: "assistant_message", Text: text},
		},
	}
}

func newTestStallMonitor(t *testing.T) (*StallMonitor, chan string, *syncBuffer) {
	t.Helper()
	interrupted := make(chan string, 1)
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
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
	return m, interrupted, logs
}

// waitForInterrupt waits for a stall interrupt with a generous deadline.
func waitForInterrupt(t *testing.T, interrupted chan string) string {
	t.Helper()
	select {
	case id := <-interrupted:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("expected stall interrupt within 5s")
		return ""
	}
}

func TestStallMonitor_Inactivity(t *testing.T) {
	m, interrupted, logs := newTestStallMonitor(t)
	m.RegisterAgent("agent-a")

	// No events → should stall after inactivity threshold + one check interval.
	if id := waitForInterrupt(t, interrupted); id != "agent-a" {
		t.Fatalf("expected agent-a, got %s", id)
	}
	if !strings.Contains(logs.String(), "reason=inactivity") {
		t.Fatalf("expected inactivity stall reason, logs: %s", logs.String())
	}
}

func TestStallMonitor_ActivityResetsInactivity(t *testing.T) {
	m, interrupted, _ := newTestStallMonitor(t)
	m.RegisterAgent("agent-b")

	// Send periodic varied typed events to keep agent alive. The 40ms period
	// leaves a wide margin below the 150ms inactivity threshold.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20; i++ {
			m.RecordEvent("agent-b", timelineAssistantEvent(fmt.Sprintf("msg %d", i)))
			time.Sleep(40 * time.Millisecond)
		}
	}()

	select {
	case id := <-interrupted:
		t.Fatalf("agent should not stall while receiving varied events, got interrupt for %s", id)
	case <-done:
		// Agent survived 800ms without stalling (threshold is 150ms).
	}
}

// recordUntilInterrupted keeps sending identical typed assistant messages
// until the stall monitor interrupts the agent (or the test stops the
// goroutine). Continuous events keep lastEventTime fresh, so the inactivity
// check can never fire while this runs — any interrupt must come from the
// repetition path. The 20ms period is well below the 150ms inactivity
// threshold, leaving a wide margin even under -race scheduling delays.
func recordUntilInterrupted(m *StallMonitor, agentID string, texts []string, stop <-chan struct{}) {
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
		}
		m.RecordEvent(agentID, timelineAssistantEvent(texts[i%len(texts)]))
		time.Sleep(20 * time.Millisecond)
	}
}

func TestStallMonitor_Repetition(t *testing.T) {
	m, interrupted, logs := newTestStallMonitor(t)
	m.RegisterAgent("agent-c")

	// 3 identical normalized texts within the window of 5 meets the
	// repetition threshold. Events keep flowing until the interrupt fires,
	// so the trigger cannot be inactivity (which needs a 150ms gap).
	stop := make(chan struct{})
	defer close(stop)
	go recordUntilInterrupted(m, "agent-c",
		[]string{"There are 5 agent.py files in this project:"}, stop)

	if id := waitForInterrupt(t, interrupted); id != "agent-c" {
		t.Fatalf("expected agent-c, got %s", id)
	}
	// The interrupt channel only carries the agentID; the reason is observed
	// via the monitor log. Asserting reason=repetition proves the repetition
	// path fired and this is not the inactivity fallback (which would log
	// reason=inactivity ~150ms after the last event instead).
	if !strings.Contains(logs.String(), "reason=repetition") {
		t.Fatalf("expected repetition stall reason, logs: %s", logs.String())
	}
}

func TestStallMonitor_NoFalsePositiveForVariedOutput(t *testing.T) {
	m, interrupted, _ := newTestStallMonitor(t)
	m.RegisterAgent("agent-d")

	// Send varied typed messages — no stall expected.
	for i := 0; i < 10; i++ {
		m.RecordEvent("agent-d", timelineAssistantEvent(fmt.Sprintf("message %d", i)))
	}

	// Wait less than the inactivity threshold (150ms) so inactivity cannot
	// fire; varied texts cannot trip repetition either.
	time.Sleep(80 * time.Millisecond)

	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for %s", id)
	default:
	}
}

func TestStallMonitor_UnregisterStopsTracking(t *testing.T) {
	m, interrupted, _ := newTestStallMonitor(t)
	m.RegisterAgent("agent-e")
	m.UnregisterAgent("agent-e")

	// Even if no events arrive, unregistered agent should not be interrupted.
	// Threshold (150ms) + check interval (50ms) = 200ms minimum; wait well past
	// that with a generous margin.
	time.Sleep(500 * time.Millisecond)

	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for unregistered agent %s", id)
	default:
	}
}

func TestStallMonitor_HasRecentProgress(t *testing.T) {
	m, _, _ := newTestStallMonitor(t)
	m.RegisterAgent("agent-f")

	if !m.HasRecentProgress("agent-f") {
		t.Fatal("expected recent progress right after registration")
	}

	// Wait well past the inactivity threshold (150ms).
	time.Sleep(400 * time.Millisecond)

	if m.HasRecentProgress("agent-f") {
		t.Fatal("expected no recent progress after inactivity threshold")
	}
}

func TestStallMonitor_RecordEventCreatesState(t *testing.T) {
	m, _, _ := newTestStallMonitor(t)
	// No explicit RegisterAgent — RecordEvent should lazily create state.
	m.RecordEvent("agent-g", timelineAssistantEvent("hello"))

	if !m.HasRecentProgress("agent-g") {
		t.Fatal("expected recent progress after RecordEvent")
	}
}

func TestStallMonitor_ReasoningEventsCountAsActivity(t *testing.T) {
	m, interrupted, _ := newTestStallMonitor(t)
	m.RegisterAgent("agent-h")

	// Send only reasoning events — these should count as activity. The 40ms
	// period leaves a wide margin below the 150ms inactivity threshold.
	for i := 0; i < 5; i++ {
		m.RecordEvent("agent-h", AgentStreamEvent{
			Event: protocol.TimelineStreamEvent{
				Item: protocol.TimelineItem{Type: "reasoning", Text: "thinking..."},
			},
		})
		time.Sleep(40 * time.Millisecond)
	}

	// Wait less than the threshold after the last event.
	time.Sleep(80 * time.Millisecond)

	select {
	case id := <-interrupted:
		t.Fatalf("unexpected interrupt for %s", id)
	default:
	}
}

func TestStallMonitor_RepetitionIgnoresWhitespace(t *testing.T) {
	m, interrupted, logs := newTestStallMonitor(t)
	m.RegisterAgent("agent-i")

	// normalizeText lowercases and trims, so these three variants are
	// identical after normalization. Rotating through them reaches the
	// repetition threshold (3 identical within the window of 5) while
	// keeping the agent active, so the trigger cannot be inactivity.
	stop := make(chan struct{})
	defer close(stop)
	go recordUntilInterrupted(m, "agent-i",
		[]string{"  Hello World  ", "hello world", "HELLO WORLD   "}, stop)

	if id := waitForInterrupt(t, interrupted); id != "agent-i" {
		t.Fatalf("expected agent-i, got %s", id)
	}
	if !strings.Contains(logs.String(), "reason=repetition") {
		t.Fatalf("expected repetition stall for normalized-identical texts, logs: %s", logs.String())
	}
}

// TestStallMonitor_MapShapedEventsDoNotFeedRepetition pins current behavior:
// map-shaped timeline events update lastEventTime (they count as activity)
// but extractAssistantTextFromStreamEvent only matches typed
// protocol.TimelineStreamEvent, so their text never reaches the repetition
// tracker. Five identical map-shaped "assistant_message" events therefore
// cannot trigger a repetition stall — the only stall that can fire is
// inactivity. (With typed events the same input triggers repetition; see
// TestStallMonitor_Repetition.) Suspected gap, pinned pending a decision on
// whether map-shaped events should be parsed.
func TestStallMonitor_MapShapedEventsDoNotFeedRepetition(t *testing.T) {
	m, interrupted, logs := newTestStallMonitor(t)
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

	// The only stall that can fire is inactivity once the last event ages out.
	if id := waitForInterrupt(t, interrupted); id != "agent-j" {
		t.Fatalf("expected agent-j, got %s", id)
	}
	if strings.Contains(logs.String(), "reason=repetition") {
		t.Fatalf("map-shaped events must not feed repetition detection, logs: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "reason=inactivity") {
		t.Fatalf("expected inactivity stall for map-shaped events, logs: %s", logs.String())
	}
}
