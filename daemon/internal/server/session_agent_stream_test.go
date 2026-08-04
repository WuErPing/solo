package server

// Direct unit tests for session_agent_stream.go: handleAgentEvent,
// handleStreamEvent, handleCoalescedFlush and sendAgentStream. They pin down
// the seq/epoch wire semantics (monotonic seq, stable epoch, epoch reset only
// on timeline recreation), duplicate-event dedup, and coalesced flush merging
// — behaviour previously covered only indirectly by integration tests.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

// streamRecordingConn captures every written frame on a buffered channel so
// tests can assert the exact outbound message sequence deterministically.
type streamRecordingConn struct {
	written chan []byte
	readErr chan error
}

func newStreamRecordingConn() *streamRecordingConn {
	return &streamRecordingConn{
		written: make(chan []byte, 256),
		readErr: make(chan error, 1),
	}
}

func (c *streamRecordingConn) ReadMessage() (int, []byte, error) {
	err := <-c.readErr
	return websocket.TextMessage, nil, err
}

func (c *streamRecordingConn) WriteMessage(_ int, data []byte) error {
	c.written <- data
	return nil
}

func (c *streamRecordingConn) Close() error { return nil }

// capturedAgentStream is the decoded form of an outbound agent_stream message.
type capturedAgentStream struct {
	AgentID   string
	EventType string
	ItemType  string
	ItemText  string
	ItemCallID string
	ItemStatus string
	Seq       *int
	Epoch     *string
}

// newAgentStreamTestSession builds a Session whose writePump forwards every
// outbound message into conn.written. The session's Run loop is not started;
// handlers are invoked directly.
func newAgentStreamTestSession(t *testing.T) (*Session, *streamRecordingConn) {
	t.Helper()
	conn := newStreamRecordingConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod)
	go sess.writePump()
	t.Cleanup(func() { sess.closeSendQueue() })
	return sess, conn
}

// nextAgentStream reads the next outbound frame and asserts it is an
// agent_stream session message.
func nextAgentStream(t *testing.T, conn *streamRecordingConn) capturedAgentStream {
	t.Helper()
	select {
	case data := <-conn.written:
		var env struct {
			Type    string `json:"type"`
			Message struct {
				Type    string `json:"type"`
				Payload struct {
					AgentID string          `json:"agentId"`
					Event   json.RawMessage `json:"event"`
					Seq     *int            `json:"seq"`
					Epoch   *string         `json:"epoch"`
				} `json:"payload"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("unmarshal outbound frame: %v\nframe: %s", err, data)
		}
		if env.Type != "session" || env.Message.Type != "agent_stream" {
			t.Fatalf("expected agent_stream session message, got: %s", data)
		}
		var evt struct {
			Type string `json:"type"`
			Item struct {
				Type   string `json:"type"`
				Text   string `json:"text"`
				CallID string `json:"callId"`
				Status string `json:"status"`
			} `json:"item"`
		}
		if err := json.Unmarshal(env.Message.Payload.Event, &evt); err != nil {
			t.Fatalf("unmarshal stream event: %v", err)
		}
		return capturedAgentStream{
			AgentID:    env.Message.Payload.AgentID,
			EventType:  evt.Type,
			ItemType:   evt.Item.Type,
			ItemText:   evt.Item.Text,
			ItemCallID: evt.Item.CallID,
			ItemStatus: evt.Item.Status,
			Seq:        env.Message.Payload.Seq,
			Epoch:      env.Message.Payload.Epoch,
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for agent_stream message")
		return capturedAgentStream{}
	}
}

// expectNoAgentStream asserts that no outbound frame arrives within the window.
func expectNoAgentStream(t *testing.T, conn *streamRecordingConn, window time.Duration) {
	t.Helper()
	select {
	case data := <-conn.written:
		t.Fatalf("expected no outbound message within %v, got: %s", window, data)
	case <-time.After(window):
	}
}

func timelineEvent(itemType, text string) protocol.TimelineStreamEvent {
	return protocol.TimelineStreamEvent{
		Item:     protocol.TimelineItem{Type: itemType, Text: text},
		Provider: "test",
	}
}

func streamEvent(agentID string, evt interface{}) agent.AgentStreamEvent {
	return agent.AgentStreamEvent{AgentID: agentID, Event: evt, Timestamp: time.Now()}
}

// TestHandleStreamEvent_SeqMonotonicAndEpochStable pins the core wire contract:
// timeline events carry strictly increasing seq values and a stable epoch,
// while thread_started carries neither.
func TestHandleStreamEvent_SeqMonotonicAndEpochStable(t *testing.T) {
	sess, conn := newAgentStreamTestSession(t)
	agentID := "agent-seq"

	sess.handleStreamEvent(streamEvent(agentID, protocol.ThreadStartedStreamEvent{Provider: "test"}))

	started := nextAgentStream(t, conn)
	if started.EventType != "thread_started" {
		t.Fatalf("first event: got %q, want thread_started", started.EventType)
	}
	if started.Seq != nil || started.Epoch != nil {
		t.Errorf("thread_started must carry no seq/epoch, got seq=%v epoch=%v", started.Seq, started.Epoch)
	}

	// user_message and error items are not coalescable, so they take the
	// direct Append path and each gets its own row.
	items := []protocol.TimelineStreamEvent{
		timelineEvent("user_message", "first"),
		timelineEvent("user_message", "second"),
		{Item: protocol.TimelineItem{Type: "error", Message: "boom"}, Provider: "test"},
	}
	for _, evt := range items {
		sess.handleStreamEvent(streamEvent(agentID, evt))
	}

	var prevSeq int
	var epoch string
	for i := 0; i < len(items); i++ {
		msg := nextAgentStream(t, conn)
		if msg.EventType != "timeline" {
			t.Fatalf("event %d: got type %q, want timeline", i, msg.EventType)
		}
		if msg.Seq == nil {
			t.Fatalf("event %d: timeline event must carry seq", i)
		}
		if msg.Epoch == nil || *msg.Epoch == "" {
			t.Fatalf("event %d: timeline event must carry non-empty epoch", i)
		}
		if i == 0 {
			if *msg.Seq != 0 {
				t.Errorf("first appended row: got seq %d, want 0", *msg.Seq)
			}
			epoch = *msg.Epoch
		} else {
			if *msg.Seq != prevSeq+1 {
				t.Errorf("event %d: got seq %d, want %d (monotonic +1)", i, *msg.Seq, prevSeq+1)
			}
			if *msg.Epoch != epoch {
				t.Errorf("event %d: epoch changed from %q to %q without timeline reset", i, epoch, *msg.Epoch)
			}
		}
		prevSeq = *msg.Seq
	}

	if got := sess.timelineSize(agentID); got != len(items) {
		t.Errorf("timeline NextSeq: got %d, want %d", got, len(items))
	}
}

// TestHandleStreamEvent_DuplicateEventKeepsRow pins the dedup semantics: an
// exact repeat of the most recent timeline item does not create a new row;
// the re-sent stream event carries the original seq.
func TestHandleStreamEvent_DuplicateEventKeepsRow(t *testing.T) {
	sess, conn := newAgentStreamTestSession(t)
	agentID := "agent-dup"

	evt := timelineEvent("user_message", "same text")
	sess.handleStreamEvent(streamEvent(agentID, evt))
	sess.handleStreamEvent(streamEvent(agentID, evt))

	first := nextAgentStream(t, conn)
	second := nextAgentStream(t, conn)
	if first.Seq == nil || second.Seq == nil {
		t.Fatal("timeline events must carry seq")
	}
	if *first.Seq != *second.Seq {
		t.Errorf("duplicate event: got seqs %d then %d, want identical (dedup)", *first.Seq, *second.Seq)
	}
	if got := sess.timelineSize(agentID); got != 1 {
		t.Errorf("duplicate append created extra rows: NextSeq=%d, want 1", got)
	}

	// A distinct item afterwards continues the sequence from the shared row.
	sess.handleStreamEvent(streamEvent(agentID, timelineEvent("user_message", "different")))
	third := nextAgentStream(t, conn)
	if third.Seq == nil || *third.Seq != *first.Seq+1 {
		t.Errorf("next distinct event: got seq %v, want %d", third.Seq, *first.Seq+1)
	}
}

// TestHandleStreamEvent_ThreadStartedDoesNotResetEpoch pins reconnect/resume
// semantics: a repeated thread_started (e.g. a resumed session) must not bump
// the epoch or reset seq. Only deleting the timeline produces a new epoch,
// and seq then restarts at 0.
func TestHandleStreamEvent_ThreadStartedDoesNotResetEpoch(t *testing.T) {
	sess, conn := newAgentStreamTestSession(t)
	agentID := "agent-epoch"

	sess.handleStreamEvent(streamEvent(agentID, protocol.ThreadStartedStreamEvent{Provider: "test"}))
	nextAgentStream(t, conn) // consume thread_started
	sess.handleStreamEvent(streamEvent(agentID, timelineEvent("user_message", "one")))
	first := nextAgentStream(t, conn)
	if first.Epoch == nil || *first.Epoch == "" {
		t.Fatal("expected non-empty epoch")
	}
	epoch1 := *first.Epoch

	// Repeated thread_started (resume path) — Initialize is idempotent.
	sess.handleStreamEvent(streamEvent(agentID, protocol.ThreadStartedStreamEvent{Provider: "test"}))
	nextAgentStream(t, conn) // consume thread_started
	sess.handleStreamEvent(streamEvent(agentID, timelineEvent("user_message", "two")))
	second := nextAgentStream(t, conn)
	if second.Epoch == nil || *second.Epoch != epoch1 {
		t.Errorf("repeated thread_started changed epoch: got %v, want %q", second.Epoch, epoch1)
	}
	if second.Seq == nil || *second.Seq != 1 {
		t.Errorf("seq after repeated thread_started: got %v, want 1 (no reset)", second.Seq)
	}

	// Timeline recreation (Delete) is the only path to a new epoch; seq
	// restarts from 0.
	sess.timelineStore.Delete(agentID)
	sess.handleStreamEvent(streamEvent(agentID, timelineEvent("user_message", "three")))
	third := nextAgentStream(t, conn)
	if third.Epoch == nil || *third.Epoch == epoch1 {
		t.Errorf("expected new epoch after timeline recreation, still %v", third.Epoch)
	}
	if third.Seq == nil || *third.Seq != 0 {
		t.Errorf("seq after timeline recreation: got %v, want 0", third.Seq)
	}
}

// TestHandleCoalescedFlush_MergesAssistantDeltas verifies that rapid
// assistant_message deltas are absorbed by the coalescer and that a terminal
// event flushes them as a single merged row before the terminal event itself.
func TestHandleCoalescedFlush_MergesAssistantDeltas(t *testing.T) {
	sess, conn := newAgentStreamTestSession(t)
	agentID := "agent-coalesce"

	sess.handleStreamEvent(streamEvent(agentID, timelineEvent("assistant_message", "Hello ")))
	sess.handleStreamEvent(streamEvent(agentID, timelineEvent("assistant_message", "world")))

	// Absorbed by the coalescer: nothing is sent immediately.
	expectNoAgentStream(t, conn, 150*time.Millisecond)

	// turn_completed flushes the coalescer first, then is sent itself.
	sess.handleStreamEvent(streamEvent(agentID, protocol.TurnCompletedStreamEvent{Provider: "test"}))

	flush := nextAgentStream(t, conn)
	if flush.EventType != "timeline" || flush.ItemType != "assistant_message" {
		t.Fatalf("flush event: got %s/%s, want timeline/assistant_message", flush.EventType, flush.ItemType)
	}
	if flush.ItemText != "Hello world" {
		t.Errorf("merged text: got %q, want %q", flush.ItemText, "Hello world")
	}
	if flush.Seq == nil || *flush.Seq != 0 {
		t.Errorf("flush seq: got %v, want 0", flush.Seq)
	}
	if flush.Epoch == nil || *flush.Epoch == "" {
		t.Error("flush event must carry epoch")
	}

	terminal := nextAgentStream(t, conn)
	if terminal.EventType != "turn_completed" {
		t.Fatalf("terminal event: got %q, want turn_completed", terminal.EventType)
	}
	if terminal.Seq != nil || terminal.Epoch != nil {
		t.Errorf("turn_completed must carry no seq/epoch, got seq=%v epoch=%v", terminal.Seq, terminal.Epoch)
	}

	// The merged row is exactly one timeline entry.
	if got := sess.timelineSize(agentID); got != 1 {
		t.Errorf("merged flush: NextSeq=%d, want 1", got)
	}
}

// TestHandleCoalescedFlush_ToolCallStatusUpdate verifies tool_call coalescing:
// a running tool_call followed by a completed status update for the same
// callId merges in place and flushes immediately as a single row.
func TestHandleCoalescedFlush_ToolCallStatusUpdate(t *testing.T) {
	sess, conn := newAgentStreamTestSession(t)
	agentID := "agent-tool"

	running := protocol.TimelineStreamEvent{
		Item:     protocol.TimelineItem{Type: "tool_call", CallID: "call-1", Name: "Bash", Status: "running"},
		Provider: "test",
	}
	completed := protocol.TimelineStreamEvent{
		Item:     protocol.TimelineItem{Type: "tool_call", CallID: "call-1", Name: "Bash", Status: "completed"},
		Provider: "test",
	}

	sess.handleStreamEvent(streamEvent(agentID, running))
	expectNoAgentStream(t, conn, 150*time.Millisecond)

	// Terminal tool_call status triggers an immediate flush.
	sess.handleStreamEvent(streamEvent(agentID, completed))

	flush := nextAgentStream(t, conn)
	if flush.ItemType != "tool_call" {
		t.Fatalf("flush item: got %q, want tool_call", flush.ItemType)
	}
	if flush.ItemCallID != "call-1" || flush.ItemStatus != "completed" {
		t.Errorf("flush item: got callId=%q status=%q, want call-1/completed", flush.ItemCallID, flush.ItemStatus)
	}
	if flush.Seq == nil || *flush.Seq != 0 {
		t.Errorf("flush seq: got %v, want 0", flush.Seq)
	}

	// Only one row: the status update replaced the running entry in place.
	expectNoAgentStream(t, conn, 150*time.Millisecond)
	if got := sess.timelineSize(agentID); got != 1 {
		t.Errorf("tool call coalescing: NextSeq=%d, want 1", got)
	}
}

// TestHandleAgentEvent_StreamRouting verifies handleAgentEvent routes stream
// events under the manager-supplied agent ID and ignores malformed events.
func TestHandleAgentEvent_StreamRouting(t *testing.T) {
	sess, conn := newAgentStreamTestSession(t)

	// Nil stream payload: no-op, no message, no panic.
	sess.handleAgentEvent(agent.AgentEvent{Type: agent.EventAgentStream, AgentID: "agent-x"})
	// Nil agent payload on state event: no-op.
	sess.handleAgentEvent(agent.AgentEvent{Type: agent.EventAgentState, AgentID: "agent-x"})
	expectNoAgentStream(t, conn, 150*time.Millisecond)

	// The manager's AgentID overrides the (empty) stream event AgentID.
	sess.handleAgentEvent(agent.AgentEvent{
		Type:    agent.EventAgentStream,
		AgentID: "agent-x",
		Stream:  &agent.AgentStreamEvent{Event: protocol.ThreadStartedStreamEvent{Provider: "test"}, Timestamp: time.Now()},
	})

	msg := nextAgentStream(t, conn)
	if msg.AgentID != "agent-x" {
		t.Errorf("routed agentId: got %q, want %q", msg.AgentID, "agent-x")
	}
	if msg.EventType != "thread_started" {
		t.Errorf("routed event type: got %q, want thread_started", msg.EventType)
	}
}
