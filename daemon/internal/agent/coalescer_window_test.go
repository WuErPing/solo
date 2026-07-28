package agent

import (
	"sync"
	"testing"
	"time"
)

// TestStreamCoalescer_500msWindow_MergesReasoning verifies that reasoning
// events use the extended 2s window regardless of the base window setting,
// so events spaced 250ms apart are merged into a single entry.
func TestStreamCoalescer_500msWindow_MergesReasoning(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c, clk := newTestCoalescer(500, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// First reasoning chunk schedules the extended 2s window.
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "Let me think"}, "claude", "")
	if clk.PendingCount() != 1 {
		t.Fatalf("expected 1 pending timer, got %d", clk.PendingCount())
	}
	ds := clk.PendingDurations()
	if len(ds) != 1 || ds[0] != time.Duration(ReasoningCoalesceWindowMs)*time.Millisecond {
		t.Fatalf("expected reasoning window %dms, got %v", ReasoningCoalesceWindowMs, ds)
	}

	// Second reasoning chunk — still within the extended reasoning window
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: " about this"}, "claude", "")

	// Fire the extended window timer.
	clk.FireAll()

	mu.Lock()
	defer mu.Unlock()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 merged item with extended reasoning window, got %d", len(flushed))
	}

	if flushed[0].Item.Text != "Let me think about this" {
		t.Errorf("merged text: got %q, want %q", flushed[0].Item.Text, "Let me think about this")
	}
}
