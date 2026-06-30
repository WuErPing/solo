package streamevents_test

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/streamevents"
	"github.com/WuErPing/solo/protocol"
)

// fakeTranslator is a minimal provider translator that relies solely on the
// shared builder. It demonstrates the dedup goal: adding a provider only
// requires parsing the wire format and calling builder methods — no
// envelope construction and no bespoke terminal detector.
type fakeTranslator struct {
	provider string
}

func (ft fakeTranslator) Translate(line string, ts time.Time) ([]interface{}, bool) {
	b := streamevents.New(ft.provider, ts)
	switch line {
	case "start":
		b.ThreadStarted("sess").UserMessage("hi", "m1")
	case "text":
		b.AssistantMessage("hello")
	case "think":
		b.Reasoning("hmm")
	case "tool":
		b.ToolCall("c1", "bash", nil, "running")
	case "done":
		b.TurnCompleted(nil)
	}
	return b.Events(), b.Terminal()
}

// TestProviderEnvelopeContract proves the invariant every refactored provider
// now relies on: a translator that uses the builder always emits
// agent.AgentStreamEvent envelopes that are provider-stamped and timestamped.
func TestProviderEnvelopeContract(t *testing.T) {
	const provider = "acme"
	ts := time.Unix(1700001234, 0).UTC()
	ft := fakeTranslator{provider: provider}

	var all []interface{}
	terminalSeen := false
	for _, line := range []string{"start", "text", "think", "tool", "done"} {
		events, terminal := ft.Translate(line, ts)
		all = append(all, events...)
		if terminal {
			terminalSeen = true
		}
	}

	if !terminalSeen {
		t.Fatal("expected a terminal batch")
	}
	if len(all) == 0 {
		t.Fatal("expected events")
	}

	for i, evt := range all {
		se, ok := evt.(agent.AgentStreamEvent)
		if !ok {
			t.Fatalf("event %d is %T, want agent.AgentStreamEvent", i, evt)
		}
		if !se.Timestamp.Equal(ts) {
			t.Errorf("event %d timestamp = %v, want %v", i, se.Timestamp, ts)
		}
		if got := providerOf(se.Event); got != "" && got != provider {
			t.Errorf("event %d provider = %q, want %q", i, got, provider)
		}
	}
}

// TestBuilderFeedsSharedDetector proves the other half of the contract: the
// terminal events a builder emits are recognized by the shared detector, so a
// provider gets correct terminal detection for free.
func TestBuilderFeedsSharedDetector(t *testing.T) {
	det := streamevents.TerminalDetector{}
	ft := fakeTranslator{provider: "acme"}

	events, terminal := ft.Translate("done", time.Now())
	if !terminal {
		t.Fatal("expected terminal batch")
	}

	var result *base.AgentRunResult
	found := false
	for _, evt := range events {
		if r, isTerm, _ := det.IsTerminal(evt); isTerm {
			result = r
			found = true
		}
	}
	if !found || result == nil {
		t.Fatalf("shared detector did not recognize builder terminal event in %v", events)
	}
}

// providerOf extracts the Provider field from the stream-event payloads that
// carry one. Payloads without a provider (e.g. flush signals) return "".
func providerOf(event interface{}) string {
	switch e := event.(type) {
	case protocol.ThreadStartedStreamEvent:
		return e.Provider
	case protocol.TimelineStreamEvent:
		return e.Provider
	case protocol.UsageUpdatedStreamEvent:
		return e.Provider
	case protocol.PermissionRequestedStreamEvent:
		return e.Provider
	case protocol.TurnCompletedStreamEvent:
		return e.Provider
	case protocol.TurnFailedStreamEvent:
		return e.Provider
	case protocol.TurnCanceledStreamEvent:
		return e.Provider
	}
	return ""
}
