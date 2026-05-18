package server

import (
	"encoding/json"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// --- Send helpers ---

func (s *Session) sendPong() {
	s.sendMessage(protocol.NewPongMessage())
}

// isCriticalMessage returns true for messages that must never be dropped:
// - agent_update (lifecycle state changes like idle/error after turn completes)
// - agent_stream with terminal events (turn_completed, turn_failed, turn_canceled)
func isCriticalMessage(msg protocol.WSOutboundMessage) bool {
	if msg.Type != "session" {
		return false
	}
	sessionMsg, ok := msg.Message.(protocol.SessionOutboundMessage)
	if !ok {
		return false
	}
	switch sessionMsg.MsgType() {
	case "agent_update":
		return true
	case "agent_stream":
		if m, ok := sessionMsg.(*protocol.AgentStreamMessage); ok {
			if evt, ok := m.Payload.Event.(map[string]interface{}); ok {
				if t, ok := evt["type"].(string); ok {
					return t == "turn_completed" || t == "turn_failed" || t == "turn_canceled"
				}
			}
		}
	}
	return false
}

// isAgentStreamMessage returns true for agent_stream messages that should be
// buffered during grace period (e.g., timeline events with assistant text or
// tool call results). This ensures the client sees the complete turn output
// even when it reconnects mid-turn.
func isAgentStreamMessage(msg protocol.WSOutboundMessage) bool {
	if msg.Type != "session" {
		return false
	}
	sessionMsg, ok := msg.Message.(protocol.SessionOutboundMessage)
	if !ok {
		return false
	}
	return sessionMsg.MsgType() == "agent_stream"
}

func (s *Session) sendMessage(msg protocol.WSOutboundMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("cannot marshal message", "error", err)
		return
	}
	item := sendQueueItem{msgType: websocket.TextMessage, data: data}
	critical := isCriticalMessage(msg)

	// Multi-socket path: if any sockets are attached via AttachSocket, broadcast
	// to all of them. This is the new model where the session is independent of
	// any single connection.
	s.socketsMu.RLock()
	hasMultiSockets := len(s.sockets) > 0
	s.socketsMu.RUnlock()

	if hasMultiSockets {
		s.broadcastToSockets(item)
		return
	}

	// Legacy single-socket path (used by Run() / ReplaceConn()):
	// During grace period (or after sendQueue has been closed but before enterGrace()
	// is called), no WebSocket is connected.
	// Buffer critical messages AND all agent_stream messages for replay on
	// ReplaceConn. Agent_stream timeline events (assistant text, tool calls)
	// are essential for the client to see the complete turn result on reconnect.
	if s.sendQueue == nil || s.sendQueue.IsClosed() || s.IsInGrace() {
		if critical || isAgentStreamMessage(msg) {
			s.graceMu.Lock()
			s.graceCriticalBuf = append(s.graceCriticalBuf, item)
			s.graceMu.Unlock()
		}
		return
	}
	s.sendQueue.Push(item)
}

func (s *Session) sendRPCError(requestID, requestType, errMsg string, code *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.RpcErrorMessage{
		Type: "rpc_error",
		Payload: protocol.RpcErrorPayload{
			RequestID:   requestID,
			RequestType: &requestType,
			Error:       errMsg,
			Code:        code,
		},
	}))
}

func (s *Session) SendBinaryFrame(frame protocol.TerminalStreamFrame) {
	data := protocol.EncodeTerminalFrame(frame)
	item := sendQueueItem{msgType: websocket.BinaryMessage, data: data}

	// Multi-socket path: if any sockets are attached via AttachSocket, broadcast
	// to all of them.
	s.socketsMu.RLock()
	hasMultiSockets := len(s.sockets) > 0
	s.socketsMu.RUnlock()

	if hasMultiSockets {
		s.broadcastToSockets(item)
		return
	}

	// Legacy single-socket path (used by Run() / ReplaceConn()):
	// During grace period (or after sendQueue has been closed), no WebSocket is connected — drop silently.
	if s.sendQueue == nil || s.sendQueue.IsClosed() || s.IsInGrace() {
		return
	}
	s.sendQueue.Push(item)
}
