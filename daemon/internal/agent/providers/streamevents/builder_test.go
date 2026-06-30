package streamevents_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/streamevents"
	"github.com/WuErPing/solo/protocol"
)

var testTS = time.Unix(1700000000, 0).UTC()

func wrap(event interface{}) agent.AgentStreamEvent {
	return agent.AgentStreamEvent{Event: event, Timestamp: testTS}
}

func timeline(provider string, item protocol.TimelineItem) agent.AgentStreamEvent {
	return wrap(protocol.TimelineStreamEvent{Item: item, Provider: provider})
}

func TestBuilderEnvelopes(t *testing.T) {
	tok := 7.0
	usage := &protocol.AgentUsage{InputTokens: &tok}
	detail := protocol.PlainTextDetail{Type: "plain_text", Text: "summary"}
	req := protocol.PermissionRequest{ID: "r1", Provider: "claude", Name: "bash", Kind: "tool"}

	tests := []struct {
		name string
		run  func(b *streamevents.Builder)
		want []interface{}
	}{
		{
			name: "thread_started",
			run:  func(b *streamevents.Builder) { b.ThreadStarted("sess-1") },
			want: []interface{}{wrap(protocol.ThreadStartedStreamEvent{Provider: "claude", SessionID: "sess-1"})},
		},
		{
			name: "user_message",
			run:  func(b *streamevents.Builder) { b.UserMessage("hi", "msg-1") },
			want: []interface{}{timeline("claude", protocol.TimelineItem{Type: "user_message", Text: "hi", MessageID: "msg-1"})},
		},
		{
			name: "user_message empty is no-op",
			run:  func(b *streamevents.Builder) { b.UserMessage("", "msg-1") },
			want: nil,
		},
		{
			name: "assistant_message",
			run:  func(b *streamevents.Builder) { b.AssistantMessage("hello") },
			want: []interface{}{timeline("claude", protocol.TimelineItem{Type: "assistant_message", Text: "hello"})},
		},
		{
			name: "assistant_message empty is no-op",
			run:  func(b *streamevents.Builder) { b.AssistantMessage("") },
			want: nil,
		},
		{
			name: "reasoning",
			run:  func(b *streamevents.Builder) { b.Reasoning("think") },
			want: []interface{}{timeline("claude", protocol.TimelineItem{Type: "reasoning", Text: "think"})},
		},
		{
			name: "reasoning empty is no-op",
			run:  func(b *streamevents.Builder) { b.Reasoning("") },
			want: nil,
		},
		{
			name: "tool_call running with detail",
			run:  func(b *streamevents.Builder) { b.ToolCall("c1", "task", detail, "running") },
			want: []interface{}{timeline("claude", protocol.TimelineItem{Type: "tool_call", CallID: "c1", Name: "task", Detail: detail, Status: "running"})},
		},
		{
			name: "tool_call result without name",
			run:  func(b *streamevents.Builder) { b.ToolCall("c1", "", nil, "completed") },
			want: []interface{}{timeline("claude", protocol.TimelineItem{Type: "tool_call", CallID: "c1", Status: "completed"})},
		},
		{
			name: "usage",
			run:  func(b *streamevents.Builder) { b.Usage(usage) },
			want: []interface{}{wrap(protocol.UsageUpdatedStreamEvent{Provider: "claude", Usage: usage})},
		},
		{
			name: "usage nil is no-op",
			run:  func(b *streamevents.Builder) { b.Usage(nil) },
			want: nil,
		},
		{
			name: "permission_requested",
			run:  func(b *streamevents.Builder) { b.PermissionRequested(req) },
			want: []interface{}{wrap(protocol.PermissionRequestedStreamEvent{Provider: "claude", Request: req})},
		},
		{
			name: "timeline escape hatch",
			run: func(b *streamevents.Builder) {
				b.Timeline(protocol.TimelineItem{Type: "todo", TodoItems: []protocol.TodoItem{{Text: "x"}}})
			},
			want: []interface{}{timeline("claude", protocol.TimelineItem{Type: "todo", TodoItems: []protocol.TodoItem{{Text: "x"}}})},
		},
		{
			name: "raw escape hatch",
			run:  func(b *streamevents.Builder) { b.Raw(protocol.FlushSignalStreamEvent{Type: "flush_signal"}) },
			want: []interface{}{wrap(protocol.FlushSignalStreamEvent{Type: "flush_signal"})},
		},
		{
			name: "turn_completed with usage",
			run:  func(b *streamevents.Builder) { b.TurnCompleted(usage) },
			want: []interface{}{wrap(protocol.TurnCompletedStreamEvent{Provider: "claude", Usage: usage})},
		},
		{
			name: "turn_failed",
			run:  func(b *streamevents.Builder) { b.TurnFailed("boom") },
			want: []interface{}{wrap(protocol.TurnFailedStreamEvent{Provider: "claude", Error: "boom"})},
		},
		{
			name: "turn_canceled",
			run:  func(b *streamevents.Builder) { b.TurnCanceled("interrupted") },
			want: []interface{}{wrap(protocol.TurnCanceledStreamEvent{Provider: "claude", Reason: "interrupted"})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := streamevents.New("claude", testTS)
			tt.run(b)
			got := b.Events()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("events mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestBuilderTerminalFlag(t *testing.T) {
	cases := []struct {
		name     string
		run      func(b *streamevents.Builder)
		terminal bool
	}{
		{"assistant not terminal", func(b *streamevents.Builder) { b.AssistantMessage("x") }, false},
		{"reasoning not terminal", func(b *streamevents.Builder) { b.Reasoning("x") }, false},
		{"tool_call not terminal", func(b *streamevents.Builder) { b.ToolCall("c", "n", nil, "running") }, false},
		{"turn_completed terminal", func(b *streamevents.Builder) { b.TurnCompleted(nil) }, true},
		{"turn_failed terminal", func(b *streamevents.Builder) { b.TurnFailed("e") }, true},
		{"turn_canceled terminal", func(b *streamevents.Builder) { b.TurnCanceled("r") }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := streamevents.New("kimi", testTS)
			tc.run(b)
			if got := b.Terminal(); got != tc.terminal {
				t.Errorf("Terminal() = %v, want %v", got, tc.terminal)
			}
		})
	}
}

func TestBuilderChaining(t *testing.T) {
	b := streamevents.New("pi", testTS)
	b.ThreadStarted("s").AssistantMessage("a").TurnCompleted(nil)
	got := b.Events()
	want := []interface{}{
		wrap(protocol.ThreadStartedStreamEvent{Provider: "pi", SessionID: "s"}),
		timeline("pi", protocol.TimelineItem{Type: "assistant_message", Text: "a"}),
		wrap(protocol.TurnCompletedStreamEvent{Provider: "pi"}),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("chained events mismatch\n got: %#v\nwant: %#v", got, want)
	}
	if !b.Terminal() {
		t.Errorf("expected terminal after TurnCompleted")
	}
}
