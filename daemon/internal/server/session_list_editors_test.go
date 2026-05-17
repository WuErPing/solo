package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestListAvailableEditorsPayloadSerializesErrorField(t *testing.T) {
	payload := protocol.ListAvailableEditorsPayload{
		RequestID: "test-req",
		Editors:   []protocol.EditorTarget{},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"error"`) {
		t.Errorf("error field missing from JSON serialization: %s", string(data))
	}
	if !strings.Contains(string(data), `"error":null`) {
		t.Errorf("error field should be null when not set: %s", string(data))
	}
}
