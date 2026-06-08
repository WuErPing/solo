package protocol

import (
	"encoding/json"
	"fmt"
)

// --- Stream Event Interface ---

// StreamEvent is a discriminated union of all events that can flow through
// the agent stream pipeline. Implementations use a "type" JSON field for
// dispatch. This replaces the previous map[string]interface{} representation.
type StreamEvent interface {
	StreamEventType() string
}

// --- Timeline Types (moved from daemon/internal/agent to enable protocol-level typing) ---

// TimelineItem represents a single timeline entry.
type TimelineItem struct {
	Type string `json:"type"`
	// Fields vary by type:
	// user_message: Text, MessageID
	// assistant_message: Text
	// reasoning: Text
	// tool_call: CallID, Name, Detail, Status, Error, Metadata
	// todo: TodoItems
	// error: Message
	// compaction: CompactionStatus, Trigger, PreTokens
	Text             string                 `json:"text,omitempty"`
	MessageID        string                 `json:"messageId,omitempty"`
	CallID           string                 `json:"callId,omitempty"`
	Name             string                 `json:"name,omitempty"`
	Detail           ToolCallDetail         `json:"detail,omitempty"`
	Status           string                 `json:"status,omitempty"` // running|completed|failed|canceled
	Error            *ToolError             `json:"error,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	TodoItems        []TodoItem             `json:"items,omitempty"`
	Message          string                 `json:"message,omitempty"` // for error type
	CompactionStatus string                 `json:"compactionStatus,omitempty"`
	Trigger          string                 `json:"trigger,omitempty"`
	PreTokens        int                    `json:"preTokens,omitempty"`
}

// TodoItem represents a single todo entry.
type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

// UnmarshalJSON implements custom deserialization for Detail (ToolCallDetail) and Error (*ToolError).
func (item *TimelineItem) UnmarshalJSON(data []byte) error {
	type alias TimelineItem
	var raw struct {
		*alias
		Detail json.RawMessage `json:"detail,omitempty"`
		Error  json.RawMessage `json:"error,omitempty"`
	}
	raw.alias = (*alias)(item)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if len(raw.Detail) > 0 && string(raw.Detail) != "null" {
		var wrapper ToolCallDetailWrapper
		if err := json.Unmarshal(raw.Detail, &wrapper); err == nil {
			item.Detail = wrapper.Detail
		}
	}

	if len(raw.Error) > 0 && string(raw.Error) != "null" {
		var errVal ToolError
		if err := json.Unmarshal(raw.Error, &errVal); err == nil {
			item.Error = &errVal
		}
	}

	return nil
}

// ToProtocolMap converts a TimelineItem to a protocol-compatible map.
// This matches the wire format expected by clients.
func (item *TimelineItem) ToProtocolMap() map[string]interface{} {
	m := map[string]interface{}{
		"type": item.Type,
	}
	switch item.Type {
	case "user_message":
		m["text"] = item.Text
		if item.MessageID != "" {
			m["messageId"] = item.MessageID
		}
	case "assistant_message":
		m["text"] = item.Text
	case "reasoning":
		m["text"] = item.Text
	case "tool_call":
		m["callId"] = item.CallID
		m["name"] = item.Name
		if item.Detail != nil {
			data, _ := json.Marshal(item.Detail)
			var detailMap map[string]interface{}
			if err := json.Unmarshal(data, &detailMap); err == nil {
				m["detail"] = detailMap
			}
		}
		m["status"] = item.Status
		if item.Error != nil {
			m["error"] = item.Error.Message
		}
		if item.Metadata != nil {
			m["metadata"] = item.Metadata
		}
	case "todo":
		m["items"] = item.TodoItems
	case "error":
		m["message"] = item.Message
	case "compaction":
		m["status"] = item.CompactionStatus
		if item.Trigger != "" {
			m["trigger"] = item.Trigger
		}
	}
	return m
}

// PermissionRequest represents a permission request payload.
type PermissionRequest struct {
	ID          string                 `json:"id"`
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Kind        string                 `json:"kind"`
	Title       string                 `json:"title"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Detail      map[string]interface{} `json:"detail,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// --- Stream Event Implementations ---

// TimelineStreamEvent wraps a timeline item.
type TimelineStreamEvent struct {
	Type     string       `json:"type"` // always "timeline"
	Item     TimelineItem `json:"item"`
	Provider string       `json:"provider"`
	TurnID   string       `json:"turnId,omitempty"`
}

func (TimelineStreamEvent) StreamEventType() string { return "timeline" }

// MarshalJSON ensures the "type" field is always correct.
func (e TimelineStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias TimelineStreamEvent
	return json.Marshal(alias(e))
}

// TurnCompletedStreamEvent signals a successful turn finish.
type TurnCompletedStreamEvent struct {
	Type     string      `json:"type"` // always "turn_completed"
	Provider string      `json:"provider"`
	Usage    *AgentUsage `json:"usage,omitempty"`
}

func (TurnCompletedStreamEvent) StreamEventType() string { return "turn_completed" }

// MarshalJSON ensures the "type" field is always correct.
func (e TurnCompletedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias TurnCompletedStreamEvent
	return json.Marshal(alias(e))
}

// TurnFailedStreamEvent signals a failed turn.
type TurnFailedStreamEvent struct {
	Type     string `json:"type"` // always "turn_failed"
	Provider string `json:"provider"`
	Error    string `json:"error,omitempty"`
}

func (TurnFailedStreamEvent) StreamEventType() string { return "turn_failed" }

// MarshalJSON ensures the "type" field is always correct.
func (e TurnFailedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias TurnFailedStreamEvent
	return json.Marshal(alias(e))
}

// TurnCanceledStreamEvent signals a canceled turn.
type TurnCanceledStreamEvent struct {
	Type     string `json:"type"` // always "turn_canceled"
	Provider string `json:"provider"`
	Reason   string `json:"reason,omitempty"`
}

func (TurnCanceledStreamEvent) StreamEventType() string { return "turn_canceled" }

// MarshalJSON ensures the "type" field is always correct.
func (e TurnCanceledStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias TurnCanceledStreamEvent
	return json.Marshal(alias(e))
}

// UsageUpdatedStreamEvent carries incremental token usage.
type UsageUpdatedStreamEvent struct {
	Type     string      `json:"type"` // always "usage_updated"
	Provider string      `json:"provider"`
	Usage    *AgentUsage `json:"usage,omitempty"`
}

func (UsageUpdatedStreamEvent) StreamEventType() string { return "usage_updated" }

// MarshalJSON ensures the "type" field is always correct.
func (e UsageUpdatedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias UsageUpdatedStreamEvent
	return json.Marshal(alias(e))
}

// ThreadStartedStreamEvent signals a new session thread.
type ThreadStartedStreamEvent struct {
	Type      string `json:"type"` // always "thread_started"
	Provider  string `json:"provider"`
	SessionID string `json:"sessionId,omitempty"`
}

func (ThreadStartedStreamEvent) StreamEventType() string { return "thread_started" }

// MarshalJSON ensures the "type" field is always correct.
func (e ThreadStartedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias ThreadStartedStreamEvent
	return json.Marshal(alias(e))
}

// PermissionRequestedStreamEvent signals a pending permission.
type PermissionRequestedStreamEvent struct {
	Type     string            `json:"type"` // always "permission_requested"
	Provider string            `json:"provider"`
	Request  PermissionRequest `json:"request"`
}

func (PermissionRequestedStreamEvent) StreamEventType() string { return "permission_requested" }

// MarshalJSON ensures the "type" field is always correct.
func (e PermissionRequestedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias PermissionRequestedStreamEvent
	return json.Marshal(alias(e))
}

// PermissionResolvedStreamEvent signals a permission response.
type PermissionResolvedStreamEvent struct {
	Type      string `json:"type"` // always "permission_resolved"
	Provider  string `json:"provider"`
	RequestID string `json:"requestId,omitempty"`
}

func (PermissionResolvedStreamEvent) StreamEventType() string { return "permission_resolved" }

// MarshalJSON ensures the "type" field is always correct.
func (e PermissionResolvedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias PermissionResolvedStreamEvent
	return json.Marshal(alias(e))
}

// FlushSignalStreamEvent signals the coalescer to flush immediately.
type FlushSignalStreamEvent struct {
	Type string `json:"type"` // always "flush_signal"
}

func (FlushSignalStreamEvent) StreamEventType() string { return "flush_signal" }

// MarshalJSON ensures the "type" field is always correct.
func (e FlushSignalStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias FlushSignalStreamEvent
	return json.Marshal(alias(e))
}

// AttentionRequiredStreamEvent signals that an agent needs user attention.
type AttentionRequiredStreamEvent struct {
	Type         string                 `json:"type"` // always "attention_required"
	Provider     string                 `json:"provider"`
	Reason       string                 `json:"reason"`
	Timestamp    string                 `json:"timestamp,omitempty"`
	ShouldNotify bool                   `json:"shouldNotify"`
	Notification map[string]interface{} `json:"notification,omitempty"`
}

func (AttentionRequiredStreamEvent) StreamEventType() string { return "attention_required" }

// MarshalJSON ensures the "type" field is always correct.
func (e AttentionRequiredStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias AttentionRequiredStreamEvent
	return json.Marshal(alias(e))
}

// SessionClosedStreamEvent signals that an agent session has closed.
type SessionClosedStreamEvent struct {
	Type     string `json:"type"` // always "session_closed"
	Provider string `json:"provider"`
}

func (SessionClosedStreamEvent) StreamEventType() string { return "session_closed" }

// MarshalJSON ensures the "type" field is always correct.
func (e SessionClosedStreamEvent) MarshalJSON() ([]byte, error) {
	e.Type = e.StreamEventType()
	type alias SessionClosedStreamEvent
	return json.Marshal(alias(e))
}

// --- AgentStreamPayload Serialization ---

// rawEvent is used during unmarshalling to peek at the "type" field.
type rawEvent struct {
	Type string `json:"type"`
}

// UnmarshalJSON implements tagged-union deserialization for AgentStreamPayload.
// It supports both the new StreamEvent structs and legacy map[string]interface{}.
func (p *AgentStreamPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		AgentID   string          `json:"agentId"`
		Event     json.RawMessage `json:"event"`
		Timestamp string          `json:"timestamp"`
		Seq       *int            `json:"seq,omitempty"`
		Epoch     *string         `json:"epoch,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.AgentID = raw.AgentID
	p.Timestamp = raw.Timestamp
	p.Seq = raw.Seq
	p.Epoch = raw.Epoch

	if len(raw.Event) == 0 {
		p.Event = nil
		return nil
	}

	// Peek at the type field to dispatch.
	var typePeek rawEvent
	if err := json.Unmarshal(raw.Event, &typePeek); err != nil {
		// If we can't peek, fall back to map (legacy safety).
		var fallback map[string]interface{}
		if err2 := json.Unmarshal(raw.Event, &fallback); err2 == nil {
			p.Event = fallback
			return nil
		}
		return fmt.Errorf("unmarshal stream event: %w", err)
	}

	switch typePeek.Type {
	case "timeline":
		var evt TimelineStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal timeline event: %w", err)
		}
		p.Event = evt
	case "turn_completed":
		var evt TurnCompletedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal turn_completed event: %w", err)
		}
		p.Event = evt
	case "turn_failed":
		var evt TurnFailedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal turn_failed event: %w", err)
		}
		p.Event = evt
	case "turn_canceled":
		var evt TurnCanceledStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal turn_canceled event: %w", err)
		}
		p.Event = evt
	case "usage_updated":
		var evt UsageUpdatedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal usage_updated event: %w", err)
		}
		p.Event = evt
	case "thread_started":
		var evt ThreadStartedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal thread_started event: %w", err)
		}
		p.Event = evt
	case "permission_requested":
		var evt PermissionRequestedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal permission_requested event: %w", err)
		}
		p.Event = evt
	case "permission_resolved":
		var evt PermissionResolvedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal permission_resolved event: %w", err)
		}
		p.Event = evt
	case "flush_signal":
		var evt FlushSignalStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal flush_signal event: %w", err)
		}
		p.Event = evt
	case "attention_required":
		var evt AttentionRequiredStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal attention_required event: %w", err)
		}
		p.Event = evt
	case "session_closed":
		var evt SessionClosedStreamEvent
		if err := json.Unmarshal(raw.Event, &evt); err != nil {
			return fmt.Errorf("unmarshal session_closed event: %w", err)
		}
		p.Event = evt
	default:
		// Unknown type: fall back to map for forward compatibility.
		var fallback map[string]interface{}
		if err := json.Unmarshal(raw.Event, &fallback); err != nil {
			return fmt.Errorf("unmarshal unknown event type %q: %w", typePeek.Type, err)
		}
		p.Event = fallback
	}

	return nil
}
