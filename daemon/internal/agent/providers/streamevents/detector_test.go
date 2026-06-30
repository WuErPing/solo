package streamevents_test

import (
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/streamevents"
	"github.com/WuErPing/solo/protocol"
)

func TestTerminalDetector(t *testing.T) {
	d := streamevents.TerminalDetector{}
	tok := 3.0
	usage := &protocol.AgentUsage{InputTokens: &tok}

	t.Run("turn_completed", func(t *testing.T) {
		res, isTerm, err := d.IsTerminal(agent.AgentStreamEvent{Event: protocol.TurnCompletedStreamEvent{Usage: usage}})
		if !isTerm || err != nil {
			t.Fatalf("got isTerm=%v err=%v, want true/nil", isTerm, err)
		}
		if res == nil || res.Usage != usage {
			t.Fatalf("result usage = %#v, want %#v", res, usage)
		}
		if res.Canceled {
			t.Errorf("turn_completed must not be canceled")
		}
	})

	t.Run("turn_failed", func(t *testing.T) {
		res, isTerm, err := d.IsTerminal(agent.AgentStreamEvent{Event: protocol.TurnFailedStreamEvent{Error: "boom"}})
		if !isTerm {
			t.Fatalf("expected terminal")
		}
		if err == nil {
			t.Fatalf("expected error for turn_failed")
		}
		if res == nil {
			t.Fatalf("expected non-nil result")
		}
	})

	t.Run("turn_canceled", func(t *testing.T) {
		res, isTerm, err := d.IsTerminal(agent.AgentStreamEvent{Event: protocol.TurnCanceledStreamEvent{Reason: "stop"}})
		if !isTerm || err != nil {
			t.Fatalf("got isTerm=%v err=%v, want true/nil", isTerm, err)
		}
		if res == nil || !res.Canceled {
			t.Fatalf("expected canceled result, got %#v", res)
		}
	})

	t.Run("non-terminal timeline event", func(t *testing.T) {
		res, isTerm, err := d.IsTerminal(agent.AgentStreamEvent{Event: protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "assistant_message"}}})
		if isTerm || res != nil || err != nil {
			t.Fatalf("got (%#v, %v, %v), want (nil,false,nil)", res, isTerm, err)
		}
	})

	t.Run("non AgentStreamEvent", func(t *testing.T) {
		res, isTerm, err := d.IsTerminal(protocol.TurnCompletedStreamEvent{})
		if isTerm || res != nil || err != nil {
			t.Fatalf("got (%#v, %v, %v), want (nil,false,nil)", res, isTerm, err)
		}
	})

	// Ensure it satisfies the base interface.
	var _ base.TerminalEventDetector = streamevents.TerminalDetector{}
}
