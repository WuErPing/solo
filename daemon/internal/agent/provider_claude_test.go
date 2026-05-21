package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
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

func (f *fakeProcessManager) Start(ctx context.Context, args []string, cwd string, env []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error) {
	return f.stdout, f.stderr, nil, f.cmd, nil
}

func (f *fakeProcessManager) Stop(cmd *exec.Cmd, timeout time.Duration) error { return nil }
func (f *fakeProcessManager) Interrupt(cmd *exec.Cmd) error                   { return nil }
func (f *fakeProcessManager) Kill(cmd *exec.Cmd) error                        { return nil }
func (f *fakeProcessManager) DrainStderr(stderr io.ReadCloser)                {}
func (f *fakeProcessManager) WaitForExit(cmd *exec.Cmd) (int, error)          { return 0, nil }

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
		accumulatedUsage: &protocol.AgentUsage{},
	}
	return s
}

func TestClaudeTerminalEventValueIsDispatcherCritical(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestClaudeSession(logger)
	translator := &claudeTranslator{session: sess, streamedContentBlocks: make(map[int]int)}

	events, _, err := translator.Translate([]byte(`{"type":"result","subtype":"success","session_id":"claude-test"}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}

	var terminal *AgentStreamEvent
	for _, raw := range events {
		evt, ok := raw.(AgentStreamEvent)
		if !ok {
			continue
		}
		payload, ok := evt.Event.(map[string]interface{})
		if !ok {
			continue
		}
		if payload["type"] == "turn_completed" {
			copied := evt
			terminal = &copied
			break
		}
	}
	if terminal == nil {
		t.Fatal("expected Claude result translation to emit turn_completed")
	}
	if !terminal.IsCriticalEvent() {
		t.Fatal("expected Claude turn_completed event to be critical")
	}
	if _, ok := interface{}(*terminal).(base.CriticalEvent); !ok {
		t.Fatal("Claude terminal AgentStreamEvent value must be dispatcher-critical")
	}
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
		sess.Run(ctx1, "first", nil, nil)
	}()

	// Give first Run time to acquire the turn.
	time.Sleep(50 * time.Millisecond)

	// Second Run should be rejected immediately.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	_, err := sess.Run(ctx2, "second", nil, nil)
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
		sess.Run(ctx, "first", nil, nil)
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
		accumulatedUsage: &protocol.AgentUsage{},
	}

	// Initially empty.
	sess.mu.Lock()
	empty := sess.activeTurnID == ""
	sess.mu.Unlock()
	if !empty {
		t.Fatal("expected empty activeTurnID initially")
	}

	// Start Run; it blocks on the fake pipe.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sess.Run(ctx, "test", nil, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	sess.mu.Lock()
	turnID := sess.activeTurnID
	sess.mu.Unlock()
	if turnID == "" {
		t.Fatal("expected activeTurnID to be set during Run")
	}
	if !strings.HasPrefix(turnID, "claude-turn-") {
		t.Fatalf("expected turnID to start with 'claude-turn-', got: %s", turnID)
	}

	// Cancel and close the pipe so Run exits.
	cancel()
	pw.Close()
	time.Sleep(100 * time.Millisecond)

	sess.mu.Lock()
	cleared := sess.activeTurnID == ""
	sess.mu.Unlock()
	if !cleared {
		t.Fatalf("expected activeTurnID to be cleared after Run, got: %s", sess.activeTurnID)
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
		sess.Run(ctx, "first", nil, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	// Because the first Run holds the lock, a second Run must be rejected
	// before it can overwrite stdoutPipe.
	_, err := sess.Run(context.Background(), "second", nil, nil)
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
		sess.Run(ctx, "test", nil, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	sess.mu.Lock()
	active := sess.activeTurnID != ""
	sess.mu.Unlock()
	if !active {
		t.Fatal("expected activeTurnID to be set")
	}

	// Close should kill the process and clear state.
	sess.Close()

	sess.mu.Lock()
	cleared := sess.activeTurnID == ""
	sess.mu.Unlock()
	if !cleared {
		t.Fatalf("expected activeTurnID cleared after Close, got: %s", sess.activeTurnID)
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
		sess.Run(ctx, "test", nil, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	sess.Interrupt(context.Background())

	// After interrupt, activeTurnID should be cleared so a new Run can start.
	sess.mu.Lock()
	cleared := sess.activeTurnID == ""
	sess.mu.Unlock()
	if !cleared {
		t.Fatalf("expected activeTurnID cleared after Interrupt, got: %s", sess.activeTurnID)
	}
}

// TestClaudeSession_ConcurrentRunAndInterrupt_NoRace runs Run and Interrupt
// concurrently to ensure the mutex prevents data races.
func TestClaudeSession_ConcurrentRunAndInterrupt_NoRace(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for i := 0; i < 100; i++ {
		sess := newTestClaudeSession(logger)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			sess.Run(ctx, "test", nil, nil)
		}()

		go func() {
			time.Sleep(time.Duration(i%10) * time.Millisecond)
			sess.Interrupt(context.Background())
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()
		time.Sleep(10 * time.Millisecond)

		// No assertion needed — if there is a race, the race detector will flag it.
	}
}
