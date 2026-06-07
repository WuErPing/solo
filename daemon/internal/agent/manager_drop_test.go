package agent

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// TestAgentManager_DroppedEventCount_StartsAtZero verifies the counter
// initialises to 0 on a fresh manager.
func TestAgentManager_DroppedEventCount_StartsAtZero(t *testing.T) {
	mgr := createTestManager(t)
	if got := mgr.DroppedEventCount(); got != 0 {
		t.Errorf("DroppedEventCount() = %d, want 0", got)
	}
}

// TestAgentManager_DroppedEventCount_IncrementOnDrop verifies that when
// the per-agent workCh is full, non-critical events increment the counter.
//
// We shrink workChCapacity to 1 so the buffer fills almost immediately,
// then flood the session channel with non-critical events and assert the
// drop counter increases.
func TestAgentManager_DroppedEventCount_IncrementOnDrop(t *testing.T) {
	orig := workChCapacity.Load()
	workChCapacity.Store(1)
	defer func() { workChCapacity.Store(orig) }()

	mgr := createTestManager(t)

	// Build a MockAgentSession with a large event buffer so we can send
	// many events without blocking.
	mockSession := &MockAgentSession{}
	mockSession.events = make(chan AgentStreamEvent, 500)

	// Create a minimal ManagedAgent pointing at our mock session.
	ag := &ManagedAgent{
		ID:        "drop-test-agent",
		Provider:  "mock",
		Lifecycle: protocol.AgentRunning,
		Session:   mockSession,
	}

	// Start subscribeToSession — it reads from mockSession.events via Subscribe().
	// The consumer goroutine inside subscribeToSession is intentionally slow
	// (blocked on workCh <- event) because workChCapacity=1.
	go mgr.subscribeToSession(ag)

	// Give subscribeToSession goroutine time to start
	time.Sleep(30 * time.Millisecond)

	// Send 200 non-critical timeline events rapidly.
	// With workChCapacity=1, most will be dropped.
	nonCritical := AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
	}
	for i := 0; i < 200; i++ {
		mockSession.events <- nonCritical
	}
	// Close the channel to signal end of events
	close(mockSession.events)

	// Wait for the subscriber goroutine to drain
	time.Sleep(300 * time.Millisecond)

	dropped := mgr.DroppedEventCount()
	if dropped == 0 {
		t.Error("DroppedEventCount() = 0; expected >0 after flooding workCh with non-critical events")
	}
	t.Logf("DroppedEventCount() = %d (out of 200 sent)", dropped)
}
