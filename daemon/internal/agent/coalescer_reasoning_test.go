package agent

import (
	"sync"
	"testing"
	"time"
)

// TestStreamCoalescer_ReasoningDelayedFlush verifies that reasoning events
// spaced apart but within the extended 2s reasoning window are still merged
// into a single entry. Previously (before the fix), a 200ms base window
// caused events 250ms apart to split into separate "Thinking" entries.
//
// This is the FIXED version of the regression test — the bug is now resolved.
func TestStreamCoalescer_ReasoningDelayedFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// First reasoning chunk schedules the extended 2s window.
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "Let me think"}, "claude", "")
	ds := clk.PendingDurations()
	if len(ds) != 1 || ds[0] != time.Duration(ReasoningCoalesceWindowMs)*time.Millisecond {
		t.Fatalf("expected extended reasoning window %dms, got %v", ReasoningCoalesceWindowMs, ds)
	}

	// Second reasoning chunk — should still merge with the first
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: " about this"}, "claude", "")

	// Fire the extended window timer.
	clk.FireAll()

	mu.Lock()
	defer mu.Unlock()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 merged item (fix: reasoning uses extended window), got %d", len(flushed))
	}

	if flushed[0].Item.Type != "reasoning" {
		t.Errorf("expected reasoning, got %s", flushed[0].Item.Type)
	}

	if flushed[0].Item.Text != "Let me think about this" {
		t.Errorf("merged text: got %q, want %q", flushed[0].Item.Text, "Let me think about this")
	}
}

// TestStreamCoalescer_ReasoningMergedInWindow verifies that reasoning events
// within the coalesce window are properly merged into a single entry.
func TestStreamCoalescer_ReasoningMergedInWindow(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c, clk := newTestCoalescer(200, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// Both chunks within the window
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "Let me think"}, "claude", "")
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: " about this"}, "claude", "")

	// Fire the extended window timer.
	clk.FireAll()

	mu.Lock()
	defer mu.Unlock()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 merged item, got %d", len(flushed))
	}

	if flushed[0].Item.Text != "Let me think about this" {
		t.Errorf("merged text: got %q, want %q", flushed[0].Item.Text, "Let me think about this")
	}
}
