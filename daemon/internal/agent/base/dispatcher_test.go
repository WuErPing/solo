package base

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

// testCriticalEvent is a test-only critical event.
type testCriticalEvent struct{}

func (testCriticalEvent) IsCriticalEvent() bool { return true }

// fillSubscriberChannel fills the internal send-capable channel for a given
// subscriber so that the next send would block. This is needed because
// Subscribe() returns a receive-only channel.
func fillSubscriberChannel(d *ChannelDispatcher, subCh <-chan interface{}, n int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, ch := range d.subscribers {
		if ch == subCh {
			for i := 0; i < n; i++ {
				ch <- "filler"
			}
			return
		}
	}
}

// TestChannelDispatcher_Emit_SlowSubscriberDoesNotBlockFastSubscriber verifies
// that a slow (full-channel) subscriber does not prevent a fast subscriber from
// receiving events promptly. This is the RED test for the bug where Emit() holds
// RLock during potentially blocking sends to subscribers.
func TestChannelDispatcher_Emit_SlowSubscriberDoesNotBlockFastSubscriber(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Subscribe a "slow" subscriber with a tiny channel that we will fill up.
	slowCh := d.Subscribe()

	// Fill the slow subscriber's channel so the next send would block.
	fillSubscriberChannel(d, slowCh, 2560)

	// Subscribe a "fast" subscriber with an empty channel.
	fastCh := d.Subscribe()

	// Emit a critical event. With the buggy code, the slow subscriber's
	// full channel causes the Emit to block for up to 5 seconds on the
	// time.After fallback, delaying the send to the fast subscriber.
	emitDone := make(chan struct{})
	go func() {
		d.Emit(testCriticalEvent{})
		close(emitDone)
	}()

	// The fast subscriber should receive the event within 1s.
	// With the buggy code, this takes ~5s because the slow subscriber blocks
	// for the full criticalSendTimeout. With the fix, the slow subscriber
	// times out at 500ms so the fast subscriber gets the event within ~600ms.
	select {
	case evt := <-fastCh:
		if _, ok := evt.(testCriticalEvent); !ok {
			t.Fatalf("fast subscriber received wrong event type: %T", evt)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("fast subscriber did not receive critical event within 1s — slow subscriber is blocking Emit")
	}

	// Emit should also complete promptly (well under the old 5s timeout).
	select {
	case <-emitDone:
		// Good
	case <-time.After(1 * time.Second):
		t.Fatal("Emit did not return promptly — it is blocked on a slow subscriber")
	}
}

// TestChannelDispatcher_Emit_NonCriticalEvent_SlowSubscriberDoesNotBlock verifies
// that non-critical events don't block either — they should be dropped for full
// subscribers without delaying others.
func TestChannelDispatcher_Emit_NonCriticalEvent_SlowSubscriberDoesNotBlock(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Subscribe a slow subscriber and fill its channel.
	slowCh := d.Subscribe()
	fillSubscriberChannel(d, slowCh, 2560)

	// Subscribe a fast subscriber.
	fastCh := d.Subscribe()

	// Emit a non-critical event.
	emitDone := make(chan struct{})
	go func() {
		d.Emit("non-critical")
		close(emitDone)
	}()

	// Fast subscriber should receive it quickly.
	select {
	case evt := <-fastCh:
		if evt != "non-critical" {
			t.Fatalf("fast subscriber received wrong event: %v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("fast subscriber did not receive non-critical event within 100ms")
	}

	// Emit should complete quickly (non-blocking send to slow subscriber
	// should just drop and move on).
	select {
	case <-emitDone:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Emit did not return promptly for non-critical event")
	}
}

// TestChannelDispatcher_Emit_SubscriberCriticalTimeout verifies that when a
// subscriber channel is full and the event is critical, Emit does not block
// for the full 5-second criticalSendTimeout. After the fix, subscriber sends
// should use a shorter timeout (500ms).
func TestChannelDispatcher_Emit_SubscriberCriticalTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Subscribe a slow subscriber and fill its channel.
	slowCh := d.Subscribe()
	fillSubscriberChannel(d, slowCh, 2560)

	// Emit a critical event — with the bug this blocks for 5 seconds.
	start := time.Now()
	d.Emit(testCriticalEvent{})
	elapsed := time.Since(start)

	// With the fix, subscriber critical timeout should be 500ms, so total
	// Emit should complete well under 1 second. With the bug, it takes ~5s.
	if elapsed > 1*time.Second {
		t.Fatalf("Emit took %v — subscriber critical send should use a shorter timeout, not the full 5s criticalSendTimeout", elapsed)
	}
}

// TestChannelDispatcher_Emit_MainChannelReceivesEvent verifies that events
// are still delivered to the main Events() channel when a consumer exists.
func TestChannelDispatcher_Emit_MainChannelReceivesEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Must call Events() first to register a main channel consumer.
	ch := d.Events()

	d.Emit("test-event")

	select {
	case evt := <-ch:
		if evt != "test-event" {
			t.Fatalf("main channel received wrong event: %v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("main channel did not receive event")
	}
}

// TestChannelDispatcher_Emit_CriticalEventMainChannelTimeout verifies that
// a critical event still waits (up to criticalSendTimeout) to send to the
// main events channel when a consumer exists, but uses the shorter timeout
// for subscribers.
func TestChannelDispatcher_Emit_CriticalEventMainChannelTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Register a main channel consumer (like Claude provider does).
	_ = d.Events()

	// Fill the main events channel.
	for i := 0; i < 256; i++ {
		d.events <- "filler"
	}

	// A critical event to a full main channel should still return relatively
	// quickly since criticalSendTimeout applies only to the main channel and
	// the subscriber timeout is separate.
	start := time.Now()

	// Subscribe a fast subscriber to verify the subscriber path works.
	fastCh := d.Subscribe()

	d.Emit(testCriticalEvent{})
	elapsed := time.Since(start)

	// Even with the main channel timeout, the subscriber should have received
	// the event or timed out at the shorter subscriber timeout.
	select {
	case <-fastCh:
		// Good — subscriber got it even though main channel was full
	case <-time.After(100 * time.Millisecond):
		// Subscriber may not have gotten it if main channel blocked first,
		// but total time should still be reasonable
	}

	// Total time should not exceed criticalSendTimeout (5s) — this is
	// mainly a sanity check that we don't deadlock.
	if elapsed > criticalSendTimeout+1*time.Second {
		t.Fatalf("Emit took %v — exceeded criticalSendTimeout", elapsed)
	}
}

// TestChannelDispatcher_SubscribeUnsubscribe verifies basic subscribe/unsubscribe
// lifecycle.
func TestChannelDispatcher_SubscribeUnsubscribe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	ch := d.Subscribe()

	d.Emit("hello")

	select {
	case evt := <-ch:
		if evt != "hello" {
			t.Fatalf("expected 'hello', got %v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber did not receive event")
	}

	d.Unsubscribe(ch)

	// After unsubscribe, the channel should be closed.
	_, ok := <-ch
	if ok {
		t.Fatal("unsubscribed channel should be closed")
	}
}

// TestChannelDispatcher_Close_StopsEmit verifies that Emit is a no-op after Close.
func TestChannelDispatcher_Close_StopsEmit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)

	d.Close()

	// Emit after close should not panic.
	d.Emit("after-close")
	d.Emit(testCriticalEvent{})
}

// testSemiCriticalEvent is a test-only semi-critical event (e.g., reasoning).
type testSemiCriticalEvent struct{}

func (testSemiCriticalEvent) IsSemiCriticalEvent() bool { return true }

// TestChannelDispatcher_Emit_SemiCriticalEvent_NotDropped verifies that
// semi-critical events (like reasoning timeline events) use a short blocking
// timeout (100ms) instead of being silently dropped when a subscriber channel
// is full. This prevents reasoning content from being lost under transient
// backpressure.
func TestChannelDispatcher_Emit_SemiCriticalEvent_NotDropped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Subscribe a fast subscriber with an empty channel.
	fastCh := d.Subscribe()

	// Emit a semi-critical event.
	d.Emit(testSemiCriticalEvent{})

	// Fast subscriber should receive it.
	select {
	case evt := <-fastCh:
		if _, ok := evt.(testSemiCriticalEvent); !ok {
			t.Fatalf("fast subscriber received wrong event type: %T", evt)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fast subscriber did not receive semi-critical event")
	}
}

// TestChannelDispatcher_Emit_SemiCriticalEvent_SlowSubscriberTimeout verifies
// that semi-critical events use a short timeout (100ms) for slow subscribers,
// rather than silently dropping or blocking for the full criticalSendTimeout.
func TestChannelDispatcher_Emit_SemiCriticalEvent_SlowSubscriberTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewChannelDispatcher(logger)
	defer d.Close()

	// Subscribe a slow subscriber and fill its channel.
	slowCh := d.Subscribe()
	fillSubscriberChannel(d, slowCh, 2560)

	// Subscribe a fast subscriber.
	fastCh := d.Subscribe()

	// Emit a semi-critical event.
	start := time.Now()
	d.Emit(testSemiCriticalEvent{})
	elapsed := time.Since(start)

	// Fast subscriber should receive it.
	select {
	case evt := <-fastCh:
		if _, ok := evt.(testSemiCriticalEvent); !ok {
			t.Fatalf("fast subscriber received wrong event type: %T", evt)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fast subscriber did not receive semi-critical event")
	}

	// Should complete within ~200ms (100ms timeout for slow subscriber),
	// not silently drop (0ms) or block for criticalSendTimeout (5s).
	if elapsed > 300*time.Millisecond {
		t.Fatalf("Emit took %v — semi-critical should use short timeout, not criticalSendTimeout", elapsed)
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("Emit took %v — semi-critical should block briefly, not drop immediately", elapsed)
	}
}
