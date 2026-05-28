package protocol

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestTerminalFrameRoundTrip(t *testing.T) {
	frame := TerminalStreamFrame{
		Opcode:  TerminalOutput,
		Slot:    1,
		Payload: []byte("hello terminal"),
	}
	encoded := EncodeTerminalFrame(frame)
	decoded := DecodeTerminalFrame(encoded)
	if decoded == nil {
		t.Fatal("failed to decode frame")
	}
	if decoded.Opcode != frame.Opcode {
		t.Errorf("opcode: got %d, want %d", decoded.Opcode, frame.Opcode)
	}
	if decoded.Slot != frame.Slot {
		t.Errorf("slot: got %d, want %d", decoded.Slot, frame.Slot)
	}
	if string(decoded.Payload) != string(frame.Payload) {
		t.Errorf("payload: got %q, want %q", decoded.Payload, frame.Payload)
	}
}

func TestTerminalFrameTooShort(t *testing.T) {
	if DecodeTerminalFrame([]byte{0x01}) != nil {
		t.Error("expected nil for too-short frame")
	}
	if DecodeTerminalFrame(nil) != nil {
		t.Error("expected nil for nil frame")
	}
}

func TestTerminalFrameInvalidOpcode(t *testing.T) {
	if DecodeTerminalFrame([]byte{0xFF, 0x00}) != nil {
		t.Error("expected nil for invalid opcode")
	}
}

func TestResizePayload(t *testing.T) {
	data, _ := json.Marshal(TerminalResizePayload{Rows: 24, Cols: 80})
	r, err := DecodeTerminalResize(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r.Rows != 24 || r.Cols != 80 {
		t.Errorf("got rows=%d cols=%d", r.Rows, r.Cols)
	}
}

func TestInboundMessageRegistry(t *testing.T) {
	tests := []struct {
		msgType string
		json    string
	}{
		{"ping", `{"type":"ping","requestId":"r1"}`},
		{"client_heartbeat", `{"type":"client_heartbeat","deviceType":"web","focusedAgentId":null,"lastActivityAt":"2024-01-01T00:00:00Z","appVisible":true}`},
		{"create_agent_request", `{"type":"create_agent_request","config":{"provider":"claude","cwd":"/tmp"},"requestId":"r2","labels":{}}`},
		{"fetch_agents_request", `{"type":"fetch_agents_request","requestId":"r3"}`},
		{"send_agent_message_request", `{"type":"send_agent_message_request","requestId":"r4","agentId":"a1","text":"hello"}`},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			msg, err := DecodeSessionInboundMessage(json.RawMessage(tt.json))
			if err != nil {
				t.Fatalf("decode %s: %v", tt.msgType, err)
			}
			if msg.MsgType() != tt.msgType {
				t.Errorf("MsgType: got %q, want %q", msg.MsgType(), tt.msgType)
			}
		})
	}
}

func TestUnknownInboundMessage(t *testing.T) {
	_, err := DecodeSessionInboundMessage(json.RawMessage(`{"type":"nonexistent_type"}`))
	if err == nil {
		t.Error("expected error for unknown message type")
	}
}

func TestClearAgentAttentionDecodesStringOrArrayAgentID(t *testing.T) {
	single, err := DecodeSessionInboundMessage(json.RawMessage(`{"type":"clear_agent_attention","requestId":"r1","agentId":"a1"}`))
	if err != nil {
		t.Fatalf("decode single: %v", err)
	}
	singleMsg := single.(*ClearAgentAttention)
	if len(singleMsg.AgentID) != 1 || singleMsg.AgentID[0] != "a1" {
		t.Fatalf("single agentId: got %#v, want [a1]", singleMsg.AgentID)
	}

	many, err := DecodeSessionInboundMessage(json.RawMessage(`{"type":"clear_agent_attention","requestId":"r2","agentId":["a1","a2"]}`))
	if err != nil {
		t.Fatalf("decode array: %v", err)
	}
	manyMsg := many.(*ClearAgentAttention)
	if len(manyMsg.AgentID) != 2 || manyMsg.AgentID[0] != "a1" || manyMsg.AgentID[1] != "a2" {
		t.Fatalf("array agentId: got %#v, want [a1 a2]", manyMsg.AgentID)
	}
}

func TestWSOutboundMarshal(t *testing.T) {
	msg := NewSessionMessage(&StatusMessage{
		Type: "status",
		Payload: ServerInfoPayload{
			Status:   "server_info",
			ServerID: "test",
		},
	})
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["type"] != "session" {
		t.Errorf("expected type=session, got %v", parsed["type"])
	}
}

func TestNewPongMessage(t *testing.T) {
	msg := NewPongMessage()
	if msg.Type != "pong" {
		t.Errorf("Type: got %q, want pong", msg.Type)
	}
}

func TestAllInboundMessageTypes(t *testing.T) {
	for msgType := range inboundRegistry {
		t.Run(msgType, func(t *testing.T) {
			var raw json.RawMessage
			if msgType == "clear_agent_attention" {
				raw = json.RawMessage(`{"type":"clear_agent_attention","agentId":"a1"}`)
			} else {
				raw = json.RawMessage(fmt.Sprintf(`{"type":"%s"}`, msgType))
			}
			msg, err := DecodeSessionInboundMessage(raw)
			if err != nil {
				t.Fatalf("decode %s: %v", msgType, err)
			}
			if msg.MsgType() != msgType {
				t.Errorf("MsgType: got %q, want %q", msg.MsgType(), msgType)
			}
		})
	}
}

func TestDecodeSessionInboundMessageInvalidJSON(t *testing.T) {
	_, err := DecodeSessionInboundMessage(json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeSessionInboundMessageMissingType(t *testing.T) {
	_, err := DecodeSessionInboundMessage(json.RawMessage(`{"requestId":"r1"}`))
	if err == nil {
		t.Error("expected error for missing type field")
	}
}

func TestClearAgentAttentionInvalidAgentID(t *testing.T) {
	_, err := DecodeSessionInboundMessage(json.RawMessage(`{"type":"clear_agent_attention","agentId":123}`))
	if err == nil {
		t.Error("expected error for invalid agentId type")
	}
}

func TestProtocolVersionTypeConsistency(t *testing.T) {
	// WSProtocolVersion must be assignable to WSInboundMessage.ProtocolVersion (int)
	msg := WSInboundMessage{
		ProtocolVersion: WSProtocolVersion,
	}
	if msg.ProtocolVersion != 1 {
		t.Errorf("WSProtocolVersion = %d, want 1", msg.ProtocolVersion)
	}

	// RelayProtocolVersion must be a string
	if RelayProtocolVersion != "2" {
		t.Errorf("RelayProtocolVersion = %q, want \"2\"", RelayProtocolVersion)
	}
}
