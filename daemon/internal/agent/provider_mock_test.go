package agent

import (
	"context"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// TestMockSessionRunEmitsStreamEventTypes guards the event contract of the
// mock session: downstream consumers (manager, coalescer tests) rely on this
// exact sequence of stream event types.
func TestMockSessionRunEmitsStreamEventTypes(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{Provider: "mock"})
	mockSess := sess.(*MockAgentSession)

	done := make(chan struct{})
	var events []AgentStreamEvent
	go func() {
		defer close(done)
		for ev := range mockSess.events {
			events = append(events, ev)
		}
	}()

	result, err := mockSess.Run(context.Background(), "hi", nil, nil, "msg-1")
	mockSess.Close()
	<-done

	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.FinalText == "" {
		t.Error("expected non-empty FinalText")
	}

	var types []string
	for _, ev := range events {
		se, ok := ev.Event.(protocol.StreamEvent)
		if !ok {
			t.Errorf("event[%d] is not a protocol.StreamEvent, got %T", len(types), ev.Event)
			continue
		}
		types = append(types, se.StreamEventType())
	}
	want := []string{"thread_started", "timeline", "timeline", "turn_completed"}
	if len(types) != len(want) {
		t.Fatalf("event types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("event[%d] type = %q, want %q", i, types[i], want[i])
		}
	}
}
