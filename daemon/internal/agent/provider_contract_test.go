package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// -----------------------------------------------------------------------
// Contract test helpers
// -----------------------------------------------------------------------

func drainEventsUntilTerminal(t *testing.T, ch <-chan AgentStreamEvent, timeout time.Duration) []AgentStreamEvent {
	t.Helper()
	var events []AgentStreamEvent
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, evt)
			if isTerminalStreamEvent(evt) {
				return events
			}
		case <-deadline:
			t.Fatalf("timed out after %v waiting for terminal event; received %d events: %v",
				timeout, len(events), eventTypesList(events))
		}
	}
}

func isTerminalStreamEvent(evt AgentStreamEvent) bool {
	switch evt.Event.(type) {
	case protocol.TurnCompletedStreamEvent,
		protocol.TurnFailedStreamEvent,
		protocol.TurnCanceledStreamEvent:
		return true
	}
	return false
}

func findUserMessageEvent(events []AgentStreamEvent) (protocol.TimelineStreamEvent, bool) {
	for _, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok {
			if e.Item.Type == "user_message" {
				return e, true
			}
		}
	}
	return protocol.TimelineStreamEvent{}, false
}

func findThreadStartedIndex(events []AgentStreamEvent) int {
	for i, evt := range events {
		switch evt.Event.(type) {
		case protocol.ThreadStartedStreamEvent:
			return i
		}
	}
	return -1
}

func findUserMessageIndex(events []AgentStreamEvent) int {
	for i, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok {
			if e.Item.Type == "user_message" {
				return i
			}
		}
	}
	return -1
}

func findTerminalIndex(events []AgentStreamEvent) int {
	for i, evt := range events {
		if isTerminalStreamEvent(evt) {
			return i
		}
	}
	return -1
}

func eventTypesList(events []AgentStreamEvent) []string {
	types := make([]string, len(events))
	for i, evt := range events {
		s := streamEventTypeString(evt)
		if s == "" {
			s = fmt.Sprintf("%T", evt.Event)
		}
		types[i] = s
	}
	return types
}

func timelineItemTypes(events []AgentStreamEvent) []string {
	var types []string
	for _, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok {
			types = append(types, e.Item.Type)
		}
	}
	return types
}

// -----------------------------------------------------------------------
// Contract suite
// -----------------------------------------------------------------------

const contractTestMessageID = "msg-contract-test-001"
const contractTestPrompt = "Reply with exactly one word: hello"

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
	client AgentClient,
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
		defer session.Close()

		ch := session.Subscribe()
		go func() {
			_, _ = session.Run(ctx, contractTestPrompt, nil, nil, contractTestMessageID)
		}()

		events := drainEventsUntilTerminal(t, ch, 2*time.Minute)

		userMsg, found := findUserMessageEvent(events)
		if !found {
			t.Fatalf("no user_message event in %d events: %v\nitem types: %v",
				len(events), eventTypesList(events), timelineItemTypes(events))
		}
		if userMsg.Item.MessageID != contractTestMessageID {
			t.Errorf("user_message MessageID: got %q, want %q",
				userMsg.Item.MessageID, contractTestMessageID)
		}
	})

	t.Run("turn_completed_emitted", func(t *testing.T) {
		session, err := client.CreateSession(ctx, config)
		if err != nil {
			t.Skipf("CreateSession failed: %v", err)
		}
		defer session.Close()

		ch := session.Subscribe()
		go func() {
			_, _ = session.Run(ctx, contractTestPrompt, nil, nil, contractTestMessageID)
		}()

		events := drainEventsUntilTerminal(t, ch, 2*time.Minute)

		terminalIdx := findTerminalIndex(events)
		if terminalIdx < 0 {
			t.Fatalf("no terminal event in %d events: %v", len(events), eventTypesList(events))
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
		defer session.Close()

		ch := session.Subscribe()
		go func() {
			_, _ = session.Run(ctx, contractTestPrompt, nil, nil, contractTestMessageID)
		}()

		events := drainEventsUntilTerminal(t, ch, 2*time.Minute)

		threadIdx := findThreadStartedIndex(events)
		userIdx := findUserMessageIndex(events)
		terminalIdx := findTerminalIndex(events)

		if threadIdx < 0 {
			t.Errorf("no ThreadStartedStreamEvent in events: %v", eventTypesList(events))
		}
		if userIdx < 0 {
			t.Errorf("no user_message event in events: %v", eventTypesList(events))
		}
		if terminalIdx < 0 {
			t.Errorf("no terminal event in events: %v", eventTypesList(events))
		}

		if threadIdx >= 0 && userIdx >= 0 && threadIdx > userIdx {
			t.Errorf("thread_started (idx %d) must come before user_message (idx %d)", threadIdx, userIdx)
		}
		if userIdx >= 0 && terminalIdx >= 0 && userIdx > terminalIdx {
			t.Errorf("user_message (idx %d) must come before terminal event (idx %d)", userIdx, terminalIdx)
		}
	})
}

// -----------------------------------------------------------------------
// Per-provider contract tests
// -----------------------------------------------------------------------

func TestMockProviderContract(t *testing.T) {
	client := NewMockAgentClient()
	RunProviderContractSuite(t, "mock", client)
}

func TestClaudeProviderContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Claude contract test in short mode")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClaudeAgentClient("", logger)
	RunProviderContractSuite(t, "claude", client)
}

func TestOpenCodeProviderContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OpenCode contract test in short mode")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewOpenCodeAgentClient("", logger)
	RunProviderContractSuite(t, "opencode", client)
}

func TestPiProviderContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Pi contract test in short mode")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewPiAgentClient("", logger)
	RunProviderContractSuite(t, "pi", client)
}
