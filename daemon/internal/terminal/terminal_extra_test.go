package terminal

import (
	"testing"
	"time"
)

func TestTerminalProcess_WriteInput_NoPTY(t *testing.T) {
	proc := &TerminalProcess{
		ID:   "t1",
		Name: "Test",
		done: make(chan struct{}),
	}
	err := proc.WriteInput([]byte("hello"))
	if err == nil {
		t.Fatal("expected error when ptmx is nil")
	}
	if err.Error() != "terminal not running" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTerminalProcess_Resize_NoPTY(t *testing.T) {
	proc := &TerminalProcess{
		ID:   "t1",
		Name: "Test",
		done: make(chan struct{}),
	}
	err := proc.Resize(40, 120)
	if err == nil {
		t.Fatal("expected error when ptmx is nil")
	}
	if err.Error() != "terminal not running" {
		t.Errorf("unexpected error: %v", err)
	}
	// rows/cols should still be updated even without PTY
	if proc.Rows() != 40 {
		t.Errorf("Rows: got %d, want 40", proc.Rows())
	}
	if proc.Cols() != 120 {
		t.Errorf("Cols: got %d, want 120", proc.Cols())
	}
}

func TestTerminalProcess_Subscribe(t *testing.T) {
	proc := &TerminalProcess{
		ID:          "t1",
		Name:        "Test",
		done:        make(chan struct{}),
		subscribers: make(map[uint64]OutputFunc),
	}

	var received [][]byte
	unsub := proc.Subscribe(func(data []byte) {
		received = append(received, data)
	})

	// Manually trigger subscribers (simulating readLoop behavior)
	proc.mu.Lock()
	for _, fn := range proc.subscribers {
		fn([]byte("test output"))
	}
	proc.mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(received))
	}
	if string(received[0]) != "test output" {
		t.Errorf("unexpected data: %q", received[0])
	}

	// Unsubscribe
	unsub()
	proc.mu.Lock()
	count := len(proc.subscribers)
	proc.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 subscribers after unsub, got %d", count)
	}
}

func TestTerminalProcess_OnExit(t *testing.T) {
	proc := &TerminalProcess{
		ID:   "t1",
		Name: "Test",
		done: make(chan struct{}),
	}

	var called bool
	proc.OnExit(func(info ExitInfo) {
		called = true
	})

	proc.mu.Lock()
	count := len(proc.onExitFuncs)
	proc.mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 onExitFunc, got %d", count)
	}

	// Manually invoke
	proc.mu.Lock()
	for _, fn := range proc.onExitFuncs {
		fn(ExitInfo{Code: 0})
	}
	proc.mu.Unlock()
	if !called {
		t.Error("OnExit callback not invoked")
	}
}

func TestTerminalProcess_Done(t *testing.T) {
	proc := &TerminalProcess{
		ID:   "t1",
		done: make(chan struct{}),
	}
	ch := proc.Done()
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	close(proc.done)
	select {
	case <-ch:
		// good
	case <-time.After(time.Second):
		t.Error("Done channel not closed")
	}
}

func TestTerminalProcess_Kill_AlreadyExited(t *testing.T) {
	proc := &TerminalProcess{
		ID:     "t1",
		exited: true,
		done:   make(chan struct{}),
	}
	// Should be a no-op (no panic, no signal sent)
	proc.Kill()
}
