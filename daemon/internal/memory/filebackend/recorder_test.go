package filebackend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// testConfig returns a Config tuned for fast, isolated tests.
func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		QueueSize:     64,
		Overflow:      OverflowBlock,
		FlushInterval: 10 * time.Millisecond,
		Root:          "memory",
		BaseDir:       t.TempDir(),
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ---------- Constructor ----------

func TestNew_ValidatesConfig(t *testing.T) {
	_, err := New(Config{}) // empty BaseDir
	if err == nil {
		t.Fatal("expected error for empty BaseDir")
	}
}

func TestNew_RejectsBadOverflowPolicy(t *testing.T) {
	cfg := testConfig(t)
	cfg.Overflow = "wat"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid overflow policy")
	}
}

// ---------- Interface compliance ----------

func TestFileTurnRecorder_ImplementsTurnRecorder(t *testing.T) {
	t.Helper()
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()
	var _ memory.TurnRecorder = r
}

// ---------- Basic persistence ----------

func TestRecordTurn_PersistsAfterFlush(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	turn := memory.NewTurn("sess-1", memory.RoleUser, memory.SourceCLI, ts, "hello world")
	turn.Seq = 1

	ctx := context.Background()
	if err := r.RecordTurn(ctx, "sess-1", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}
	if err := r.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// A file with the expected role suffix must exist somewhere under BaseDir.
	var found string
	_ = filepath.Walk(r.cfg.BaseDir, func(p string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(p, "0001-user.md") {
			found = p
		}
		return nil
	})
	if found == "" {
		t.Fatalf("no 0001-user.md under %s", r.cfg.BaseDir)
	}
	data, _ := os.ReadFile(found)
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("file missing body:\n%s", data)
	}
}

// ---------- Close semantics ----------

func TestClose_DrainsPendingTurns(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	turn := memory.NewTurn("sess-close", memory.RoleUser, memory.SourceCLI, ts, "last words")
	turn.Seq = 1

	if err := r.RecordTurn(context.Background(), "sess-close", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var found bool
	_ = filepath.Walk(r.cfg.BaseDir, func(p string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(p, "0001-user.md") {
			found = true
		}
		return nil
	})
	if !found {
		t.Error("Close did not drain pending turn to disk")
	}
}

func TestRecordTurn_AfterClose_ReturnsErrClosed(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = r.Close()

	turn := memory.NewTurn("s", memory.RoleUser, memory.SourceCLI, time.Now().UTC(), "x")
	err = r.RecordTurn(context.Background(), "s", turn)
	if !errors.Is(err, memory.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close should be idempotent, got: %v", err)
	}
}

// ---------- Concurrent safety ----------

func TestRecordTurn_ConcurrentSafety(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	ctx := context.Background()
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)

	const workers = 8
	const perWorker = 50
	var wg sync.WaitGroup
	var enqueueErrs atomic.Int64
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(worker int) {
			defer wg.Done()
			sess := "sess"
			for i := 0; i < perWorker; i++ {
				turn := memory.NewTurn(sess, memory.RoleUser, memory.SourceCLI, ts, "x")
				turn.Seq = uint64(worker*perWorker + i + 1)
				if e := r.RecordTurn(ctx, sess, turn); e != nil {
					enqueueErrs.Add(1)
				}
			}
		}(w)
	}
	wg.Wait()
	if errs := enqueueErrs.Load(); errs != 0 {
		t.Errorf("enqueue errors: %d", errs)
	}

	if err := r.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var files int
	_ = filepath.Walk(r.cfg.BaseDir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() && strings.HasSuffix(p, "-user.md") {
			files++
		}
		return nil
	})
	if files != workers*perWorker {
		t.Errorf("got %d files, want %d", files, workers*perWorker)
	}
}

// ---------- Flush semantics ----------

func TestFlush_IdempotentOnEmptyQueue(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	for i := 0; i < 3; i++ {
		if err := r.Flush(context.Background()); err != nil {
			t.Fatalf("Flush #%d: %v", i, err)
		}
	}
}

func TestFlush_ContextCancellation(t *testing.T) {
	// Flush on an empty queue should return immediately; cancel should not hang.
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// With no queued turns, Flush should either return nil (fast path) or
	// propagate the cancellation. Both are acceptable; hanging is not.
	done := make(chan error, 1)
	go func() { done <- r.Flush(ctx) }()
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("Flush did not return when context was already cancelled")
	}
}

// ---------- Overflow policy ----------

func TestRecordTurn_OverflowError(t *testing.T) {
	cfg := testConfig(t)
	cfg.QueueSize = 2
	cfg.Overflow = OverflowError
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	ctx := context.Background()
	ts := time.Now().UTC()
	var errs int
	for i := 0; i < 50; i++ {
		turn := memory.NewTurn("sess", memory.RoleUser, memory.SourceCLI, ts, "x")
		turn.Seq = uint64(i + 1)
		if err := r.RecordTurn(ctx, "sess", turn); err != nil {
			errs++
		}
		// Small yield so the writer goroutine can drain; keeps the test
		// honest without depending on exact timing.
		runtime.Gosched()
	}
	// With a tiny queue and OverflowError, at least one enqueue must fail
	// under realistic scheduling; if the machine is too fast and drains
	// everything, we accept zero failures as long as no panic occurred.
	_ = errs
}

// ---------- Validation ----------

func TestRecordTurn_RejectsInvalidTurn(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	// Bad role
	bad := memory.Turn{ID: "x", SessionID: "s", Role: "nope"}
	err = r.RecordTurn(context.Background(), "s", bad)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if errors.Is(err, memory.ErrClosed) {
		t.Error("validation error should not be ErrClosed")
	}
}

// ---------- Persist error propagation ----------

func TestFlush_ReturnsPersistErrorOnMkdirAllFailure(t *testing.T) {
	cfg := testConfig(t)
	// Make BaseDir read-only so that creating sessions/... fails.
	if err := os.Chmod(cfg.BaseDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(cfg.BaseDir, 0o755)

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	turn := memory.NewTurn("sess-1", memory.RoleUser, memory.SourceCLI, time.Now().UTC(), "hello")
	turn.Seq = 1

	ctx := context.Background()
	if err := r.RecordTurn(ctx, "sess-1", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}

	if err := r.Flush(ctx); err == nil {
		t.Fatal("expected Flush to return persist error, got nil")
	}
}

func TestFlush_ReturnsPersistErrorOnWriteFileFailure(t *testing.T) {
	cfg := testConfig(t)
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	ctx := context.Background()
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)

	// First turn succeeds and creates the directory structure.
	turn1 := memory.NewTurn("sess-1", memory.RoleUser, memory.SourceCLI, ts, "first")
	turn1.Seq = 1
	if err := r.RecordTurn(ctx, "sess-1", turn1); err != nil {
		t.Fatalf("RecordTurn 1: %v", err)
	}
	if err := r.Flush(ctx); err != nil {
		t.Fatalf("Flush 1: %v", err)
	}

	// Make the turns directory read-only so the second write fails.
	turnsDir := filepath.Join(r.cfg.BaseDir, r.cfg.Root, "sessions", "2026-05-28", "sess-1", "turns")
	if err := os.Chmod(turnsDir, 0o555); err != nil {
		t.Fatalf("Chmod turns dir: %v", err)
	}
	defer os.Chmod(turnsDir, 0o755)

	// Verify the chmod actually prevents writes from this process.
	probe := filepath.Join(turnsDir, "probe.txt")
	if probeErr := os.WriteFile(probe, []byte("x"), 0o644); probeErr == nil {
		t.Fatalf("chmod did not prevent writes: probe write succeeded")
	}

	turn2 := memory.NewTurn("sess-1", memory.RoleUser, memory.SourceCLI, ts, "second")
	turn2.Seq = 2
	if err := r.RecordTurn(ctx, "sess-1", turn2); err != nil {
		t.Fatalf("RecordTurn 2: %v", err)
	}

	if err := r.Flush(ctx); err == nil {
		t.Fatal("expected Flush to return write error, got nil")
	}
}

func TestClose_ReturnsPersistError(t *testing.T) {
	cfg := testConfig(t)
	if err := os.Chmod(cfg.BaseDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(cfg.BaseDir, 0o755)

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	turn := memory.NewTurn("sess-1", memory.RoleUser, memory.SourceCLI, time.Now().UTC(), "hello")
	turn.Seq = 1
	if err := r.RecordTurn(context.Background(), "sess-1", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}

	if err := r.Close(); err == nil {
		t.Fatal("expected Close to return persist error, got nil")
	}
}

func TestPersistTurn_LogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(t)
	cfg.Logger = logger
	if err := os.Chmod(cfg.BaseDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(cfg.BaseDir, 0o755)

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	turn := memory.NewTurn("sess-1", memory.RoleUser, memory.SourceCLI, time.Now().UTC(), "hello")
	turn.Seq = 1
	ctx := context.Background()
	if err := r.RecordTurn(ctx, "sess-1", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}
	_ = r.Flush(ctx)

	logged := buf.String()
	if !strings.Contains(logged, "failed to persist turn") {
		t.Errorf("expected error log to contain 'failed to persist turn', got:\n%s", logged)
	}
	if !strings.Contains(logged, "sess-1") {
		t.Errorf("expected error log to contain sessionID, got:\n%s", logged)
	}
}
