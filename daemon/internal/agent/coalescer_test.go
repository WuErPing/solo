package agent

import (
	"sync"
	"testing"
	"time"
)

func TestStreamCoalescerTextMerging(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(50, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// Handle two consecutive assistant_message items
	absorbed1 := c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "hel"}, "claude", "")
	absorbed2 := c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "lo"}, "claude", "")

	if !absorbed1 || !absorbed2 {
		t.Error("expected events to be absorbed")
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Errorf("expected 1 flushed item, got %d", len(flushed))
	}
	if flushed[0].Item.Text != "hello" {
		t.Errorf("Text: got %q, want %q", flushed[0].Item.Text, "hello")
	}
}

func TestStreamCoalescerDifferentTypesNotMerged(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(50, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// assistant_message + reasoning should NOT be merged
	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "answer"}, "claude", "")
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "thinking"}, "claude", "")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 2 {
		t.Errorf("expected 2 flushed items, got %d", len(flushed))
	}
}

func TestStreamCoalescerEmptyTextDiscarded(t *testing.T) {
	c := NewStreamCoalescer(50, func(p FlushPayload) {
		t.Error("should not flush empty text")
	})

	absorbed := c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: ""}, "claude", "")
	if !absorbed {
		t.Error("empty text should be absorbed (discarded)")
	}
}

func TestStreamCoalescerNonCoalescablePassesThrough(t *testing.T) {
	c := NewStreamCoalescer(50, func(p FlushPayload) {
		t.Error("should not flush non-coalescable events")
	})

	// user_message is not coalescable
	absorbed := c.Handle("agent-1", "timeline", TimelineItem{Type: "user_message", Text: "hello"}, "claude", "")
	if absorbed {
		t.Error("non-coalescable events should pass through")
	}
}

func TestStreamCoalescerToolCallTerminalFlushes(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(200, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// Running tool call should be buffered
	c.Handle("agent-1", "timeline", TimelineItem{
		Type:   "tool_call",
		CallID: "call-1",
		Name:   "read",
		Status: "running",
	}, "claude", "")

	// Completed tool call should trigger immediate flush
	c.Handle("agent-1", "timeline", TimelineItem{
		Type:   "tool_call",
		CallID: "call-1",
		Name:   "read",
		Status: "completed",
	}, "claude", "")

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Errorf("expected 1 flushed item, got %d", len(flushed))
	}
	if flushed[0].Item.Status != "completed" {
		t.Errorf("Status: got %q, want completed", flushed[0].Item.Status)
	}
}

func TestStreamCoalescerFlushFor(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(5000, func(p FlushPayload) { // long window
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "partial"}, "claude", "")
	c.FlushFor("agent-1")

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Errorf("expected 1 flushed item, got %d", len(flushed))
	}
}
