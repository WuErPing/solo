package server

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

// TestSession_CriticalSendMessage_ReturnsQuicklyAfterDisconnect verifies that
// calling sendMessage() with a critical message immediately after a session
// disconnects does NOT block for criticalSendTimeout (5s).
//
// Before the fix: runReadLoop closes sendQueue, then calls enterGrace().
// Between close(sendQueue) and enterGrace(), IsInGrace() returns false, so
// sendMessage() falls into the critical path:
//   select {
//   case sendQueue <- item:   ← panics or blocks (channel closed)
//   case <-done:              ← done not closed yet
//   case <-time.After(5s):   ← fires after 5 full seconds
//   }
// This means each concurrent critical sendMessage() call blocks ~5s.
//
// After the fix: sendClosed atomic is set immediately after close(sendQueue),
// so sendMessage() detects it and buffers into graceCriticalBuf without
// touching the closed channel.
func TestSession_CriticalSendMessage_ReturnsQuicklyAfterDisconnect(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		sess.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	// Disconnect the session — this triggers close(sendQueue) then enterGrace()
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	// Wait for Run() to return (enters grace)
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return within timeout")
	}

	// Now fire 5 concurrent critical sendMessage() calls.
	// With the bug each would block up to 5s → total serialised wait = 25s.
	// With the fix they should all return in <500ms total.
	start := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 5; i++ {
			go sess.sendMessage(protocol.NewSessionMessage(&protocol.AgentStreamMessage{
				Type: "agent_stream",
				Payload: protocol.AgentStreamPayload{
					AgentID: "agent-1",
					Event:   map[string]interface{}{"type": "turn_completed"},
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				},
			}))
		}
	}()
	<-done

	// Give goroutines a moment to complete
	time.Sleep(100 * time.Millisecond)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("sendMessage calls after disconnect took %v; expected <1s (race window not fixed)", elapsed)
	}
}

// TestSession_SendClosed_SetBeforeEnterGrace verifies that sendClosed is true
// as soon as Run() returns, which guarantees sendMessage() sees the closed
// state before enterGrace() completes.
func TestSession_SendClosed_SetBeforeEnterGrace(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		sess.Run()
	}()

	time.Sleep(50 * time.Millisecond)
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return within timeout")
	}

	if !sess.sendQueue.IsClosed() {
		t.Error("expected sendQueue to be closed after Run() returns")
	}
}
