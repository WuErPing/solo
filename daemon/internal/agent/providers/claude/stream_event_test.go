package claude

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/streamevents"
	"github.com/WuErPing/solo/protocol"
)

// TestClaudeTranslator_EmitsTypedStreamEvents verifies that the Claude
// translator emits protocol.StreamEvent implementations instead of legacy
// map[string]interface{} payloads.
func TestClaudeTranslator_EmitsTypedStreamEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		permissions:      base.NewPermissionManager(),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	translator := &claudeTranslator{
		session:               sess,
		streamedContentBlocks: make(map[int]int),
	}
	now := time.Now()

	t.Run("thread_started", func(t *testing.T) {
		msg := sdkMessage{Type: "system", Subtype: "init", SessionID: "sess-1"}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		assertTypedEvent[protocol.ThreadStartedStreamEvent](t, evts[0])
	})

	t.Run("compaction loading", func(t *testing.T) {
		msg := sdkMessage{Type: "system", Subtype: "status", Status: "compacting"}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "compaction" || evt.Item.CompactionStatus != "loading" {
			t.Fatalf("expected compaction loading, got %+v", evt.Item)
		}
	})

	t.Run("compaction completed", func(t *testing.T) {
		msg := sdkMessage{Type: "system", Subtype: "compact_boundary", CompactMetadata: &sdkCompactMetadata{Trigger: "auto", PreTokens: 42}}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "compaction" || evt.Item.CompactionStatus != "completed" || evt.Item.Trigger != "auto" || evt.Item.PreTokens != 42 {
			t.Fatalf("expected compaction completed, got %+v", evt.Item)
		}
	})

	t.Run("user message", func(t *testing.T) {
		msg := sdkMessage{
			Type: "user",
			Message: mustMarshal(t, sdkUserMessage{
				Role:    "user",
				Content: []sdkUserMessageContent{{Type: "text", Text: "hello"}},
			}),
		}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "user_message" || evt.Item.Text != "hello" {
			t.Fatalf("expected user_message, got %+v", evt.Item)
		}
	})

	t.Run("assistant text via stream_event", func(t *testing.T) {
		msg := sdkMessage{
			Type: "stream_event",
			Event: mustMarshal(t, sdkStreamEvent{
				Type:  "content_block_start",
				Index: 0,
				ContentBlock: &sdkContentBlock{
					Type: "text",
					Text: "hi",
				},
			}),
		}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "assistant_message" || evt.Item.Text != "hi" {
			t.Fatalf("expected assistant_message, got %+v", evt.Item)
		}
	})

	t.Run("reasoning via stream_event", func(t *testing.T) {
		msg := sdkMessage{
			Type: "stream_event",
			Event: mustMarshal(t, sdkStreamEvent{
				Type:  "content_block_start",
				Index: 0,
				ContentBlock: &sdkContentBlock{
					Type:     "thinking",
					Thinking: "think",
				},
			}),
		}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "reasoning" || evt.Item.Text != "think" {
			t.Fatalf("expected reasoning, got %+v", evt.Item)
		}
	})

	t.Run("tool_call running via stream_event", func(t *testing.T) {
		msg := sdkMessage{
			Type: "stream_event",
			Event: mustMarshal(t, sdkStreamEvent{
				Type:  "content_block_start",
				Index: 0,
				ContentBlock: &sdkContentBlock{
					Type: "tool_use",
					ID:   "call-1",
					Name: "shell",
				},
			}),
		}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "tool_call" || evt.Item.CallID != "call-1" || evt.Item.Name != "shell" || evt.Item.Status != "running" {
			t.Fatalf("expected running tool_call, got %+v", evt.Item)
		}
	})

	t.Run("flush_signal", func(t *testing.T) {
		msg := sdkMessage{
			Type: "stream_event",
			Event: mustMarshal(t, sdkStreamEvent{
				Type:  "content_block_stop",
				Index: 0,
			}),
		}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		assertTypedEvent[protocol.FlushSignalStreamEvent](t, evts[0])
	})

	t.Run("tool_progress", func(t *testing.T) {
		msg := sdkMessage{Type: "tool_progress", ToolUseID: "call-1", ToolName: "shell"}
		evts := translator.translateMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.TimelineStreamEvent](t, evts[0])
		if evt.Item.Type != "tool_call" || evt.Item.Status != "running" {
			t.Fatalf("expected running tool_call, got %+v", evt.Item)
		}
	})

	t.Run("permission_request", func(t *testing.T) {
		// Ensure permission manager is initialized for translatePermissionRequest.
		if translator.session.permissions == nil {
			translator.session.permissions = base.NewPermissionManager()
		}
		msg := sdkMessage{Type: "permission_request", UUID: "perm-1", ToolName: "shell", Description: "run cmd"}
		evts := translator.translatePermissionRequest(msg, now)
		requireStreamEventCount(t, evts, 1)
		evt := assertTypedEvent[protocol.PermissionRequestedStreamEvent](t, evts[0])
		if evt.Request.ID != "perm-1" || evt.Request.Name != "shell" {
			t.Fatalf("expected permission request, got %+v", evt.Request)
		}
	})

	t.Run("turn_completed", func(t *testing.T) {
		msg := sdkMessage{
			Type:    "result",
			Subtype: "success",
			Usage:   &sdkUsage{InputTokens: 1, OutputTokens: 2, CacheReadInputTokens: 3},
		}
		evts := translator.translateResultMessage(msg, now)
		requireStreamEventCount(t, evts, 2)
		assertTypedEvent[protocol.UsageUpdatedStreamEvent](t, evts[0])
		completed := assertTypedEvent[protocol.TurnCompletedStreamEvent](t, evts[1])
		if completed.Provider != claudeProviderName {
			t.Fatalf("expected provider %q, got %q", claudeProviderName, completed.Provider)
		}
	})

	t.Run("turn_failed", func(t *testing.T) {
		msg := sdkMessage{Type: "result", Subtype: "error", Errors: []string{"boom"}}
		evts := translator.translateResultMessage(msg, now)
		requireStreamEventCount(t, evts, 1)
		failed := assertTypedEvent[protocol.TurnFailedStreamEvent](t, evts[0])
		if failed.Error != "boom" {
			t.Fatalf("expected error boom, got %q", failed.Error)
		}
	})
}

// TestClaudeSession_Interrupt_EmitsTypedTurnCanceled verifies that Interrupt
// emits a typed TurnCanceledStreamEvent rather than a legacy map.
func TestClaudeSession_Interrupt_EmitsTypedTurnCanceled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newTestClaudeSession(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sess.Run(ctx, "test", nil, nil, "")
	}()
	time.Sleep(50 * time.Millisecond)

	ch := sess.Subscribe()
	sess.Interrupt(context.Background())

	time.Sleep(50 * time.Millisecond)

	found := false
outer:
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				break outer
			}
			if _, ok := evt.Event.(protocol.TurnCanceledStreamEvent); ok {
				found = true
				break outer
			}
		case <-time.After(500 * time.Millisecond):
			break outer
		}
	}
	if !found {
		t.Fatal("expected Interrupt to emit a typed TurnCanceledStreamEvent")
	}
}

// TestClaudeTerminalDetector_RecognisesTypedTerminalEvents verifies that the
// terminal detector recognises typed TurnCompletedStreamEvent and
// TurnFailedStreamEvent values.
func TestClaudeTerminalDetector_RecognisesTypedTerminalEvents(t *testing.T) {
	detector := streamevents.TerminalDetector{}

	t.Run("turn_completed", func(t *testing.T) {
		evt := agent.AgentStreamEvent{Event: protocol.TurnCompletedStreamEvent{Provider: claudeProviderName}}
		result, ok, err := detector.IsTerminal(evt)
		if !ok {
			t.Fatal("expected turn_completed to be terminal")
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Canceled {
			t.Fatal("expected Canceled=false")
		}
	})

	t.Run("turn_failed", func(t *testing.T) {
		evt := agent.AgentStreamEvent{Event: protocol.TurnFailedStreamEvent{Provider: claudeProviderName, Error: "boom"}}
		result, ok, err := detector.IsTerminal(evt)
		if !ok {
			t.Fatal("expected turn_failed to be terminal")
		}
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom error, got: %v", err)
		}
		if result.Canceled {
			t.Fatal("expected Canceled=false")
		}
	})

	t.Run("timeline is not terminal", func(t *testing.T) {
		evt := agent.AgentStreamEvent{Event: protocol.TimelineStreamEvent{Item: protocol.TimelineItem{Type: "assistant_message", Text: "hi"}}}
		_, ok, _ := detector.IsTerminal(evt)
		if ok {
			t.Fatal("expected timeline event to be non-terminal")
		}
	})
}

// assertTypedEvent asserts that the emitted value is an agent.AgentStreamEvent whose
// inner Event is the requested protocol.StreamEvent type.
func assertTypedEvent[T protocol.StreamEvent](t *testing.T, v interface{}) T {
	t.Helper()
	var zero T
	streamEvt, ok := v.(agent.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected agent.AgentStreamEvent, got %T", v)
	}
	if _, ok := streamEvt.Event.(protocol.StreamEvent); !ok {
		t.Fatalf("expected Event to implement protocol.StreamEvent, got %T", streamEvt.Event)
	}
	typed, ok := streamEvt.Event.(T)
	if !ok {
		t.Fatalf("expected Event of type %T, got %T", zero, streamEvt.Event)
	}
	return typed
}

func requireStreamEventCount(t *testing.T, evts []interface{}, n int) {
	t.Helper()
	if len(evts) != n {
		t.Fatalf("expected %d events, got %d", n, len(evts))
	}
}
