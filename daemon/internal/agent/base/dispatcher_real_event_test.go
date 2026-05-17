package base_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

func TestChannelDispatcherTreatsAgentStreamEventValueAsCritical(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := base.NewChannelDispatcher(logger)
	defer dispatcher.Close()

	slowCh := dispatcher.Subscribe()
	for i := 0; i < 2560; i++ {
		dispatcher.Emit(map[string]interface{}{"type": "timeline"})
	}
	_ = slowCh

	evt := agent.AgentStreamEvent{
		Event: map[string]interface{}{
			"type": "turn_completed",
		},
		Timestamp: time.Now(),
	}

	start := time.Now()
	dispatcher.Emit(evt)
	elapsed := time.Since(start)

	if elapsed < 100*time.Millisecond {
		t.Fatalf("AgentStreamEvent value was treated as droppable traffic; Emit returned in %v", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("critical AgentStreamEvent send should use the bounded subscriber timeout, took %v", elapsed)
	}
}
