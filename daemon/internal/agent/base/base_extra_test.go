package base

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// ---- CallbackDispatcher tests ----

func TestCallbackDispatcher_EmitDelivers(t *testing.T) {
	d := NewCallbackDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))

	var got []interface{}
	var mu sync.Mutex
	unsub := d.Subscribe(func(evt interface{}) {
		mu.Lock()
		got = append(got, evt)
		mu.Unlock()
	})

	d.Emit("hello")
	d.Emit(42)

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	_ = unsub
}

func TestCallbackDispatcher_UnsubscribeStopsDelivery(t *testing.T) {
	d := NewCallbackDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))

	count := 0
	unsub := d.Subscribe(func(evt interface{}) { count++ })

	d.Emit("a")
	unsub()
	d.Emit("b")

	if count != 1 {
		t.Fatalf("expected 1 event after unsub, got %d", count)
	}
}

func TestCallbackDispatcher_CloseDropsEvents(t *testing.T) {
	d := NewCallbackDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))

	count := 0
	d.Subscribe(func(evt interface{}) { count++ })

	d.Close()
	d.Emit("after-close")

	if count != 0 {
		t.Fatalf("expected 0 events after close, got %d", count)
	}
}

func TestCallbackDispatcher_CloseIdempotent(t *testing.T) {
	d := NewCallbackDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.Close()
	d.Close() // must not panic
}

func TestCallbackDispatcher_MultipleSubscribers(t *testing.T) {
	d := NewCallbackDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))

	var counts [3]int
	for i := range counts {
		i := i
		d.Subscribe(func(evt interface{}) { counts[i]++ })
	}

	d.Emit("x")

	for i, c := range counts {
		if c != 1 {
			t.Errorf("subscriber %d: expected 1, got %d", i, c)
		}
	}
}

// ---- PermissionManager tests ----

func TestPermissionManager_RegisterAndRespond(t *testing.T) {
	pm := NewPermissionManager()

	ch := pm.Register("req-1")
	go func() {
		_ = pm.Respond("req-1", protocol.AgentPermissionResponse{Behavior: "allow"})
	}()

	select {
	case resp := <-ch:
		if resp.Behavior != "allow" {
			t.Errorf("expected allow, got %q", resp.Behavior)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for permission response")
	}
}

func TestPermissionManager_RespondUnknownReturnsError(t *testing.T) {
	pm := NewPermissionManager()
	err := pm.Respond("no-such-id", protocol.AgentPermissionResponse{})
	if err == nil {
		t.Fatal("expected error for unknown request ID")
	}
}

func TestPermissionManager_GetPending(t *testing.T) {
	pm := NewPermissionManager()
	pm.Register("req-a")
	pm.Register("req-b")

	pending := pm.GetPending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}
}

func TestPermissionManager_RejectAll(t *testing.T) {
	pm := NewPermissionManager()
	ch1 := pm.Register("req-1")
	ch2 := pm.Register("req-2")

	pm.RejectAll()

	for _, ch := range []<-chan protocol.AgentPermissionResponse{ch1, ch2} {
		select {
		case resp := <-ch:
			if resp.Behavior != "deny" {
				t.Errorf("expected deny, got %q", resp.Behavior)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for RejectAll response")
		}
	}

	if len(pm.GetPending()) != 0 {
		t.Error("expected no pending after RejectAll")
	}
}

func TestPermissionManager_CloseClosesChannels(t *testing.T) {
	pm := NewPermissionManager()
	ch := pm.Register("req-1")

	pm.Close()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after Close()")
	}
}

func TestPermissionManager_RegisterWithTimeout_AutoRejects(t *testing.T) {
	pm := NewPermissionManager()

	ch := pm.RegisterWithTimeout("req-1", 50*time.Millisecond, nil)

	select {
	case resp := <-ch:
		if resp.Behavior != "deny" {
			t.Errorf("expected auto-deny, got %q", resp.Behavior)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for auto-reject")
	}

	if len(pm.GetPending()) != 0 {
		t.Error("expected no pending after auto-reject")
	}
}

func TestPermissionManager_RegisterWithTimeout_RespondBeforeTimeout(t *testing.T) {
	pm := NewPermissionManager()

	ch := pm.RegisterWithTimeout("req-1", 500*time.Millisecond, nil)

	go func() {
		_ = pm.Respond("req-1", protocol.AgentPermissionResponse{Behavior: "allow"})
	}()

	select {
	case resp := <-ch:
		if resp.Behavior != "allow" {
			t.Errorf("expected allow, got %q", resp.Behavior)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for response")
	}

	// Give timer time to fire (it shouldn't because Respond stopped it).
	time.Sleep(100 * time.Millisecond)

	if len(pm.GetPending()) != 0 {
		t.Error("expected no pending after respond")
	}
}

func TestPermissionManager_RegisterWithTimeout_OnTimeoutCallback(t *testing.T) {
	pm := NewPermissionManager()

	callbackFired := make(chan struct{}, 1)
	ch := pm.RegisterWithTimeout("req-1", 50*time.Millisecond, func() {
		close(callbackFired)
	})

	select {
	case <-ch:
		// expected
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for auto-reject")
	}

	select {
	case <-callbackFired:
		// expected
	case <-time.After(time.Second):
		t.Fatal("timeout callback was not fired")
	}
}

// ---- JSONEventTranslator tests ----

func TestJSONEventTranslator_ParseJSONLine_Valid(t *testing.T) {
	tr := NewJSONEventTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)))
	line := []byte(`{"type":"text","content":"hello"}`)
	m, err := tr.ParseJSONLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := m["type"]; !ok {
		t.Error("expected 'type' key in parsed map")
	}
}

func TestJSONEventTranslator_ParseJSONLine_Invalid(t *testing.T) {
	tr := NewJSONEventTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := tr.ParseJSONLine([]byte(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---- EventPump tests ----

// stubTranslator emits a list of pre-configured events then terminates.
type stubTranslator struct {
	lines []string // raw line content -> event map to emit
}

func (s *stubTranslator) Translate(raw []byte, _ time.Time) ([]interface{}, bool, error) {
	line := strings.TrimSpace(string(raw))
	for _, l := range s.lines {
		if l == line {
			return []interface{}{map[string]interface{}{"type": line}}, false, nil
		}
	}
	return nil, false, nil
}

// terminalDetector marks "terminal" events as terminal.
type terminalDetector struct{}

func (terminalDetector) IsTerminal(evt interface{}) (*AgentRunResult, error, bool) {
	m, ok := evt.(map[string]interface{})
	if !ok {
		return nil, nil, false
	}
	if m["type"] == "terminal" {
		return &AgentRunResult{FinalText: "done"}, nil, true
	}
	return nil, nil, false
}

func TestEventPump_SetProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := NewCallbackDispatcher(logger)
	p := NewEventPump(logger, dispatcher)
	p.SetProvider("test-provider")
	if p.provider != "test-provider" {
		t.Errorf("expected provider test-provider, got %q", p.provider)
	}
}

func TestEventPump_RunBlocking_TerminalReached(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var received []interface{}
	var mu sync.Mutex
	dispatcher := NewCallbackDispatcher(logger)
	dispatcher.Subscribe(func(evt interface{}) {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
	})

	pump := NewEventPump(logger, dispatcher)
	reader := strings.NewReader("event1\nterminal\n")
	translator := &stubTranslator{lines: []string{"event1", "terminal"}}
	detector := terminalDetector{}

	result, err := pump.RunBlocking(context.Background(), reader, translator, detector)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.FinalText != "done" {
		t.Errorf("expected FinalText=done, got %v", result)
	}
}

func TestEventPump_RunBlocking_StreamEndsWithoutTerminal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := NewCallbackDispatcher(logger)
	pump := NewEventPump(logger, dispatcher)
	reader := strings.NewReader("event1\n")
	translator := &stubTranslator{lines: []string{"event1"}}

	_, err := pump.RunBlocking(context.Background(), reader, translator, terminalDetector{})
	if err == nil {
		t.Fatal("expected error when stream ends without terminal state")
	}
}

func TestEventPump_RunBackground_Executes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	done := make(chan struct{})
	dispatcher := NewCallbackDispatcher(logger)
	dispatcher.Subscribe(func(evt interface{}) {
		if m, ok := evt.(map[string]interface{}); ok {
			if m["type"] == "terminal" {
				close(done)
			}
		}
	})

	pump := NewEventPump(logger, dispatcher)
	reader := strings.NewReader("terminal\n")
	translator := &stubTranslator{lines: []string{"terminal"}}

	pump.RunBackground(context.Background(), reader, translator, terminalDetector{})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunBackground did not emit terminal event")
	}
}

func TestEventPump_RunBlocking_ContextCancelMidStream(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := NewCallbackDispatcher(logger)
	pump := NewEventPump(logger, dispatcher)
	pump.SetProvider("test")

	ctx, cancel := context.WithCancel(context.Background())

	pr, pw := io.Pipe()
	translator := &stubTranslator{lines: []string{"event1"}}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
		_ = pw.Close()
	}()

	_, err := pump.RunBlocking(ctx, pr, translator, terminalDetector{})
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestEventPump_RunBlocking_NilDispatcher(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pump := NewEventPump(logger, nil) // nil dispatcher must not panic
	reader := strings.NewReader("terminal\n")
	translator := &stubTranslator{lines: []string{"terminal"}}

	result, err := pump.RunBlocking(context.Background(), reader, translator, terminalDetector{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

// ---- BaseSession missing method tests ----

func newBaseSessionForExtra(t *testing.T) *BaseSession {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewBaseSession("mock", &protocol.AgentSessionConfig{}, logger)
}

func TestBaseSession_Logger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewBaseSession("mock", &protocol.AgentSessionConfig{}, logger)
	if s.Logger() != logger {
		t.Error("Logger() returned wrong value")
	}
}

func TestBaseSession_GetCurrentModePtr(t *testing.T) {
	s := newBaseSessionForExtra(t)
	_ = s.SetMode("fast")
	ptr := s.GetCurrentModePtr()
	if ptr == nil || *ptr != "fast" {
		t.Errorf("GetCurrentModePtr: got %v, want 'fast'", ptr)
	}
}

func TestBaseSession_LockUnlock(t *testing.T) {
	s := newBaseSessionForExtra(t)
	s.Lock()
	s.Unlock()
	s.RLock()
	s.RUnlock()
	// reaching here without deadlock is the test
}
