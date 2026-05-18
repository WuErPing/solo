package agent

import (
	"encoding/json"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// TestClaudeTranslator_ThinkingDuplicate_StartAndDelta verifies that when
// a thinking block is sent via content_block_start AND content_block_delta,
// the combined text does not create duplicate entries that survive the
// coalescer.
//
// Regression: content_block_start with initial thinking text followed by
// thinking_delta produced two reasoning timeline entries instead of one.
func TestClaudeTranslator_ThinkingDuplicate_StartAndDelta(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	translator := &claudeTranslator{
		session:               sess,
		streamedContentBlocks: make(map[int]int),
	}

	now := time.Now()

	// 1. content_block_start with initial thinking text
	startMsg := sdkMessage{
		Type: "stream_event",
		Event: mustMarshal(t, sdkStreamEvent{
			Type:  "content_block_start",
			Index: 0,
			ContentBlock: &sdkContentBlock{
				Type:     "thinking",
				Thinking: "Let me think",
			},
		}),
	}
	events1 := translator.translateMessage(startMsg, now)
	if len(events1) != 1 {
		t.Fatalf("expected 1 event from start, got %d", len(events1))
	}
	checkReasoningEvent(t, events1[0], "Let me think")

	// 2. content_block_delta with thinking_delta
	deltaMsg := sdkMessage{
		Type: "stream_event",
		Event: mustMarshal(t, sdkStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &sdkStreamDelta{
				Type:     "thinking_delta",
				Thinking: " about this",
			},
		}),
	}
	events2 := translator.translateMessage(deltaMsg, now)
	if len(events2) != 1 {
		t.Fatalf("expected 1 event from delta, got %d", len(events2))
	}
	checkReasoningEvent(t, events2[0], " about this")

	// 3. assistant message with the COMPLETE thinking text.
	// Since index 0 was marked by both start and delta, it should be skipped.
	assistantMsg := sdkMessage{
		Type: "assistant",
		Message: mustMarshal(t, sdkAssistantMessage{
			Role: "assistant",
			Content: []sdkContentBlock{
				{Type: "thinking", Thinking: "Let me think about this"},
				{Type: "text", Text: "Hello! How can I help you?"},
			},
		}),
	}
	events3 := translator.translateMessage(assistantMsg, now)
	if len(events3) != 1 {
		t.Fatalf("expected 1 event (only text block), got %d", len(events3))
	}
	checkAssistantMessageEvent(t, events3[0], "Hello! How can I help you?")
}

// TestClaudeTranslator_ThinkingDuplicate_MultipleDeltas verifies that
// multiple thinking_delta events all mark the same index and the final
// assistant message skips the block correctly.
func TestClaudeTranslator_ThinkingDuplicate_MultipleDeltas(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	translator := &claudeTranslator{
		session:               sess,
		streamedContentBlocks: make(map[int]int),
	}

	now := time.Now()

	deltas := []string{"Let me", " think", " about", " this"}
	for _, d := range deltas {
		deltaMsg := sdkMessage{
			Type: "stream_event",
			Event: mustMarshal(t, sdkStreamEvent{
				Type:  "content_block_delta",
				Index: 0,
				Delta: &sdkStreamDelta{
					Type:     "thinking_delta",
					Thinking: d,
				},
			}),
		}
		translator.translateMessage(deltaMsg, now)
	}

	// All deltas should have marked index 0
	if translator.streamedContentBlocks[0] == 0 {
		t.Fatal("expected index 0 to be marked after deltas")
	}

	// assistant message should skip index 0
	assistantMsg := sdkMessage{
		Type: "assistant",
		Message: mustMarshal(t, sdkAssistantMessage{
			Role: "assistant",
			Content: []sdkContentBlock{
				{Type: "thinking", Thinking: "Let me think about this"},
			},
		}),
	}
	events := translator.translateMessage(assistantMsg, now)
	if len(events) != 0 {
		t.Fatalf("expected 0 events (all blocks skipped), got %d", len(events))
	}
}

// TestClaudeTranslator_AssistantMessageDuplicate_WithStartAndDelta verifies
// the full flow: text block sent via start + delta, then assistant message.
func TestClaudeTranslator_AssistantMessageDuplicate_WithStartAndDelta(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	translator := &claudeTranslator{
		session:               sess,
		streamedContentBlocks: make(map[int]int),
	}

	now := time.Now()

	// 1. content_block_start (text: "Hello")
	startMsg := sdkMessage{
		Type: "stream_event",
		Event: mustMarshal(t, sdkStreamEvent{
			Type:  "content_block_start",
			Index: 0,
			ContentBlock: &sdkContentBlock{
				Type: "text",
				Text: "Hello",
			},
		}),
	}
	translator.translateMessage(startMsg, now)

	// 2. content_block_delta (text_delta: " World")
	deltaMsg := sdkMessage{
		Type: "stream_event",
		Event: mustMarshal(t, sdkStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &sdkStreamDelta{
				Type: "text_delta",
				Text: " World",
			},
		}),
	}
	translator.translateMessage(deltaMsg, now)

	// 3. assistant message with full text — should be skipped
	assistantMsg := sdkMessage{
		Type: "assistant",
		Message: mustMarshal(t, sdkAssistantMessage{
			Role: "assistant",
			Content: []sdkContentBlock{
				{Type: "text", Text: "Hello World"},
			},
		}),
	}
	events := translator.translateMessage(assistantMsg, now)
	if len(events) != 0 {
		t.Fatalf("expected 0 events (block skipped), got %d", len(events))
	}
}

// TestClaudeTranslator_ThinkingDeltaDropped_RecoversFromAssistantMessage verifies
// that when thinking deltas were marked in streamedContentBlocks but the actual
// content was shorter than the assistant message's thinking block (e.g., because
// some deltas were dropped in transit), the remaining content is emitted rather
// than silently suppressed.
//
// This is a regression test for the bug where:
//  1. content_block_start marks streamedContentBlocks[0] = true with partial text
//  2. thinking_delta is dropped (never arrives at the translator)
//  3. assistant message with the COMPLETE thinking block is suppressed because
//     streamedContentBlocks[0] was already set
//  4. Result: thinking content is permanently lost with no recovery path
func TestClaudeTranslator_ThinkingDeltaDropped_RecoversFromAssistantMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess := &claudeSession{
		base:             base.NewBaseSession(claudeProviderName, &protocol.AgentSessionConfig{}, logger),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	translator := &claudeTranslator{
		session:               sess,
		streamedContentBlocks: make(map[int]int),
	}

	now := time.Now()

	// 1. content_block_start with partial thinking text
	startMsg := sdkMessage{
		Type: "stream_event",
		Event: mustMarshal(t, sdkStreamEvent{
			Type:  "content_block_start",
			Index: 0,
			ContentBlock: &sdkContentBlock{
				Type:     "thinking",
				Thinking: "Let me think",
			},
		}),
	}
	events1 := translator.translateMessage(startMsg, now)
	if len(events1) != 1 {
		t.Fatalf("expected 1 event from start, got %d", len(events1))
	}
	checkReasoningEvent(t, events1[0], "Let me think")

	// 2. The thinking_delta with " about this" is DROPPED in transit.
	//    streamedContentBlocks[0] is already set to true from step 1.

	// 3. assistant message with the COMPLETE thinking text arrives.
	//    The assistant has "Let me think about this" but we only streamed "Let me think".
	//    The remaining " about this" should be emitted as a recovery, not suppressed.
	assistantMsg := sdkMessage{
		Type: "assistant",
		Message: mustMarshal(t, sdkAssistantMessage{
			Role: "assistant",
			Content: []sdkContentBlock{
				{Type: "thinking", Thinking: "Let me think about this"},
				{Type: "text", Text: "Here's my answer"},
			},
		}),
	}
	events2 := translator.translateMessage(assistantMsg, now)

	// Should emit 2 events: reasoning recovery + assistant_message
	if len(events2) != 2 {
		t.Fatalf("expected 2 events (reasoning recovery + text), got %d", len(events2))
	}
	checkReasoningEvent(t, events2[0], " about this")
	checkAssistantMessageEvent(t, events2[1], "Here's my answer")
}

func checkReasoningEvent(t *testing.T, evt interface{}, expectedText string) {
	t.Helper()
	streamEvt, ok := evt.(AgentStreamEvent)
	if !ok {
		t.Fatalf("expected AgentStreamEvent, got %T", evt)
	}
	payload, ok := streamEvt.Event.(map[string]interface{})
	if !ok {
		t.Fatal("expected map payload")
	}
	item, ok := payload["item"].(TimelineItem)
	if !ok {
		t.Fatal("expected TimelineItem")
	}
	if item.Type != "reasoning" {
		t.Fatalf("expected reasoning, got %s", item.Type)
	}
	if item.Text != expectedText {
		t.Fatalf("expected text %q, got %q", expectedText, item.Text)
	}
}

func checkAssistantMessageEvent(t *testing.T, evt interface{}, expectedText string) {
	t.Helper()
	streamEvt, ok := evt.(AgentStreamEvent)
	if !ok {
		t.Fatalf("expected AgentStreamEvent, got %T", evt)
	}
	payload, ok := streamEvt.Event.(map[string]interface{})
	if !ok {
		t.Fatal("expected map payload")
	}
	item, ok := payload["item"].(TimelineItem)
	if !ok {
		t.Fatal("expected TimelineItem")
	}
	if item.Type != "assistant_message" {
		t.Fatalf("expected assistant_message, got %s", item.Type)
	}
	if item.Text != expectedText {
		t.Fatalf("expected text %q, got %q", expectedText, item.Text)
	}
}
