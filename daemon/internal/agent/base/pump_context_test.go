package base

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

// TestEventPump_ContextCancelUnblocksScannerHang verifies that cancelling the
// context causes pump.RunBlocking to return promptly even when the io.Reader
// is a pipe with no data and no EOF — i.e. scanner.Scan() is blocked in Read().
//
// Before the fix: scanner.Scan() has no context awareness. ctx.Done() is only
// checked *between* scan iterations (inside the for-loop body), so a hanging
// process that never writes to stdout keeps the pump blocked indefinitely,
// leaving the agent stuck in LifecycleRunning even after the context is cancelled.
func TestEventPump_ContextCancelUnblocksScannerHang(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := NewChannelDispatcher(logger)
	defer dispatcher.Close()

	pump := NewEventPump(logger, dispatcher)

	// io.Pipe simulates a hanging process stdout: never writes, never closes.
	// pw (write end) is deliberately held open so scanner.Scan() blocks in Read().
	pr, _ := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		pump.RunBlocking(ctx, pr, &pumpNoopTranslator{}, nil)
	}()

	time.Sleep(50 * time.Millisecond) // ensure pump is blocked inside scanner.Scan()
	cancel()                          // trigger context cancellation

	select {
	case <-done:
		// pump returned promptly after context cancel — good
	case <-time.After(300 * time.Millisecond):
		t.Fatal("pump.RunBlocking did not return within 300ms after context cancel; scanner.Scan() is not being unblocked by context cancellation")
	}
}

// pumpNoopTranslator is a minimal EventTranslator that discards all input.
type pumpNoopTranslator struct{}

func (n *pumpNoopTranslator) Translate(_ []byte, _ time.Time) ([]interface{}, bool, error) {
	return nil, false, nil
}
