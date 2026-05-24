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

// fakePiProcessManager is a test double that never starts real processes.
type fakePiProcessManager struct {
	stdout io.ReadCloser
	stderr io.ReadCloser
	cmd    *exec.Cmd
}

func newFakePiProcessManager(stdout io.ReadCloser, stderr io.ReadCloser, cmd *exec.Cmd) *fakePiProcessManager {
	return &fakePiProcessManager{stdout: stdout, stderr: stderr, cmd: cmd}
}

func (f *fakePiProcessManager) Start(ctx context.Context, args []string, cwd string, env []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error) {
	return f.stdout, f.stderr, nil, f.cmd, nil
}
func (f *fakePiProcessManager) Stop(cmd *exec.Cmd, timeout time.Duration) error  { return nil }
func (f *fakePiProcessManager) Interrupt(cmd *exec.Cmd) error                   { return nil }
func (f *fakePiProcessManager) Kill(cmd *exec.Cmd) error                        { return nil }
func (f *fakePiProcessManager) DrainStderr(stderr io.ReadCloser)                {}
func (f *fakePiProcessManager) WaitForExit(cmd *exec.Cmd) (int, error)          { return 0, nil }

// newTestPiSession creates a piSession wired to a fake process manager.
func newTestPiSession(logger *slog.Logger) *piSession {
	pr, _ := io.Pipe()
	fakeCmd := exec.Command("sleep", "3600")
	s := &piSession{
		base:       base.NewBaseSession(piProviderName, &protocol.AgentSessionConfig{}, logger),
		dispatcher: base.NewChannelDispatcher(logger),
		process:    newFakePiProcessManager(pr, io.NopCloser(nil), fakeCmd),
		binaryPath: "fake-pi",
	}
	return s
}

// TestPiAgentClient_Provider returns the provider name.
func TestPiAgentClient_Provider(t *testing.T) {
	client := NewPiAgentClient("/fake/pi", slog.Default())
	if got := client.Provider(); got != "pi" {
		t.Fatalf("expected provider 'pi', got %q", got)
	}
}

// TestPiAgentClient_IsAvailable_BinaryNotFound fails when binary is missing.
func TestPiAgentClient_IsAvailable_BinaryNotFound(t *testing.T) {
	client := NewPiAgentClient("/nonexistent/pi", slog.Default())
	if err := client.IsAvailable(context.Background()); err == nil {
		t.Fatal("expected error when binary not found, got nil")
	}
}

// TestPiAgentClient_IsAvailable_BinaryFound succeeds when binary exists.
func TestPiAgentClient_IsAvailable_BinaryFound(t *testing.T) {
	client := NewPiAgentClient("/bin/echo", slog.Default())
	if err := client.IsAvailable(context.Background()); err != nil {
		t.Fatalf("expected no error when binary found, got %v", err)
	}
}

// TestPiSession_Run_RejectsConcurrentRun verifies that a second Run fails
// while a foreground turn is already active.
func TestPiSession_Run_RejectsConcurrentRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := newTestPiSession(logger)

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	go func() {
		sess.Run(ctx1, "first", nil, nil)
	}()

	time.Sleep(50 * time.Millisecond)

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

// TestPiTranslator_Session emits thread_started on session event.
func TestPiTranslator_Session(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"session","id":"pi-test-123","cwd":"/tmp"}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if isTerminal {
		t.Fatal("expected session event to not be terminal")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	streamEvt, ok := events[0].(AgentStreamEvent)
	if !ok {
		t.Fatalf("expected AgentStreamEvent, got %T", events[0])
	}
	payload, ok := streamEvt.Event.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map event, got %T", streamEvt.Event)
	}
	if payload["type"] != "thread_started" {
		t.Fatalf("expected thread_started, got %v", payload["type"])
	}
	if payload["sessionId"] != "pi-test-123" {
		t.Fatalf("expected sessionId 'pi-test-123', got %v", payload["sessionId"])
	}
	if payload["provider"] != "pi" {
		t.Fatalf("expected provider 'pi', got %v", payload["provider"])
	}
}

// TestPiTranslator_UserMessage emits user_message timeline on user message_start.
func TestPiTranslator_UserMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, _, err := translator.Translate([]byte(`{"type":"message_start","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	if payload["type"] != "timeline" {
		t.Fatalf("expected timeline, got %v", payload["type"])
	}
	item := payload["item"].(TimelineItem)
	if item.Type != "user_message" {
		t.Fatalf("expected user_message, got %s", item.Type)
	}
	if item.Text != "hello" {
		t.Fatalf("expected text 'hello', got %s", item.Text)
	}
}

// TestPiTranslator_ThinkingDelta emits reasoning timeline.
func TestPiTranslator_ThinkingDelta(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, _, err := translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"thinking..."}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	item := payload["item"].(TimelineItem)
	if item.Type != "reasoning" {
		t.Fatalf("expected reasoning, got %s", item.Type)
	}
	if item.Text != "thinking..." {
		t.Fatalf("expected text 'thinking...', got %s", item.Text)
	}
}

// TestPiTranslator_TextDelta emits assistant_message timeline.
func TestPiTranslator_TextDelta(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, _, err := translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"Hello world"}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	item := payload["item"].(TimelineItem)
	if item.Type != "assistant_message" {
		t.Fatalf("expected assistant_message, got %s", item.Type)
	}
	if item.Text != "Hello world" {
		t.Fatalf("expected text 'Hello world', got %s", item.Text)
	}
}

// TestPiTranslator_ToolCall emits tool_call timeline events.
func TestPiTranslator_ToolCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	// tool_call_start
	events, _, err := translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"tool_call_start","toolCall":{"id":"tc-1","name":"read","arguments":"{\"file\":\"test.go\"}"}}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	item := payload["item"].(TimelineItem)
	if item.Type != "tool_call" {
		t.Fatalf("expected tool_call, got %s", item.Type)
	}
	if item.Status != "running" {
		t.Fatalf("expected status running, got %s", item.Status)
	}
	if item.Detail == nil {
		t.Fatal("expected detail for tool_call_start")
	}
	if item.Error != nil {
		t.Fatalf("expected nil error for running tool call, got %v", item.Error)
	}

	// tool_call_end
	events, _, err = translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"tool_call_end","toolCall":{"id":"tc-1","name":"read"}}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	streamEvt = events[0].(AgentStreamEvent)
	payload = streamEvt.Event.(map[string]interface{})
	item = payload["item"].(TimelineItem)
	if item.Status != "completed" {
		t.Fatalf("expected status completed, got %s", item.Status)
	}
	if item.Detail == nil {
		t.Fatal("expected detail for tool_call_end")
	}
	if item.Error != nil {
		t.Fatalf("expected nil error for completed tool call, got %v", item.Error)
	}
}

// TestPiTranslator_TurnEnd emits turn_completed with usage.
func TestPiTranslator_TurnEnd(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"turn_end","message":{"role":"assistant","usage":{"input":10,"output":20,"cacheRead":5,"totalTokens":30,"cost":{"input":0.00001,"output":0.00002,"cacheRead":0.000005,"total":0.000035}}}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if !isTerminal {
		t.Fatal("expected turn_end to be terminal")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	if payload["type"] != "turn_completed" {
		t.Fatalf("expected turn_completed, got %v", payload["type"])
	}
	usage, ok := payload["usage"].(*protocol.AgentUsage)
	if !ok {
		t.Fatalf("expected usage, got %T", payload["usage"])
	}
	if *usage.InputTokens != 10 {
		t.Fatalf("expected input tokens 10, got %v", *usage.InputTokens)
	}
	if *usage.OutputTokens != 20 {
		t.Fatalf("expected output tokens 20, got %v", *usage.OutputTokens)
	}
	if *usage.CachedInputTokens != 5 {
		t.Fatalf("expected cached tokens 5, got %v", *usage.CachedInputTokens)
	}
}

// TestPiTranslator_TurnEnd_NoUsage emits turn_completed without usage when nil.
func TestPiTranslator_TurnEnd_NoUsage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"turn_end"}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if !isTerminal {
		t.Fatal("expected turn_end to be terminal")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	if payload["type"] != "turn_completed" {
		t.Fatalf("expected turn_completed, got %v", payload["type"])
	}
	if _, hasUsage := payload["usage"]; hasUsage {
		t.Fatal("expected usage to be omitted when nil")
	}
}

// TestPiTerminalDetector_TurnEnd detects turn_completed as terminal.
func TestPiTerminalDetector_TurnEnd(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	detector := &piTerminalDetector{session: sess}

	evt := AgentStreamEvent{
		Event: map[string]interface{}{"type": "turn_completed", "provider": "pi"},
	}
	result, err, isTerminal := detector.IsTerminal(evt)
	if !isTerminal {
		t.Fatal("expected turn_completed to be terminal")
	}
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestPiTerminalEventValueIsDispatcherCritical verifies that the terminal
// event emitted by the PI translator is recognized as critical by the dispatcher.
func TestPiTerminalEventValueIsDispatcherCritical(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, _, err := translator.Translate([]byte(`{"type":"turn_end"}`), time.Now())
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
		t.Fatal("expected PI turn_end translation to emit turn_completed")
	}
	if !terminal.IsCriticalEvent() {
		t.Fatal("expected PI turn_completed event to be critical")
	}
	if _, ok := interface{}(*terminal).(base.CriticalEvent); !ok {
		t.Fatal("PI terminal AgentStreamEvent value must be dispatcher-critical")
	}
}
