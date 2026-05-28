package bridge

import (
	"strings"
	"testing"
)

// ---------- Chunk accumulation ----------

func TestBridge_OnAssistantChunk_AccumulatesIntoOneTurn(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantChunk("agent-1", "sess-1", "Hello")
	b.OnAssistantChunk("agent-1", "sess-1", ", ")
	b.OnAssistantChunk("agent-1", "sess-1", "world!")
	b.OnAssistantTurnEnd("agent-1", "sess-1")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("got %d turns, want 1 (chunks must coalesce)", len(got))
	}
	if got[0].Role != "assistant" {
		t.Errorf("Role = %q, want assistant", got[0].Role)
	}
	if got[0].Content != "Hello, world!" {
		t.Errorf("Content = %q, want 'Hello, world!'", got[0].Content)
	}
	if got[0].SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", got[0].SessionID)
	}
}

func TestBridge_OnAssistantChunk_FlushedOnClose(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantChunk("agent-1", "sess-1", "partial")
	// No OnAssistantTurnEnd — simulate a daemon shutdown mid-turn.
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("got %d turns, want 1 (pending buffer must flush on Close)", len(got))
	}
	if got[0].Content != "partial" {
		t.Errorf("Content = %q, want partial", got[0].Content)
	}
}

func TestBridge_OnAssistantChunk_NoChunks_NoTurn(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantTurnEnd("agent-1", "sess-1")

	if got := len(rec.Turns()); got != 0 {
		t.Errorf("got %d turns, want 0 (no chunks → no record)", got)
	}
}

func TestBridge_OnAssistantChunk_MultipleAgents_Independent(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantChunk("agent-A", "sess-1", "A1")
	b.OnAssistantChunk("agent-B", "sess-2", "B1")
	b.OnAssistantChunk("agent-A", "sess-1", "A2")
	b.OnAssistantChunk("agent-B", "sess-2", "B2")

	b.OnAssistantTurnEnd("agent-A", "sess-1")
	b.OnAssistantTurnEnd("agent-B", "sess-2")

	got := rec.Turns()
	if len(got) != 2 {
		t.Fatalf("got %d turns, want 2", len(got))
	}

	bySession := map[string]string{}
	for _, tr := range got {
		bySession[tr.SessionID] = tr.Content
	}
	if bySession["sess-1"] != "A1A2" {
		t.Errorf("agent-A content = %q, want 'A1A2'", bySession["sess-1"])
	}
	if bySession["sess-2"] != "B1B2" {
		t.Errorf("agent-B content = %q, want 'B1B2'", bySession["sess-2"])
	}
}

func TestBridge_OnAssistantChunk_ResetsAfterEnd(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantChunk("agent-1", "sess-1", "turn1")
	b.OnAssistantTurnEnd("agent-1", "sess-1")
	b.OnAssistantChunk("agent-1", "sess-1", "turn2")
	b.OnAssistantTurnEnd("agent-1", "sess-1")

	got := rec.Turns()
	if len(got) != 2 {
		t.Fatalf("got %d turns, want 2", len(got))
	}
	if got[0].Content != "turn1" {
		t.Errorf("turn[0].Content = %q, want turn1", got[0].Content)
	}
	if got[1].Content != "turn2" {
		t.Errorf("turn[1].Content = %q, want turn2", got[1].Content)
	}
	if got[0].Seq != 1 || got[1].Seq != 2 {
		t.Errorf("Seq = [%d, %d], want [1, 2]", got[0].Seq, got[1].Seq)
	}
	if got[1].ParentID != got[0].ID {
		t.Errorf("turn[1].ParentID = %q, want %q", got[1].ParentID, got[0].ID)
	}
}

func TestBridge_OnAssistantChunk_EmptyChunk_Noop(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantChunk("agent-1", "sess-1", "")
	b.OnAssistantChunk("agent-1", "sess-1", "real")
	b.OnAssistantTurnEnd("agent-1", "sess-1")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("got %d turns, want 1", len(got))
	}
	if got[0].Content != "real" {
		t.Errorf("Content = %q, want 'real' (empty chunks ignored)", got[0].Content)
	}
}

func TestBridge_OnAssistantChunk_RedactorAppliedToCombinedContent(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec, WithRedactor(uppercaseRedactor{}))

	b.OnAssistantChunk("agent-1", "sess-1", "hello ")
	b.OnAssistantChunk("agent-1", "sess-1", "world")
	b.OnAssistantTurnEnd("agent-1", "sess-1")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("got %d turns, want 1", len(got))
	}
	if got[0].Content != "HELLO WORLD" {
		t.Errorf("redactor not applied to combined content: got %q", got[0].Content)
	}
}

func TestBridge_OnAssistantChunk_UnknownAgentEnd_Noop(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantTurnEnd("ghost", "sess-1")

	if got := len(rec.Turns()); got != 0 {
		t.Errorf("got %d turns, want 0", got)
	}
}

func TestBridge_UserThenStreamingAssistant_ProducesTwoTurns(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnUserTurn("sess-1", "agent-1", "hello")
	b.OnAssistantChunk("agent-1", "sess-1", "hi ")
	b.OnAssistantChunk("agent-1", "sess-1", "there")
	b.OnAssistantTurnEnd("agent-1", "sess-1")

	got := rec.Turns()
	if len(got) != 2 {
		t.Fatalf("got %d turns, want 2", len(got))
	}
	if got[0].Role != "user" || got[1].Role != "assistant" {
		t.Errorf("roles = [%s, %s], want [user, assistant]", got[0].Role, got[1].Role)
	}
	if got[1].Content != "hi there" {
		t.Errorf("assistant content = %q, want 'hi there'", got[1].Content)
	}
	if got[1].ParentID != got[0].ID {
		t.Errorf("assistant.ParentID = %q, want user.ID = %q", got[1].ParentID, got[0].ID)
	}
	if !strings.Contains(got[0].Content, "hello") {
		t.Errorf("user content lost: %q", got[0].Content)
	}
}

func TestBridge_OnAssistantTurn_StillRecordsImmediately(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantTurn("sess-1", "agent-1", "one-shot answer")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("got %d turns, want 1", len(got))
	}
	if got[0].Content != "one-shot answer" {
		t.Errorf("Content = %q", got[0].Content)
	}
}

func TestBridge_OnAssistantChunk_ConcurrentStress(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	const agents = 8
	const chunks = 50
	done := make(chan struct{})
	go func() {
		for i := 0; i < chunks; i++ {
			for a := 0; a < agents; a++ {
				agentID := "agent-" + string(rune('A'+a))
				sessID := "sess-" + string(rune('A'+a))
				b.OnAssistantChunk(agentID, sessID, "x")
			}
		}
		for a := 0; a < agents; a++ {
			agentID := "agent-" + string(rune('A'+a))
			sessID := "sess-" + string(rune('A'+a))
			b.OnAssistantTurnEnd(agentID, sessID)
		}
		close(done)
	}()
	<-done

	got := rec.Turns()
	if len(got) != agents {
		t.Errorf("got %d turns, want %d (one per agent)", len(got), agents)
	}
	for _, tr := range got {
		if len(tr.Content) != chunks {
			t.Errorf("agent turn content len = %d, want %d", len(tr.Content), chunks)
		}
	}
}
