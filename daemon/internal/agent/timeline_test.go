package agent

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

func TestTimelineStoreInitialize(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")
	if !s.Has("agent-1") {
		t.Error("expected timeline to exist")
	}
	if s.GetEpoch("agent-1") == "" {
		t.Error("expected non-empty epoch")
	}
}

func TestTimelineStoreAppend(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	row := s.Append("agent-1", TimelineItem{
		Type: "user_message",
		Text: "hello",
	})
	if row.Seq != 0 {
		t.Errorf("Seq: got %d, want 0", row.Seq)
	}

	row2 := s.Append("agent-1", TimelineItem{
		Type: "assistant_message",
		Text: "world",
	})
	if row2.Seq != 1 {
		t.Errorf("Seq: got %d, want 1", row2.Seq)
	}
}

func TestTimelineStoreFetchTail(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	for i := 0; i < 10; i++ {
		s.Append("agent-1", TimelineItem{
			Type: "assistant_message",
			Text: fmt.Sprintf("msg-%d", i),
		})
	}

	result := s.Fetch("agent-1", "tail", nil, 3)
	if len(result.Rows) != 3 {
		t.Errorf("Rows: got %d, want 3", len(result.Rows))
	}
	if !result.HasOlder {
		t.Error("expected HasOlder")
	}
	if result.Rows[0].Seq != 7 {
		t.Errorf("first row Seq: got %d, want 7", result.Rows[0].Seq)
	}
}

func TestTimelineStoreFetchAfter(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	for i := 0; i < 10; i++ {
		s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("msg-%d", i)})
	}

	cursor := protocol.AgentTimelineCursor{Epoch: s.GetEpoch("agent-1"), Seq: 4}
	result := s.Fetch("agent-1", "after", &cursor, 0)
	if len(result.Rows) != 5 {
		t.Errorf("Rows: got %d, want 5", len(result.Rows))
	}
	if result.Rows[0].Seq != 5 {
		t.Errorf("first row Seq: got %d, want 5", result.Rows[0].Seq)
	}
}

func TestTimelineStoreFetchBefore(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	for i := 0; i < 10; i++ {
		s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("msg-%d", i)})
	}

	cursor := protocol.AgentTimelineCursor{Epoch: s.GetEpoch("agent-1"), Seq: 5}
	result := s.Fetch("agent-1", "before", &cursor, 3)
	if len(result.Rows) != 3 {
		t.Errorf("Rows: got %d, want 3", len(result.Rows))
	}
	if result.Rows[2].Seq != 4 {
		t.Errorf("last row Seq: got %d, want 4", result.Rows[2].Seq)
	}
}

func TestTimelineStoreEpochMismatch(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")
	s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "msg"})

	cursor := protocol.AgentTimelineCursor{Epoch: "old-epoch", Seq: 0}
	result := s.Fetch("agent-1", "tail", &cursor, 10)
	if !result.Reset {
		t.Error("expected Reset on epoch mismatch")
	}
	if !result.StaleCursor {
		t.Error("expected StaleCursor on epoch mismatch")
	}
}

func TestTimelineStoreDelete(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")
	s.Append("agent-1", TimelineItem{Type: "user_message", Text: "hi"})
	s.Delete("agent-1")
	if s.Has("agent-1") {
		t.Error("expected timeline to be deleted")
	}
}

func TestTimelineStoreGetLastAssistantMessage(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	s.Append("agent-1", TimelineItem{Type: "user_message", Text: "hello"})
	s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "part1 "})
	s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "part2"})

	msg := s.GetLastAssistantMessage("agent-1")
	if msg == nil || *msg != "part1 part2" {
		t.Errorf("got %v, want 'part1 part2'", msg)
	}
}

func TestTimelineStoreWaitForAssistantMessageReturnsExisting(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")
	s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "existing"})

	msg := s.WaitForAssistantMessage("agent-1", 50*time.Millisecond)
	if msg == nil || *msg != "existing" {
		t.Errorf("got %v, want 'existing'", msg)
	}
}

func TestTimelineStoreWaitForAssistantMessageWaitsForAppend(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	go func() {
		time.Sleep(30 * time.Millisecond)
		s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "delayed"})
	}()

	msg := s.WaitForAssistantMessage("agent-1", 200*time.Millisecond)
	if msg == nil || *msg != "delayed" {
		t.Fatalf("expected 'delayed', got %v", msg)
	}
}

func TestTimelineStoreWaitForAssistantMessageTimeout(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	msg := s.WaitForAssistantMessage("agent-1", 50*time.Millisecond)
	if msg != nil {
		t.Fatalf("expected nil, got %v", msg)
	}
}

func TestTimelineItemToProtocolMapIncludesRequiredDetailFields(t *testing.T) {
	// Simulate a running shell tool call with minimal detail (nil input/output)
	item := TimelineItem{
		Type:   "tool_call",
		CallID: "call-1",
		Name:   "bash",
		Status: "running",
		Detail: base.DeriveToolCallDetail("shell", nil, nil),
	}

	m := item.ToProtocolMap()
	if m["type"] != "tool_call" {
		t.Errorf("type = %v", m["type"])
	}

	detail, ok := m["detail"].(map[string]interface{})
	if !ok {
		t.Fatalf("detail is not a map: %T", m["detail"])
	}
	if detail["type"] != "shell" {
		t.Errorf("detail.type = %v", detail["type"])
	}
	if _, exists := detail["command"]; !exists {
		t.Errorf("detail.command missing")
	}

	// Verify JSON serialization includes the command field
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !json.Valid(data) {
		t.Error("invalid JSON")
	}
}

// TestAppendFromHistory_DeduplicatesNonAdjacentLiveEvents verifies that
// history hydration does not insert duplicates of items already added by
// live events when they are no longer the most recent row.
func TestAppendFromHistory_DeduplicatesNonAdjacentLiveEvents(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	// Simulate live events: user message followed by assistant response.
	s.Append("agent-1", TimelineItem{Type: "user_message", Text: "hello"})
	s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "hi there"})

	// Hydrate history containing the same user message (now not the last row).
	s.AppendFromHistory("agent-1", TimelineItem{Type: "user_message", Text: "hello"})

	result := s.Fetch("agent-1", "tail", nil, 0)
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows after hydration, got %d: %+v", len(result.Rows), result.Rows)
	}
	if result.Rows[0].Item.Type != "user_message" || result.Rows[0].Item.Text != "hello" {
		t.Errorf("row 0: got %+v, want user_message/hello", result.Rows[0].Item)
	}
	if result.Rows[1].Item.Type != "assistant_message" || result.Rows[1].Item.Text != "hi there" {
		t.Errorf("row 1: got %+v, want assistant_message/hi there", result.Rows[1].Item)
	}
}

// TestAppendFromHistory_DeduplicatesDuplicateHistoryEntries verifies that
// identical entries within a single history hydration batch are deduplicated.
func TestAppendFromHistory_DeduplicatesDuplicateHistoryEntries(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	s.AppendFromHistory("agent-1", TimelineItem{Type: "assistant_message", Text: "once"})
	s.AppendFromHistory("agent-1", TimelineItem{Type: "assistant_message", Text: "once"})

	result := s.Fetch("agent-1", "tail", nil, 0)
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

// TestAppend_DoesNotDeduplicateNonAdjacentLiveDuplicates verifies that Append
// still allows distinct live events with identical content separated by other
// events (it only checks the last row).
func TestAppend_DoesNotDeduplicateNonAdjacentLiveDuplicates(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("agent-1")

	s.Append("agent-1", TimelineItem{Type: "user_message", Text: "hello"})
	s.Append("agent-1", TimelineItem{Type: "assistant_message", Text: "hi"})
	s.Append("agent-1", TimelineItem{Type: "user_message", Text: "hello"})

	result := s.Fetch("agent-1", "tail", nil, 0)
	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}
}
