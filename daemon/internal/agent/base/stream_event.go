package base

import (
	"time"

	"github.com/WuErPing/solo/protocol"
)

// AgentStreamEvent wraps a protocol stream event with metadata.
//
// The type lives in the base package (not the parent agent package) so that
// EventPump can emit envelope-wrapped typed terminal events without an
// import cycle — the agent package already imports base. The agent package
// re-exports it as a type alias, so agent.AgentStreamEvent and
// base.AgentStreamEvent are the identical type.
type AgentStreamEvent struct {
	AgentID   string
	Event     interface{} // one of the protocol.*StreamEvent payload variants
	Timestamp time.Time
}

// IsCriticalEvent returns true for terminal stream events that must never be dropped.
func (e AgentStreamEvent) IsCriticalEvent() bool {
	switch e.Event.(type) {
	case protocol.TurnCompletedStreamEvent,
		protocol.TurnFailedStreamEvent,
		protocol.TurnCanceledStreamEvent:
		return true
	}
	return false
}

// IsSemiCriticalEvent returns true for reasoning/thinking timeline events that
// should survive transient backpressure with a short blocking timeout rather
// than being silently dropped.
func (e AgentStreamEvent) IsSemiCriticalEvent() bool {
	switch evt := e.Event.(type) {
	case protocol.TimelineStreamEvent:
		return evt.Item.Type == "reasoning"
	}
	return false
}
