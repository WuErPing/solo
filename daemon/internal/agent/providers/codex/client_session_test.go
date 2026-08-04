package codex

// Session-level tests for codexSession: Run/StartTurn/Interrupt/Close and
// session-resume argument passing, all driven through a fake process manager
// so no real codex binary or network is involved. Modelled on the Claude
// provider's client_test.go fake-process-manager pattern and the contract
// assertions in daemon/internal/agent/provider_contract_test.go.

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// fakeProcessManager is a test double that never starts real processes.
// It records the args passed to Start (for resume-argument assertions) and
// whether Interrupt/Kill were invoked (for Interrupt/Close assertions).
type fakeProcessManager struct {
	stdout io.ReadCloser
	stderr io.ReadCloser
	stdin  io.WriteCloser
	cmd    *exec.Cmd

	mu          sync.Mutex
	lastArgs    []string
	interrupted bool
	killed      bool
}

func newFakeProcessManager(stdout io.ReadCloser, cmd *exec.Cmd) *fakeProcessManager {
	return &fakeProcessManager{
		stdout: stdout,
		stderr: io.NopCloser(nil),
		stdin:  nopWriteCloser{},
		cmd:    cmd,
	}
}

// nopWriteCloser discards writes; codexSession.startProcessLocked closes stdin
// unconditionally, so the fake must return a non-nil WriteCloser.
type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

func (f *fakeProcessManager) Start(_ context.Context, args []string, _ string, _ []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error) {
	f.mu.Lock()
	f.lastArgs = append([]string(nil), args...)
	f.mu.Unlock()
	return f.stdout, f.stderr, f.stdin, f.cmd, nil
}

func (f *fakeProcessManager) Stop(_ *exec.Cmd, _ time.Duration) error { return nil }

func (f *fakeProcessManager) Interrupt(_ *exec.Cmd) error {
	f.mu.Lock()
	f.interrupted = true
	f.mu.Unlock()
	return nil
}

func (f *fakeProcessManager) Kill(_ *exec.Cmd) error {
	f.mu.Lock()
	f.killed = true
	f.mu.Unlock()
	return nil
}

func (f *fakeProcessManager) DrainStderr(_ io.ReadCloser)          {}
func (f *fakeProcessManager) WaitForExit(_ *exec.Cmd) (int, error) { return 0, nil }

func (f *fakeProcessManager) args() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastArgs...)
}

func (f *fakeProcessManager) wasInterrupted() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.interrupted
}

func (f *fakeProcessManager) wasKilled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.killed
}

// newTestCodexSession creates a codexSession wired to a fake process manager
// whose stdout is the given reader. cmd must not be started so that
// ProcessState stays nil and the immediate-exit health check is skipped.
func newTestCodexSession(logger *slog.Logger, stdout io.ReadCloser, fake *fakeProcessManager) *codexSession {
	return &codexSession{
		base:       base.NewBaseSession(codexProviderName, &protocol.AgentSessionConfig{}, logger),
		dispatcher: base.NewChannelDispatcher(logger),
		process:    fake,
		binaryPath: "fake-codex",
		turnGuard:  base.NewTurnGuard(),
	}
}

func testSessionLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// blockingStdout returns a pipe whose writer is held by the caller; the pump
// blocks reading until the writer is closed or the run context is cancelled.
func blockingStdout() (io.ReadCloser, *io.PipeWriter) {
	return io.Pipe()
}

// scriptedStdout returns a reader that yields the given JSON lines then EOF.
func scriptedStdout(lines ...string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		for _, l := range lines {
			if _, err := pw.Write([]byte(l + "\n")); err != nil {
				return
			}
		}
		_ = pw.Close()
	}()
	return pr
}

// drainUntilTerminal collects events from ch until a terminal event arrives
// or the timeout elapses.
func drainUntilTerminal(t *testing.T, ch <-chan agent.AgentStreamEvent, timeout time.Duration) []agent.AgentStreamEvent {
	t.Helper()
	var events []agent.AgentStreamEvent
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, evt)
			switch evt.Event.(type) {
			case protocol.TurnCompletedStreamEvent,
				protocol.TurnFailedStreamEvent,
				protocol.TurnCanceledStreamEvent:
				return events
			}
		case <-deadline:
			t.Fatalf("timed out after %v waiting for terminal event; received %d events", timeout, len(events))
		}
	}
}

func indexOfThreadStarted(events []agent.AgentStreamEvent) int {
	for i, evt := range events {
		if _, ok := evt.Event.(protocol.ThreadStartedStreamEvent); ok {
			return i
		}
	}
	return -1
}

func indexOfUserMessage(events []agent.AgentStreamEvent) int {
	for i, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok && e.Item.Type == "user_message" {
			return i
		}
	}
	return -1
}

// TestCodexSession_Run_HappyPath verifies the full event sequence of a
// successful turn: thread_started → user_message → assistant deltas → usage →
// turn_completed, plus the Run result payload.
func TestCodexSession_Run_HappyPath(t *testing.T) {
	logger := testSessionLogger()
	stdout := scriptedStdout(
		`{"type":"thread.started","thread_id":"t-1"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Hello"}}`,
		`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":" world"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5}}`,
	)
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()

	ch := sess.Subscribe()

	type runOutcome struct {
		result *agent.AgentRunResult
		err    error
	}
	outcomeCh := make(chan runOutcome, 1)
	go func() {
		result, err := sess.Run(context.Background(), "say hi", nil, nil, "msg-1")
		outcomeCh <- runOutcome{result: result, err: err}
	}()

	events := drainUntilTerminal(t, ch, 5*time.Second)

	// Contract: thread_started → user_message → terminal ordering.
	threadIdx := indexOfThreadStarted(events)
	userIdx := indexOfUserMessage(events)
	if threadIdx < 0 {
		t.Error("no ThreadStartedStreamEvent emitted")
	}
	if userIdx < 0 {
		t.Error("no user_message timeline event emitted")
	}
	if threadIdx >= 0 && userIdx >= 0 && threadIdx > userIdx {
		t.Errorf("thread_started (idx %d) must precede user_message (idx %d)", threadIdx, userIdx)
	}
	last := events[len(events)-1]
	if _, ok := last.Event.(protocol.TurnCompletedStreamEvent); !ok {
		t.Errorf("terminal event: got %T, want TurnCompletedStreamEvent", last.Event)
	}

	// user_message echoes the prompt and the Run messageID.
	if userIdx >= 0 {
		item := events[userIdx].Event.(protocol.TimelineStreamEvent).Item
		if item.MessageID != "msg-1" {
			t.Errorf("user_message MessageID: got %q, want %q", item.MessageID, "msg-1")
		}
		if item.Text != "say hi" {
			t.Errorf("user_message Text: got %q, want %q", item.Text, "say hi")
		}
	}

	// Assistant deltas surfaced as timeline events.
	var assistantText string
	for _, evt := range events {
		if e, ok := evt.Event.(protocol.TimelineStreamEvent); ok && e.Item.Type == "assistant_message" {
			assistantText += e.Item.Text
		}
	}
	if assistantText != "Hello world" {
		t.Errorf("assistant text: got %q, want %q", assistantText, "Hello world")
	}

	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("Run returned error: %v", outcome.err)
		}
		if outcome.result == nil {
			t.Fatal("Run returned nil result")
		}
		if outcome.result.FinalText != "Hello world" {
			t.Errorf("FinalText: got %q, want %q", outcome.result.FinalText, "Hello world")
		}
		if outcome.result.Canceled {
			t.Error("expected Canceled=false on happy path")
		}
		if outcome.result.Usage == nil {
			t.Error("expected usage captured from ThreadTokenUsageUpdatedNotification")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after terminal event")
	}

	// Turn guard released so a follow-up turn may start.
	if sess.turnGuard.IsActive() {
		t.Error("expected turn guard released after Run completed")
	}
}

// TestCodexSession_Run_RejectsConcurrentRun verifies the turn guard rejects a
// second Run while a foreground turn is active.
func TestCodexSession_Run_RejectsConcurrentRun(t *testing.T) {
	logger := testSessionLogger()
	stdout, pw := blockingStdout()
	defer pw.Close()
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	go func() {
		_, _ = sess.Run(ctx1, "first", nil, nil, "")
	}()

	// startProcessLocked sleeps 100ms for the health check before pumping.
	time.Sleep(200 * time.Millisecond)

	_, err := sess.Run(context.Background(), "second", nil, nil, "")
	if err == nil {
		t.Fatal("expected concurrent Run to fail, got nil")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected 'already active' error, got: %v", err)
	}

	// StartTurn is likewise rejected while Run holds the turn.
	if _, err := sess.StartTurn(context.Background(), "third", nil, nil); err == nil {
		t.Fatal("expected StartTurn to fail while Run active, got nil")
	}

	cancel1()
}

// TestCodexSession_StartTurn_StreamsEvents verifies StartTurn returns a
// channel that streams the turn's events through to the terminal event, and
// that a concurrent Run is rejected while the turn channel is open.
func TestCodexSession_StartTurn_StreamsEvents(t *testing.T) {
	logger := testSessionLogger()
	stdout := scriptedStdout(
		`{"type":"thread.started","thread_id":"t-2"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"hi"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
	)
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()

	ch, err := sess.StartTurn(context.Background(), "hello", nil, nil)
	if err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	events := drainUntilTerminal(t, ch, 5*time.Second)
	if indexOfThreadStarted(events) < 0 {
		t.Error("no ThreadStartedStreamEvent on StartTurn channel")
	}
	last := events[len(events)-1]
	if _, ok := last.Event.(protocol.TurnCompletedStreamEvent); !ok {
		t.Errorf("terminal event: got %T, want TurnCompletedStreamEvent", last.Event)
	}

	// Status quo (same as Claude's StartTurn): the turn guard stays held
	// until Interrupt/Close because the forwarding goroutine only releases
	// it when the dispatcher subscription ends. A second Run is rejected.
	if _, err := sess.Run(context.Background(), "again", nil, nil, ""); err == nil {
		t.Error("expected Run to be rejected while StartTurn turn is open, got nil")
	}
}

// TestCodexSession_Interrupt_CancelsTurn verifies Interrupt cancels the run
// context, interrupts the child process, releases the turn guard, and emits a
// turn_canceled event.
func TestCodexSession_Interrupt_CancelsTurn(t *testing.T) {
	logger := testSessionLogger()
	stdout, pw := blockingStdout()
	defer pw.Close()
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()

	ch := sess.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outcomeCh := make(chan error, 1)
	go func() {
		_, err := sess.Run(ctx, "work", nil, nil, "")
		outcomeCh <- err
	}()

	time.Sleep(200 * time.Millisecond)
	if !sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard active during Run")
	}

	if err := sess.Interrupt(context.Background()); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	// Interrupt emits turn_canceled to subscribers.
	deadline := time.After(2 * time.Second)
	sawCanceled := false
	for !sawCanceled {
		select {
		case evt, ok := <-ch:
			if !ok {
				t.Fatal("subscription closed before turn_canceled")
			}
			if ce, ok := evt.Event.(protocol.TurnCanceledStreamEvent); ok {
				sawCanceled = true
				if ce.Reason != "interrupted" {
					t.Errorf("turn_canceled reason: got %q, want %q", ce.Reason, "interrupted")
				}
			}
		case <-deadline:
			t.Fatal("no turn_canceled event after Interrupt")
		}
	}

	if !fake.wasInterrupted() {
		t.Error("expected process Interrupt to be called")
	}
	if sess.turnGuard.IsActive() {
		t.Error("expected turn guard released after Interrupt")
	}

	// The interrupted Run must terminate (context cancelled), not hang.
	select {
	case err := <-outcomeCh:
		if err == nil {
			t.Error("expected interrupted Run to return the cancelled-context error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Interrupt")
	}

	// Turn guard released: a new turn may start afterwards.
	stdout2 := scriptedStdout(
		`{"type":"thread.started","thread_id":"t-3"}`,
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
	)
	fake2 := newFakeProcessManager(stdout2, exec.Command("sleep", "3600"))
	sess.process = fake2
	if _, err := sess.Run(context.Background(), "next", nil, nil, ""); err != nil {
		t.Fatalf("Run after Interrupt: %v", err)
	}
}

// TestCodexSession_Close_CleansUp verifies Close kills the child process,
// releases the turn guard, and is idempotent.
func TestCodexSession_Close_CleansUp(t *testing.T) {
	logger := testSessionLogger()
	stdout, pw := blockingStdout()
	defer pw.Close()
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = sess.Run(ctx, "work", nil, nil, "")
	}()

	time.Sleep(200 * time.Millisecond)
	if !sess.turnGuard.IsActive() {
		t.Fatal("expected turn guard active during Run")
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !fake.wasKilled() {
		t.Error("expected process Kill to be called on Close")
	}
	if sess.turnGuard.IsActive() {
		t.Error("expected turn guard released after Close")
	}
	if !sess.base.IsClosed() {
		t.Error("expected base session marked closed")
	}

	// Close is idempotent.
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestCodexSession_Run_ResumeArgs verifies that a session with an existing
// session ID spawns codex with the resume subcommand and omits the prompt
// from the argument list.
func TestCodexSession_Run_ResumeArgs(t *testing.T) {
	logger := testSessionLogger()
	stdout := scriptedStdout(
		`{"type":"thread.started","thread_id":"t-4"}`,
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
	)
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()
	sess.base.SetSessionID("sess-abc-123")

	if _, err := sess.Run(context.Background(), "continue please", nil, nil, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := fake.args()
	// codex-cli 0.146: resume is a subcommand of exec and accepts the prompt
	// positionally after the session id; without it the turn's text is lost.
	assertContains(t, args, "exec")
	assertContains(t, args, "resume")
	assertContains(t, args, "sess-abc-123")
	assertContains(t, args, "continue please")
}

// TestCodexSession_Run_ExecArgsForNewSession verifies a fresh session (no
// session ID) uses the exec subcommand and passes the prompt positionally.
func TestCodexSession_Run_ExecArgsForNewSession(t *testing.T) {
	logger := testSessionLogger()
	stdout := scriptedStdout(
		`{"type":"thread.started","thread_id":"t-5"}`,
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
	)
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()

	if _, err := sess.Run(context.Background(), "hello codex", nil, nil, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := fake.args()
	if len(args) == 0 || args[0] != "exec" {
		t.Errorf("expected first arg %q, got %v", "exec", args)
	}
	assertContains(t, args, "hello codex")
	assertNotContains(t, args, "resume")
}

// TestCodexSession_Run_WritesBackRealThreadID pins the fix for the defect
// where the real thread_id carried by thread.started was never written back
// to the session, so codex native resume could never be used for sessions
// created via CreateSession. After a turn completes, the session must hold
// the real thread id, and the next Run must resume with it — the same
// writeback pattern claude (system init message) and pi (session event) use.
func TestCodexSession_Run_WritesBackRealThreadID(t *testing.T) {
	logger := testSessionLogger()
	stdout := scriptedStdout(
		`{"type":"thread.started","thread_id":"real-thread-xyz"}`,
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
	)
	fake := newFakeProcessManager(stdout, exec.Command("sleep", "3600"))
	sess := newTestCodexSession(logger, stdout, fake)
	defer sess.Close()

	result, err := sess.Run(context.Background(), "first turn", nil, nil, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := sess.base.SessionID(); got != "real-thread-xyz" {
		t.Fatalf("session ID not written back from thread.started: got %q, want %q", got, "real-thread-xyz")
	}
	if result.SessionID != "real-thread-xyz" {
		t.Errorf("run result SessionID: got %q, want %q", result.SessionID, "real-thread-xyz")
	}

	// The next turn must resume with the real thread id.
	stdout2 := scriptedStdout(
		`{"type":"thread.started","thread_id":"real-thread-xyz"}`,
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
	)
	fake2 := newFakeProcessManager(stdout2, exec.Command("sleep", "3600"))
	sess.process = fake2
	if _, err := sess.Run(context.Background(), "second turn", nil, nil, ""); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	args := fake2.args()
	assertContains(t, args, "exec")
	assertContains(t, args, "resume")
	assertContains(t, args, "real-thread-xyz")
	assertContains(t, args, "second turn")
}
