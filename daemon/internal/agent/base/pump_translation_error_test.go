package base

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// pumpFailOnceTranslator fails translation on lines containing "garbage" and
// emits a terminal turn_completed on lines containing "done".
type pumpFailOnceTranslator struct{}

func (tr *pumpFailOnceTranslator) Translate(raw []byte, now time.Time) ([]interface{}, bool, error) {
	line := string(raw)
	if strings.Contains(line, "garbage") {
		return nil, false, errors.New("cannot parse provider line")
	}
	if strings.Contains(line, "done") {
		evt := AgentStreamEvent{
			Event:     protocol.TurnCompletedStreamEvent{Provider: "mock"},
			Timestamp: now,
		}
		return []interface{}{evt}, true, nil
	}
	return nil, false, nil
}

// pumpTurnCompletedDetector mirrors the real streamevents.TerminalDetector
// contract for turn_completed (the base package cannot import streamevents).
type pumpTurnCompletedDetector struct{}

func (pumpTurnCompletedDetector) IsTerminal(evt interface{}) (*AgentRunResult, bool, error) {
	if env, ok := evt.(AgentStreamEvent); ok {
		if _, ok := env.Event.(protocol.TurnCompletedStreamEvent); ok {
			return &AgentRunResult{}, true, nil
		}
	}
	return nil, false, nil
}

// TestEventPump_TranslationErrorEmitsTypedTimelineError pins the fix for the
// historical defect where the pump emitted translation failures as a raw
// map[string]interface{}: provider sessions filter dispatcher events by the
// AgentStreamEvent envelope type, so the diagnostic was silently dropped and
// never reached any client. The diagnostic is a non-terminal timeline "error"
// item (same convention as kimi's step-retry error items), and the pump must
// keep going — the turn still terminates normally afterwards.
func TestEventPump_TranslationErrorEmitsTypedTimelineError(t *testing.T) {
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
	if _, err := pump.RunBlocking(context.Background(), strings.NewReader("garbage\ndone\n"), &pumpFailOnceTranslator{}, pumpTurnCompletedDetector{}); err != nil {
		t.Fatalf("RunBlocking: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Fatalf("expected exactly the error diagnostic and the terminal event, got %d events", len(events))
	}

	evt, ok := events[0].(AgentStreamEvent)
	if !ok {
		t.Fatalf("translation-failure diagnostic must be an AgentStreamEvent envelope, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("envelope payload must be protocol.TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Provider != "mock" {
		t.Errorf("provider: got %q, want mock", timeline.Provider)
	}
	if timeline.Item.Type != "error" {
		t.Errorf("timeline item type: got %q, want error", timeline.Item.Type)
	}
	if !strings.Contains(timeline.Item.Message, "cannot parse provider line") {
		t.Errorf("timeline item message must carry the translation error, got %q", timeline.Item.Message)
	}
	if evt.IsCriticalEvent() {
		t.Error("translation-failure diagnostic is non-terminal and must not be marked critical")
	}

	// The pump must continue past the failed line and still reach the terminal
	// event from the subsequent line.
	term, ok := events[1].(AgentStreamEvent)
	if !ok {
		t.Fatalf("terminal event must be an AgentStreamEvent envelope, got %T", events[1])
	}
	if _, ok := term.Event.(protocol.TurnCompletedStreamEvent); !ok {
		t.Errorf("terminal payload: got %T, want protocol.TurnCompletedStreamEvent", term.Event)
	}
}
