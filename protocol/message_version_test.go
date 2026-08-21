package protocol

import (
	"encoding/json"
	"testing"
)

func TestListDaemonVersionsRequest_DecodesViaRegistry(t *testing.T) {
	raw := `{"type":"list_daemon_versions_request","requestId":"req-1"}`
	msg, err := DecodeSessionInboundMessage(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m, ok := msg.(*ListDaemonVersionsRequest)
	if !ok {
		t.Fatalf("decoded %T, want *ListDaemonVersionsRequest", msg)
	}
	if m.RequestID != "req-1" {
		t.Errorf("requestId = %q", m.RequestID)
	}
}

func TestSwitchDaemonVersionRequest_DecodesViaRegistry(t *testing.T) {
	raw := `{"type":"switch_daemon_version_request","version":"solo-v0.8.0","requestId":"req-2"}`
	msg, err := DecodeSessionInboundMessage(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m, ok := msg.(*SwitchDaemonVersionRequest)
	if !ok {
		t.Fatalf("decoded %T, want *SwitchDaemonVersionRequest", msg)
	}
	if m.Version == nil || *m.Version != "solo-v0.8.0" {
		t.Errorf("version = %v", m.Version)
	}
	if m.RequestID != "req-2" {
		t.Errorf("requestId = %q", m.RequestID)
	}
}

func TestSwitchDaemonVersionRequest_VersionOmitted(t *testing.T) {
	raw := `{"type":"switch_daemon_version_request","requestId":"req-3"}`
	msg, err := DecodeSessionInboundMessage(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := msg.(*SwitchDaemonVersionRequest)
	if m.Version != nil {
		t.Errorf("version should be nil when omitted, got %v", *m.Version)
	}
}

func TestListDaemonVersionsResponse_JSONShape(t *testing.T) {
	current := "solo-v0.8.0"
	resp := ListDaemonVersionsResponse{
		Type: "list_daemon_versions_response",
		Payload: ListDaemonVersionsPayload{
			RequestID:      "req-1",
			RunningVersion: "v0.7.4",
			CurrentVersion: &current,
			Versions: []DaemonVersionInfo{
				{Version: "solo-v0.8.0", MtimeMs: 2000},
				{Version: "solo-v0.7.4", MtimeMs: 1000},
			},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "list_daemon_versions_response" {
		t.Errorf("type = %v", got["type"])
	}
	payload := got["payload"].(map[string]interface{})
	if payload["runningVersion"] != "v0.7.4" {
		t.Errorf("runningVersion = %v", payload["runningVersion"])
	}
	if payload["currentVersion"] != "solo-v0.8.0" {
		t.Errorf("currentVersion = %v", payload["currentVersion"])
	}
	versions := payload["versions"].([]interface{})
	if len(versions) != 2 {
		t.Fatalf("versions len = %d", len(versions))
	}
	first := versions[0].(map[string]interface{})
	if first["version"] != "solo-v0.8.0" || first["mtimeMs"] != float64(2000) {
		t.Errorf("versions[0] = %v", first)
	}
}

func TestDaemonVersionSwitchRequestedStatusPayload_JSONShape(t *testing.T) {
	p := DaemonVersionSwitchRequestedStatusPayload{
		Status:    "daemon_version_switch_requested",
		ClientID:  "client-1",
		Version:   "solo-v0.8.0",
		RequestID: "req-4",
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "daemon_version_switch_requested" {
		t.Errorf("status = %v", got["status"])
	}
	if got["version"] != "solo-v0.8.0" {
		t.Errorf("version = %v", got["version"])
	}
	if got["requestId"] != "req-4" {
		t.Errorf("requestId = %v", got["requestId"])
	}
}
