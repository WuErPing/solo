package claude

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/contracttest"
	"github.com/WuErPing/solo/protocol"
)

// fakeProcessManager is a test double that never starts real processes.
type fakeProcessManager struct {
	stdout io.ReadCloser
	stderr io.ReadCloser
	cmd    *exec.Cmd
}

func newFakeProcessManager(stdout io.ReadCloser, stderr io.ReadCloser, cmd *exec.Cmd) *fakeProcessManager {
	return &fakeProcessManager{stdout: stdout, stderr: stderr, cmd: cmd}
}

func (f *fakeProcessManager) Start(_ context.Context, _ []string, _ string, _ []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error) {
	return f.stdout, f.stderr, nil, f.cmd, nil
}

func (f *fakeProcessManager) Stop(_ *exec.Cmd, _ time.Duration) error { return nil }
func (f *fakeProcessManager) Interrupt(_ *exec.Cmd) error             { return nil }
func (f *fakeProcessManager) Kill(_ *exec.Cmd) error                  { return nil }
func (f *fakeProcessManager) DrainStderr(_ io.ReadCloser)             {}
func (f *fakeProcessManager) WaitForExit(_ *exec.Cmd) (int, error)    { return 0, nil }

// newTestClaudeSession creates a claudeSession wired to a fake process manager
// so tests can observe concurrency behaviour without launching real binaries.
func newTestClaudeSession(logger *slog.Logger) *claudeSession {
	pr, _ := io.Pipe()
	fakeCmd := exec.Command("sleep", "3600") // never finishes during test
	s := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		dispatcher:       base.NewChannelDispatcher(logger),
		permissions:      base.NewPermissionManager(),
		process:          newFakeProcessManager(pr, io.NopCloser(nil), fakeCmd),
		binaryPath:       "fake-claude",
		turnGuard:        base.NewTurnGuard(),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	return s
}

// TestClaudeSession_Run_RejectsConcurrentRun verifies that a second Run fails
// while a foreground turn is already active.
func TestClaudeSession_Run_RejectsConcurrentRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := newTestClaudeSession(logger)

	// Start first Run in background; it will block reading from the fake pipe.
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	go func() {
		sess.Run(ctx1, "first", nil, nil, "")
	}()

	// Give first Run time to acquire the turn.
	time.Sleep(50 * time.Millisecond)

	// Second Run should be rejected immediately.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	_, err := sess.Run(ctx2, "second", nil, nil, "")
	if err == nil {
		t.Fatal("expected concurrent Run to fail, got nil")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected 'already active' error, got: %v", err)
	}

	cancel1()
}

// TestClaudeSession_StartTurn_RejectsWhenRunActive verifies StartTurn fails
// when Run is already in progress.
func TestClaudeSession_StartTurn_RejectsWhenRunActive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := newTestClaudeSession(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sess.Run(ctx, "first", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)

	_, err := sess.StartTurn(context.Background(), "second", nil, nil)
	if err == nil {
		t.Fatal("expected StartTurn to fail when Run active, got nil")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected 'already active' error, got: %v", err)
	}
}

// TestClaudeSession_Run_SetsAndClearsActiveTurnID verifies that activeTurnID
// is populated during Run and cleared afterwards.
func TestClaudeSession_Run_SetsAndClearsActiveTurnID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use a pipe so we can unblock the pump by closing the writer.
	pr, pw := io.Pipe()
	fakeCmd := exec.Command("sleep", "3600")
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		dispatcher:       base.NewChannelDispatcher(logger),
		permissions:      base.NewPermissionManager(),
		process:          newFakeProcessManager(pr, io.NopCloser(nil), fakeCmd),
		binaryPath:       "fake-claude",
		turnGuard:        base.NewTurnGuard(),
		accumulatedUsage: &protocol.AgentUsage{},
	}

	// Initially inactive.
	if sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard to be inactive initially")
	}

	// Start Run; it blocks on the fake pipe.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sess.Run(ctx, "test", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)

	if !sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard to be active during Run")
	}

	// Cancel and close the pipe so Run exits.
	cancel()
	pw.Close()
	time.Sleep(100 * time.Millisecond)

	if sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard to be cleared after Run")
	}
}

// TestClaudeSession_Run_CapturesStdoutPipeUnderLock ensures that the stdout
// pipe used by the pump is the one present at the moment the lock is held,
// preventing a concurrent Run from swapping it mid-flight.
func TestClaudeSession_Run_CapturesStdoutPipeUnderLock(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := newTestClaudeSession(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sess.Run(ctx, "first", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)

	// Because the first Run holds the lock, a second Run must be rejected
	// before it can overwrite stdoutPipe.
	_, err := sess.Run(context.Background(), "second", nil, nil, "")
	if err == nil {
		t.Fatal("expected second Run to be rejected, got nil")
	}
}

// TestClaudeSession_Close_ClearsActiveTurnID verifies that Close cleans up
// an in-flight turn so the session can be reused safely.
func TestClaudeSession_Close_ClearsActiveTurnID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := newTestClaudeSession(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sess.Run(ctx, "test", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)

	if !sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard to be active")
	}

	// Close should kill the process and clear state.
	sess.Close()

	if sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard cleared after Close")
	}
}

// TestClaudeSession_Interrupt_ClearsActiveTurnID verifies that Interrupt
// cancels the current turn and releases the turn lock.
func TestClaudeSession_Interrupt_ClearsActiveTurnID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := newTestClaudeSession(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sess.Run(ctx, "test", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)

	sess.Interrupt(context.Background())

	// After interrupt, turn guard should be cleared so a new Run can start.
	if sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard cleared after Interrupt")
	}
}

// TestClaudeSession_ConcurrentRunAndInterrupt_NoRace runs Run and Interrupt
// concurrently to ensure the mutex prevents data races, and asserts that an
// interrupted Run always terminates — returning either an error or a result
// marked Canceled — instead of hanging on the fake (never-ending) process.
func TestClaudeSession_ConcurrentRunAndInterrupt_NoRace(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for i := 0; i < 100; i++ {
		sess := newTestClaudeSession(logger)

		ctx, cancel := context.WithCancel(context.Background())

		type runOutcome struct {
			result *agent.AgentRunResult
			err    error
		}
		outcomeCh := make(chan runOutcome, 1)
		go func() {
			result, err := sess.Run(ctx, "test", nil, nil, "")
			outcomeCh <- runOutcome{result: result, err: err}
		}()

		go func() {
			time.Sleep(time.Duration(i%10) * time.Millisecond)
			sess.Interrupt(context.Background())
		}()

		cancelTimer := time.AfterFunc(20*time.Millisecond, cancel)

		select {
		case outcome := <-outcomeCh:
			if outcome.err == nil && (outcome.result == nil || !outcome.result.Canceled) {
				t.Errorf("iteration %d: interrupted Run returned no error and result=%+v, want Canceled or error", i, outcome.result)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: Run did not return after Interrupt", i)
		}
		cancelTimer.Stop()
		cancel()
	}
}

func TestProviderContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Claude contract test in short mode")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient("", logger)
	contracttest.RunProviderContractSuite(t, "claude", client)
}
