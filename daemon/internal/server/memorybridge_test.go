package server

import (
	"log/slog"
	"sync"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
)

// fakeBridge records every hook invocation for assertions.
type fakeBridge struct {
	mu sync.Mutex

	userTurns []turnArgs
	asstTurns []turnArgs // one-shot path
	sysTurns  []turnArgs

	asstChunks []chunkArgs // streaming accumulate
	asstEnds   []endArgs   // streaming flush
}

type turnArgs struct {
	sessionID, agentID, content string
}

type chunkArgs struct {
	agentID, sessionID, fragment string
}

type endArgs struct {
	agentID, sessionID string
}

func (f *fakeBridge) OnUserTurn(s, a, c string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userTurns = append(f.userTurns, turnArgs{s, a, c})
}

func (f *fakeBridge) OnAssistantTurn(s, a, c string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asstTurns = append(f.asstTurns, turnArgs{s, a, c})
}

func (f *fakeBridge) OnAssistantChunk(a, s, fragment string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asstChunks = append(f.asstChunks, chunkArgs{a, s, fragment})
}

func (f *fakeBridge) OnAssistantTurnEnd(a, s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asstEnds = append(f.asstEnds, endArgs{a, s})
}

func (f *fakeBridge) OnSystemTurn(s, a, c string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sysTurns = append(f.sysTurns, turnArgs{s, a, c})
}

func (f *fakeBridge) Close() error { return nil }

// newMinimalSession builds a Session suitable for hook testing, with no
// websocket connection and no agent manager (added later via SetAgentMgr
// if needed).
func newMinimalSession() *Session {
	return &Session{
		clientID:   "client-1",
		clientType: "cli",
		logger:     slog.Default(),
		cfg:        &config.Config{},
	}
}

// ---------- maybeRecordAssistantTurn ----------

func TestMaybeRecordAssistantTurn_NilBridge_Noop(t *testing.T) {
	t.Helper()
	s := newMinimalSession()
	// Must not panic when bridge is nil.
	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "timeline",
		"item": map[string]interface{}{"type": "assistant_message", "text": "hi"},
	})
}

func TestMaybeRecordAssistantTurn_AssistantMessage_Fires(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)

	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "timeline",
		"item": map[string]interface{}{"type": "assistant_message", "text": "hello world"},
	})

	if len(b.asstChunks) != 1 {
		t.Fatalf("got %d OnAssistantChunk calls, want 1", len(b.asstChunks))
	}
	got := b.asstChunks[0]
	if got.agentID != "agent-1" {
		t.Errorf("agentID = %q, want agent-1", got.agentID)
	}
	if got.sessionID != "client-1" {
		t.Errorf("sessionID = %q, want client-1", got.sessionID)
	}
	if got.fragment != "hello world" {
		t.Errorf("fragment = %q, want 'hello world'", got.fragment)
	}
	// Streaming chunks must NOT also fire the one-shot turn path.
	if len(b.asstTurns) != 0 {
		t.Errorf("streaming assistant_message must NOT fire OnAssistantTurn, got %d", len(b.asstTurns))
	}
	if len(b.asstEnds) != 0 {
		t.Errorf("no turn-end should fire on a timeline event, got %d", len(b.asstEnds))
	}
}

func TestMaybeRecordAssistantTurn_UserMessage_Skipped(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)

	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "timeline",
		"item": map[string]interface{}{"type": "user_message", "text": "hi"},
	})

	if len(b.asstChunks)+len(b.asstTurns)+len(b.asstEnds) != 0 {
		t.Errorf("user_message must be fully ignored")
	}
}

func TestMaybeRecordAssistantTurn_TurnCompleted_RoutesToEnd(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)

	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "turn_completed",
	})

	if len(b.asstEnds) != 1 {
		t.Fatalf("got %d OnAssistantTurnEnd calls, want 1", len(b.asstEnds))
	}
	if b.asstEnds[0].agentID != "agent-1" {
		t.Errorf("agentID = %q, want agent-1", b.asstEnds[0].agentID)
	}
	if b.asstEnds[0].sessionID != "client-1" {
		t.Errorf("sessionID = %q, want client-1", b.asstEnds[0].sessionID)
	}
}

func TestMaybeRecordAssistantTurn_TurnFailedAndCanceled_AlsoRouteToEnd(t *testing.T) {
	for _, evtType := range []string{"turn_failed", "turn_canceled"} {
		s := newMinimalSession()
		b := &fakeBridge{}
		s.SetMemoryBridge(b)

		s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{"type": evtType})

		if len(b.asstEnds) != 1 {
			t.Errorf("%s: got %d OnAssistantTurnEnd calls, want 1", evtType, len(b.asstEnds))
		}
	}
}

func TestMaybeRecordAssistantTurn_Reasoning_Skipped(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)

	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "timeline",
		"item": map[string]interface{}{"type": "reasoning", "text": "thinking..."},
	})

	if len(b.asstChunks)+len(b.asstTurns)+len(b.asstEnds) != 0 {
		t.Errorf("reasoning must be fully ignored")
	}
}

func TestMaybeRecordAssistantTurn_MalformedEvent_Noop(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)

	// Not a map
	s.maybeRecordAssistantTurn("agent-1", "not a map")
	// Missing item
	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{"type": "timeline"})
	// item not a map
	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "timeline",
		"item": "string",
	})

	if len(b.asstChunks)+len(b.asstTurns)+len(b.asstEnds) != 0 {
		t.Errorf("malformed events must be silent no-ops")
	}
}

// ---------- Streaming sequence: chunks then turn end ----------

func TestMaybeRecordAssistantTurn_FullStreamingSequence(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)

	for _, frag := range []string{"Hello", ", ", "world!"} {
		s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
			"type": "timeline",
			"item": map[string]interface{}{"type": "assistant_message", "text": frag},
		})
	}
	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{"type": "turn_completed"})

	if len(b.asstChunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(b.asstChunks))
	}
	if len(b.asstEnds) != 1 {
		t.Errorf("expected 1 turn-end, got %d", len(b.asstEnds))
	}
	if len(b.asstTurns) != 0 {
		t.Errorf("one-shot OnAssistantTurn must not fire during streaming, got %d", len(b.asstTurns))
	}
}

// ---------- SetMemoryBridge ----------

func TestSetMemoryBridge_NilDisables(t *testing.T) {
	s := newMinimalSession()
	b := &fakeBridge{}
	s.SetMemoryBridge(b)
	s.SetMemoryBridge(nil)

	s.maybeRecordAssistantTurn("agent-1", map[string]interface{}{
		"type": "timeline",
		"item": map[string]interface{}{"type": "assistant_message", "text": "hi"},
	})
	if len(b.asstChunks) != 0 {
		t.Errorf("nil bridge should disable recording, got %d chunks", len(b.asstChunks))
	}
}

// Compile-time check that fakeBridge satisfies the interface.
func TestFakeBridge_ImplementsMemoryBridge(t *testing.T) {
	t.Helper()
	var _ MemoryBridge = (*fakeBridge)(nil)
}
