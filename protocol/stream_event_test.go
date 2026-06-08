package protocol

import (
	"encoding/json"
	"testing"
)

// TestStreamEventMarshalRoundTrip verifies that each StreamEvent serializes
// to the same JSON shape previously produced by map[string]interface{}.
func TestStreamEventMarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		event    StreamEvent
		wantJSON string
	}{
		{
			name: "timeline_assistant_message",
			event: TimelineStreamEvent{
				Item: TimelineItem{
					Type: "assistant_message",
					Text: "hello world",
				},
				Provider: "opencode",
			},
			wantJSON: `{"type":"timeline","item":{"type":"assistant_message","text":"hello world"},"provider":"opencode"}`,
		},
		{
			name: "timeline_tool_call",
			event: TimelineStreamEvent{
				Item: TimelineItem{
					Type:   "tool_call",
					CallID: "call-1",
					Name:   "shell",
					Status: "running",
					Detail: ShellDetail{Type: "shell", Command: "ls"},
				},
				Provider: "opencode",
				TurnID:   "turn-1",
			},
			wantJSON: `{"type":"timeline","item":{"type":"tool_call","callId":"call-1","name":"shell","detail":{"type":"shell","command":"ls"},"status":"running"},"provider":"opencode","turnId":"turn-1"}`,
		},
		{
			name: "timeline_reasoning",
			event: TimelineStreamEvent{
				Item: TimelineItem{
					Type: "reasoning",
					Text: "thinking...",
				},
				Provider: "claude",
			},
			wantJSON: `{"type":"timeline","item":{"type":"reasoning","text":"thinking..."},"provider":"claude"}`,
		},
		{
			name: "turn_completed_without_usage",
			event: TurnCompletedStreamEvent{
				Provider: "opencode",
			},
			wantJSON: `{"type":"turn_completed","provider":"opencode"}`,
		},
		{
			name: "turn_completed_with_usage",
			event: TurnCompletedStreamEvent{
				Provider: "kimi",
				Usage: &AgentUsage{
					InputTokens:  ptrFloat64(100),
					OutputTokens: ptrFloat64(50),
				},
			},
			wantJSON: `{"type":"turn_completed","provider":"kimi","usage":{"inputTokens":100,"outputTokens":50}}`,
		},
		{
			name: "turn_failed",
			event: TurnFailedStreamEvent{
				Provider: "claude",
				Error:    "something went wrong",
			},
			wantJSON: `{"type":"turn_failed","provider":"claude","error":"something went wrong"}`,
		},
		{
			name: "turn_canceled",
			event: TurnCanceledStreamEvent{
				Provider: "opencode",
			},
			wantJSON: `{"type":"turn_canceled","provider":"opencode"}`,
		},
		{
			name: "usage_updated",
			event: UsageUpdatedStreamEvent{
				Provider: "opencode",
				Usage: &AgentUsage{
					InputTokens: ptrFloat64(200),
				},
			},
			wantJSON: `{"type":"usage_updated","provider":"opencode","usage":{"inputTokens":200}}`,
		},
		{
			name: "thread_started",
			event: ThreadStartedStreamEvent{
				Provider:  "opencode",
				SessionID: "sess-1",
			},
			wantJSON: `{"type":"thread_started","provider":"opencode","sessionId":"sess-1"}`,
		},
		{
			name: "permission_requested",
			event: PermissionRequestedStreamEvent{
				Provider: "opencode",
				Request: PermissionRequest{
					ID:       "perm-1",
					Provider: "opencode",
					Name:     "bash",
					Kind:     "tool",
					Title:    "Run shell command",
				},
			},
			wantJSON: `{"type":"permission_requested","provider":"opencode","request":{"id":"perm-1","provider":"opencode","name":"bash","kind":"tool","title":"Run shell command"}}`,
		},
		{
			name:     "flush_signal",
			event:    FlushSignalStreamEvent{},
			wantJSON: `{"type":"flush_signal"}`,
		},
		{
			name: "attention_required",
			event: AttentionRequiredStreamEvent{
				Provider:     "claude",
				Reason:       "finished",
				ShouldNotify: true,
			},
			wantJSON: `{"type":"attention_required","provider":"claude","reason":"finished","shouldNotify":true}`,
		},
		{
			name: "session_closed",
			event: SessionClosedStreamEvent{
				Provider: "opencode",
			},
			wantJSON: `{"type":"session_closed","provider":"opencode"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("marshal\ngot  %s\nwant %s", got, tt.wantJSON)
			}
		})
	}
}

// TestAgentStreamPayloadMarshal verifies that AgentStreamPayload serializes
// the Event field correctly when it holds a StreamEvent.
func TestAgentStreamPayloadMarshal(t *testing.T) {
	payload := AgentStreamPayload{
		AgentID:   "agent-1",
		Event:     TurnCompletedStreamEvent{Provider: "opencode"},
		Timestamp: "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if parsed["agentId"] != "agent-1" {
		t.Errorf("agentId: got %v, want agent-1", parsed["agentId"])
	}

	event, ok := parsed["event"].(map[string]interface{})
	if !ok {
		t.Fatalf("event is not a map, got %T", parsed["event"])
	}
	if event["type"] != "turn_completed" {
		t.Errorf("event.type: got %v, want turn_completed", event["type"])
	}
	if event["provider"] != "opencode" {
		t.Errorf("event.provider: got %v, want opencode", event["provider"])
	}
}

// TestAgentStreamPayloadUnmarshal verifies round-trip deserialization.
func TestAgentStreamPayloadUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantType string
		wantProv string
	}{
		{
			name:     "timeline",
			json:     `{"agentId":"a1","event":{"type":"timeline","item":{"type":"assistant_message","text":"hi"},"provider":"opencode"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "timeline",
			wantProv: "opencode",
		},
		{
			name:     "turn_completed",
			json:     `{"agentId":"a1","event":{"type":"turn_completed","provider":"claude"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "turn_completed",
			wantProv: "claude",
		},
		{
			name:     "turn_failed",
			json:     `{"agentId":"a1","event":{"type":"turn_failed","provider":"kimi","error":"oops"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "turn_failed",
			wantProv: "kimi",
		},
		{
			name:     "turn_canceled",
			json:     `{"agentId":"a1","event":{"type":"turn_canceled","provider":"opencode"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "turn_canceled",
			wantProv: "opencode",
		},
		{
			name:     "usage_updated",
			json:     `{"agentId":"a1","event":{"type":"usage_updated","provider":"opencode","usage":{"inputTokens":10}},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "usage_updated",
			wantProv: "opencode",
		},
		{
			name:     "thread_started",
			json:     `{"agentId":"a1","event":{"type":"thread_started","provider":"opencode","sessionId":"s1"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "thread_started",
			wantProv: "opencode",
		},
		{
			name:     "permission_requested",
			json:     `{"agentId":"a1","event":{"type":"permission_requested","provider":"opencode","request":{"id":"p1","provider":"opencode","name":"bash","kind":"tool","title":"Run shell"}},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "permission_requested",
			wantProv: "opencode",
		},
		{
			name:     "flush_signal",
			json:     `{"agentId":"a1","event":{"type":"flush_signal"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "flush_signal",
			wantProv: "",
		},
		{
			name:     "attention_required",
			json:     `{"agentId":"a1","event":{"type":"attention_required","provider":"claude","reason":"finished","shouldNotify":false},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "attention_required",
			wantProv: "claude",
		},
		{
			name:     "session_closed",
			json:     `{"agentId":"a1","event":{"type":"session_closed","provider":"opencode"},"timestamp":"2024-01-01T00:00:00Z"}`,
			wantType: "session_closed",
			wantProv: "opencode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload AgentStreamPayload
			if err := json.Unmarshal([]byte(tt.json), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			evt, ok := payload.Event.(StreamEvent)
			if !ok {
				t.Fatalf("Event is not a StreamEvent, got %T", payload.Event)
			}
			if evt.StreamEventType() != tt.wantType {
				t.Errorf("type: got %q, want %q", evt.StreamEventType(), tt.wantType)
			}

			// Verify provider where applicable
			switch e := payload.Event.(type) {
			case TimelineStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case TurnCompletedStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case TurnFailedStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case TurnCanceledStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case UsageUpdatedStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case ThreadStartedStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case PermissionRequestedStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case AttentionRequiredStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			case SessionClosedStreamEvent:
				if e.Provider != tt.wantProv {
					t.Errorf("provider: got %q, want %q", e.Provider, tt.wantProv)
				}
			}
		})
	}
}

// TestAgentStreamPayloadBackwardCompatible verifies that old JSON produced by
// map[string]interface{} still deserializes correctly.
func TestAgentStreamPayloadBackwardCompatible(t *testing.T) {
	// This is the exact JSON shape produced by the old map-based code.
	oldJSON := `{"agentId":"a1","event":{"type":"turn_completed","provider":"claude","usage":{"inputTokens":100}},"timestamp":"2024-01-01T00:00:00Z"}`

	var payload AgentStreamPayload
	if err := json.Unmarshal([]byte(oldJSON), &payload); err != nil {
		t.Fatalf("unmarshal old JSON: %v", err)
	}

	evt, ok := payload.Event.(TurnCompletedStreamEvent)
	if !ok {
		t.Fatalf("expected TurnCompletedStreamEvent, got %T", payload.Event)
	}
	if evt.Provider != "claude" {
		t.Errorf("provider: got %q, want claude", evt.Provider)
	}
	if evt.Usage == nil || evt.Usage.InputTokens == nil || *evt.Usage.InputTokens != 100 {
		t.Errorf("usage.inputTokens wrong")
	}
}

// TestTimelineItemMarshal verifies that moved TimelineItem serializes identically.
func TestTimelineItemMarshal(t *testing.T) {
	item := TimelineItem{
		Type:     "tool_call",
		CallID:   "c1",
		Name:     "read",
		Status:   "completed",
		Detail:   ReadDetail{Type: "read", FilePath: "/tmp/a"},
		Error:    nil,
		Metadata: map[string]interface{}{"key": "val"},
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["type"] != "tool_call" {
		t.Errorf("type: got %v", parsed["type"])
	}
	if parsed["callId"] != "c1" {
		t.Errorf("callId: got %v", parsed["callId"])
	}
	if parsed["status"] != "completed" {
		t.Errorf("status: got %v", parsed["status"])
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
