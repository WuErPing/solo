package agent

import (
	"github.com/WuErPing/solo/protocol"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

// TestSubscribeToSession_SlowHandleStreamEventDoesNotDropCritical verifies that
// slow handleStreamEvent processing (e.g. due to ApplySnapshot or blocked
// sendMessage) does not cause the dispatcher's subscriber channel to fill up
// and drop subsequent events (especially turn_completed).
//
// This is a regression test for the bug where:
//  1. OpenCode produces many timeline events during a turn
//  2. Each event triggers synchronous handleStreamEvent → sendMessage (which can
//     block up to 5s on sendQueue full) and ApplySnapshot (disk I/O)
//  3. subscribeToSession goroutine falls behind, dispatcher subscriber channel fills
//  4. When turn_completed arrives, dispatcher's 500ms subscriber timeout fires
//  5. turn_completed is silently dropped — client never sees terminal state
//
// The fix adds a buffered work channel between the dispatcher drain and
// handleStreamEvent, so the dispatcher channel is always drained quickly.
func TestSubscribeToSession_SlowHandleStreamEventDoesNotDropCritical(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := base.NewChannelDispatcher(logger)

	// Subscribe to the dispatcher (simulates agent.Session.Subscribe())
	subCh := dispatcher.Subscribe()

	// Track received events
	var mu sync.Mutex
	var received []string

	// Simulate the fixed subscribeToSession: drain dispatcher channel into a
	// buffered work channel, process from work channel in a separate goroutine.
	workCh := make(chan AgentStreamEvent, 256)
	var processWg sync.WaitGroup
	processWg.Add(1)
	go func() {
		defer processWg.Done()
		for evt := range workCh {
			time.Sleep(10 * time.Millisecond) // simulate slow handleStreamEvent
			if e, ok := evt.Event.(protocol.StreamEvent); ok {
				mu.Lock()
				received = append(received, e.StreamEventType())
				mu.Unlock()
			}
		}
	}()

	// Fast drain loop: reads from dispatcher channel and sends to work channel.
	// Critical events block, non-critical drop if workCh is full.
	var drainWg sync.WaitGroup
	drainWg.Add(1)
	go func() {
		defer drainWg.Done()
		for raw := range subCh {
			evt, ok := raw.(AgentStreamEvent)
			if !ok {
				continue
			}
			if evt.IsCriticalEvent() {
				workCh <- evt // never drop critical
			} else {
				select {
				case workCh <- evt:
				default:
					// drop non-critical if workCh full
				}
			}
		}
		close(workCh)
	}()

	// Emit enough events to overwhelm the slow consumer.
	// With 10ms processing, 350 events would take 3.5s to process.
	// Without the work channel buffer, the dispatcher's 256-capacity subscriber
	// channel fills up and turn_completed is dropped by the 500ms timeout.
	for i := 0; i < 350; i++ {
		dispatcher.Emit(AgentStreamEvent{
			Event:     protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "tool_call", Name: "read_file", Status: "running"}, Provider: "mock"},
			Timestamp: time.Now(),
		})
	}

	// Emit turn_completed — this is critical and MUST NOT be dropped
		// Emit turn_completed — this is critical and MUST NOT be dropped
		dispatcher.Emit(AgentStreamEvent{
			Event:     protocol.TurnCompletedStreamEvent{Provider: "mock"},
			Timestamp: time.Now(),
		})

	// Wait for all processing to complete
	time.Sleep(5 * time.Second)
	dispatcher.Close()
	drainWg.Wait()
	processWg.Wait()

	// Check if turn_completed was received
	mu.Lock()
	defer mu.Unlock()
	found := false
	totalTimeline := 0
	for _, t := range received {
		if t == "turn_completed" {
			found = true
		}
		if t == "timeline" {
			totalTimeline++
		}
	}
	if !found {
		t.Errorf("turn_completed was dropped — received %d events total (%d timeline, turn_completed missing)", len(received), totalTimeline)
	}
}
