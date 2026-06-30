// Package streamevents provides a shared builder and terminal detector that
// every stdout-JSONL agent provider uses to emit standardized timeline events.
//
// Before this package each provider translator hand-built the same
// agent.AgentStreamEvent envelope (stamping the provider name and timestamp on
// every event) and shipped a near-identical terminal detector. Builder removes
// that duplication: a provider's translator parses its own wire format and calls
// the convenience methods here, so adding a provider no longer means copying the
// envelope-construction and terminal-detection boilerplate.
package streamevents

import (
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

// Builder accumulates the events produced while translating a single batch of
// provider output. It stamps Provider and Timestamp on every event and tracks
// whether a terminal (turn_completed/failed/canceled) event was emitted.
//
// A Builder is created per Translate call and is not safe for concurrent use.
type Builder struct {
	provider string
	ts       time.Time
	events   []interface{}
	terminal bool
}

// New returns a Builder that stamps provider and ts onto every emitted event.
func New(provider string, ts time.Time) *Builder {
	return &Builder{provider: provider, ts: ts}
}

func (b *Builder) emit(event interface{}) *Builder {
	b.events = append(b.events, agent.AgentStreamEvent{Event: event, Timestamp: b.ts})
	return b
}

func (b *Builder) timeline(item protocol.TimelineItem) *Builder {
	return b.emit(protocol.TimelineStreamEvent{Item: item, Provider: b.provider})
}

// ThreadStarted emits a thread_started lifecycle event.
func (b *Builder) ThreadStarted(sessionID string) *Builder {
	return b.emit(protocol.ThreadStartedStreamEvent{Provider: b.provider, SessionID: sessionID})
}

// UserMessage emits a user_message timeline item. No-op when text is empty.
func (b *Builder) UserMessage(text, messageID string) *Builder {
	if text == "" {
		return b
	}
	return b.timeline(protocol.TimelineItem{Type: "user_message", Text: text, MessageID: messageID})
}

// AssistantMessage emits an assistant_message timeline item. No-op when text is empty.
func (b *Builder) AssistantMessage(text string) *Builder {
	if text == "" {
		return b
	}
	return b.timeline(protocol.TimelineItem{Type: "assistant_message", Text: text})
}

// Reasoning emits a reasoning timeline item. No-op when text is empty.
func (b *Builder) Reasoning(text string) *Builder {
	if text == "" {
		return b
	}
	return b.timeline(protocol.TimelineItem{Type: "reasoning", Text: text})
}

// ToolCall emits a tool_call timeline item. name and detail may be zero for
// result-only events (e.g. a tool completion that only carries a call ID).
func (b *Builder) ToolCall(callID, name string, detail protocol.ToolCallDetail, status string) *Builder {
	return b.timeline(protocol.TimelineItem{Type: "tool_call", CallID: callID, Name: name, Detail: detail, Status: status})
}

// Usage emits a usage_updated event. No-op when usage is nil.
func (b *Builder) Usage(usage *protocol.AgentUsage) *Builder {
	if usage == nil {
		return b
	}
	return b.emit(protocol.UsageUpdatedStreamEvent{Provider: b.provider, Usage: usage})
}

// PermissionRequested emits a permission_requested event.
func (b *Builder) PermissionRequested(req protocol.PermissionRequest) *Builder {
	return b.emit(protocol.PermissionRequestedStreamEvent{Provider: b.provider, Request: req})
}

// Timeline emits an arbitrary timeline item, stamping the provider name. Use it
// for the rare items without a dedicated convenience method (compaction, todo,
// task notifications, error text).
func (b *Builder) Timeline(item protocol.TimelineItem) *Builder {
	return b.timeline(item)
}

// Raw emits an arbitrary stream-event payload as-is (provider name not stamped).
// Use it for payloads such as FlushSignalStreamEvent that carry no provider field.
func (b *Builder) Raw(event interface{}) *Builder {
	return b.emit(event)
}

// TurnCompleted emits a turn_completed terminal event and marks the batch terminal.
func (b *Builder) TurnCompleted(usage *protocol.AgentUsage) *Builder {
	b.terminal = true
	return b.emit(protocol.TurnCompletedStreamEvent{Provider: b.provider, Usage: usage})
}

// TurnFailed emits a turn_failed terminal event and marks the batch terminal.
func (b *Builder) TurnFailed(errMsg string) *Builder {
	b.terminal = true
	return b.emit(protocol.TurnFailedStreamEvent{Provider: b.provider, Error: errMsg})
}

// TurnCanceled emits a turn_canceled terminal event and marks the batch terminal.
func (b *Builder) TurnCanceled(reason string) *Builder {
	b.terminal = true
	return b.emit(protocol.TurnCanceledStreamEvent{Provider: b.provider, Reason: reason})
}

// Events returns the accumulated events in emission order.
func (b *Builder) Events() []interface{} {
	return b.events
}

// Terminal reports whether a terminal event was emitted in this batch.
func (b *Builder) Terminal() bool {
	return b.terminal
}
