package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// TestSendToChannel_CriticalEventsBlock verifies that sendToChannel blocks on
// critical events when the channel is full, ensuring they are never dropped.
// This function is used by StartTurn's subscribeEvents callback.
func TestSendToChannel_CriticalEventsBlock(t *testing.T) {
	ch := make(chan AgentStreamEvent, 1)

	// Fill the channel to capacity
	ch <- AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: "test", Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}
	// ch is now full (capacity 1, 1 item)

	// Send a critical event: should block until room is available
	criticalEvt := AgentStreamEvent{
		AgentID:   "test-agent",
		Event:     protocol.TurnCompletedStreamEvent{Provider: "test"},
		Timestamp: time.Now(),
	}

	var sendDone sync.WaitGroup
	sendDone.Add(1)
	go func() {
		defer sendDone.Done()
		sendToChannel(ch, criticalEvt) // should block
	}()

	// Give the goroutine time to block on the send
	time.Sleep(50 * time.Millisecond)

	// Drain one item to make room -- the critical send should complete
	<-ch

	sendDone.Wait()

	// The critical event should now be in the channel
	select {
	case evt := <-ch:
		if !evt.IsCriticalEvent() {
			t.Error("expected critical event in channel, got non-critical")
		}
	case <-time.After(1 * time.Second):
		t.Error("critical event was not delivered after channel was drained -- " +
			"sendToChannel must block on critical events")
	}
}

func TestOpenCodeTerminalEventValueIsDispatcherCritical(t *testing.T) {
	evt := AgentStreamEvent{
		AgentID:   "test-agent",
		Event:     protocol.TurnCompletedStreamEvent{Provider: opencodeProviderName},
		Timestamp: time.Now(),
	}

	if !evt.IsCriticalEvent() {
		t.Fatal("expected OpenCode turn_completed event to be critical")
	}
	if _, ok := interface{}(evt).(base.CriticalEvent); !ok {
		t.Fatal("OpenCode terminal AgentStreamEvent value must be dispatcher-critical before sendToChannel")
	}
}

// TestSendToChannel_NonCriticalEventsDrop verifies that sendToChannel drops
// non-critical events when the channel is full, preventing backpressure.
func TestSendToChannel_NonCriticalEventsDrop(t *testing.T) {
	ch := make(chan AgentStreamEvent, 1)

	// Fill the channel to capacity
	ch <- AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: "test", Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}

	// Send a non-critical event: should be dropped immediately (non-blocking)
	nonCriticalEvt := AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: "test", Item: protocol.TimelineItem{Type: "text", Text: "non-critical"}},
		Timestamp: time.Now(),
	}

	sendToChannel(ch, nonCriticalEvt) // should not block

	// Channel should still have only 1 item (the original filler)
	if len(ch) != 1 {
		t.Errorf("expected channel length 1 after dropping non-critical event, got %d", len(ch))
	}
}

// TestSendToChannel_BuggyPatternDropsCritical proves that the OLD buggy pattern
// (non-blocking send for ALL events) drops critical events when the channel is full.
// This demonstrates the bug that existed before sendToChannel was introduced.
func TestSendToChannel_BuggyPatternDropsCritical(t *testing.T) {
	ch := make(chan AgentStreamEvent, 1)

	// This is the BUGGY pattern that StartTurn's callback used to have
	buggySend := func(ch chan<- AgentStreamEvent, evt AgentStreamEvent) {
		select {
		case ch <- evt:
		default: // BUG: drops ALL events including critical ones
		}
	}

	// Fill the channel to capacity
	ch <- AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: "test", Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}

	// Send a critical event with the buggy pattern -- it gets dropped
	criticalEvt := AgentStreamEvent{
		AgentID:   "test-agent",
		Event:     protocol.TurnFailedStreamEvent{Provider: "test", Error: "failed"},
		Timestamp: time.Now(),
	}

	buggySend(ch, criticalEvt) // drops silently

	// Channel should still have only the filler
	if len(ch) != 1 {
		t.Errorf("expected channel length 1 (buggy pattern should have dropped the critical event), got %d", len(ch))
	}

	// The filler should be the only item
	evt := <-ch
	if evt.IsCriticalEvent() {
		t.Error("buggy pattern unexpectedly delivered the critical event")
	}
}

// TestStartTurnIntegration_PreservesCriticalEvents is an integration test that
// verifies the full subscribeEvents + sendToChannel chain (as used by StartTurn)
// delivers critical events even when the output channel is full.
func TestStartTurnIntegration_PreservesCriticalEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession("http://127.0.0.1:0", "test-session", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("command warmup did not complete")
	}

	// Use a small buffered channel and the sendToChannel callback (as StartTurn does)
	ch := make(chan AgentStreamEvent, 2)
	unsub := session.subscribeEvents(func(evt AgentStreamEvent) {
		sendToChannel(ch, evt)
	})
	defer unsub()

	// Pre-fill ch directly to guarantee it's full
	filler1 := AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: opencodeProviderName, Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}
	filler2 := AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: opencodeProviderName, Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}
	ch <- filler1
	ch <- filler2
	// ch is now full (capacity 2, 2 items)

	// Set up a drainer that waits for our signal
	drainReady := make(chan struct{})
	var foundCritical bool
	var drainerDone sync.WaitGroup
	drainerDone.Add(1)
	go func() {
		defer drainerDone.Done()
		<-drainReady
		timeout := time.After(3 * time.Second)
		for {
			select {
			case evt := <-ch:
				if evt.IsCriticalEvent() {
					foundCritical = true
					return
				}
			case <-timeout:
				return
			}
		}
	}()

	// Emit a critical event through the dispatcher.
	// sendToChannel blocks on critical events, so the callback goroutine will
	// block until the drainer makes room.
	session.notifySubscribers(AgentStreamEvent{
		AgentID:   "test-agent",
		Event:     protocol.TurnCompletedStreamEvent{Provider: opencodeProviderName},
		Timestamp: time.Now(),
	})

	// Give the dispatcher time to deliver the event to the callback
	time.Sleep(50 * time.Millisecond)

	// Signal the drainer to start
	close(drainReady)

	drainerDone.Wait()

	if !foundCritical {
		t.Error("critical event (turn_completed) was dropped when channel was full -- " +
			"sendToChannel must block on critical events")
	}
}

// TestStartTurnIntegration_BuggyPatternDropsCritical is an integration test that
// uses the buggy callback pattern to prove critical events get dropped through
// the full subscribeEvents chain when the channel is full.
func TestStartTurnIntegration_BuggyPatternDropsCritical(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession("http://127.0.0.1:0", "test-session", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("command warmup did not complete")
	}

	// BUGGY callback: non-blocking send for ALL events (old StartTurn pattern)
	ch := make(chan AgentStreamEvent, 2)
	unsub := session.subscribeEvents(func(evt AgentStreamEvent) {
		select {
		case ch <- evt:
		default: // BUG: drops ALL events including critical ones
		}
	})
	defer unsub()

	// Pre-fill ch directly so it's guaranteed full
	filler1 := AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: opencodeProviderName, Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}
	filler2 := AgentStreamEvent{
		Event:     protocol.TimelineStreamEvent{Provider: opencodeProviderName, Item: protocol.TimelineItem{Type: "text", Text: "filler"}},
		Timestamp: time.Now(),
	}
	ch <- filler1
	ch <- filler2

	// Emit a critical event. The buggy callback drops it immediately.
	session.notifySubscribers(AgentStreamEvent{
		AgentID:   "test-agent",
		Event:     protocol.TurnCanceledStreamEvent{Provider: opencodeProviderName},
		Timestamp: time.Now(),
	})
	time.Sleep(50 * time.Millisecond)

	// Drain ch and confirm no critical event was delivered
	var foundCritical bool
	drainDone := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt := <-ch:
			if evt.IsCriticalEvent() {
				foundCritical = true
			}
		case <-drainDone:
			break drainLoop
		}
	}

	if foundCritical {
		t.Error("buggy callback unexpectedly delivered the critical event")
	}
	// Confirmed: the buggy pattern drops critical events when channel is full.
}

// TestSubscribe_NeverDropsCriticalEvents verifies that the Subscribe() method
// correctly blocks on critical events even when the channel is full.
func TestSubscribe_NeverDropsCriticalEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession("http://127.0.0.1:0", "test-session", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("command warmup did not complete")
	}

	ch := session.Subscribe()

	// Fill the channel to capacity with non-critical events
	for i := 0; i < 256; i++ {
		session.notifySubscribers(AgentStreamEvent{
			Event:     protocol.TimelineStreamEvent{Provider: opencodeProviderName, Item: TimelineItem{Type: "assistant_message", Text: "filler"}},
			Timestamp: time.Now(),
		})
	}
	time.Sleep(50 * time.Millisecond)

	// Emit a critical event while the channel is full
	session.notifySubscribers(AgentStreamEvent{
		AgentID:   "test-agent",
		Event:     protocol.TurnFailedStreamEvent{Provider: opencodeProviderName, Error: "failed"},
		Timestamp: time.Now(),
	})

	// Drain all events and check that the critical event is present.
	var foundCritical bool
	timeout := time.After(3 * time.Second)
drainLoop:
	for {
		select {
		case evt := <-ch:
			if evt.IsCriticalEvent() {
				foundCritical = true
				break drainLoop
			}
		case <-timeout:
			break drainLoop
		}
	}

	if !foundCritical {
		t.Error("Subscribe() dropped a critical event -- blocking send for critical events is broken")
	}
}

// TestRun_EmitsUserMessageBeforeAssistantEvents verifies that OpenCode's Run()
// synthesizes and emits a user_message event before any assistant events arrive
// from the SSE stream. OpenCode does not echo the user prompt, so Solo must
// emit this event itself; otherwise other connected clients never see the
// prompt. This is a regression test for the cross-device sync bug where the
// app missed prompts sent from the web.
func TestRun_EmitsUserMessageBeforeAssistantEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/command":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[]`)
			return
		case r.URL.Path == "/global/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("response writer does not support flushing")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// Wait for the prompt to be sent so the test does not race.
			time.Sleep(50 * time.Millisecond)

			// Emit an assistant text delta.
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.delta\",\"properties\":{\"sessionID\":\"test-session\",\"partID\":\"p1\",\"field\":\"text\",\"delta\":\"Hi there\"}}}\n\n")
			flusher.Flush()

			// Terminal event so Run() can return successfully.
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"session.status\",\"properties\":{\"sessionID\":\"test-session\",\"status\":{\"type\":\"idle\"}}}}\n\n")
			flusher.Flush()
			return
		case strings.HasSuffix(r.URL.Path, "/prompt_async"):
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("command warmup did not complete")
	}

	// Subscribe before Run() so we receive the user_message emit.
	eventsCh := session.Subscribe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.Run(ctx, "hello opencode", nil, nil, "msg-123")
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}

	// Drain events and verify ordering.
	var sawUserMessage bool
	var userMessageText string
	var assistantBeforeUser bool
	timeout := time.After(3 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-eventsCh:
			if !ok {
				break drainLoop
			}
			switch e := evt.Event.(type) {
			case protocol.TimelineStreamEvent:
				switch e.Item.Type {
				case "user_message":
					sawUserMessage = true
					userMessageText = e.Item.Text
				case "assistant_message":
					if !sawUserMessage {
						assistantBeforeUser = true
					}
				}
			case protocol.TurnCompletedStreamEvent:
				// Terminal event; the channel will close shortly after consumeSSE returns.
			}
		case <-timeout:
			break drainLoop
		}
	}

	if !sawUserMessage {
		t.Fatal("Run() did not emit user_message -- other clients will not see the prompt")
	}
	if userMessageText != "hello opencode" {
		t.Fatalf("user_message text = %q, want %q", userMessageText, "hello opencode")
	}
	if assistantBeforeUser {
		t.Fatal("assistant_message appeared before user_message -- ordering is broken")
	}
}

// TestFinishForegroundTurn_ClearsRunningTools verifies that runningToolCalls
// is cleared on turn completion, failure, and cancel.
func TestFinishForegroundTurn_ClearsRunningTools(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &protocol.AgentSessionConfig{Provider: "opencode"}
	session := newOpenCodeSession("http://127.0.0.1:0", "test-session", config, logger, func() {}, nil)

	session.mu.Lock()
	session.runningToolCalls = map[string]timelineItem{
		"tc-1": {CallID: "tc-1", Name: "read_file", Status: "running"},
		"tc-2": {CallID: "tc-2", Name: "edit_file", Status: "running"},
	}
	session.activeForegroundTurnID = "turn-1"
	session.mu.Unlock()

	session.finishForegroundTurn(AgentStreamEvent{
		Event: protocol.TurnCompletedStreamEvent{Provider: "opencode"},
	}, "turn-1")

	session.mu.Lock()
	remaining := len(session.runningToolCalls)
	session.mu.Unlock()

	if remaining != 0 {
		t.Fatalf("expected runningToolCalls to be cleared after turn_completed, got %d remaining", remaining)
	}
}

// TestFinishForegroundTurn_EmitsTurnCompleted verifies that finishForegroundTurn
// emits TurnCompletedStreamEvent through the dispatcher. Previously, the type
// switch in finishForegroundTurn was missing the TurnCompletedStreamEvent case,
// causing the event to be silently dropped.
func TestFinishForegroundTurn_EmitsTurnCompleted(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &protocol.AgentSessionConfig{Provider: "opencode"}
	session := newOpenCodeSession("http://127.0.0.1:0", "test-session", config, logger, func() {}, nil)

	ch := session.Subscribe()

	session.mu.Lock()
	session.activeForegroundTurnID = "turn-1"
	session.mu.Unlock()

	session.finishForegroundTurn(AgentStreamEvent{
		Event: protocol.TurnCompletedStreamEvent{Provider: "opencode"},
	}, "turn-1")

	select {
	case evt := <-ch:
		if _, ok := evt.Event.(protocol.TurnCompletedStreamEvent); !ok {
			t.Fatalf("expected TurnCompletedStreamEvent, got %T", evt.Event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for TurnCompletedStreamEvent from dispatcher")
	}
}
