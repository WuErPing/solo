package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// memRecorder is a minimal in-memory TurnRecorder used to pin down the
// interface contract in M1. It captures every turn in order so assertions
// can reason about what the recorder "saw". It is intentionally local to
// this test file; production recorders live in separate subpackages.
type memRecorder struct {
	mu       sync.Mutex
	captured []Turn
	closed   bool
}

func newMemRecorder() *memRecorder { return &memRecorder{} }

func (m *memRecorder) RecordTurn(_ context.Context, _ string, turn Turn) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.captured = append(m.captured, turn)
	return nil
}

func (m *memRecorder) Flush(context.Context) error { return nil }

func (m *memRecorder) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *memRecorder) Captured() []Turn {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Turn, len(m.captured))
	copy(out, m.captured)
	return out
}

// ---------- Contract: satisfies TurnRecorder ----------

func TestMemRecorder_ImplementsTurnRecorder(t *testing.T) {
	t.Helper()
	var _ TurnRecorder = (*memRecorder)(nil)
}

// ---------- RecordTurn ----------

func TestTurnRecorder_RecordTurn_CapturesTurn(t *testing.T) {
	r := newMemRecorder()
	defer r.Close()

	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	turn := NewTurn("sess-1", RoleUser, SourceCLI, ts, "hi")

	if err := r.RecordTurn(context.Background(), "sess-1", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}

	got := r.Captured()
	if len(got) != 1 {
		t.Fatalf("captured %d turns, want 1", len(got))
	}
	if got[0].Content != "hi" {
		t.Errorf("Content = %q, want %q", got[0].Content, "hi")
	}
	if got[0].SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", got[0].SessionID, "sess-1")
	}
}

func TestTurnRecorder_RecordTurn_Accumulates(t *testing.T) {
	r := newMemRecorder()
	defer r.Close()

	ctx := context.Background()
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		turn := NewTurn("sess-1", RoleUser, SourceCLI, ts, "msg")
		turn.Seq = uint64(i)
		if err := r.RecordTurn(ctx, "sess-1", turn); err != nil {
			t.Fatalf("RecordTurn %d: %v", i, err)
		}
	}

	got := r.Captured()
	if len(got) != 5 {
		t.Fatalf("captured %d turns, want 5", len(got))
	}
	for i, g := range got {
		if g.Seq != uint64(i) {
			t.Errorf("turn[%d].Seq = %d", i, g.Seq)
		}
	}
}

func TestTurnRecorder_RecordTurn_ConcurrentSafety(t *testing.T) {
	r := newMemRecorder()
	defer r.Close()

	ctx := context.Background()
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)

	const workers = 8
	const perWorker = 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				turn := NewTurn("sess-1", RoleUser, SourceCLI, ts, "x")
				_ = r.RecordTurn(ctx, "sess-1", turn)
			}
		}()
	}
	wg.Wait()

	if got := len(r.Captured()); got != workers*perWorker {
		t.Errorf("captured %d turns, want %d", got, workers*perWorker)
	}
}

// ---------- Close / ErrClosed ----------

func TestTurnRecorder_Close_Idempotent(t *testing.T) {
	r := newMemRecorder()
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close should be idempotent, got: %v", err)
	}
}

func TestTurnRecorder_RecordAfterClose_ReturnsErrClosed(t *testing.T) {
	r := newMemRecorder()
	_ = r.Close()

	turn := NewTurn("sess-1", RoleUser, SourceCLI, time.Now().UTC(), "late")
	err := r.RecordTurn(context.Background(), "sess-1", turn)
	if !errors.Is(err, ErrClosed) {
		t.Errorf("got err = %v, want ErrClosed", err)
	}
}

// ---------- Flush ----------

func TestTurnRecorder_Flush_NoError(t *testing.T) {
	r := newMemRecorder()
	defer r.Close()

	ctx := context.Background()
	turn := NewTurn("sess-1", RoleUser, SourceCLI, time.Now().UTC(), "x")
	_ = r.RecordTurn(ctx, "sess-1", turn)

	if err := r.Flush(ctx); err != nil {
		t.Errorf("Flush: %v", err)
	}
}

// ---------- ErrClosed sentinel ----------

func TestErrClosed_NonNilAndStable(t *testing.T) {
	if ErrClosed == nil {
		t.Fatal("ErrClosed must be non-nil")
	}
	// errors.Is identity must be preserved across packages.
	if !errors.Is(ErrClosed, ErrClosed) {
		t.Error("errors.Is(ErrClosed, ErrClosed) must be true")
	}
}
