package agent

import (
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

const DefaultCoalesceWindowMs = 200

// ReasoningCoalesceWindowMs is the extended window used for reasoning/thinking
// events. Thinking blocks often have natural pauses (tool-use, API backpressure)
// that exceed the base 500ms window. A 2s window keeps the thinking block intact
// so the client sees one continuous "Thinking" section instead of truncated
// "loading" thoughts that never finalize.
const ReasoningCoalesceWindowMs = 2000

// CoalescableTimelineTypes are the item types that get coalesced.
var CoalescableTimelineTypes = map[string]bool{
	"assistant_message": true,
	"reasoning":         true,
	"tool_call":         true,
}

// Clock abstracts time.AfterFunc so tests can drive timer fires without sleeping.
type Clock interface {
	AfterFunc(d time.Duration, f func()) *time.Timer
}

// realClock is the production Clock backed by time.AfterFunc.
type realClock struct{}

func (realClock) AfterFunc(d time.Duration, f func()) *time.Timer {
	return time.AfterFunc(d, f)
}

// defaultClock is the package-level singleton used by NewStreamCoalescer.
var defaultClock Clock = realClock{}

// pendingTextEntry is a buffered text item (assistant_message or reasoning).
type pendingTextEntry struct {
	Kind     string // "assistant_message" or "reasoning"
	Text     string
	Provider string
	TurnID   string
}

// pendingToolCallEntry is a buffered tool_call item.
type pendingToolCallEntry struct {
	CallID   string
	Name     string
	Detail   protocol.ToolCallDetail
	Status   string
	Error    *protocol.ToolError
	Metadata map[string]interface{}
	Provider string
	TurnID   string
}

// pendingEntry is a union of text or tool_call entries.
type pendingEntry struct {
	isText   bool
	text     pendingTextEntry
	toolCall pendingToolCallEntry
}

// coalescerBuffer holds buffered entries for a single agent.
type coalescerBuffer struct {
	entries         []pendingEntry
	toolCallIndexes map[string]int // callId -> index in entries
	timer           *time.Timer
	flushing        bool
}

// FlushPayload is what gets emitted when the coalescer flushes.
type FlushPayload struct {
	AgentID  string
	Item     TimelineItem
	Provider string
	TurnID   string
}

// StreamCoalescer batches rapid assistant_message, reasoning, and tool_call
// timeline events within a time window, then emits merged items.
type StreamCoalescer struct {
	mu       sync.Mutex
	buffers  map[string]*coalescerBuffer
	windowMs time.Duration
	onFlush  func(FlushPayload)
	clock    Clock
}

// NewStreamCoalescer creates a new coalescer with the given window and callback.
// Pass a non-nil clock to override timer behaviour (useful in tests).
func NewStreamCoalescer(windowMs int, onFlush func(FlushPayload)) *StreamCoalescer {
	return newStreamCoalescerWithClock(windowMs, onFlush, nil)
}

func newStreamCoalescerWithClock(windowMs int, onFlush func(FlushPayload), clk Clock) *StreamCoalescer {
	if windowMs <= 0 {
		windowMs = DefaultCoalesceWindowMs
	}
	if clk == nil {
		clk = defaultClock
	}
	return &StreamCoalescer{
		buffers:  make(map[string]*coalescerBuffer),
		windowMs: time.Duration(windowMs) * time.Millisecond,
		onFlush:  onFlush,
		clock:    clk,
	}
}

// Handle processes a stream event. Returns true if the event was absorbed
// (coalesced), false if it should be dispatched immediately.
func (c *StreamCoalescer) Handle(agentID string, eventType string, item TimelineItem, provider string, turnID string) bool {
	if !CoalescableTimelineTypes[item.Type] {
		return false
	}

	// Discard empty text items
	if (item.Type == "assistant_message" || item.Type == "reasoning") && item.Text == "" {
		return true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	buf, ok := c.buffers[agentID]
	if !ok {
		buf = &coalescerBuffer{
			entries:         make([]pendingEntry, 0),
			toolCallIndexes: make(map[string]int),
		}
		c.buffers[agentID] = buf
	}

	// Append to buffer
	switch item.Type {
	case "assistant_message", "reasoning":
		buf.entries = append(buf.entries, pendingEntry{
			isText: true,
			text: pendingTextEntry{
				Kind:     item.Type,
				Text:     item.Text,
				Provider: provider,
				TurnID:   turnID,
			},
		})
	case "tool_call":
		entry := pendingToolCallEntry{
			CallID:   item.CallID,
			Name:     item.Name,
			Detail:   item.Detail,
			Status:   item.Status,
			Error:    item.Error,
			Metadata: item.Metadata,
			Provider: provider,
			TurnID:   turnID,
		}
		if idx, exists := buf.toolCallIndexes[item.CallID]; exists {
			// Replace in-place (status update for same call)
			buf.entries[idx] = pendingEntry{isText: false, toolCall: entry}
		} else {
			buf.toolCallIndexes[item.CallID] = len(buf.entries)
			buf.entries = append(buf.entries, pendingEntry{isText: false, toolCall: entry})
		}

		// Terminal tool call states trigger immediate flush
		if item.Status == "completed" || item.Status == "failed" || item.Status == "canceled" {
			c.flushBuffer(agentID, buf)
			return true
		}
	}

	// Schedule flush if no timer is running.
	// Reasoning events use an extended window (2s) to keep thinking blocks
	// intact across natural pauses; other types use the base window.
	if buf.timer == nil {
		window := c.windowMs
		if item.Type == "reasoning" {
			window = time.Duration(ReasoningCoalesceWindowMs) * time.Millisecond
		}
		buf.timer = c.clock.AfterFunc(window, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if b, ok := c.buffers[agentID]; ok {
				c.flushBuffer(agentID, b)
			}
		})
	}

	return true
}

// FlushFor immediately flushes all buffered entries for an agent.
func (c *StreamCoalescer) FlushFor(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if buf, ok := c.buffers[agentID]; ok {
		c.flushBuffer(agentID, buf)
	}
}

// FlushAll flushes all agents' buffers.
func (c *StreamCoalescer) FlushAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for agentID, buf := range c.buffers {
		c.flushBuffer(agentID, buf)
	}
}

// FlushAndDiscard flushes and removes the buffer for an agent.
func (c *StreamCoalescer) FlushAndDiscard(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if buf, ok := c.buffers[agentID]; ok {
		c.flushBuffer(agentID, buf)
		delete(c.buffers, agentID)
	}
}

func (c *StreamCoalescer) flushBuffer(agentID string, buf *coalescerBuffer) {
	if buf.flushing {
		return
	}
	if buf.timer != nil {
		buf.timer.Stop()
		buf.timer = nil
	}
	if len(buf.entries) == 0 {
		return
	}

	// Swap out entries
	entries := buf.entries
	buf.entries = make([]pendingEntry, 0)
	buf.toolCallIndexes = make(map[string]int)
	buf.flushing = true

	// Collapse consecutive text entries of the same type, provider, and turnID
	collapsed := c.collapseEntries(entries)

	buf.flushing = false

	// Emit each collapsed entry
	for _, entry := range collapsed {
		var item TimelineItem
		var provider, turnID string

		if entry.isText {
			item = TimelineItem{
				Type: entry.text.Kind,
				Text: entry.text.Text,
			}
			provider = entry.text.Provider
			turnID = entry.text.TurnID
		} else {
			item = TimelineItem{
				Type:     "tool_call",
				CallID:   entry.toolCall.CallID,
				Name:     entry.toolCall.Name,
				Detail:   entry.toolCall.Detail,
				Status:   entry.toolCall.Status,
				Error:    entry.toolCall.Error,
				Metadata: entry.toolCall.Metadata,
			}
			provider = entry.toolCall.Provider
			turnID = entry.toolCall.TurnID
		}

		if c.onFlush != nil {
			c.onFlush(FlushPayload{
				AgentID:  agentID,
				Item:     item,
				Provider: provider,
				TurnID:   turnID,
			})
		}
	}
}

func (c *StreamCoalescer) collapseEntries(entries []pendingEntry) []pendingEntry {
	if len(entries) == 0 {
		return entries
	}

	var result []pendingEntry
	current := entries[0]

	for i := 1; i < len(entries); i++ {
		next := entries[i]

		// Merge consecutive text entries of the same kind, provider, and turnID
		if current.isText && next.isText &&
			current.text.Kind == next.text.Kind &&
			current.text.Provider == next.text.Provider &&
			current.text.TurnID == next.text.TurnID {
			current.text.Text += next.text.Text
		} else {
			result = append(result, current)
			current = next
		}
	}
	result = append(result, current)
	return result
}
