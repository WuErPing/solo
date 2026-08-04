package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// Historical defect: base/pump.go used to emit its fallback terminal events
// (turn_canceled on context cancel, turn_failed on early EOF) as raw
// map[string]interface{} payloads. Those maps were not agent.AgentStreamEvent
// envelopes, so provider sessions filtered them out before they ever reached
// the manager; and even when injected directly, AgentStreamEvent.IsCriticalEvent
// (agent.go) and applyTerminalStreamState (manager.go) only matched typed
// protocol.Turn*StreamEvent values, so map-shaped terminals were droppable
// under backpressure and never applied terminal state. Terminal state relied
// entirely on the Run-return fallback in SendAgentMessage.
//
// The fix makes the pump emit AgentStreamEvent envelopes wrapping typed
// protocol.TurnCanceledStreamEvent / protocol.TurnFailedStreamEvent. These
// tests run a real base.EventPump and verify the emitted terminal events are
// critical and apply terminal state.

// pumpFallbackTranslator never produces a terminal event, so a real
// base.EventPump run always reaches its own fallback terminal emission.
type pumpFallbackTranslator struct{}

func (pumpFallbackTranslator) Translate(_ []byte, _ time.Time) ([]interface{}, bool, error) {
	return nil, false, nil
}

// runPumpFallback runs a real base.EventPump over input until it emits its
// fallback terminal event, returning every dispatched event.
func runPumpFallback(t *testing.T, ctx context.Context, input string) []interface{} {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := base.NewCallbackDispatcher(logger)

	var mu sync.Mutex
	var events []interface{}
	dispatcher.Subscribe(func(evt interface{}) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})

	pump := base.NewEventPump(logger, dispatcher)
	pump.SetProvider("mock")
	if _, err := pump.RunBlocking(ctx, strings.NewReader(input), pumpFallbackTranslator{}, nil); err == nil {
		t.Fatal("pump without a translator terminal event must finish with a cancel/crash result")
	}

	mu.Lock()
	defer mu.Unlock()
	return append([]interface{}(nil), events...)
}

// lastPumpEventAsStream returns the last dispatched event as an
// AgentStreamEvent, failing the test if the pump emitted any other shape
// (e.g. the historical raw map[string]interface{}).
func lastPumpEventAsStream(t *testing.T, events []interface{}) AgentStreamEvent {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("pump emitted no events")
	}
	last := events[len(events)-1]
	evt, ok := last.(AgentStreamEvent)
	if !ok {
		t.Fatalf("pump terminal fallback must be an AgentStreamEvent envelope, got %T", last)
	}
	return evt
}

// TestEventPump_TurnCanceledFallbackIsCritical verifies that the pump's
// context-cancel fallback emits a typed, critical turn_canceled event that
// can never be dropped as non-critical under dispatcher/workCh backpressure.
func TestEventPump_TurnCanceledFallbackIsCritical(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled: first scan iteration hits the ctx.Done() branch

	evt := lastPumpEventAsStream(t, runPumpFallback(t, ctx, "line\n"))

	canceled, ok := evt.Event.(protocol.TurnCanceledStreamEvent)
	if !ok {
		t.Fatalf("pump cancel fallback must carry protocol.TurnCanceledStreamEvent, got %T", evt.Event)
	}
	if canceled.Provider != "mock" {
		t.Fatalf("provider: got %q, want mock", canceled.Provider)
	}
	if !evt.IsCriticalEvent() {
		t.Fatal("pump turn_canceled fallback must report IsCriticalEvent()=true")
	}
}

// TestEventPump_TurnFailedFallbackIsCritical verifies that the pump's
// early-EOF fallback emits a typed, critical turn_failed event.
func TestEventPump_TurnFailedFallbackIsCritical(t *testing.T) {
	evt := lastPumpEventAsStream(t, runPumpFallback(t, context.Background(), "line\n"))

	failed, ok := evt.Event.(protocol.TurnFailedStreamEvent)
	if !ok {
		t.Fatalf("pump early-EOF fallback must carry protocol.TurnFailedStreamEvent, got %T", evt.Event)
	}
	if failed.Error == "" {
		t.Fatal("turn_failed fallback must carry a non-empty error message")
	}
	if !evt.IsCriticalEvent() {
		t.Fatal("pump turn_failed fallback must report IsCriticalEvent()=true")
	}
}

// TestApplyTerminalStreamState_PumpFallbackEventsApply verifies that the
// terminal events a real pump emits actually drive the agent lifecycle:
// turn_canceled → idle, turn_failed → error.
func TestApplyTerminalStreamState_PumpFallbackEventsApply(t *testing.T) {
	mgr := createTestManager(t)

	newRunningAgent := func(id string) *ManagedAgent {
		return &ManagedAgent{
			ID:          id,
			Provider:    "mock",
			Lifecycle:   protocol.AgentRunning,
			subscribers: make(map[uint64]AgentEventFunc),
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelEvt := lastPumpEventAsStream(t, runPumpFallback(t, ctx, "line\n"))
	cancelEvt.AgentID = "pump-cancel-agent"

	ag := newRunningAgent(cancelEvt.AgentID)
	if applied := mgr.applyTerminalStreamState(ag, cancelEvt); !applied {
		t.Fatal("applyTerminalStreamState must apply the pump's turn_canceled fallback (return true)")
	}
	if ag.Lifecycle != protocol.AgentIdle {
		t.Fatalf("turn_canceled fallback must set lifecycle idle, got %s", ag.Lifecycle)
	}

	failEvt := lastPumpEventAsStream(t, runPumpFallback(t, context.Background(), "line\n"))
	failEvt.AgentID = "pump-fail-agent"

	ag2 := newRunningAgent(failEvt.AgentID)
	if applied := mgr.applyTerminalStreamState(ag2, failEvt); !applied {
		t.Fatal("applyTerminalStreamState must apply the pump's turn_failed fallback (return true)")
	}
	if ag2.Lifecycle != protocol.AgentError {
		t.Fatalf("turn_failed fallback must set lifecycle error, got %s", ag2.Lifecycle)
	}
}
