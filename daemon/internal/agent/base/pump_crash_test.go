package base

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestEventPump_StreamEndBeforeTerminal_WrapsCrash verifies that when the
// provider stream hits EOF without a terminal event (i.e. the subprocess died
// mid-turn), the returned error wraps ErrProviderCrashed so upstream crash
// recovery can detect it via errors.Is.
func TestEventPump_StreamEndBeforeTerminal_WrapsCrash(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pump := NewEventPump(logger, nil)

	_, err := pump.RunBlocking(context.Background(), strings.NewReader("{\"partial\":true}\n"), &pumpNoopTranslator{}, nil)
	if err == nil {
		t.Fatal("expected error when stream ends before terminal state")
	}
	if !errors.Is(err, ErrProviderCrashed) {
		t.Fatalf("err = %v, want wrapped ErrProviderCrashed", err)
	}
}

// TestEventPump_ContextCancel_NotCrash verifies that a cancelled turn is not
// misclassified as a provider crash.
func TestEventPump_ContextCancel_NotCrash(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pump := NewEventPump(logger, nil)

	pr, _ := io.Pipe() // hangs until ctx cancel closes it
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := pump.RunBlocking(ctx, pr, &pumpNoopTranslator{}, nil)
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if errors.Is(err, ErrProviderCrashed) {
			t.Fatalf("canceled turn misclassified as crash: %v", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not return after context cancel")
	}
}
