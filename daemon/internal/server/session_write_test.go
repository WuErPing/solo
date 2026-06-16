package server

import (
	"testing"
	"time"
)

type mockWriteDeadlineConn struct {
	mockConn
	writeDeadlineSet bool
	lastDeadline     time.Time
}

func (m *mockWriteDeadlineConn) SetWriteDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeDeadlineSet = true
	m.lastDeadline = t
	return nil
}

func TestWriteMessageWithDeadline_NilConn(t *testing.T) {
	err := writeMessageWithDeadline(nil, 1, []byte("test"))
	if err != nil {
		t.Errorf("expected nil error for nil conn, got %v", err)
	}
}

func TestWriteMessageWithDeadline_PlainConn(t *testing.T) {
	conn := newMockConn()
	data := []byte(`{"type":"pong"}`)
	err := writeMessageWithDeadline(conn, 1, data)
	if err != nil {
		t.Fatalf("writeMessageWithDeadline: %v", err)
	}
	conn.mu.Lock()
	got := len(conn.messages)
	conn.mu.Unlock()
	if got != 1 {
		t.Errorf("expected 1 message written, got %d", got)
	}
}

func TestWriteMessageWithDeadline_WriteDeadlineConn(t *testing.T) {
	conn := &mockWriteDeadlineConn{}
	data := []byte(`{"type":"pong"}`)
	err := writeMessageWithDeadline(conn, 1, data)
	if err != nil {
		t.Fatalf("writeMessageWithDeadline: %v", err)
	}
	if !conn.writeDeadlineSet {
		t.Error("expected SetWriteDeadline to be called")
	}
	// Deadline should be reset to zero after write
	if !conn.lastDeadline.IsZero() {
		t.Error("expected deadline to be reset to zero after write")
	}
}

func TestWaitForWritePump_DoneAlreadyClosed(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if !waitForWritePump(done, time.Second) {
		t.Error("expected true when done channel is already closed")
	}
}

func TestWaitForWritePump_Timeout(t *testing.T) {
	done := make(chan struct{})
	if waitForWritePump(done, 10*time.Millisecond) {
		t.Error("expected false when timeout expires before done")
	}
}

func TestWaitForWritePump_ZeroTimeout(t *testing.T) {
	done := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(done)
	}()
	if !waitForWritePump(done, 0) {
		t.Error("expected true with zero timeout (blocks until done)")
	}
}

func TestCloseWSConn_Nil(_ *testing.T) {
	closeWSConn(nil) // should not panic
}

func TestCloseWSConn_NonNil(t *testing.T) {
	conn := newMockConn()
	closeWSConn(conn)
	conn.mu.Lock()
	closed := conn.closed
	conn.mu.Unlock()
	if !closed {
		t.Error("expected conn to be closed")
	}
}
