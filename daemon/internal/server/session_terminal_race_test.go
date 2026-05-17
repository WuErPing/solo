package server

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

// TestSession_SendBinaryFrame_AfterAttachSocket_DoesNotPanic verifies that
// SendBinaryFrame works correctly after a session is resumed via AttachSocket
// following a legacy Run() disconnect.
//
// This is a regression test for the bug where AttachSocket recreated inboundQueue
// and processDone but left the legacy sendQueue closed, causing SendBinaryFrame
// to panic when terminal output arrived.
func TestSession_SendBinaryFrame_AfterAttachSocket_DoesNotPanic(t *testing.T) {
	conn1 := newMockConn()
	sess := newTestSessionGrace(t, conn1, 5*time.Second)

	// Start session via legacy Run()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		sess.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	// Disconnect — this closes sendQueue and enters grace
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return within timeout")
	}

	// Verify session is in grace
	if !sess.IsInGrace() {
		t.Fatal("expected session to be in grace")
	}

	// Attach a new socket — this should recreate sendQueue
	conn2 := newMockConn()
	attachDone := make(chan struct{})
	go func() {
		defer close(attachDone)
		sess.AttachSocket(conn2)
	}()

	time.Sleep(50 * time.Millisecond)

	// SendBinaryFrame must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SendBinaryFrame panicked after AttachSocket: %v", r)
		}
	}()

	sess.SendBinaryFrame(protocol.TerminalStreamFrame{
		Opcode:  protocol.TerminalOutput,
		Slot:    1,
		Payload: []byte("test data"),
	})

	// Cleanup
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-attachDone:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachSocket did not return within timeout")
	}
}

// TestSession_TerminalOutput_DoesNotPanicDuringShutdown verifies that
// terminal output flushed via OutputCoalescer does not panic when the
// session shuts down concurrently.
func TestSession_TerminalOutput_DoesNotPanicDuringShutdown(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)

	done := make(chan struct{})
	var panicVal interface{}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
			}
			close(done)
		}()
		sess.Run()
	}()

	// Wait for Run() to start
	time.Sleep(50 * time.Millisecond)

	// Simulate terminal output subscription (same as handleCreateTerminal)
	coalescer := terminal.NewOutputCoalescer(func(data []byte) {
		sess.SendBinaryFrame(protocol.TerminalStreamFrame{
			Opcode:  protocol.TerminalOutput,
			Slot:    1,
			Payload: data,
		})
	})

	// Add output data - this starts the coalescer timer
	coalescer.Add([]byte("terminal output"))

	// Immediately disconnect while the coalescer timer may still be pending
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	// Wait for Run() to complete
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not complete within timeout")
	}

	if panicVal != nil {
		t.Fatalf("panic occurred during shutdown: %v", panicVal)
	}
}
