package agent

import (
	"sync"
	"testing"
	"time"
)

// ---- fakeClock ----

type fakeClock struct {
	mu      sync.Mutex
	pending []fakeTimer
}

type fakeTimer struct {
	d time.Duration
	f func()
}

func (fc *fakeClock) AfterFunc(d time.Duration, f func()) *time.Timer {
	fc.mu.Lock()
	fc.pending = append(fc.pending, fakeTimer{d: d, f: f})
	fc.mu.Unlock()
	// Return a real timer that will never fire (very long duration) so callers
	// can still call Stop() on it without panicking.
	return time.AfterFunc(10*time.Hour, func() {})
}

// FireAll fires all pending timers synchronously and clears the queue.
func (fc *fakeClock) FireAll() {
	fc.mu.Lock()
	timers := fc.pending
	fc.pending = nil
	fc.mu.Unlock()
	for _, t := range timers {
		t.f()
	}
}

// PendingCount returns the number of scheduled (unfired) timer callbacks.
func (fc *fakeClock) PendingCount() int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return len(fc.pending)
}

// newTestCoalescer creates a StreamCoalescer wired to a fakeClock.
func newTestCoalescer(windowMs int, onFlush func(FlushPayload)) (*StreamCoalescer, *fakeClock) {
	clk := &fakeClock{}
	c := newStreamCoalescerWithClock(windowMs, onFlush, clk)
	return c, clk
}

// ---- Timer-driven flush tests ----

func TestCoalescerTimerFlushFiresViaFakeClock(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	item := TimelineItem{Type: "assistant_message", Text: "hello"}
	absorbed := c.Handle("agent1", "timeline", item, "mock", "turn1")
	if !absorbed {
		t.Fatal("expected event to be absorbed")
	}
	if len(flushed) != 0 {
		t.Fatal("expected no flush before timer fires")
	}
	if clk.PendingCount() != 1 {
		t.Fatalf("expected 1 pending timer, got %d", clk.PendingCount())
	}

	// Fire the timer — flush must happen synchronously
	clk.FireAll()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed payload, got %d", len(flushed))
	}
	if flushed[0].Item.Text != "hello" {
		t.Errorf("flushed text = %q, want hello", flushed[0].Item.Text)
	}
	if flushed[0].AgentID != "agent1" {
		t.Errorf("AgentID = %q, want agent1", flushed[0].AgentID)
	}
}

func TestCoalescerTimerMergesMultipleChunks(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	for _, chunk := range []string{"Hello", " ", "World"} {
		c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: chunk}, "mock", "t1")
	}

	clk.FireAll()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 merged payload, got %d", len(flushed))
	}
	if flushed[0].Item.Text != "Hello World" {
		t.Errorf("merged text = %q, want 'Hello World'", flushed[0].Item.Text)
	}
}

func TestCoalescerTimerReasoningUsesExtendedWindow(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	c.Handle("a1", "timeline", TimelineItem{Type: "reasoning", Text: "thinking..."}, "mock", "t1")

	if clk.PendingCount() != 1 {
		t.Fatal("expected a timer to be scheduled for reasoning event")
	}

	clk.FireAll()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed payload after timer, got %d", len(flushed))
	}
	if flushed[0].Item.Type != "reasoning" {
		t.Errorf("type = %q, want reasoning", flushed[0].Item.Type)
	}
}

func TestCoalescerTimerOnlyScheduledOnce(t *testing.T) {
	c, clk := newTestCoalescer(200, func(FlushPayload) {})

	for i := 0; i < 5; i++ {
		c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "x"}, "mock", "t1")
	}

	if clk.PendingCount() != 1 {
		t.Errorf("expected exactly 1 timer, got %d", clk.PendingCount())
	}
}

func TestCoalescerTimerRescheduledAfterFire(t *testing.T) {
	c, clk := newTestCoalescer(200, func(FlushPayload) {})

	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "first"}, "mock", "t1")
	clk.FireAll() // flushes

	// New event after flush — a fresh timer should be scheduled
	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "second"}, "mock", "t1")
	if clk.PendingCount() != 1 {
		t.Errorf("expected new timer after flush, got %d", clk.PendingCount())
	}
}

func TestCoalescerTerminalToolCallFlushesImmediately(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	// Buffer some text first
	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "text"}, "mock", "t1")

	// Terminal tool_call triggers immediate flush without firing the clock
	c.Handle("a1", "timeline", TimelineItem{
		Type: "tool_call", CallID: "c1", Status: "completed",
	}, "mock", "t1")

	if len(flushed) == 0 {
		t.Fatal("expected immediate flush on terminal tool_call, got none without firing clock")
	}
	_ = clk // clock was not needed
}

func TestCoalescerFlushForClearsTimer(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "early"}, "mock", "t1")
	c.FlushFor("a1")

	// Flush happened without firing clock
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush from FlushFor, got %d", len(flushed))
	}
	// Timer should have been stopped (FireAll does nothing new)
	clk.FireAll()
	if len(flushed) != 1 {
		t.Errorf("expected no extra flush after clock fire, got %d", len(flushed))
	}
}

func TestCoalescerFlushAllClearsAllAgents(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "A"}, "mock", "t1")
	c.Handle("a2", "timeline", TimelineItem{Type: "assistant_message", Text: "B"}, "mock", "t1")
	c.FlushAll()

	mu.Lock()
	count := len(flushed)
	mu.Unlock()

	if count != 2 {
		t.Fatalf("expected 2 flushed payloads, got %d", count)
	}
	_ = clk
}

func TestCoalescerFlushAndDiscardNoTimerFire(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "data"}, "mock", "t1")
	c.FlushAndDiscard("a1")

	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(flushed))
	}

	// Firing the clock after discard should not produce extra flushes
	clk.FireAll()
	if len(flushed) != 1 {
		t.Errorf("expected no extra flush after discard, got %d", len(flushed))
	}
}

func TestCoalescerMultipleAgentsIndependent(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		mu.Lock()
		counts[p.AgentID]++
		mu.Unlock()
	})

	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "X"}, "mock", "t1")
	c.Handle("a2", "timeline", TimelineItem{Type: "assistant_message", Text: "Y"}, "mock", "t1")
	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "X2"}, "mock", "t1")

	clk.FireAll()

	mu.Lock()
	defer mu.Unlock()
	if counts["a1"] != 1 {
		t.Errorf("a1 flush count = %d, want 1", counts["a1"])
	}
	if counts["a2"] != 1 {
		t.Errorf("a2 flush count = %d, want 1", counts["a2"])
	}
}

func TestCoalescerDifferentTurnIDsNotMerged(t *testing.T) {
	var flushed []FlushPayload
	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		flushed = append(flushed, p)
	})

	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "A"}, "mock", "turn1")
	c.Handle("a1", "timeline", TimelineItem{Type: "assistant_message", Text: "B"}, "mock", "turn2")
	clk.FireAll()

	if len(flushed) != 2 {
		t.Errorf("expected 2 separate payloads for different turnIDs, got %d", len(flushed))
	}
}

func TestCoalescerNonCoalescablePassthrough(t *testing.T) {
	c, _ := newTestCoalescer(200, func(FlushPayload) {})

	absorbed := c.Handle("a1", "timeline", TimelineItem{Type: "user_message", Text: "hi"}, "mock", "t1")
	if absorbed {
		t.Error("user_message should not be absorbed by coalescer")
	}
}
