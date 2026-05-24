package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_RealProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewPiAgentClient("", logger)

	if err := client.IsAvailable(context.Background()); err != nil {
		t.Skipf("pi not available: %v", err)
	}

	config := &protocol.AgentSessionConfig{
		Provider: "pi",
		Cwd:      "/tmp",
	}

	sess, err := client.CreateSession(context.Background(), config)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer sess.Close()

	ch := sess.Subscribe()
	done := make(chan struct{})
	var events []AgentStreamEvent
	go func() {
		for evt := range ch {
			events = append(events, evt)
			payload, ok := evt.Event.(map[string]interface{})
			if ok {
				t.Logf("event: %s", payload["type"])
			}
		}
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sess.Run(ctx, "hi", nil, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	t.Logf("result: sessionID=%s canceled=%v", result.SessionID, result.Canceled)

	// Give the subscriber goroutine time to drain remaining events.
	// The subscribe channel is not closed until sess.Close() is called.
	time.Sleep(200 * time.Millisecond)

	t.Logf("total events: %d", len(events))

	var hasThreadStarted, hasAssistantMessage, hasTurnCompleted bool
	for _, evt := range events {
		payload, ok := evt.Event.(map[string]interface{})
		if !ok {
			continue
		}
		switch payload["type"] {
		case "thread_started":
			hasThreadStarted = true
		case "timeline":
			if item, ok := payload["item"].(TimelineItem); ok && item.Type == "assistant_message" {
				hasAssistantMessage = true
			}
		case "turn_completed":
			hasTurnCompleted = true
		}
	}

	if !hasThreadStarted {
		t.Error("missing thread_started event")
	}
	if !hasAssistantMessage {
		t.Error("missing assistant_message event")
	}
	if !hasTurnCompleted {
		t.Error("missing turn_completed event")
	}
}
