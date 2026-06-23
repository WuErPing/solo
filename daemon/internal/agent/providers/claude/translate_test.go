package claude

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// newTestTranslator creates a claudeTranslator wired to a fake session.
func newTestTranslator(t *testing.T) *claudeTranslator {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		dispatcher:       base.NewChannelDispatcher(logger),
		permissions:      base.NewPermissionManager(),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	return &claudeTranslator{
		session:               sess,
		messageID:             "test-msg",
		streamedContentBlocks: make(map[int]int),
	}
}

// --- accumulateUsage ---

func TestAccumulateUsage_Nil(t *testing.T) {
	sess := &claudeSession{accumulatedUsage: &protocol.AgentUsage{}}
	sess.accumulateUsage(nil)
	// Should not panic; all fields remain nil
	if sess.accumulatedUsage.InputTokens != nil {
		t.Error("expected nil InputTokens after nil accumulate")
	}
}

func TestAccumulateUsage_Merges(t *testing.T) {
	sess := &claudeSession{accumulatedUsage: &protocol.AgentUsage{}}
	in1, out1, cost1 := 100.0, 50.0, 0.01
	sess.accumulateUsage(&protocol.AgentUsage{
		InputTokens:  &in1,
		OutputTokens: &out1,
		TotalCostUSD: &cost1,
	})

	if *sess.accumulatedUsage.InputTokens != 100.0 {
		t.Errorf("expected InputTokens=100, got %v", *sess.accumulatedUsage.InputTokens)
	}
	if *sess.accumulatedUsage.OutputTokens != 50.0 {
		t.Errorf("expected OutputTokens=50, got %v", *sess.accumulatedUsage.OutputTokens)
	}

	// Second accumulation should add
	in2, out2, cost2 := 200.0, 75.0, 0.02
	sess.accumulateUsage(&protocol.AgentUsage{
		InputTokens:  &in2,
		OutputTokens: &out2,
		TotalCostUSD: &cost2,
	})

	if *sess.accumulatedUsage.InputTokens != 300.0 {
		t.Errorf("expected InputTokens=300, got %v", *sess.accumulatedUsage.InputTokens)
	}
	if *sess.accumulatedUsage.OutputTokens != 125.0 {
		t.Errorf("expected OutputTokens=125, got %v", *sess.accumulatedUsage.OutputTokens)
	}
	if *sess.accumulatedUsage.TotalCostUSD != 0.03 {
		t.Errorf("expected TotalCostUSD=0.03, got %v", *sess.accumulatedUsage.TotalCostUSD)
	}
}

// --- buildArgs ---

func TestBuildArgs_BasicPrompt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	args := sess.buildArgs("hello world")

	found := false
	for _, a := range args {
		if a == "hello world" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prompt in args")
	}

	// Should always include --print and --output-format
	hasPrint, hasFormat := false, false
	for _, a := range args {
		if a == "--print" {
			hasPrint = true
		}
		if a == "--output-format" {
			hasFormat = true
		}
	}
	if !hasPrint || !hasFormat {
		t.Error("expected --print and --output-format flags")
	}
}

func TestBuildArgs_WithModel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &protocol.AgentSessionConfig{Provider: claudeProviderName}
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, cfg, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	sess.base.SetCurrentModel("claude-sonnet-4-20250514")

	args := sess.buildArgs("")
	found := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "claude-sonnet-4-20250514" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --model claude-sonnet-4-20250514 in args")
	}
}

// --- buildEnv ---

func TestBuildEnv_StripsClaudeVars(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}

	// Set a Claude env var that should be stripped
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("SOME_OTHER_VAR", "value")

	env := sess.buildEnv()

	hasClaudeCode := false
	hasOther := false
	for _, e := range env {
		if e == "CLAUDECODE=1" {
			hasClaudeCode = true
		}
		if e == "SOME_OTHER_VAR=value" {
			hasOther = true
		}
	}
	if hasClaudeCode {
		t.Error("expected CLAUDECODE to be stripped from env")
	}
	if !hasOther {
		t.Error("expected SOME_OTHER_VAR to be preserved in env")
	}
}

// --- Translate ---

func TestTranslate_EmptyInput(t *testing.T) {
	tr := newTestTranslator(t)
	events, isTerminal, err := tr.Translate([]byte{}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
	if isTerminal {
		t.Error("expected non-terminal for empty input")
	}
}

func TestTranslate_KeepAlive(t *testing.T) {
	tr := newTestTranslator(t)
	events, isTerminal, err := tr.Translate([]byte(`{"type":"keep_alive"}`), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for keep_alive, got %d", len(events))
	}
	if isTerminal {
		t.Error("expected non-terminal for keep_alive")
	}
}

func TestTranslate_InvalidJSON(t *testing.T) {
	tr := newTestTranslator(t)
	_, _, err := tr.Translate([]byte(`not json`), time.Now())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- translateResultMessage ---

func TestTranslateResultMessage_Success(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{
		Type:         "result",
		Subtype:      "success",
		TotalCostUSD: 0.05,
		Usage:        &sdkUsage{InputTokens: 100, OutputTokens: 50, CacheReadInputTokens: 25},
	}
	events := tr.translateResultMessage(msg, now)

	// Should emit usage_updated + turn_completed
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	var foundTurnCompleted bool
	var foundUsageUpdated bool
	for _, e := range events {
		evt, ok := e.(agent.AgentStreamEvent)
		if !ok {
			continue
		}
		switch evt.Event.(type) {
		case protocol.TurnCompletedStreamEvent:
			foundTurnCompleted = true
		case protocol.UsageUpdatedStreamEvent:
			foundUsageUpdated = true
		}
	}
	if !foundTurnCompleted {
		t.Error("expected TurnCompletedStreamEvent")
	}
	if !foundUsageUpdated {
		t.Error("expected UsageUpdatedStreamEvent")
	}
}

func TestTranslateResultMessage_Failure(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{
		Type:    "result",
		Subtype: "error",
		Errors:  []string{"something went wrong", "bad state"},
	}
	events := tr.translateResultMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	failed, ok := evt.Event.(protocol.TurnFailedStreamEvent)
	if !ok {
		t.Fatalf("expected TurnFailedStreamEvent, got %T", evt.Event)
	}
	if failed.Error != "something went wrong; bad state" {
		t.Errorf("expected joined errors, got %q", failed.Error)
	}
}

func TestTranslateResultMessage_IsTerminal(t *testing.T) {
	tr := newTestTranslator(t)
	raw := []byte(`{"type":"result","subtype":"success"}`)
	events, isTerminal, err := tr.Translate(raw, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isTerminal {
		t.Error("expected result message to be terminal")
	}
	_ = events
}

// --- translateSystemMessage ---

func TestTranslateSystemMessage_Init(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{Type: "system", Subtype: "init", SessionID: "sess-1", Model: "claude-sonnet-4", PermissionMode: "default"}
	events := tr.translateSystemMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	started, ok := evt.Event.(protocol.ThreadStartedStreamEvent)
	if !ok {
		t.Fatalf("expected ThreadStartedStreamEvent, got %T", evt.Event)
	}
	if started.SessionID != "sess-1" {
		t.Errorf("expected sessionID sess-1, got %q", started.SessionID)
	}
}

func TestTranslateSystemMessage_Compacting(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{Type: "system", Subtype: "status", Status: "compacting"}
	events := tr.translateSystemMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Type != "compaction" {
		t.Errorf("expected type compaction, got %q", timeline.Item.Type)
	}
}

func TestTranslateSystemMessage_TaskNotification(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{Type: "system", Subtype: "task_notification", TaskID: "task-1", TaskStatus: "completed", Summary: "done"}
	events := tr.translateSystemMessage(msg, now)

	// Should emit tool_call + todo
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
}

// --- translateUserMessage ---

func TestTranslateUserMessage(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	content := []sdkUserMessageContent{{Type: "text", Text: "hello"}}
	msgBytes, _ := json.Marshal(sdkUserMessage{Role: "user", Content: content})
	msg := sdkMessage{Type: "user", Message: msgBytes}

	events := tr.translateUserMessage(msg, now, "msg-1")

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Type != "user_message" {
		t.Errorf("expected type user_message, got %q", timeline.Item.Type)
	}
	if timeline.Item.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", timeline.Item.Text)
	}
}

func TestTranslateUserMessage_NilMessage(t *testing.T) {
	tr := newTestTranslator(t)
	events := tr.translateUserMessage(sdkMessage{Type: "user"}, time.Now(), "")
	if len(events) != 0 {
		t.Errorf("expected 0 events for nil message, got %d", len(events))
	}
}

// --- translateAssistantMessage ---

func TestTranslateAssistantMessage_TextBlock(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	content := []sdkContentBlock{{Type: "text", Text: "response text"}}
	msgBytes, _ := json.Marshal(sdkAssistantMessage{Role: "assistant", Content: content})
	msg := sdkMessage{Type: "assistant", Message: msgBytes}

	events := tr.translateAssistantMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Type != "assistant_message" {
		t.Errorf("expected type assistant_message, got %q", timeline.Item.Type)
	}
}

func TestTranslateAssistantMessage_ThinkingBlock(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	content := []sdkContentBlock{{Type: "thinking", Thinking: "let me think"}}
	msgBytes, _ := json.Marshal(sdkAssistantMessage{Role: "assistant", Content: content})
	msg := sdkMessage{Type: "assistant", Message: msgBytes}

	events := tr.translateAssistantMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Type != "reasoning" {
		t.Errorf("expected type reasoning, got %q", timeline.Item.Type)
	}
}

func TestTranslateAssistantMessage_ToolUseBlock(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	content := []sdkContentBlock{{Type: "tool_use", ID: "call-1", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)}}
	msgBytes, _ := json.Marshal(sdkAssistantMessage{Role: "assistant", Content: content})
	msg := sdkMessage{Type: "assistant", Message: msgBytes}

	events := tr.translateAssistantMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Type != "tool_call" {
		t.Errorf("expected type tool_call, got %q", timeline.Item.Type)
	}
	if timeline.Item.CallID != "call-1" {
		t.Errorf("expected callID call-1, got %q", timeline.Item.CallID)
	}
	if timeline.Item.Status != "completed" {
		t.Errorf("expected status completed, got %q", timeline.Item.Status)
	}
}

// --- translateStreamEvent ---

func TestTranslateStreamEvent_TextDelta(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	eventBytes, _ := json.Marshal(sdkStreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &sdkStreamDelta{Type: "text_delta", Text: "hello"},
	})
	msg := sdkMessage{Type: "stream_event", Event: eventBytes}

	events := tr.translateStreamEvent(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", timeline.Item.Text)
	}

	// Verify streamed content tracking
	if tr.streamedContentBlocks[0] != 5 {
		t.Errorf("expected streamedContentBlocks[0]=5, got %d", tr.streamedContentBlocks[0])
	}
}

func TestTranslateStreamEvent_ThinkingDelta(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	eventBytes, _ := json.Marshal(sdkStreamEvent{
		Type:  "content_block_delta",
		Index: 1,
		Delta: &sdkStreamDelta{Type: "thinking_delta", Thinking: "hmm"},
	})
	msg := sdkMessage{Type: "stream_event", Event: eventBytes}

	events := tr.translateStreamEvent(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Type != "reasoning" {
		t.Errorf("expected type reasoning, got %q", timeline.Item.Type)
	}
}

func TestTranslateStreamEvent_ToolUseStart(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	eventBytes, _ := json.Marshal(sdkStreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &sdkContentBlock{Type: "tool_use", ID: "call-2", Name: "read_file"},
	})
	msg := sdkMessage{Type: "stream_event", Event: eventBytes}

	events := tr.translateStreamEvent(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Status != "running" {
		t.Errorf("expected status running, got %q", timeline.Item.Status)
	}
}

func TestTranslateStreamEvent_NilEvent(t *testing.T) {
	tr := newTestTranslator(t)
	events := tr.translateStreamEvent(sdkMessage{Type: "stream_event"}, time.Now())
	if len(events) != 0 {
		t.Errorf("expected 0 events for nil event, got %d", len(events))
	}
}

// --- translatePermissionRequest ---

func TestTranslatePermissionRequest(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{Type: "permission_request", UUID: "perm-1", ToolName: "bash", Description: "run ls"}
	events := tr.translatePermissionRequest(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	perm, ok := evt.Event.(protocol.PermissionRequestedStreamEvent)
	if !ok {
		t.Fatalf("expected PermissionRequestedStreamEvent, got %T", evt.Event)
	}
	if perm.Request.ID != "perm-1" {
		t.Errorf("expected permission ID perm-1, got %q", perm.Request.ID)
	}
}

func TestTranslatePermissionRequest_NoID(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{Type: "permission_request"}
	events := tr.translatePermissionRequest(msg, now)

	if len(events) != 0 {
		t.Errorf("expected 0 events for permission with no ID, got %d", len(events))
	}
}

// --- translateMessage (dispatch) ---

func TestTranslateMessage_ToolProgress(t *testing.T) {
	tr := newTestTranslator(t)
	now := time.Now()

	msg := sdkMessage{Type: "tool_progress", ToolUseID: "call-1", ToolName: "bash"}
	events := tr.translateMessage(msg, now)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt, ok := events[0].(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", events[0])
	}
	timeline, ok := evt.Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
	}
	if timeline.Item.Status != "running" {
		t.Errorf("expected status running, got %q", timeline.Item.Status)
	}
}
