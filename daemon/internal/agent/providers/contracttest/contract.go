// Package contracttest provides shared contract test helpers for agent providers.
package contracttest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

const (
	ContractTestMessageID = "msg-contract-test-001"
	ContractTestPrompt    = "Reply with exactly one word: hello"
)

func DrainEventsUntilTerminal(t *testing.T, ch <-chan agent.AgentStreamEvent, timeout time.Duration) []agent.AgentStreamEvent {
	t.Helper()
	var events []agent.AgentStreamEvent
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, evt)
			if IsTerminalStreamEvent(evt) {
				return events
			}
		case <-deadline:
			t.Fatalf("timed out after %v waiting for terminal event; received %d events: %v",
				timeout, len(events), EventTypesList(events))
		}
	}
}

func IsTerminalStreamEvent(evt agent.AgentStreamEvent) bool {
	switch evt.Event.(type) {
	case protocol.TurnCompletedStreamEvent,
		protocol.TurnFailedStreamEvent,
		protocol.TurnCanceledStreamEvent:
		return true
	}
	return false
}

func FindUserMessageEvent(events []agent.AgentStreamEvent) (protocol.TimelineStreamEvent, bool) {
	for _, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok {
			if e.Item.Type == "user_message" {
				return e, true
			}
		}
	}
	return protocol.TimelineStreamEvent{}, false
}

func FindThreadStartedIndex(events []agent.AgentStreamEvent) int {
	for i, evt := range events {
		switch evt.Event.(type) {
		case protocol.ThreadStartedStreamEvent:
			return i
		}
	}
	return -1
}

func FindUserMessageIndex(events []agent.AgentStreamEvent) int {
	for i, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok {
			if e.Item.Type == "user_message" {
				return i
			}
		}
	}
	return -1
}

func FindTerminalIndex(events []agent.AgentStreamEvent) int {
	for i, evt := range events {
		if IsTerminalStreamEvent(evt) {
			return i
		}
	}
	return -1
}

func EventTypesList(events []agent.AgentStreamEvent) []string {
	types := make([]string, len(events))
	for i, evt := range events {
		s := streamEventTypeString(evt.Event)
		if s == "" {
			s = fmt.Sprintf("%T", evt.Event)
		}
		types[i] = s
	}
	return types
}

func TimelineItemTypes(events []agent.AgentStreamEvent) []string {
	var types []string
	for _, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok {
			types = append(types, e.Item.Type)
		}
	}
	return types
}

func streamEventTypeString(event interface{}) string {
	switch e := event.(type) {
	case protocol.StreamEvent:
		return e.StreamEventType()
	case map[string]interface{}:
		if t, ok := e["type"].(string); ok {
			return t
		}
	}
	return ""
}

// RunProviderContractSuite verifies that a provider follows the turn lifecycle
// contract:
//
//  1. Emits ThreadStartedStreamEvent
//  2. Emits user_message with MessageID matching the Run messageID parameter
//  3. Emits TurnCompletedStreamEvent as the terminal event
//  4. Event ordering: thread_started → user_message → terminal
func RunProviderContractSuite(
	t *testing.T,
	providerName string,
	client agent.AgentClient,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := client.IsAvailable(ctx); err != nil {
		t.Skipf("%s not available: %v", providerName, err)
	}

	cwd := t.TempDir()
	config := &protocol.AgentSessionConfig{
		Provider: providerName,
		Cwd:      cwd,
	}

	t.Run("user_message_has_MessageID", func(t *testing.T) {
		session, err := client.CreateSession(ctx, config)
		if err != nil {
			t.Skipf("CreateSession failed: %v", err)
		}
		defer func() { _ = session.Close() }()

		ch := session.Subscribe()
		go func() {
			_, _ = session.Run(ctx, ContractTestPrompt, nil, nil, ContractTestMessageID)
		}()

		events := DrainEventsUntilTerminal(t, ch, 2*time.Minute)

		userMsg, found := FindUserMessageEvent(events)
		if !found {
			t.Fatalf("no user_message event in %d events: %v\nitem types: %v",
				len(events), EventTypesList(events), TimelineItemTypes(events))
		}
		if userMsg.Item.MessageID != ContractTestMessageID {
			t.Errorf("user_message MessageID: got %q, want %q",
				userMsg.Item.MessageID, ContractTestMessageID)
		}
	})

	t.Run("turn_completed_emitted", func(t *testing.T) {
		session, err := client.CreateSession(ctx, config)
		if err != nil {
			t.Skipf("CreateSession failed: %v", err)
		}
		defer func() { _ = session.Close() }()

		ch := session.Subscribe()
		go func() {
			_, _ = session.Run(ctx, ContractTestPrompt, nil, nil, ContractTestMessageID)
		}()

		events := DrainEventsUntilTerminal(t, ch, 2*time.Minute)

		terminalIdx := FindTerminalIndex(events)
		if terminalIdx < 0 {
			t.Fatalf("no terminal event in %d events: %v", len(events), EventTypesList(events))
		}
		switch evt := events[terminalIdx].Event.(type) {
		case protocol.TurnCompletedStreamEvent:
			// pass
		case protocol.TurnFailedStreamEvent:
			t.Skipf("turn failed (provider error, not contract violation): %s", evt.Error)
		case protocol.TurnCanceledStreamEvent:
			t.Skip("turn canceled")
		default:
			t.Errorf("unexpected terminal event type: %T", evt)
		}
	})

	t.Run("event_ordering", func(t *testing.T) {
		session, err := client.CreateSession(ctx, config)
		if err != nil {
			t.Skipf("CreateSession failed: %v", err)
		}
		defer func() { _ = session.Close() }()

		ch := session.Subscribe()
		go func() {
			_, _ = session.Run(ctx, ContractTestPrompt, nil, nil, ContractTestMessageID)
		}()

		events := DrainEventsUntilTerminal(t, ch, 2*time.Minute)

		threadIdx := FindThreadStartedIndex(events)
		userIdx := FindUserMessageIndex(events)
		terminalIdx := FindTerminalIndex(events)

		if threadIdx < 0 {
			t.Errorf("no ThreadStartedStreamEvent in events: %v", EventTypesList(events))
		}
		if userIdx < 0 {
			t.Errorf("no user_message event in events: %v", EventTypesList(events))
		}
		if terminalIdx < 0 {
			t.Errorf("no terminal event in events: %v", EventTypesList(events))
		}

		if threadIdx >= 0 && userIdx >= 0 && threadIdx > userIdx {
			t.Errorf("thread_started (idx %d) must come before user_message (idx %d)", threadIdx, userIdx)
		}
		if userIdx >= 0 && terminalIdx >= 0 && userIdx > terminalIdx {
			t.Errorf("user_message (idx %d) must come before terminal event (idx %d)", userIdx, terminalIdx)
		}
	})
}
