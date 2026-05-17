package server

// P2-1: Multi-socket architecture RED tests.
//
// These tests define the contract for the new AttachSocket-based model where
// Session continues running independently of any single WebSocket connection.
// All tests in this file MUST FAIL to compile until AttachSocket is implemented
// (RED phase of TDD).
//
// Design:
//   - Session is independent of any single WebSocket connection.
//   - AttachSocket(conn) attaches a new socket and blocks until it disconnects.
//   - Multiple sockets can be attached concurrently (broadcast model).
//   - When all sockets disconnect, a grace timer starts but the session survives.
//   - A new AttachSocket call during grace cancels the grace timer.
//   - On attach, current state (active agents + provider snapshot) is pushed.

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

// --- T1: attached socket receives messages ---

func TestSession_AttachSocket_ReceivesMessages(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	// Start session background loops without a connection.
	// AttachSocket starts its own read/write loops; we only need the
	// session's agent subscription and coalescer to be running.
	sess.startBackgroundLoops()

	conn2 := newMockConn()
	attachDone := make(chan struct{})
	go func() {
		defer close(attachDone)
		sess.AttachSocket(conn2)
	}()
	time.Sleep(50 * time.Millisecond)

	// Send a message — it should arrive on conn2.
	sess.sendMessage(protocol.NewPongMessage())
	time.Sleep(50 * time.Millisecond)

	conn2.mu.Lock()
	n := len(conn2.messages)
	conn2.mu.Unlock()
	if n == 0 {
		t.Error("expected message on attached socket, got none")
	}

	// Cleanup: disconnect conn2 to unblock AttachSocket goroutine.
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attachDone:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachSocket goroutine did not return within timeout")
	}
}

// --- T2: detached socket stops receiving messages ---

func TestSession_DetachSocket_StopsReceivingMessages(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	sess.startBackgroundLoops()

	conn2 := newMockConn()
	attachDone := make(chan struct{})
	go func() {
		defer close(attachDone)
		sess.AttachSocket(conn2)
	}()
	time.Sleep(50 * time.Millisecond)

	// Disconnect conn2 — it should be removed from the socket set.
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attachDone:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachSocket goroutine did not return within timeout")
	}

	// Record message count after detach.
	conn2.mu.Lock()
	before := len(conn2.messages)
	conn2.mu.Unlock()

	// Sending after detach should NOT deliver to the disconnected socket.
	sess.sendMessage(protocol.NewPongMessage())
	time.Sleep(50 * time.Millisecond)

	conn2.mu.Lock()
	after := len(conn2.messages)
	conn2.mu.Unlock()
	if after > before {
		t.Errorf("detached socket should not receive new messages (before=%d after=%d)", before, after)
	}
}

// --- T3: session survives single socket disconnect ---

func TestSession_SurvivesSocketDisconnect(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 500*time.Millisecond)
	sess.startBackgroundLoops()

	conn2 := newMockConn()
	go sess.AttachSocket(conn2)
	time.Sleep(50 * time.Millisecond)

	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	time.Sleep(50 * time.Millisecond)

	// Session.done must NOT be closed — grace period hasn't expired.
	select {
	case <-sess.done:
		t.Error("session.done closed after single socket disconnect; session should survive until grace expires")
	default:
		// Correct — session still alive.
	}
}

// --- T4: broadcast — both sockets receive the message via broadcastToSockets ---

func TestSession_BroadcastsToAllSockets(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	sess.startBackgroundLoops()

	conn1, conn2 := newMockConn(), newMockConn()

	go sess.AttachSocket(conn1)
	go sess.AttachSocket(conn2)
	// Wait for initial state push (sendProviderSnapshotTo) to land.
	time.Sleep(100 * time.Millisecond)

	// Snapshot message counts before broadcast.
	conn1.mu.Lock()
	before1 := len(conn1.messages)
	conn1.mu.Unlock()

	conn2.mu.Lock()
	before2 := len(conn2.messages)
	conn2.mu.Unlock()

	// Now send a message — must go through broadcastToSockets to reach both.
	sess.sendMessage(protocol.NewPongMessage())
	time.Sleep(50 * time.Millisecond)

	conn1.mu.Lock()
	after1 := len(conn1.messages)
	conn1.mu.Unlock()

	conn2.mu.Lock()
	after2 := len(conn2.messages)
	conn2.mu.Unlock()

	if after1 <= before1 {
		t.Errorf("conn1 did not receive broadcast message (before=%d after=%d)", before1, after1)
	}
	if after2 <= before2 {
		t.Errorf("conn2 did not receive broadcast message (before=%d after=%d)", before2, after2)
	}

	// Cleanup.
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	time.Sleep(50 * time.Millisecond)
}

// --- T5: grace starts when ALL sockets disconnect ---

func TestSession_EntersGraceWhenAllSocketsDisconnect(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 500*time.Millisecond)
	sess.startBackgroundLoops()

	conn2 := newMockConn()
	attachDone := make(chan struct{})
	go func() {
		defer close(attachDone)
		sess.AttachSocket(conn2)
	}()
	time.Sleep(50 * time.Millisecond)

	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attachDone:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachSocket goroutine did not return within timeout")
	}

	// After the last socket disconnects, grace should be in effect.
	time.Sleep(50 * time.Millisecond)
	if !sess.IsInGrace() {
		t.Error("expected session to be in grace period after last socket disconnects")
	}
}

func TestSession_AttachSocket_EntersGraceWhenSocketWritePumpIsBlocked(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	sess.startBackgroundLoops()

	blockedConn := newBlockingWriteConn()
	attachDone := make(chan struct{})
	go func() {
		defer close(attachDone)
		sess.AttachSocket(blockedConn)
	}()

	select {
	case <-blockedConn.writeStarted:
	case <-time.After(time.Second):
		blockedConn.Close()
		t.Fatal("attached socket writePump did not start blocked write")
	}

	blockedConn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case <-attachDone:
	case <-time.After(200 * time.Millisecond):
		blockedConn.Close()
		select {
		case <-attachDone:
		case <-time.After(time.Second):
		}
		t.Fatal("AttachSocket did not return while writePump was blocked; disconnect cleanup must enter grace without waiting indefinitely")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to enter grace after last blocked socket disconnects")
	}
}

// --- T9: AttachSocket grace replay does not stall with unbounded sendQueue ---
//
// With the unbounded sendQueue, Push() never blocks and never drops items.
// Grace replay enqueues all buffered items as fast as the CPU can go.
// This test verifies that AttachSocket processes a large grace buffer
// immediately, without the old criticalSendTimeout stall.
func TestSession_AttachSocket_GraceReplayDoesNotStallOnFullQueue(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	sess.startBackgroundLoops()

	// Enter grace.
	conn1 := newMockConn()
	attach1Done := make(chan struct{})
	go func() {
		defer close(attach1Done)
		sess.AttachSocket(conn1)
	}()
	time.Sleep(50 * time.Millisecond)
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attach1Done:
	case <-time.After(2 * time.Second):
		t.Fatal("first AttachSocket did not return")
	}
	if !sess.IsInGrace() {
		t.Fatal("expected grace after first socket disconnect")
	}

	// Fill graceCriticalBuf with 400 items. The unbounded sendQueue accepts
	// all of them without blocking or dropping.
	const total = 400
	for i := 0; i < total; i++ {
		data, _ := json.Marshal(protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
			Type: "agent_update",
			Payload: protocol.AgentUpdatePayload{Kind: "upsert", Agent: &protocol.AgentSnapshotPayload{
				ID: fmt.Sprintf("agent-%d", i), Status: protocol.AgentIdle,
			}},
		}))
		sess.graceMu.Lock()
		sess.graceCriticalBuf = append(sess.graceCriticalBuf, sendQueueItem{msgType: websocket.TextMessage, data: data})
		sess.graceMu.Unlock()
	}

	blockConn := newBlockingWriteConn()

	// Grace replay with unbounded queue: Push() never blocks, all items
	// enqueue immediately. Grace critical buffer must clear within 200ms.
	start := time.Now()
	attach2Done := make(chan struct{})
	go func() {
		defer close(attach2Done)
		sess.AttachSocket(blockConn)
	}()

	deadline := time.After(200 * time.Millisecond)
	for {
		sess.graceMu.Lock()
		remaining := len(sess.graceCriticalBuf)
		sess.graceMu.Unlock()
		if remaining == 0 {
			break
		}
		select {
		case <-deadline:
			elapsed := time.Since(start)
			t.Fatalf(
				"grace replay stalled: %d/%d items remain after %v — Push should never block",
				remaining, total, elapsed.Round(time.Millisecond),
			)
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}

	// Clean up.
	blockConn.Close()
	blockConn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attach2Done:
	case <-time.After(3 * time.Second):
		t.Fatal("second AttachSocket goroutine did not return")
	}
}


// --- T6: AttachSocket during grace cancels the grace timer ---

func TestSession_AttachSocket_CancelsGrace(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 500*time.Millisecond)
	sess.startBackgroundLoops()

	conn1 := newMockConn()
	attach1Done := make(chan struct{})
	go func() {
		defer close(attach1Done)
		sess.AttachSocket(conn1)
	}()
	time.Sleep(50 * time.Millisecond)

	// Disconnect first socket → enters grace.
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attach1Done:
	case <-time.After(2 * time.Second):
		t.Fatal("first AttachSocket goroutine did not return within timeout")
	}
	time.Sleep(50 * time.Millisecond)

	if !sess.IsInGrace() {
		t.Fatal("expected grace after first socket disconnect")
	}

	// Attach a second socket before grace expires.
	conn2 := newMockConn()
	attach2Done := make(chan struct{})
	go func() {
		defer close(attach2Done)
		sess.AttachSocket(conn2)
	}()
	time.Sleep(50 * time.Millisecond)

	// Grace should be cancelled now.
	if sess.IsInGrace() {
		t.Error("grace should be cancelled when a new socket attaches")
	}

	// Session.done must still be open.
	select {
	case <-sess.done:
		t.Error("session.done should not be closed after new socket attaches during grace")
	default:
	}

	// Cleanup.
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attach2Done:
	case <-time.After(2 * time.Second):
		t.Fatal("second AttachSocket goroutine did not return within timeout")
	}
}

// --- T7: AttachSocket pushes current state immediately ---

func TestSession_AttachSocket_PushesCurrentState(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	sess.startBackgroundLoops()

	conn2 := newMockConn()
	go sess.AttachSocket(conn2)

	// Give time for the initial state push (pushActiveAgents + sendProviderSnapshot).
	time.Sleep(150 * time.Millisecond)

	conn2.mu.Lock()
	n := len(conn2.messages)
	conn2.mu.Unlock()

	// At minimum the provider_snapshot message should arrive.
	if n == 0 {
		t.Error("expected initial state push (provider_snapshot) when AttachSocket is called, got no messages")
	}

	// Cleanup.
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	time.Sleep(50 * time.Millisecond)
}

// --- T8: AttachSocket replays graceCriticalBuf on reconnect ---
//
// When a session enters grace period (all sockets disconnected) and critical
// messages (turn_completed, agent_update) are buffered, a new AttachSocket
// call must replay those buffered messages to the new socket — just like
// ReplaceConn does for the legacy single-socket path.
//
// This is a regression test for the bug where AttachSocket pushed
// pushActiveAgentsTo + sendProviderSnapshotTo but never replayed
// graceCriticalBuf, causing relay clients to miss turn_completed events
// and get stuck in "waiting for turn completion" state.

func TestSession_AttachSocket_ReplaysGraceCriticalBuffer(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)
	sess.startBackgroundLoops()

	// Attach a socket, then disconnect it to enter grace.
	conn1 := newMockConn()
	attach1Done := make(chan struct{})
	go func() {
		defer close(attach1Done)
		sess.AttachSocket(conn1)
	}()
	time.Sleep(50 * time.Millisecond)

	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attach1Done:
	case <-time.After(2 * time.Second):
		t.Fatal("first AttachSocket goroutine did not return within timeout")
	}
	time.Sleep(50 * time.Millisecond)

	if !sess.IsInGrace() {
		t.Fatal("expected grace after first socket disconnect")
	}

	// Buffer critical messages during grace period.
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

	// Verify messages were buffered
	sess.graceMu.Lock()
	buffered := len(sess.graceCriticalBuf)
	sess.graceMu.Unlock()
	if buffered != 2 {
		t.Fatalf("expected 2 buffered critical messages, got %d", buffered)
	}

	// Attach a new socket — should replay buffered critical messages.
	conn2 := newMockConn()
	attach2Done := make(chan struct{})
	go func() {
		defer close(attach2Done)
		sess.AttachSocket(conn2)
	}()
	time.Sleep(150 * time.Millisecond)

	// Verify critical messages were replayed to conn2.
	conn2.mu.Lock()
	writtenMessages := make([][]byte, len(conn2.messages))
	copy(writtenMessages, conn2.messages)
	conn2.mu.Unlock()

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
		t.Error("expected turn_completed to be replayed on AttachSocket, but it was not found")
	}
	if !foundAgentUpdate {
		t.Error("expected agent_update to be replayed on AttachSocket, but it was not found")
	}

	// Verify graceCriticalBuf was cleared
	sess.graceMu.Lock()
	remaining := len(sess.graceCriticalBuf)
	sess.graceMu.Unlock()
	if remaining != 0 {
		t.Errorf("expected graceCriticalBuf to be cleared after replay, got %d remaining", remaining)
	}

	// Cleanup.
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attach2Done:
	case <-time.After(2 * time.Second):
		t.Fatal("second AttachSocket goroutine did not return within timeout")
	}
}
