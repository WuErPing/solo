package base

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// collectPumpFallbackEvents runs an EventPump with a translator that never
// terminates, returning every dispatched event once the pump emits its own
// fallback terminal event and returns.
func collectPumpFallbackEvents(ctx context.Context, input string) []interface{} {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := NewCallbackDispatcher(logger)

	var mu sync.Mutex
	var events []interface{}
	dispatcher.Subscribe(func(evt interface{}) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})

	pump := NewEventPump(logger, dispatcher)
	pump.SetProvider("mock")
	_, _ = pump.RunBlocking(ctx, strings.NewReader(input), &pumpNoopTranslator{}, nil)

	mu.Lock()
	defer mu.Unlock()
	return append([]interface{}(nil), events...)
}

// TestEventPump_ContextCancelEmitsTypedTurnCanceled pins the fix for the
// historical defect where the pump emitted turn_canceled as a raw
// map[string]interface{}: provider sessions filter dispatcher events by the
// AgentStreamEvent envelope type, so the map never reached the manager — and
// even injected directly it was neither critical nor terminal there.
func TestEventPump_ContextCancelEmitsTypedTurnCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events := collectPumpFallbackEvents(ctx, "line\n")
	if len(events) == 0 {
		t.Fatal("pump emitted no events")
	}
	evt, ok := events[len(events)-1].(AgentStreamEvent)
	if !ok {
		t.Fatalf("turn_canceled fallback must be an AgentStreamEvent envelope, got %T", events[len(events)-1])
	}
	canceled, ok := evt.Event.(protocol.TurnCanceledStreamEvent)
	if !ok {
		t.Fatalf("envelope payload must be protocol.TurnCanceledStreamEvent, got %T", evt.Event)
	}
	if canceled.Provider != "mock" {
		t.Errorf("provider: got %q, want mock", canceled.Provider)
	}
	if canceled.Reason != "context_cancelled" {
		t.Errorf("reason: got %q, want context_cancelled", canceled.Reason)
	}
	if !evt.IsCriticalEvent() {
		t.Error("turn_canceled fallback must be critical so it is never dropped under backpressure")
	}
}

// TestEventPump_EarlyEOFEmitsTypedTurnFailed pins the same fix for the
// stream-ended-before-terminal fallback.
func TestEventPump_EarlyEOFEmitsTypedTurnFailed(t *testing.T) {
	events := collectPumpFallbackEvents(context.Background(), "line\n")
	if len(events) == 0 {
		t.Fatal("pump emitted no events")
	}
	evt, ok := events[len(events)-1].(AgentStreamEvent)
	if !ok {
		t.Fatalf("turn_failed fallback must be an AgentStreamEvent envelope, got %T", events[len(events)-1])
	}
	failed, ok := evt.Event.(protocol.TurnFailedStreamEvent)
	if !ok {
		t.Fatalf("envelope payload must be protocol.TurnFailedStreamEvent, got %T", evt.Event)
	}
	if failed.Provider != "mock" {
		t.Errorf("provider: got %q, want mock", failed.Provider)
	}
	if failed.Error == "" {
		t.Error("turn_failed fallback must carry a non-empty error message")
	}
	if !evt.IsCriticalEvent() {
		t.Error("turn_failed fallback must be critical so it is never dropped under backpressure")
	}
}
