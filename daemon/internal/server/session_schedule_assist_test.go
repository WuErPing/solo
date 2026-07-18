package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// newStubChatCompletionServer returns an httptest server implementing the
// OpenAI-compatible chat completions endpoint with a canned assistant content.
func newStubChatCompletionServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	contentJSON, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s}}]}`, contentJSON)
	}))
	t.Cleanup(stub.Close)
	return stub
}

func sendScheduleAssistRequest(t *testing.T, conn *websocket.Conn, requestID, message string) {
	t.Helper()
	if err := conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/assist",
			"requestId": requestID,
			"message":   message,
			"timezone":  "Asia/Shanghai",
			"clientNow": "2026-07-17T22:50:00+08:00",
		}),
	}); err != nil {
		t.Fatalf("write schedule/assist: %v", err)
	}
}

func TestHandleScheduleAssist_ProposalRoundTrip(t *testing.T) {
	llmOutput := "```json\n" + `{"kind":"proposal","op":"create","name":"Nightly test summary","prompt":"Summarize the nightly test runs","cadence":{"type":"cron","expression":"0 9 * * 1-5"},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/work"}},"summary":"Create nightly summary"}` + "\n```"
	stub := newStubChatCompletionServer(t, llmOutput)

	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-assist")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Configure an LLM provider through the real set_daemon_config path.
	if err := conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "set_daemon_config_request",
			"requestId": "req-set-cfg",
			"config": map[string]interface{}{
				"llmProviders": []interface{}{
					map[string]interface{}{
						"id":      "test-llm",
						"label":   "Test LLM",
						"baseURL": stub.URL + "/v1",
						"apiKey":  "test-key",
						"models": []interface{}{
							map[string]interface{}{"id": "model-1", "isDefault": true},
						},
					},
				},
			},
		}),
	}); err != nil {
		t.Fatalf("write set_daemon_config: %v", err)
	}
	readUntilType(t, conn, "set_daemon_config_response")

	sendScheduleAssistRequest(t, conn, "req-assist-1", "every weekday at 9am summarize the nightly test runs")

	resp := readUntilType(t, conn, "schedule/assist/response")
	payload := decodeSessionPayload[protocol.ScheduleAssistResponsePayload](t, resp)

	if payload.RequestID != "req-assist-1" {
		t.Errorf("requestId = %q", payload.RequestID)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s (message: %s)", *payload.Error, payload.Message)
	}
	if payload.Kind != "proposal" {
		t.Fatalf("kind = %q, want proposal", payload.Kind)
	}
	if payload.Proposal == nil {
		t.Fatal("expected proposal")
	}
	if payload.Proposal.Op != "create" {
		t.Errorf("op = %q", payload.Proposal.Op)
	}
	if payload.Proposal.Cadence == nil || payload.Proposal.Cadence.Expression != "0 9 * * 1-5" {
		t.Errorf("cadence = %+v", payload.Proposal.Cadence)
	}
	if payload.Proposal.NextRunAt == nil {
		t.Error("expected NextRunAt preview")
	}
	if payload.LLMProvider != "test-llm" {
		t.Errorf("llmProvider = %q, want test-llm", payload.LLMProvider)
	}
	if payload.Model != "model-1" {
		t.Errorf("model = %q, want model-1", payload.Model)
	}
}

func TestHandleScheduleAssist_NoProvider(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-assist-noprov")
	defer conn.Close()
	readInitialMessages(t, conn)

	sendScheduleAssistRequest(t, conn, "req-assist-noprov", "every weekday at 9am summarize tests")

	resp := readUntilType(t, conn, "schedule/assist/response")
	payload := decodeSessionPayload[protocol.ScheduleAssistResponsePayload](t, resp)

	if payload.RequestID != "req-assist-noprov" {
		t.Errorf("requestId = %q", payload.RequestID)
	}
	if payload.Kind != "error" {
		t.Fatalf("kind = %q, want error", payload.Kind)
	}
	if payload.Error == nil || *payload.Error != "no_llm_provider" {
		t.Fatalf("error = %v, want no_llm_provider", payload.Error)
	}
}
