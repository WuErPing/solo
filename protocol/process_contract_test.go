package protocol

import (
	"encoding/json"
	"testing"
)

func TestRestartRequestedStatusPayload_JSONShape(t *testing.T) {
	reason := "apply config"
	p := RestartRequestedStatusPayload{
		Status:    "restart_requested",
		ClientID:  "client-1",
		Reason:    &reason,
		RequestID: "req-1",
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "restart_requested" {
		t.Errorf("status = %v", got["status"])
	}
	if got["clientId"] != "client-1" {
		t.Errorf("clientId = %v", got["clientId"])
	}
	if got["reason"] != "apply config" {
		t.Errorf("reason = %v", got["reason"])
	}
	if got["requestId"] != "req-1" {
		t.Errorf("requestId = %v", got["requestId"])
	}
}

func TestRestartRequestedStatusPayload_ReasonOmitted(t *testing.T) {
	p := RestartRequestedStatusPayload{
		Status:    "restart_requested",
		ClientID:  "client-1",
		RequestID: "req-1",
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := got["reason"]; present {
		t.Errorf("reason should be omitted when nil, got %v", got["reason"])
	}
}

func TestShutdownRequestedStatusPayload_JSONShape(t *testing.T) {
	p := ShutdownRequestedStatusPayload{
		Status:    "shutdown_requested",
		ClientID:  "client-1",
		RequestID: "req-2",
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "shutdown_requested" {
		t.Errorf("status = %v", got["status"])
	}
	if got["clientId"] != "client-1" {
		t.Errorf("clientId = %v", got["clientId"])
	}
	if got["requestId"] != "req-2" {
		t.Errorf("requestId = %v", got["requestId"])
	}
	if _, present := got["reason"]; present {
		t.Errorf("shutdown payload must not carry reason, got %v", got["reason"])
	}
}

func TestProcessContract_ExitCodesDistinct(t *testing.T) {
	if ExitCodeRestartRequested == ExitCodeClean {
		t.Fatal("restart exit code must differ from clean exit code")
	}
}
