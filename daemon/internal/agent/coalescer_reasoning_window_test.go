package agent

import (
	"sync"
	"testing"
	"time"
)

// TestStreamCoalescer_ReasoningUsesExtendedWindow verifies that reasoning events
// use a longer coalesce window (2s) so thinking blocks with natural pauses
// (tool-use, API backpressure) are not split into multiple entries.
//
// This is a regression test for the bug where a single thinking block
// split across deltas arriving >500ms apart appeared as multiple "Thinking"
// sections in the timeline, leaving the client with a truncated "loading"
// thought that never transitions to "ready".
func TestStreamCoalescer_ReasoningUsesExtendedWindow(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(500, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// First reasoning chunk
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "Let me think"}, "claude", "")

	// Wait 800ms — longer than the base 500ms window but within the
	// extended reasoning window (2s). Without the fix, the timer fires
	// at 500ms and flushes the first chunk separately.
	time.Sleep(800 * time.Millisecond)

	// Second reasoning chunk arrives before the extended window expires
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: " about this"}, "claude", "")

	// Wait for flush
	time.Sleep(2500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(flushed) != 1 {
		t.Fatalf("expected 1 merged reasoning item with extended window, got %d", len(flushed))
	}

	if flushed[0].Item.Type != "reasoning" {
		t.Errorf("expected reasoning, got %s", flushed[0].Item.Type)
	}

	if flushed[0].Item.Text != "Let me think about this" {
		t.Errorf("merged text: got %q, want %q", flushed[0].Item.Text, "Let me think about this")
	}
}

// TestStreamCoalescer_AssistantMessageUsesBaseWindow verifies that
// assistant_message events still use the base 500ms window for
// responsiveness, even when reasoning uses the extended window.
func TestStreamCoalescer_AssistantMessageUsesBaseWindow(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(500, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// First assistant_message chunk
	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "Hello"}, "claude", "")

	// Wait 600ms — beyond the base 500ms window
	time.Sleep(600 * time.Millisecond)

	// The assistant_message should already have been flushed
	mu.Lock()
	if len(flushed) != 1 {
		t.Fatalf("expected assistant_message to be flushed after base window, got %d items", len(flushed))
	}
	if flushed[0].Item.Text != "Hello" {
		t.Errorf("text: got %q, want %q", flushed[0].Item.Text, "Hello")
	}
	mu.Unlock()

	// A second chunk arriving later should be a separate flush
	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: " World"}, "claude", "")
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 2 {
		t.Fatalf("expected 2 separate flushes for assistant_message, got %d", len(flushed))
	}
}

// TestStreamCoalescer_ContentBlockStopFlushesReasoning verifies that when
// a content_block_stop event arrives (indicating the thinking block is complete),
// the coalescer immediately flushes any buffered reasoning entries.
//
// This ensures thinking content is delivered promptly when the block ends,
// rather than waiting for the full 2s extended window.
func TestStreamCoalescer_ContentBlockStopFlushesReasoning(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(500, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	// Reasoning event enters the coalescer
	c.Handle("agent-1", "timeline", TimelineItem{Type: "reasoning", Text: "Done thinking"}, "claude", "")

	// Before the extended window fires, signal content_block_stop
	c.FlushFor("agent-1")

	mu.Lock()
	defer mu.Unlock()

	if len(flushed) != 1 {
		t.Fatalf("expected reasoning to be flushed on content_block_stop, got %d items", len(flushed))
	}
	if flushed[0].Item.Text != "Done thinking" {
		t.Errorf("text: got %q, want %q", flushed[0].Item.Text, "Done thinking")
	}
}
