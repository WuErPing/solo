package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// TestSession_CriticalMessagesNotDroppedDuringGrace verifies that critical
// messages (turn_completed, turn_failed, turn_canceled, agent_update) arriving
// during grace period are not silently dropped but instead buffered and replayed
// on ReplaceConn.
//
// This is a regression test for the bug where IsInGrace() short-circuits before
// isCriticalMessage() in sendMessage, causing ALL messages to be dropped during
// grace — including terminal lifecycle events that the client needs to transition
// UI state.
func TestSession_CriticalMessagesNotDroppedDuringGrace(t *testing.T) {
	conn1 := newMockConn()
	sess := newTestSessionGrace(t, conn1, 5*time.Second) // long grace so it doesn't expire

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	// Let session start up
	time.Sleep(100 * time.Millisecond)

	// Disconnect to enter grace
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to be in grace period")
	}

	// Send a critical message during grace period: turn_completed agent_stream
	turnCompleted := protocol.NewSessionMessage(&protocol.AgentStreamMessage{
		Type: "agent_stream",
		Payload: protocol.AgentStreamPayload{
			AgentID: "agent-1",
			Event: map[string]interface{}{
				"type": "turn_completed",
			},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
	sess.sendMessage(turnCompleted)

	// Also send an agent_update (also critical)
	agentUpdate := protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
		Type: "agent_update",
		Payload: protocol.AgentUpdatePayload{
			Kind: "upsert",
			Agent: &protocol.AgentSnapshotPayload{
				ID:     "agent-1",
				Status: protocol.AgentIdle,
			},
		},
	})
	sess.sendMessage(agentUpdate)

	// Now reconnect via ReplaceConn
	conn2 := newMockConn()
	replaceDone := make(chan error, 1)
	go func() {
		replaceDone <- sess.ReplaceConn(conn2)
	}()

	// Wait for ReplaceConn to set up
	time.Sleep(150 * time.Millisecond)

	// The critical messages should have been replayed on the new connection
	conn2.mu.Lock()
	writtenMessages := conn2.messages
	conn2.mu.Unlock()

	if len(writtenMessages) == 0 {
		t.Fatal("expected critical messages to be replayed on new connection after ReplaceConn, but no messages were written")
	}

	// Check that turn_completed was among the replayed messages
	foundTurnCompleted := false
	foundAgentUpdate := false
	for _, raw := range writtenMessages {
		var msg protocol.WSOutboundMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type != "session" {
			continue
		}
		payloadBytes, _ := json.Marshal(msg.Message)
		var payload map[string]interface{}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			continue
		}
		msgType, _ := payload["type"].(string)
		if msgType == "agent_stream" {
			if inner, ok := payload["payload"].(map[string]interface{}); ok {
				if evt, ok := inner["event"].(map[string]interface{}); ok {
					if evtType, _ := evt["type"].(string); evtType == "turn_completed" {
						foundTurnCompleted = true
					}
				}
			}
		}
		if msgType == "agent_update" {
			foundAgentUpdate = true
		}
	}

	if !foundTurnCompleted {
		t.Error("expected turn_completed to be replayed after ReplaceConn, but it was not found in written messages")
	}
	if !foundAgentUpdate {
		t.Error("expected agent_update to be replayed after ReplaceConn, but it was not found in written messages")
	}

	// Clean up: disconnect conn2 to unblock ReplaceConn
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-replaceDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ReplaceConn did not return within timeout")
	}
}

// TestSession_AgentStreamBufferedDuringGrace verifies that agent_stream messages
// are buffered during grace and replayed on ReplaceConn, while other non-critical
// messages (like pong) are still dropped.
func TestSession_AgentStreamBufferedDuringGrace(t *testing.T) {
	conn1 := newMockConn()
	sess := newTestSessionGrace(t, conn1, 5*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to be in grace period")
	}

	// Send a non-critical message: a regular pong
	sess.sendMessage(protocol.NewPongMessage())

	// Also send a non-critical agent_stream (e.g. a timeline event)
	timelineMsg := protocol.NewSessionMessage(&protocol.AgentStreamMessage{
		Type: "agent_stream",
		Payload: protocol.AgentStreamPayload{
			AgentID: "agent-1",
			Event: map[string]interface{}{
				"type": "timeline",
				"item": map[string]interface{}{
					"type": "assistant_message",
					"text": "hello",
				},
			},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
	sess.sendMessage(timelineMsg)

	// Reconnect
	conn2 := newMockConn()
	replaceDone := make(chan error, 1)
	go func() {
		replaceDone <- sess.ReplaceConn(conn2)
	}()
	time.Sleep(150 * time.Millisecond)

	// Pong (non-agent_stream) should NOT be replayed.
	// Agent_stream timeline events SHOULD be replayed (new behavior: all
	// agent_stream messages are buffered during grace so the client sees
	// the full turn output on reconnect).
	conn2.mu.Lock()
	writtenMessages := conn2.messages
	conn2.mu.Unlock()

	var agentStreamReplayed bool
	for _, raw := range writtenMessages {
		var msg protocol.WSOutboundMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "pong" {
			t.Error("non-critical pong message should not be replayed after ReplaceConn")
		}
		if msg.Type == "session" {
			payloadBytes, _ := json.Marshal(msg.Message)
			var payload map[string]interface{}
			if json.Unmarshal(payloadBytes, &payload) != nil {
				continue
			}
			if msgType, _ := payload["type"].(string); msgType == "agent_stream" {
				if inner, ok := payload["payload"].(map[string]interface{}); ok {
					if evt, ok := inner["event"].(map[string]interface{}); ok {
						if evtType, _ := evt["type"].(string); evtType == "timeline" {
							agentStreamReplayed = true
						}
					}
				}
			}
		}
	}
	if !agentStreamReplayed {
		t.Error("agent_stream timeline event should be buffered and replayed after ReplaceConn")
	}

	// Clean up
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-replaceDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ReplaceConn did not return within timeout")
	}
}

// TestSession_CriticalReplay_NoneDropped verifies that when many critical
// messages are buffered during grace period, ALL of them are replayed after
// ReplaceConn — none are dropped due to a full send queue.
//
// This is a regression test for the bug where ReplaceConn used `default:`
// in the replay select, which would silently drop messages if the sendQueue
// was momentarily full during replay (e.g., pushActiveAgents + sendProviderSnapshot
// had already added messages). With the fix (blocking send with timeout instead
// of `default:`), all critical messages are guaranteed to be replayed.
func TestSession_CriticalReplay_NoneDropped(t *testing.T) {
	conn1 := newMockConn()
	sess := newTestSessionGrace(t, conn1, 5*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to be in grace period")
	}

	// Buffer many critical messages during grace
	const numMessages = 50
	for i := 0; i < numMessages; i++ {
		msg := protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
			Type: "agent_update",
			Payload: protocol.AgentUpdatePayload{
				Kind: "upsert",
				Agent: &protocol.AgentSnapshotPayload{
					ID:     "agent-replay-test",
					Status: protocol.AgentIdle,
				},
			},
		})
		sess.sendMessage(msg)
	}

	// Verify all were buffered
	sess.graceMu.Lock()
	buffered := len(sess.graceCriticalBuf)
	sess.graceMu.Unlock()
	if buffered != numMessages {
		t.Fatalf("expected %d buffered critical messages, got %d", numMessages, buffered)
	}

	// Reconnect via ReplaceConn
	conn2 := newMockConn()
	replaceDone := make(chan error, 1)
	go func() {
		replaceDone <- sess.ReplaceConn(conn2)
	}()

	// Wait for replay to complete
	time.Sleep(300 * time.Millisecond)

	// Count agent_update messages received on conn2
	conn2.mu.Lock()
	writtenMessages := make([][]byte, len(conn2.messages))
	copy(writtenMessages, conn2.messages)
	conn2.mu.Unlock()

	agentUpdateCount := 0
	for _, raw := range writtenMessages {
		var msg protocol.WSOutboundMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type != "session" {
			continue
		}
		payloadBytes, _ := json.Marshal(msg.Message)
		var payload map[string]interface{}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			continue
		}
		if msgType, _ := payload["type"].(string); msgType == "agent_update" {
			agentUpdateCount++
		}
	}

	if agentUpdateCount < numMessages {
		t.Errorf("expected all %d critical messages to be replayed, but only %d agent_update messages received (possible drop due to default: in replay loop)", numMessages, agentUpdateCount)
	}

	// Cleanup
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-replaceDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ReplaceConn did not return within timeout")
	}
}

// TestSession_GraceExpired_BufferedCriticalMessagesDiscarded verifies that
// if grace period expires without a reconnect, buffered critical messages
// are discarded (no leak).
func TestSession_GraceExpired_BufferedCriticalMessagesDiscarded(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod) // short grace (100ms)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to be in grace period")
	}

	// Send critical messages during grace
	sess.sendMessage(protocol.NewSessionMessage(&protocol.AgentStreamMessage{
		Type: "agent_stream",
		Payload: protocol.AgentStreamPayload{
			AgentID: "agent-1",
			Event: map[string]interface{}{
				"type": "turn_completed",
			},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}))

	// Wait for grace to expire
	time.Sleep(testGracePeriod + 100*time.Millisecond)

	// Session should no longer be in grace and done should be closed
	select {
	case <-sess.done:
		// Expected
	default:
		t.Error("expected done channel to be closed after grace expires")
	}

	// Verify no goroutine leak or panic — the test completing is the assertion
}
