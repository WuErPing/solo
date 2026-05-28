package agent

import (
	"fmt"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestAppendFromHistoryWithTimelineItem(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("a1")

	s.AppendFromHistory("a1", TimelineItem{Type: "user_message", Text: "hello"})
	msg := s.GetLastAssistantMessage("a1")
	if msg != nil {
		t.Error("expected nil for non-assistant message")
	}

	s.AppendFromHistory("a1", TimelineItem{Type: "assistant_message", Text: "world"})
	msg = s.GetLastAssistantMessage("a1")
	if msg == nil || *msg != "world" {
		t.Errorf("got %v, want 'world'", msg)
	}
}

func TestAppendFromHistoryWithMap(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("a1")

	s.AppendFromHistory("a1", map[string]interface{}{
		"type": "assistant_message",
		"text": "from map",
	})
	msg := s.GetLastAssistantMessage("a1")
	if msg == nil || *msg != "from map" {
		t.Errorf("got %v, want 'from map'", msg)
	}
}

func TestAppendFromHistoryIgnoresInvalidType(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("a1")

	s.AppendFromHistory("a1", "invalid")
	if s.GetLastAssistantMessage("a1") != nil {
		t.Error("expected nil after invalid type")
	}
}

func TestAppendFromHistoryIgnoresEmptyType(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("a1")

	s.AppendFromHistory("a1", map[string]interface{}{"text": "no type"})
	if s.GetLastAssistantMessage("a1") != nil {
		t.Error("expected nil after empty type")
	}
}

func TestTimelineItemFromMap(t *testing.T) {
	m := map[string]interface{}{
		"type":      "tool_call",
		"text":      "bash",
		"messageId": "mid-1",
		"callId":    "cid-1",
		"name":      "bash",
		"status":    "running",
		"detail":    map[string]interface{}{"cmd": "ls"},
		"error":     "oops",
	}

	ti := timelineItemFromMap(m)
	if ti.Type != "tool_call" {
		t.Errorf("Type: got %q, want tool_call", ti.Type)
	}
	if ti.Text != "bash" {
		t.Errorf("Text: got %q, want bash", ti.Text)
	}
	if ti.MessageID != "mid-1" {
		t.Errorf("MessageID: got %q", ti.MessageID)
	}
	if ti.CallID != "cid-1" {
		t.Errorf("CallID: got %q", ti.CallID)
	}
	if ti.Name != "bash" {
		t.Errorf("Name: got %q", ti.Name)
	}
	if ti.Status != "running" {
		t.Errorf("Status: got %q", ti.Status)
	}
	if ti.Detail == nil {
		t.Error("expected Detail to be set")
	}
	if ti.Error == nil {
		t.Error("expected Error to be set")
	}
}

func TestToProtocolCursor(t *testing.T) {
	row := TimelineRow{Seq: 42}
	cursor := row.ToProtocolCursor("epoch-1")
	if cursor.Epoch != "epoch-1" || cursor.Seq != 42 {
		t.Errorf("got %+v", cursor)
	}
}

func TestFormatSeqRange(t *testing.T) {
	if FormatSeqRange(5, 5) != "5" {
		t.Errorf("single: got %q", FormatSeqRange(5, 5))
	}
	if FormatSeqRange(5, 8) != "5-8" {
		t.Errorf("range: got %q", FormatSeqRange(5, 8))
	}
}

func TestTimelineFetchAfter(t *testing.T) {
	s := NewInMemoryTimelineStore()
	s.Initialize("a1")

	for i := 0; i < 5; i++ {
		s.Append("a1", TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("msg-%d", i)})
	}

	cursor := protocol.AgentTimelineCursor{Epoch: s.GetEpoch("a1"), Seq: 1}
	result := s.Fetch("a1", "after", &cursor, 0)
	if len(result.Rows) != 3 {
		t.Errorf("Rows: got %d, want 3", len(result.Rows))
	}
	if result.Rows[0].Seq != 2 {
		t.Errorf("first row Seq: got %d, want 2", result.Rows[0].Seq)
	}
}
