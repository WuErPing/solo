package bridge

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// ---------- Fake dependencies ----------

// fakeRecorder captures RecordTurn calls in the order received.
type fakeRecorder struct {
	mu     sync.Mutex
	turns  []memory.Turn
	closed bool
	record func(memory.Turn) error // optional hook
}

func newFakeRecorder() *fakeRecorder { return &fakeRecorder{} }

func (f *fakeRecorder) RecordTurn(_ context.Context, _ string, turn memory.Turn) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return memory.ErrClosed
	}
	if f.record != nil {
		if err := f.record(turn); err != nil {
			return err
		}
	}
	f.turns = append(f.turns, turn)
	return nil
}

func (f *fakeRecorder) Flush(context.Context) error { return nil }

func (f *fakeRecorder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeRecorder) Turns() []memory.Turn {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]memory.Turn, len(f.turns))
	copy(out, f.turns)
	return out
}

// fakeClock returns a fixed time; tests can override Ts.
type fakeClock struct {
	ts time.Time
}

func (c *fakeClock) Now() time.Time { return c.ts }

// uppercaseRedactor upper-cases content, exercising the Redactor contract
// without depending on the real redact package.
type uppercaseRedactor struct{}

func (uppercaseRedactor) Redact(s string) string { return strings.ToUpper(s) }

// ---------- Constructor ----------

func TestNew_RequiresRecorder(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil recorder")
	}
}

func TestNew_Defaults(t *testing.T) {
	b, err := New(newFakeRecorder())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.clock == nil {
		t.Error("default clock is nil")
	}
	if b.redactor == nil {
		t.Error("default redactor is nil")
	}
	if b.logger == nil {
		t.Error("default logger is nil")
	}
}

// ---------- OnUserTurn ----------

func TestBridge_OnUserTurn_Records(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnUserTurn("sess-1", "agent-1", "hello")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("recorded %d turns, want 1", len(got))
	}
	turn := got[0]
	if turn.Role != memory.RoleUser {
		t.Errorf("Role = %q, want user", turn.Role)
	}
	if turn.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", turn.SessionID)
	}
	if turn.Content != "hello" {
		t.Errorf("Content = %q, want hello", turn.Content)
	}
	if turn.Source != memory.SourceCLI {
		t.Errorf("Source = %q, want cli", turn.Source)
	}
	if turn.Seq != 1 {
		t.Errorf("Seq = %d, want 1", turn.Seq)
	}
	if turn.ParentID != "" {
		t.Errorf("ParentID = %q, want empty (first turn)", turn.ParentID)
	}
	if !memory.IsTurnID(turn.ID) {
		t.Errorf("ID %q does not match turn ID format", turn.ID)
	}
}

// ---------- OnAssistantTurn ----------

func TestBridge_OnAssistantTurn_Records(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantTurn("sess-1", "agent-1", "hi there")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("recorded %d turns, want 1", len(got))
	}
	if got[0].Role != memory.RoleAssistant {
		t.Errorf("Role = %q, want assistant", got[0].Role)
	}
	if got[0].Content != "hi there" {
		t.Errorf("Content = %q, want 'hi there'", got[0].Content)
	}
	if got[0].Seq != 1 {
		t.Errorf("Seq = %d, want 1", got[0].Seq)
	}
}

// ---------- Redaction ----------

func TestBridge_OnUserTurn_AppliesRedactor(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec, WithRedactor(uppercaseRedactor{}))

	b.OnUserTurn("s", "a", "hello")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("recorded %d turns", len(got))
	}
	if got[0].Content != "HELLO" {
		t.Errorf("redactor not applied: Content = %q, want HELLO", got[0].Content)
	}
}

func TestBridge_OnAssistantTurn_AppliesRedactor(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec, WithRedactor(uppercaseRedactor{}))

	b.OnAssistantTurn("s", "a", "world")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("recorded %d turns", len(got))
	}
	if got[0].Content != "WORLD" {
		t.Errorf("redactor not applied: Content = %q, want WORLD", got[0].Content)
	}
}

// ---------- Clock injection ----------

func TestBridge_UsesInjectedClock(t *testing.T) {
	rec := newFakeRecorder()
	fixed := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	b, _ := New(rec, WithClock(&fakeClock{ts: fixed}))

	b.OnUserTurn("s", "a", "x")

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("recorded %d turns", len(got))
	}
	if !got[0].Ts.Equal(fixed) {
		t.Errorf("Ts = %v, want %v", got[0].Ts, fixed)
	}
}

// ---------- Session chain (seq + parent) ----------

func TestBridge_SameSession_SequentialTurns(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnUserTurn("s", "a", "u1")
	b.OnAssistantTurn("s", "a", "a1")
	b.OnUserTurn("s", "a", "u2")

	got := rec.Turns()
	if len(got) != 3 {
		t.Fatalf("recorded %d turns, want 3", len(got))
	}

	// First turn: seq=1, no parent
	if got[0].Seq != 1 {
		t.Errorf("turn[0].Seq = %d, want 1", got[0].Seq)
	}
	if got[0].ParentID != "" {
		t.Errorf("turn[0].ParentID = %q, want empty", got[0].ParentID)
	}

	// Second turn: seq=2, parent=turn[0].ID
	if got[1].Seq != 2 {
		t.Errorf("turn[1].Seq = %d, want 2", got[1].Seq)
	}
	if got[1].ParentID != got[0].ID {
		t.Errorf("turn[1].ParentID = %q, want %q", got[1].ParentID, got[0].ID)
	}

	// Third turn: seq=3, parent=turn[1].ID
	if got[2].Seq != 3 {
		t.Errorf("turn[2].Seq = %d, want 3", got[2].Seq)
	}
	if got[2].ParentID != got[1].ID {
		t.Errorf("turn[2].ParentID = %q, want %q", got[2].ParentID, got[1].ID)
	}
}

func TestBridge_DifferentSessions_IndependentChains(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnUserTurn("s1", "a", "u1")
	b.OnUserTurn("s2", "a", "u2")
	b.OnUserTurn("s1", "a", "u3")

	got := rec.Turns()
	if len(got) != 3 {
		t.Fatalf("recorded %d turns", len(got))
	}

	// s1: turn[0] seq=1, turn[2] seq=2, parent=turn[0].ID
	if got[0].Seq != 1 || got[0].SessionID != "s1" {
		t.Errorf("turn[0]: %+v", got[0])
	}
	if got[2].Seq != 2 || got[2].SessionID != "s1" {
		t.Errorf("turn[2]: %+v", got[2])
	}
	if got[2].ParentID != got[0].ID {
		t.Errorf("turn[2].ParentID = %q, want %q", got[2].ParentID, got[0].ID)
	}

	// s2: turn[1] seq=1, no parent
	if got[1].Seq != 1 || got[1].SessionID != "s2" {
		t.Errorf("turn[1]: %+v", got[1])
	}
	if got[1].ParentID != "" {
		t.Errorf("turn[1].ParentID = %q, want empty", got[1].ParentID)
	}
}

func TestBridge_OnSystemTurn_PartOfChain(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnSystemTurn("s", "a", "you are helpful")
	b.OnUserTurn("s", "a", "hi")

	got := rec.Turns()
	if len(got) != 2 {
		t.Fatalf("recorded %d turns", len(got))
	}
	if got[0].Role != memory.RoleSystem {
		t.Errorf("turn[0].Role = %q, want system", got[0].Role)
	}
	if got[1].ParentID != got[0].ID {
		t.Errorf("user turn should link to system turn")
	}
	if got[1].Seq != 2 {
		t.Errorf("user turn seq = %d, want 2", got[1].Seq)
	}
}

// ---------- Failure isolation ----------

func TestBridge_RecordFailure_DoesNotAdvanceChain(t *testing.T) {
	rec := newFakeRecorder()
	calls := 0
	rec.record = func(memory.Turn) error {
		calls++
		if calls == 1 {
			return errors.New("disk full")
		}
		return nil
	}
	b, _ := New(rec)

	b.OnUserTurn("s", "a", "first")  // fails
	b.OnUserTurn("s", "a", "second") // succeeds

	got := rec.Turns()
	if len(got) != 1 {
		t.Fatalf("recorded %d turns, want 1", len(got))
	}
	// The successful turn must be seq=1, because the failed one did not advance the chain.
	if got[0].Seq != 1 {
		t.Errorf("Seq = %d, want 1 (failure should not advance chain)", got[0].Seq)
	}
	if got[0].ParentID != "" {
		t.Errorf("ParentID = %q, want empty", got[0].ParentID)
	}
	if got[0].Content != "second" {
		t.Errorf("Content = %q, want second", got[0].Content)
	}
}

// ---------- Concurrency ----------

func TestBridge_ConcurrentCalls(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	const workers = 8
	const perWorker = 50
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				b.OnUserTurn("shared", "a", "x")
			}
		}()
	}
	wg.Wait()

	got := rec.Turns()
	if len(got) != workers*perWorker {
		t.Errorf("recorded %d turns, want %d", len(got), workers*perWorker)
	}

	// Every turn must have a unique seq in [1, workers*perWorker] and the
	// parent chain must be a valid linked list.
	seqs := make(map[uint64]bool, len(got))
	ids := make(map[string]bool, len(got))
	for _, turn := range got {
		if seqs[turn.Seq] {
			t.Fatalf("duplicate seq %d", turn.Seq)
		}
		seqs[turn.Seq] = true
		if ids[turn.ID] {
			t.Fatalf("duplicate ID %q", turn.ID)
		}
		ids[turn.ID] = true
	}
}

// ---------- Validation ----------

func TestBridge_EmptySessionID_NoRecord(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnUserTurn("", "a", "x")

	if got := len(rec.Turns()); got != 0 {
		t.Errorf("empty sessionID should be rejected, got %d turns", got)
	}
}

func TestBridge_EmptyAgentID_NoRecord(t *testing.T) {
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnAssistantTurn("s", "", "x")

	if got := len(rec.Turns()); got != 0 {
		t.Errorf("empty agentID should be rejected, got %d turns", got)
	}
}

func TestBridge_EmptyContent_StillRecords(t *testing.T) {
	// Empty content is legitimate (e.g., a system prompt placeholder).
	rec := newFakeRecorder()
	b, _ := New(rec)

	b.OnUserTurn("s", "a", "")

	if got := len(rec.Turns()); got != 1 {
		t.Errorf("empty content should still record, got %d", got)
	}
}
