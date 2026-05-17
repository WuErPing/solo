package server

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestIsCriticalMessage_AgentUpdate(t *testing.T) {
	msg := protocol.WSOutboundMessage{
		Type:    "session",
		Message: &protocol.AgentUpdateMessage{Type: "agent_update"},
	}
	if !isCriticalMessage(msg) {
		t.Error("expected agent_update to be critical")
	}
}

func TestIsCriticalMessage_AgentStreamTerminal(t *testing.T) {
	msg := protocol.WSOutboundMessage{
		Type: "session",
		Message: &protocol.AgentStreamMessage{
			Type: "agent_stream",
			Payload: protocol.AgentStreamPayload{
				Event: map[string]interface{}{"type": "turn_completed"},
			},
		},
	}
	if !isCriticalMessage(msg) {
		t.Error("expected turn_completed to be critical")
	}

	msg.Message = &protocol.AgentStreamMessage{
		Type: "agent_stream",
		Payload: protocol.AgentStreamPayload{
			Event: map[string]interface{}{"type": "turn_failed"},
		},
	}
	if !isCriticalMessage(msg) {
		t.Error("expected turn_failed to be critical")
	}
}

func TestIsCriticalMessage_NonCritical(t *testing.T) {
	msg := protocol.WSOutboundMessage{
		Type:    "session",
		Message: &protocol.PongMessage{Type: "pong"},
	}
	if isCriticalMessage(msg) {
		t.Error("expected pong to not be critical")
	}

	msg = protocol.WSOutboundMessage{
		Type: "session",
		Message: &protocol.AgentStreamMessage{
			Type: "agent_stream",
			Payload: protocol.AgentStreamPayload{
				Event: map[string]interface{}{"type": "text_delta"},
			},
		},
	}
	if isCriticalMessage(msg) {
		t.Error("expected text_delta to not be critical")
	}
}

func TestIsCriticalMessage_NonSession(t *testing.T) {
	msg := protocol.WSOutboundMessage{Type: "binary"}
	if isCriticalMessage(msg) {
		t.Error("expected non-session to not be critical")
	}
}

func TestIsAgentStreamMessage(t *testing.T) {
	msg := protocol.WSOutboundMessage{
		Type:    "session",
		Message: &protocol.AgentStreamMessage{Type: "agent_stream"},
	}
	if !isAgentStreamMessage(msg) {
		t.Error("expected agent_stream message")
	}

	msg = protocol.WSOutboundMessage{
		Type:    "session",
		Message: &protocol.AgentUpdateMessage{Type: "agent_update"},
	}
	if isAgentStreamMessage(msg) {
		t.Error("expected agent_update to not be agent_stream")
	}
}
