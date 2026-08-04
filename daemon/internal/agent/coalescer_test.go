package agent

import (
	"sync"
	"testing"
)

func TestStreamCoalescerDifferentTypesNotMerged(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c, clk := newTestCoalescer(50, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// assistant_message + reasoning should NOT be merged
	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "answer"}, "claude", "")
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "thinking"}, "claude", "")

	clk.FireAll()

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 2 {
		t.Errorf("expected 2 flushed items, got %d", len(flushed))
	}
}

func TestStreamCoalescerEmptyTextDiscarded(t *testing.T) {
	c := NewStreamCoalescer(50, func(_ FlushPayload) {
		t.Error("should not flush empty text")
	})

	absorbed := c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: ""}, "claude", "")
	if !absorbed {
		t.Error("empty text should be absorbed (discarded)")
	}
}
