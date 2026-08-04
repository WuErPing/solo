package terminal

import (
	"bytes"
	"sync/atomic"
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
	proc := NewTerminalProcess("t1", "Test", t.TempDir(), "/bin/sh",
		[]string{"-c", "echo solo-subscribe-probe; sleep 0.2"}, 24, 80, newTestLogger(t))
	if err := proc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proc.Close()

	received := make(chan []byte, 16)
	unsub := proc.Subscribe(func(data []byte) {
		select {
		case received <- data:
		default:
		}
	})

	// The readLoop must deliver PTY output to subscribers.
	deadline := time.After(2 * time.Second)
	var got []byte
	for !bytes.Contains(got, []byte("solo-subscribe-probe")) {
		select {
		case data := <-received:
			got = append(got, data...)
		case <-deadline:
			t.Fatalf("subscriber did not receive PTY output; got %q", got)
		}
	}

	// Unsubscribe removes the callback from the registry.
	unsub()
	proc.mu.Lock()
	count := len(proc.subscribers)
	proc.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 subscribers after unsub, got %d", count)
	}
}

func TestTerminalProcess_OnExit(t *testing.T) {
	proc := NewTerminalProcess("t1", "Test", t.TempDir(), "/bin/sh",
		[]string{"-c", "exit 3"}, 24, 80, newTestLogger(t))

	exitCh := make(chan ExitInfo, 1)
	proc.OnExit(func(info ExitInfo) {
		exitCh <- info
	})

	if err := proc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// The cmd.Wait goroutine must invoke OnExit with the real exit code.
	select {
	case info := <-exitCh:
		if info.Code != 3 {
			t.Errorf("exit code: got %d, want 3", info.Code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnExit callback not invoked after process exit")
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
	proc := NewTerminalProcess("t1", "Test", t.TempDir(), "/bin/sh",
		[]string{"-c", "exit 0"}, 24, 80, newTestLogger(t))

	var exitCalls atomic.Int32
	proc.OnExit(func(_ ExitInfo) {
		exitCalls.Add(1)
	})

	if err := proc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the process to exit on its own.
	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit")
	}

	// Kill on an already-exited process must be a safe no-op.
	proc.Kill()

	// OnExit must have fired exactly once (from cmd.Wait, not from Kill).
	if n := exitCalls.Load(); n != 1 {
		t.Errorf("OnExit fired %d times, want exactly 1", n)
	}
}
