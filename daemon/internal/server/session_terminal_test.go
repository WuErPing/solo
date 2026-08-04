package server

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

// subscribeTerminalResponsePayload mirrors the app-bridge
// SubscribeTerminalResponseSchema (app-bridge/src/shared/messages-terminal.ts):
// success carries {terminalId, slot, error: null, requestId}; failure carries
// {terminalId, error, requestId}.
type subscribeTerminalResponsePayload struct {
	TerminalID string  `json:"terminalId"`
	Slot       *int    `json:"slot"`
	Error      *string `json:"error"`
	RequestID  string  `json:"requestId"`
}

// killTerminalResponsePayload mirrors the app-bridge KillTerminalResponseSchema.
type killTerminalResponsePayload struct {
	TerminalID string `json:"terminalId"`
	Success    bool   `json:"success"`
	RequestID  string `json:"requestId"`
}

func createTerminalForTest(t *testing.T, conn *websocket.Conn, cwd, requestID string) string {
	t.Helper()
	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_terminal_request",
			"requestId": requestID,
			"cwd":       cwd,
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write create_terminal: %v", err)
	}
	resp := readUntilType(t, conn, "create_terminal_response")
	payload := decodeSessionPayload[protocol.CreateTerminalPayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("create terminal: unexpected error: %s", *payload.Error)
	}
	if payload.Terminal == nil || payload.Terminal.ID == "" {
		t.Fatal("expected terminal with non-empty ID in create_terminal_response")
	}
	return payload.Terminal.ID
}

func TestSubscribeTerminalSendsResponseOnSuccess(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-subscribe-terminal")
	defer conn.Close()
	readInitialMessages(t, conn)

	terminalID := createTerminalForTest(t, conn, t.TempDir(), "req-st-create")

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "subscribe_terminal_request",
			"requestId":  "req-st-1",
			"terminalId": terminalID,
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write subscribe_terminal: %v", err)
	}

	resp := readUntilType(t, conn, "subscribe_terminal_response")
	payload := decodeSessionPayload[subscribeTerminalResponsePayload](t, resp)
	if payload.RequestID != "req-st-1" {
		t.Fatalf("request ID: got %q, want req-st-1", payload.RequestID)
	}
	if payload.TerminalID != terminalID {
		t.Fatalf("terminal ID: got %q, want %q", payload.TerminalID, terminalID)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Slot == nil {
		t.Fatal("expected slot in subscribe_terminal_response")
	}
	if *payload.Slot < 0 || *payload.Slot > 255 {
		t.Fatalf("slot out of range: %d", *payload.Slot)
	}

	// Subscribing again must respond too, with the same slot (idempotent).
	req2 := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "subscribe_terminal_request",
			"requestId":  "req-st-2",
			"terminalId": terminalID,
		}),
	}
	if err := conn.WriteJSON(req2); err != nil {
		t.Fatalf("write second subscribe_terminal: %v", err)
	}
	resp2 := readUntilType(t, conn, "subscribe_terminal_response")
	payload2 := decodeSessionPayload[subscribeTerminalResponsePayload](t, resp2)
	if payload2.RequestID != "req-st-2" {
		t.Fatalf("request ID: got %q, want req-st-2", payload2.RequestID)
	}
	if payload2.Error != nil {
		t.Fatalf("unexpected error on re-subscribe: %s", *payload2.Error)
	}
	if payload2.Slot == nil || *payload2.Slot != *payload.Slot {
		t.Fatalf("re-subscribe slot: got %v, want %d", payload2.Slot, *payload.Slot)
	}
}

func TestSubscribeTerminalNotFoundSendsRPCError(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-subscribe-terminal-missing")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "subscribe_terminal_request",
			"requestId":  "req-st-missing",
			"terminalId": "no-such-terminal",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write subscribe_terminal: %v", err)
	}

	resp := readUntilType(t, conn, "rpc_error")
	payload := decodeSessionPayload[protocol.RPCErrorPayload](t, resp)
	if payload.RequestID != "req-st-missing" {
		t.Fatalf("request ID: got %q, want req-st-missing", payload.RequestID)
	}
	if payload.Error == "" {
		t.Fatal("expected non-empty rpc_error message")
	}
}

func TestKillTerminalSendsResponseOnSuccess(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-kill-terminal")
	defer conn.Close()
	readInitialMessages(t, conn)

	terminalID := createTerminalForTest(t, conn, t.TempDir(), "req-kt-create")

	killReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "kill_terminal_request",
			"requestId":  "req-kt-1",
			"terminalId": terminalID,
		}),
	}
	if err := conn.WriteJSON(killReq); err != nil {
		t.Fatalf("write kill_terminal: %v", err)
	}

	resp := readUntilType(t, conn, "kill_terminal_response")
	payload := decodeSessionPayload[killTerminalResponsePayload](t, resp)
	if payload.RequestID != "req-kt-1" {
		t.Fatalf("request ID: got %q, want req-kt-1", payload.RequestID)
	}
	if payload.TerminalID != terminalID {
		t.Fatalf("terminal ID: got %q, want %q", payload.TerminalID, terminalID)
	}
	if !payload.Success {
		t.Fatal("expected success=true in kill_terminal_response")
	}
}

func TestKillTerminalNotFoundSendsRPCError(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-kill-terminal-missing")
	defer conn.Close()
	readInitialMessages(t, conn)

	killReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "kill_terminal_request",
			"requestId":  "req-kt-missing",
			"terminalId": "no-such-terminal",
		}),
	}
	if err := conn.WriteJSON(killReq); err != nil {
		t.Fatalf("write kill_terminal: %v", err)
	}

	resp := readUntilType(t, conn, "rpc_error")
	payload := decodeSessionPayload[protocol.RPCErrorPayload](t, resp)
	if payload.RequestID != "req-kt-missing" {
		t.Fatalf("request ID: got %q, want req-kt-missing", payload.RequestID)
	}
	if payload.Error == "" {
		t.Fatal("expected non-empty rpc_error message")
	}
}
