package streamevents

import (
	"fmt"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// TerminalDetector recognizes the three standard terminal stream events wrapped
// in an agent.AgentStreamEvent. It replaces the near-identical per-provider
// detectors. SessionID is intentionally left empty: provider Run methods set it
// from their base session after RunBlocking returns.
type TerminalDetector struct{}

// IsTerminal implements base.TerminalEventDetector.
func (TerminalDetector) IsTerminal(evt interface{}) (*base.AgentRunResult, bool, error) {
	streamEvt, ok := evt.(agent.AgentStreamEvent)
	if !ok {
		return nil, false, nil
	}
	switch e := streamEvt.Event.(type) {
	case protocol.TurnCompletedStreamEvent:
		return &base.AgentRunResult{Usage: e.Usage}, true, nil
	case protocol.TurnFailedStreamEvent:
		return &base.AgentRunResult{}, true, fmt.Errorf("%s", e.Error)
	case protocol.TurnCanceledStreamEvent:
		return &base.AgentRunResult{Canceled: true}, true, nil
	}
	return nil, false, nil
}
