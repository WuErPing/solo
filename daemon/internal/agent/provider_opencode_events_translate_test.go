package agent

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// newTestOpenCodeSessionForTranslate creates a minimal openCodeSession for
// testing translate* methods. It does NOT start background goroutines.
func newTestOpenCodeSessionForTranslate(t *testing.T) *openCodeSession {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}
	s := &openCodeSession{
		baseURL:                 "http://localhost:0",
		base:                    base.NewBaseSession(opencodeProviderName, config, logger),
		dispatcher:              base.NewChannelDispatcher(logger),
		pendingPerms:            make(map[string]pendingPermission),
		messageRoles:            make(map[string]string),
		streamedPartKeys:        make(map[string]bool),
		emittedStructuredMsgIDs: make(map[string]bool),
		partTypes:               make(map[string]string),
		runningToolCalls:        make(map[string]timelineItem),
		accumulatedUsage:        &opencodeUsage{},
		modelContextWindows:     make(map[string]int),
		commandsReadyCh:         make(chan struct{}),
	}
	close(s.commandsReadyCh) // mark commands as ready
	return s
}

// rawFromJSON is a test helper that converts a JSON string into a
// map[string]json.RawMessage for use with translate* methods.
func rawFromJSON(t *testing.T, input string) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("rawFromJSON: %v", err)
	}
	return raw
}

// --- translateSessionCreatedOrUpdated ---

func TestTranslateSessionCreatedOrUpdated_MatchingSession(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","info":{"id":"`+s.base.SessionID()+`"}}}`)
	s.translateSessionCreatedOrUpdated(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.ThreadStartedStreamEvent)
	if !ok {
		t.Fatalf("expected ThreadStartedStreamEvent, got %T", emitted[0])
	}
	if evt.SessionID != s.base.SessionID() {
		t.Errorf("expected sessionID %q, got %q", s.base.SessionID(), evt.SessionID)
	}
}

func TestTranslateSessionCreatedOrUpdated_DifferentSession(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"other-session","info":{"id":"other-session"}}}`)
	s.translateSessionCreatedOrUpdated(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for different session, got %d", len(emitted))
	}
}

// --- translateSessionStatus ---

func TestTranslateSessionStatus_Idle(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","status":{"type":"idle"}}}`)
	s.translateSessionStatus(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	_, ok := emitted[0].(protocol.TurnCompletedStreamEvent)
	if !ok {
		t.Fatalf("expected TurnCompletedStreamEvent, got %T", emitted[0])
	}
}

func TestTranslateSessionStatus_RetryFatal(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","status":{"type":"retry","message":"insufficient balance"}}}`)
	s.translateSessionStatus(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	_, ok := emitted[0].(protocol.TurnFailedStreamEvent)
	if !ok {
		t.Fatalf("expected TurnFailedStreamEvent, got %T", emitted[0])
	}
}

func TestTranslateSessionStatus_RetryNonFatal(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	// "rate limit exceeded" is NOT a fatal retry token, so no event should be emitted
	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","status":{"type":"retry","message":"rate limit exceeded"}}}`)
	s.translateSessionStatus(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for non-fatal retry, got %d", len(emitted))
	}
}

func TestTranslateSessionStatus_DifferentSession(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"other","status":{"type":"idle"}}}`)
	s.translateSessionStatus(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for different session, got %d", len(emitted))
	}
}

// --- translateMessagePartDelta ---

func TestTranslateMessagePartDelta_TextDelta(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	s.messageRoles["msg-1"] = "assistant"
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","messageID":"msg-1","partID":"p-1","field":"text","delta":"hello"}}`)
	s.translateMessagePartDelta(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", emitted[0])
	}
	if evt.Item.Type != "assistant_message" {
		t.Errorf("expected type assistant_message, got %q", evt.Item.Type)
	}
	if evt.Item.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", evt.Item.Text)
	}
}

func TestTranslateMessagePartDelta_ReasoningDelta(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","messageID":"msg-2","partID":"p-2","field":"reasoning","delta":"thinking..."}}`)
	s.translateMessagePartDelta(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", emitted[0])
	}
	if evt.Item.Type != "reasoning" {
		t.Errorf("expected type reasoning, got %q", evt.Item.Type)
	}
}

func TestTranslateMessagePartDelta_UserMessageIgnored(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	s.messageRoles["msg-3"] = "user"
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","messageID":"msg-3","partID":"p-3","field":"text","delta":"user text"}}`)
	s.translateMessagePartDelta(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for user message, got %d", len(emitted))
	}
}

func TestTranslateMessagePartDelta_EmptyDeltaIgnored(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","messageID":"msg-1","partID":"p-1","field":"text","delta":""}}`)
	s.translateMessagePartDelta(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for empty delta, got %d", len(emitted))
	}
}

// --- translatePermissionAsked ---

func TestTranslatePermissionAsked(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","id":"perm-1","permission":"shell","metadata":{"command":"ls","cwd":"/tmp"}}}`)
	s.translatePermissionAsked(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.PermissionRequestedStreamEvent)
	if !ok {
		t.Fatalf("expected PermissionRequestedStreamEvent, got %T", emitted[0])
	}
	if evt.Request.ID != "perm-1" {
		t.Errorf("expected permission ID perm-1, got %q", evt.Request.ID)
	}
	if evt.Request.Kind != "tool" {
		t.Errorf("expected kind 'tool', got %q", evt.Request.Kind)
	}

	// Verify pendingPerms was recorded
	s.mu.Lock()
	p, ok := s.pendingPerms["perm-1"]
	s.mu.Unlock()
	if !ok {
		t.Error("expected pendingPerms entry for perm-1")
	}
	if p.kind != "tool" {
		t.Errorf("expected pending perm kind 'tool', got %q", p.kind)
	}
}

func TestTranslatePermissionAsked_DifferentSession(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"other","id":"perm-2","permission":"shell"}}`)
	s.translatePermissionAsked(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for different session, got %d", len(emitted))
	}
}

// --- translateQuestionAsked ---

func TestTranslateQuestionAsked(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","id":"q-1","questions":[{"question":"Continue?","header":"Confirm","options":[{"label":"Yes"},{"label":"No"}]}]}}`)
	s.translateQuestionAsked(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.PermissionRequestedStreamEvent)
	if !ok {
		t.Fatalf("expected PermissionRequestedStreamEvent, got %T", emitted[0])
	}
	if evt.Request.Kind != "question" {
		t.Errorf("expected kind 'question', got %q", evt.Request.Kind)
	}
}

func TestTranslateQuestionAsked_EmptyQuestionsIgnored(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","id":"q-2","questions":[]}}`)
	s.translateQuestionAsked(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for empty questions, got %d", len(emitted))
	}
}

// --- translateEvent (integration) ---

func TestTranslateEvent_SessionIdle(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`"}}`)

	events := s.translateEvent("session.idle", raw)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	_, ok := events[0].Event.(protocol.TurnCompletedStreamEvent)
	if !ok {
		t.Fatalf("expected TurnCompletedStreamEvent, got %T", events[0].Event)
	}
}

func TestTranslateEvent_SessionError(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	raw := rawFromJSON(t, `{"properties":{"sessionID":"`+s.base.SessionID()+`","error":"something broke"}}`)

	events := s.translateEvent("session.error", raw)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	_, ok := events[0].Event.(protocol.TurnFailedStreamEvent)
	if !ok {
		t.Fatalf("expected TurnFailedStreamEvent, got %T", events[0].Event)
	}
}

func TestTranslateEvent_ServerEventIgnored(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	raw := rawFromJSON(t, `{}`)

	events := s.translateEvent("server.connected", raw)
	if len(events) != 0 {
		t.Errorf("expected 0 events for server event, got %d", len(events))
	}
}

func TestTranslateEvent_UnknownEventIgnored(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	raw := rawFromJSON(t, `{}`)

	events := s.translateEvent("unknown.event", raw)
	if len(events) != 0 {
		t.Errorf("expected 0 events for unknown event, got %d", len(events))
	}
}

// --- translateMessagePartUpdated (tool calls) ---

func TestTranslateMessagePartUpdated_ToolCall(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"part":{"sessionID":"`+s.base.SessionID()+`","messageID":"msg-1","id":"part-1","type":"tool","tool":"bash","callID":"call-1","state":{"status":"running"}}}}`)
	s.translateMessagePartUpdated(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", emitted[0])
	}
	if evt.Item.Type != "tool_call" {
		t.Errorf("expected type tool_call, got %q", evt.Item.Type)
	}
	if evt.Item.Status != "running" {
		t.Errorf("expected status running, got %q", evt.Item.Status)
	}
}

func TestTranslateMessagePartUpdated_CompactionPart(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"part":{"sessionID":"`+s.base.SessionID()+`","messageID":"msg-1","id":"part-2","type":"compaction","auto":true}}}`)
	s.translateMessagePartUpdated(raw, emit)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", emitted[0])
	}
	if evt.Item.Type != "compaction" {
		t.Errorf("expected type compaction, got %q", evt.Item.Type)
	}
	if evt.Item.Trigger != "auto" {
		t.Errorf("expected trigger 'auto', got %q", evt.Item.Trigger)
	}
}

// --- translateMessageUpdated ---

func TestTranslateMessageUpdated_AssistantCompleted(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"info":{"id":"msg-1","sessionID":"`+s.base.SessionID()+`","role":"assistant","structured":{"type":"text","text":"response text"},"time":{"completed":true}}}}`)
	s.translateMessageUpdated(raw, emit)

	// Should record the role
	s.mu.Lock()
	role := s.messageRoles["msg-1"]
	s.mu.Unlock()
	if role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", role)
	}

	// Should emit a structured assistant message
	if len(emitted) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitted))
	}
	evt, ok := emitted[0].(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("expected TimelineStreamEvent, got %T", emitted[0])
	}
	if evt.Item.Type != "assistant_message" {
		t.Errorf("expected type assistant_message, got %q", evt.Item.Type)
	}
}

func TestTranslateMessageUpdated_DifferentSession(t *testing.T) {
	s := newTestOpenCodeSessionForTranslate(t)
	var emitted []interface{}
	emit := func(e interface{}) { emitted = append(emitted, e) }

	raw := rawFromJSON(t, `{"properties":{"info":{"id":"msg-2","sessionID":"other","role":"assistant"}}}`)
	s.translateMessageUpdated(raw, emit)

	if len(emitted) != 0 {
		t.Errorf("expected 0 events for different session, got %d", len(emitted))
	}
}
