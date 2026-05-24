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
// Pi uses 'toolcall_start' / 'toolcall_end' (no underscore) with 'partial' / 'toolCall' fields.
func TestPiTranslator_ToolCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	// toolcall_start with 'partial' field (Pi's actual format)
	events, _, err := translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start","contentIndex":1,"partial":{"type":"toolCall","id":"tc-1","name":"read","arguments":null}}}`), time.Now())
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
		t.Fatal("expected detail for toolcall_start")
	}
	if item.Error != nil {
		t.Fatalf("expected nil error for running tool call, got %v", item.Error)
	}

	// toolcall_end with 'toolCall' field (Pi's actual format)
	events, _, err = translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","contentIndex":1,"toolCall":{"type":"toolCall","id":"tc-1","name":"read","arguments":"{\"file\":\"test.go\"}"}}}`), time.Now())
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
		t.Fatal("expected detail for toolcall_end")
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

// TestPiTranslator_TurnEnd_WithText emits assistant_message from turn_end
// when no text_delta was seen (tool-call turn pattern).
func TestPiTranslator_TurnEnd_WithText(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"Here is the result."}]}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if !isTerminal {
		t.Fatal("expected turn_end to be terminal")
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (assistant_message + turn_completed), got %d", len(events))
	}

	// First event should be assistant_message
	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	if payload["type"] != "timeline" {
		t.Fatalf("expected timeline, got %v", payload["type"])
	}
	item := payload["item"].(TimelineItem)
	if item.Type != "assistant_message" {
		t.Fatalf("expected assistant_message, got %s", item.Type)
	}
	if item.Text != "Here is the result." {
		t.Fatalf("expected text 'Here is the result.', got %s", item.Text)
	}

	// Second event should be turn_completed
	streamEvt = events[1].(AgentStreamEvent)
	payload = streamEvt.Event.(map[string]interface{})
	if payload["type"] != "turn_completed" {
		t.Fatalf("expected turn_completed, got %v", payload["type"])
	}
}

// TestPiTranslator_TurnEnd_NoDuplicateText verifies that turn_end does NOT
// emit assistant_message when text_delta was already emitted.
func TestPiTranslator_TurnEnd_NoDuplicateText(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	// text_delta sets textEmitted = true
	_, _, err := translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"Hello"}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}

	// turn_end should NOT emit assistant_message because text was already emitted
	events, _, err := translator.Translate([]byte(`{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}]}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (turn_completed only), got %d", len(events))
	}
	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	if payload["type"] != "turn_completed" {
		t.Fatalf("expected turn_completed, got %v", payload["type"])
	}
}

// TestPiTranslator_MessageEnd_WithText emits assistant_message from message_end
// when no text_delta was seen.
func TestPiTranslator_MessageEnd_WithText(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	// assistant message_start resets textEmitted
	_, _, err := translator.Translate([]byte(`{"type":"message_start","message":{"role":"assistant","content":[]}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}

	// message_end should emit assistant_message because no text_delta was seen
	events, _, err := translator.Translate([]byte(`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Response text"}],"usage":{"input":1,"output":2,"totalTokens":3,"cost":{"total":0.001}}}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (assistant_message + usage_updated), got %d", len(events))
	}

	streamEvt := events[0].(AgentStreamEvent)
	payload := streamEvt.Event.(map[string]interface{})
	if payload["type"] != "timeline" {
		t.Fatalf("expected timeline, got %v", payload["type"])
	}
	item := payload["item"].(TimelineItem)
	if item.Type != "assistant_message" {
		t.Fatalf("expected assistant_message, got %s", item.Type)
	}
	if item.Text != "Response text" {
		t.Fatalf("expected text 'Response text', got %s", item.Text)
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

// TestPiTranslator_TurnEnd_ToolUse_NotTerminal verifies that turn_end with
// stopReason "toolUse" is NOT treated as terminal. This is the core bug: when
// querying "date", Pi runs a tool (bash) and emits an intermediate turn_end
// with stopReason="toolUse", followed by a second turn with the actual answer.
// The translator must not stop processing at the intermediate turn_end.
func TestPiTranslator_TurnEnd_ToolUse_NotTerminal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"turn_end","message":{"role":"assistant","content":[{"type":"thinking","thinking":"thinking...","thinkingSignature":"reasoning_content"},{"type":"toolCall","id":"call_1","name":"bash","arguments":{"command":"date"}}],"stopReason":"toolUse"}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if isTerminal {
		t.Fatal("turn_end with stopReason=toolUse must NOT be terminal — it is an intermediate turn before the final response")
	}
	// Should emit no turn_completed event for a toolUse turn
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
			t.Fatal("turn_end with stopReason=toolUse must not emit turn_completed")
		}
	}
}

// TestPiTranslator_TurnEnd_Stop_IsTerminal verifies that turn_end with
// stopReason "stop" IS treated as terminal (the final response turn).
func TestPiTranslator_TurnEnd_Stop_IsTerminal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"Today is Sunday."}],"stopReason":"stop","usage":{"input":10,"output":5,"totalTokens":15,"cost":{"total":0.00001}}}}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if !isTerminal {
		t.Fatal("turn_end with stopReason=stop must be terminal")
	}
	// Should emit turn_completed
	var gotTurnCompleted bool
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
			gotTurnCompleted = true
		}
	}
	if !gotTurnCompleted {
		t.Fatal("turn_end with stopReason=stop must emit turn_completed")
	}
}

// TestPiTranslator_TurnEnd_EmptyStopReason_IsTerminal verifies that turn_end
// without stopReason (legacy/unknown) is still treated as terminal for
// backwards compatibility.
func TestPiTranslator_TurnEnd_EmptyStopReason_IsTerminal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	events, isTerminal, err := translator.Translate([]byte(`{"type":"turn_end"}`), time.Now())
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if !isTerminal {
		t.Fatal("turn_end without stopReason must remain terminal (backwards compat)")
	}
	_ = events
}

// TestPiTranslator_TurnStart_ResetsTurnState verifies that a new turn_start
// resets the textEmitted state so the second turn in a tool-use sequence
// correctly emits the assistant response.
func TestPiTranslator_TurnStart_ResetsTurnState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestPiSession(logger)
	translator := &piTranslator{session: sess}

	// Simulate: text_delta in turn 1 sets textEmitted=true
	_, _, _ = translator.Translate([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"thinking text"}}`), time.Now())
	if !translator.textEmitted {
		t.Fatal("textEmitted should be true after text_delta")
	}

	// turn_start for second turn should reset textEmitted
	_, _, _ = translator.Translate([]byte(`{"type":"turn_start"}`), time.Now())
	if translator.textEmitted {
		t.Fatal("textEmitted must be reset to false on turn_start")
	}

	// Now turn_end with text in message should emit assistant_message
	events, _, _ := translator.Translate([]byte(`{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"Today is Sunday, May 24 2026."}],"stopReason":"stop"}}`), time.Now())
	var gotText bool
	for _, raw := range events {
		evt, ok := raw.(AgentStreamEvent)
		if !ok {
			continue
		}
		payload, ok := evt.Event.(map[string]interface{})
		if !ok {
			continue
		}
		if payload["type"] == "timeline" {
			item, ok := payload["item"].(TimelineItem)
			if ok && item.Type == "assistant_message" && item.Text == "Today is Sunday, May 24 2026." {
				gotText = true
			}
		}
	}
	if !gotText {
		t.Fatal("second turn's turn_end must emit the assistant_message text")
	}
}
