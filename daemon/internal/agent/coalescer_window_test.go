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

	c := NewStreamCoalescer(500, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// First reasoning chunk
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "Let me think"}, "claude", "")

	// Wait 250ms (less than base 500ms window)
	time.Sleep(250 * time.Millisecond)

	// Second reasoning chunk — still within the extended 2s reasoning window
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: " about this"}, "claude", "")

	// Wait for the extended reasoning window to fire (2s)
	time.Sleep(2200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 merged item with extended reasoning window, got %d", len(flushed))
	}

	if flushed[0].Item.Text != "Let me think about this" {
		t.Errorf("merged text: got %q, want %q", flushed[0].Item.Text, "Let me think about this")
	}
}
