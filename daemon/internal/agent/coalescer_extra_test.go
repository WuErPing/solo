package agent

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStreamCoalescerFlushAll(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "a1"}, "mock", "")
	c.Handle("agent-2", "timeline", TimelineItem{Type: "assistant_message", Text: "a2"}, "mock", "")

	c.FlushAll()

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 2 {
		t.Errorf("expected 2 flushed items, got %d", len(flushed))
	}
}

func TestStreamCoalescerFlushAndDiscard(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "hello"}, "mock", "")
	c.FlushAndDiscard("agent-1")

	mu.Lock()
	if len(flushed) != 1 {
		t.Errorf("expected 1 flushed item, got %d", len(flushed))
	}
	mu.Unlock()

	// Subsequent handle should create a new buffer
	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "world"}, "mock", "")
	c.FlushAndDiscard("agent-1")

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 2 {
		t.Errorf("expected 2 flushed items after discard, got %d", len(flushed))
	}
}

func TestStreamCoalescerFlushAndDiscardNonExistent(t *testing.T) {
	var flushed bool
	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		flushed = true
	})

	// Should not panic for non-existent agent
	c.FlushAndDiscard("nonexistent")
	if flushed {
		t.Error("expected no flush for non-existent agent")
	}
}

func TestStreamCoalescerFlushAllEmpty(t *testing.T) {
	var flushed bool
	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		flushed = true
	})

	c.FlushAll()
	if flushed {
		t.Error("expected no flush for empty coalescer")
	}
}

func TestStreamCoalescerFlushForExisting(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "hello"}, "mock", "")
	c.FlushFor("agent-1")

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Errorf("expected 1 flushed item, got %d", len(flushed))
	}
}

func TestStreamCoalescerFlushForNonExistent(t *testing.T) {
	var flushed bool
	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		flushed = true
	})

	c.FlushFor("nonexistent")
	if flushed {
		t.Error("expected no flush for non-existent agent")
	}
}

func TestStreamCoalescerTimerFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(50, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	c.Handle("agent-1", "timeline", TimelineItem{Type: "assistant_message", Text: "hello"}, "mock", "")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Errorf("expected 1 flushed item after timer, got %d", len(flushed))
	}
}

// TestStreamCoalescerConcurrentHandleAndFlush stresses the coalescer with concurrent
// Handle calls and explicit FlushAll/FlushAndDiscard operations. Run with -race.
func TestStreamCoalescerConcurrentHandleAndFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(50, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				c.Handle(fmt.Sprintf("agent-%d", id%3), "timeline", TimelineItem{Type: "assistant_message", Text: "hello"}, "mock", "")
			}
		}(i)
	}

	// Concurrent flusher
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			c.FlushAll()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Concurrent discarder
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			c.FlushAndDiscard(fmt.Sprintf("agent-%d", i%3))
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()
	c.FlushAll()

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) == 0 {
		t.Error("expected some flushes after concurrent operations")
	}
}

// TestStreamCoalescerConcurrentHandleAndFlushFor stresses per-agent flushes concurrently.
func TestStreamCoalescerConcurrentHandleAndFlushFor(t *testing.T) {
	var mu sync.Mutex
	var flushed []FlushPayload

	c := NewStreamCoalescer(1000, func(p FlushPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := fmt.Sprintf("agent-%d", id)
			for j := 0; j < 30; j++ {
				c.Handle(agentID, "timeline", TimelineItem{Type: "assistant_message", Text: "x"}, "mock", "")
				if j%10 == 0 {
					c.FlushFor(agentID)
				}
			}
		}(i)
	}

	wg.Wait()
	c.FlushAll()

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) == 0 {
		t.Error("expected some flushes after concurrent Handle+FlushFor")
	}
}
