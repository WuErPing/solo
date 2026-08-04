package codex

// Fixture-driven translator tests: the NDJSON fixtures in testdata/ are real
// stdout captures from codex-cli 0.146.0 (see testdata/README.md). They pin
// the translator to the schema the binary actually emits (thread.started /
// turn.started / item.completed / turn.completed / turn.failed), which the
// pre-fix translator (TurnStartedNotification / AgentMessageDeltaNotification
// / ...) never matched — a real turn produced zero stream events.

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// translateFixture feeds every line of an NDJSON fixture through a translator
// and returns the flattened, unwrapped protocol events plus the terminal flag
// of the last line.
func translateFixture(t *testing.T, path, messageID, prompt string) ([]interface{}, bool, *codexTranslator) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	tr := newCodexTranslator(testSessionLogger(), messageID, prompt)
	var all []interface{}
	terminal := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		events, isTerminal, err := tr.Translate([]byte(line), time.Now())
		if err != nil {
			t.Fatalf("Translate(%q): %v", line, err)
		}
		all = append(all, unwrapStreamEvents(events)...)
		if isTerminal {
			terminal = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	return all, terminal, tr
}

func findEventIndex(events []interface{}, match func(interface{}) bool) int {
	for i, evt := range events {
		if match(evt) {
			return i
		}
	}
	return -1
}

func isThreadStarted(evt interface{}) bool {
	_, ok := evt.(protocol.ThreadStartedStreamEvent)
	return ok
}

func isUserMessage(evt interface{}) bool {
	e, ok := evt.(protocol.TimelineStreamEvent)
	return ok && e.Item.Type == "user_message"
}

func isAssistantMessage(evt interface{}) bool {
	e, ok := evt.(protocol.TimelineStreamEvent)
	return ok && e.Item.Type == "assistant_message"
}

func isTurnCompleted(evt interface{}) bool {
	_, ok := evt.(protocol.TurnCompletedStreamEvent)
	return ok
}

// TestCodexTranslator_RealStream_HappyPath drives the translator with the real
// "say hi" capture and asserts the full lifecycle: thread_started →
// user_message → assistant message → usage → turn_completed, plus FinalText
// and usage captured from turn.completed.
func TestCodexTranslator_RealStream_HappyPath(t *testing.T) {
	events, terminal, tr := translateFixture(t,
		"testdata/real_stream_v0.146.ndjson", "msg-fixture-1", "say hi")

	threadIdx := findEventIndex(events, isThreadStarted)
	userIdx := findEventIndex(events, isUserMessage)
	assistantIdx := findEventIndex(events, isAssistantMessage)
	completedIdx := findEventIndex(events, isTurnCompleted)

	if threadIdx < 0 {
		t.Fatalf("no ThreadStartedStreamEvent; got %v", eventTypesListFromInterface(events))
	}
	if userIdx < 0 {
		t.Fatalf("no user_message timeline event; got %v", eventTypesListFromInterface(events))
	}
	if assistantIdx < 0 {
		t.Fatalf("no assistant_message timeline event; got %v", eventTypesListFromInterface(events))
	}
	if completedIdx < 0 {
		t.Fatalf("no TurnCompletedStreamEvent; got %v", eventTypesListFromInterface(events))
	}
	if !(threadIdx < userIdx && userIdx < assistantIdx && assistantIdx < completedIdx) {
		t.Errorf("bad ordering: thread=%d user=%d assistant=%d completed=%d; events=%v",
			threadIdx, userIdx, assistantIdx, completedIdx, eventTypesListFromInterface(events))
	}
	if !terminal {
		t.Error("turn.completed must be terminal")
	}

	// ThreadStarted carries the real codex thread_id (used for resume).
	ts := events[threadIdx].(protocol.ThreadStartedStreamEvent)
	if ts.SessionID != "019fcb1b-c722-76b3-b736-e2869dc24007" {
		t.Errorf("ThreadStarted SessionID: got %q", ts.SessionID)
	}

	// user_message echoes prompt + messageID.
	um := events[userIdx].(protocol.TimelineStreamEvent).Item
	if um.MessageID != "msg-fixture-1" || um.Text != "say hi" {
		t.Errorf("user_message: got MessageID=%q Text=%q", um.MessageID, um.Text)
	}

	wantText := "Hi there! 👋 What can I help you with today?"
	am := events[assistantIdx].(protocol.TimelineStreamEvent).Item
	if am.Text != wantText {
		t.Errorf("assistant text: got %q, want %q", am.Text, wantText)
	}
	if tr.finalText() != wantText {
		t.Errorf("finalText: got %q, want %q", tr.finalText(), wantText)
	}

	// Usage arrives on turn.completed.
	usage := tr.lastUsage()
	if usage == nil {
		t.Fatal("expected usage captured from turn.completed")
	}
	if usage.InputTokens == nil || *usage.InputTokens != 13232 {
		t.Errorf("InputTokens: got %v, want 13232", usage.InputTokens)
	}
	if usage.CachedInputTokens == nil || *usage.CachedInputTokens != 13184 {
		t.Errorf("CachedInputTokens: got %v, want 13184", usage.CachedInputTokens)
	}
	if usage.OutputTokens == nil || *usage.OutputTokens != 36 {
		t.Errorf("OutputTokens: got %v, want 36", usage.OutputTokens)
	}
	tc := events[completedIdx].(protocol.TurnCompletedStreamEvent)
	if tc.Usage == nil {
		t.Error("TurnCompletedStreamEvent should carry the usage")
	}
}

// TestCodexTranslator_RealStream_ToolCall drives the real command-execution
// capture: item.started/item.completed of a command_execution item must
// surface as tool_call running → completed before the assistant message.
func TestCodexTranslator_RealStream_ToolCall(t *testing.T) {
	events, terminal, tr := translateFixture(t,
		"testdata/real_stream_tool_call_v0.146.ndjson", "msg-fixture-2", "run echo")

	var toolStatuses []string
	var toolNames []string
	for _, evt := range events {
		if e, ok := evt.(protocol.TimelineStreamEvent); ok && e.Item.Type == "tool_call" {
			toolStatuses = append(toolStatuses, e.Item.Status)
			toolNames = append(toolNames, e.Item.Name)
			if e.Item.CallID != "item_0" {
				t.Errorf("tool_call CallID: got %q, want item_0", e.Item.CallID)
			}
		}
	}
	if len(toolStatuses) < 2 || toolStatuses[0] != "running" || toolStatuses[1] != "completed" {
		t.Errorf("tool_call statuses: got %v, want [running completed]", toolStatuses)
	}

	assistantIdx := findEventIndex(events, isAssistantMessage)
	if assistantIdx < 0 {
		t.Fatalf("no assistant_message; got %v", eventTypesListFromInterface(events))
	}
	if !strings.Contains(tr.finalText(), "solo-fixture-test") {
		t.Errorf("finalText should contain command output report, got %q", tr.finalText())
	}
	if !terminal || findEventIndex(events, isTurnCompleted) < 0 {
		t.Errorf("expected terminal TurnCompletedStreamEvent; got %v", eventTypesListFromInterface(events))
	}
	_ = toolNames
}

// TestCodexTranslator_RealStream_Error drives the real failure capture: a
// turn.failed event must surface as a terminal TurnFailedStreamEvent carrying
// the error message.
func TestCodexTranslator_RealStream_Error(t *testing.T) {
	events, terminal, tr := translateFixture(t,
		"testdata/real_stream_error_v0.146.ndjson", "msg-fixture-3", "say hi")

	if !terminal {
		t.Fatalf("turn.failed must be terminal; got %v", eventTypesListFromInterface(events))
	}
	last := events[len(events)-1]
	failed, ok := last.(protocol.TurnFailedStreamEvent)
	if !ok {
		t.Fatalf("last event: got %T, want TurnFailedStreamEvent; events=%v", last, eventTypesListFromInterface(events))
	}
	if !strings.Contains(failed.Error, "invalid_request_error") {
		t.Errorf("TurnFailed error should carry codex's message, got %q", failed.Error)
	}
	// No assistant text on a failed turn.
	if tr.finalText() != "" {
		t.Errorf("finalText on failed turn: got %q, want empty", tr.finalText())
	}
	// thread.started still emitted before the failure.
	if findEventIndex(events, isThreadStarted) < 0 {
		t.Error("no ThreadStartedStreamEvent on error stream")
	}
}
