package agent

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// TestSubscribeToSession_CriticalEventFallback_WhenWorkChFull verifies that
// when the workCh consumer is stalled (slow global subscriber blocking
// handleStreamEvent) and workCh is full, a critical turn_completed event is
// NOT blocked forever. The fallback must fire within criticalWorkChSendTimeout
// and transition the agent to idle directly, without waiting for the stalled
// consumer.
//
// Before the fix: workCh <- event for critical events has no timeout, so the
// drain goroutine blocks indefinitely when workCh is full. The dispatcher's
// 500ms subscriberCriticalTimeout fires first and silently drops turn_completed.
// The agent is forever stuck in protocol.AgentRunning.
func TestSubscribeToSession_CriticalEventFallback_WhenWorkChFull(t *testing.T) {
	origCap := workChCapacity.Load()
	origTimeout := criticalWorkChSendTimeout
	workChCapacity.Store(1)
	criticalWorkChSendTimeout = 50 * time.Millisecond
	defer func() {
		workChCapacity.Store(origCap)
		criticalWorkChSendTimeout = origTimeout
	}()

	mgr := createTestManager(t)

	// Register a slow global subscriber that simulates sendMessage blocked on a
	// full sendQueue (criticalSendTimeout = 5s in production).
	// 300ms > criticalWorkChSendTimeout (50ms), so the fallback fires first.
	var slowSubCalled atomic.Int64
	mgr.Subscribe(func(AgentEvent) {
		slowSubCalled.Add(1)
		time.Sleep(300 * time.Millisecond)
	})

	eventsCh := make(chan AgentStreamEvent, 100)
	mockSession := &MockAgentSession{events: eventsCh}
	ag := &ManagedAgent{
		ID:          "freeze-test-agent",
		Provider:    "mock",
		Lifecycle:   protocol.AgentRunning,
		Session:     mockSession,
		subscribers: make(map[uint64]AgentEventFunc),
	}

	go mgr.subscribeToSession(ag)
	time.Sleep(20 * time.Millisecond) // let drain goroutine start

	// Flood with non-critical events to fill workCh (capacity=1) and stall consumer.
	for i := 0; i < 10; i++ {
		eventsCh <- AgentStreamEvent{
			AgentID: ag.ID,
			Event:   protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		}
	}
	// Critical event: must not block forever even though workCh is full.
	eventsCh <- AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TurnCompletedStreamEvent{},
	}
	close(eventsCh)

	// Agent must reach idle within 3s.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			ag.mu.RLock()
			lc := ag.Lifecycle
			ag.mu.RUnlock()
			t.Fatalf("agent did not reach idle within 3s; lifecycle=%s", lc)
		default:
		}
		ag.mu.RLock()
		lc := ag.Lifecycle
		ag.mu.RUnlock()
		if lc == protocol.AgentIdle {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSubscribeToSession_OpenCodeTerminalEventFallback_WhenWorkChFull(t *testing.T) {
	origCap := workChCapacity.Load()
	origTimeout := criticalWorkChSendTimeout
	workChCapacity.Store(1)
	criticalWorkChSendTimeout = 50 * time.Millisecond
	defer func() {
		workChCapacity.Store(origCap)
		criticalWorkChSendTimeout = origTimeout
	}()

	mgr := createTestManager(t)
	mgr.Subscribe(func(AgentEvent) {
		time.Sleep(300 * time.Millisecond)
	})

	eventsCh := make(chan AgentStreamEvent, 100)
	mockSession := &MockAgentSession{events: eventsCh}
	ag := &ManagedAgent{
		ID:          "opencode-freeze-test-agent",
		Provider:    opencodeProviderName,
		Lifecycle:   protocol.AgentRunning,
		Session:     mockSession,
		subscribers: make(map[uint64]AgentEventFunc),
	}

	go mgr.subscribeToSession(ag)
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < 10; i++ {
		eventsCh <- AgentStreamEvent{
			AgentID: ag.ID,
			Event:   protocol.TimelineStreamEvent{Provider: opencodeProviderName, Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		}
	}
	eventsCh <- AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TurnCompletedStreamEvent{Provider: opencodeProviderName},
	}
	close(eventsCh)

	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			ag.mu.RLock()
			lc := ag.Lifecycle
			ag.mu.RUnlock()
			t.Fatalf("OpenCode-shaped terminal event did not reach idle within 3s; lifecycle=%s", lc)
		default:
		}
		ag.mu.RLock()
		lc := ag.Lifecycle
		ag.mu.RUnlock()
		if lc == protocol.AgentIdle {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestSubscribeToSession_SlowSubscriberDoesNotStallWorkCh verifies that a slow
// global subscriber (simulating sendMessage blocked on a full sendQueue) does
// not cause turn_completed to be permanently dropped. The fix is the P0-A
// critical-event fallback: when workCh is full because the consumer is stalled
// by a slow emit() call, applyTerminalStreamState is called directly without
// waiting for the consumer to drain.
//
// This complements TestSubscribeToSession_CriticalEventFallback_WhenWorkChFull
// by using a slow subscriber (not a slow workCh capacity) as the root cause.
func TestSubscribeToSession_SlowSubscriberDoesNotStallWorkCh(t *testing.T) {
	origCap := workChCapacity.Load()
	origTimeout := criticalWorkChSendTimeout
	workChCapacity.Store(1)
	criticalWorkChSendTimeout = 50 * time.Millisecond
	defer func() {
		workChCapacity.Store(origCap)
		criticalWorkChSendTimeout = origTimeout
	}()

	mgr := createTestManager(t)

	// Slow subscriber: simulates sendMessage blocked for criticalSendTimeout.
	// This stalls the workCh consumer for 300ms per event, causing workCh to fill.
	mgr.Subscribe(func(AgentEvent) {
		time.Sleep(300 * time.Millisecond)
	})

	eventsCh := make(chan AgentStreamEvent, 100)
	mockSession := &MockAgentSession{events: eventsCh}
	ag := &ManagedAgent{
		ID:          "slow-sub-test-agent",
		Provider:    "mock",
		Lifecycle:   protocol.AgentRunning,
		Session:     mockSession,
		subscribers: make(map[uint64]AgentEventFunc),
	}

	go mgr.subscribeToSession(ag)
	time.Sleep(20 * time.Millisecond)

	// Flood with non-critical events to stall the consumer (workCh capacity=1).
	for i := 0; i < 5; i++ {
		eventsCh <- AgentStreamEvent{
			AgentID: ag.ID,
			Event:   protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		}
	}
	// Critical event arrives while consumer is stalled. The P0-A fallback
	// must apply idle state directly, bypassing the stalled consumer.
	eventsCh <- AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TurnCompletedStreamEvent{},
	}
	close(eventsCh)

	// Agent must reach idle within 3s despite the slow subscriber.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			ag.mu.RLock()
			lc := ag.Lifecycle
			ag.mu.RUnlock()
			t.Fatalf("agent did not reach idle within 3s; lifecycle=%s", lc)
		default:
		}
		ag.mu.RLock()
		lc := ag.Lifecycle
		ag.mu.RUnlock()
		if lc == protocol.AgentIdle {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
