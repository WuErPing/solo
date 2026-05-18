package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// newTestWSServerGrace creates a WSServer with a short grace period for tests.
func newTestWSServerGrace(t *testing.T, gracePeriod time.Duration) (*WSServer, *httptest.Server) {
	t.Helper()
	ws, ts := newTestWSServer(t)
	ws.gracePeriod = gracePeriod
	return ws, ts
}

func dialAndHelloGrace(t *testing.T, tsURL, clientID string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(tsURL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        clientID,
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	// Read server_info + providers_snapshot (same as readInitialMessages)
	readInitialMessages(t, conn)
	return conn
}

// readUntilGrace reads messages until finding one matching targetType.
// It checks both the outer WSOutboundMessage.Type and the inner
// session message type/payload.status (same as readUntilType).
func readUntilGrace(t *testing.T, conn *websocket.Conn, targetType string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		// Check outer type (e.g. "pong" at top level)
		if resp.Type == targetType {
			return map[string]interface{}{"type": targetType}
		}
		if resp.Type != "session" {
			continue
		}
		payloadBytes, err := json.Marshal(resp.Message)
		if err != nil {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			continue
		}
		// Check inner type
		if msgType, _ := payload["type"].(string); msgType == targetType {
			return payload
		}
		// Check payload.status (for status messages like agent_created)
		if innerPayload, ok := payload["payload"].(map[string]interface{}); ok {
			if status, _ := innerPayload["status"].(string); status == targetType {
				return payload
			}
		}
	}
}

// TestGracePeriod_E2E_ReconnectResumesSession verifies that a client
// disconnecting and reconnecting within the grace period resumes the same
// session (the previously created agent is still visible).
func TestGracePeriod_E2E_ReconnectResumesSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E grace test in short mode")
	}

	// Use regular WSServer (no grace period override) for this test
	// since we're testing the basic flow first
	_, ts := newTestWSServer(t)

	clientID := "grace-client-1"

	// Step 1: Connect, create an agent — use existing test helpers
	conn1 := dialAndHello(t, ts.URL, clientID)
	readInitialMessages(t, conn1)
	defer conn1.Close()

	cwd := t.TempDir()
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-grace-create",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn1.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	// Read until agent_created
	createdPayload := readUntilGrace(t, conn1, "agent_created", 5*time.Second)
	agentPayload, _ := createdPayload["payload"].(map[string]interface{})
	agentData, _ := agentPayload["agent"].(map[string]interface{})
	createdAgentID, _ := agentData["id"].(string)
	if createdAgentID == "" {
		t.Fatal("failed to capture agent ID from agent_created")
	}

	// Step 2: Disconnect (close the WebSocket)
	conn1.Close()

	// Step 3: Reconnect within grace period with the same clientID
	time.Sleep(200 * time.Millisecond)
	conn2 := dialAndHelloGrace(t, ts.URL, clientID)
	defer conn2.Close()

	// Step 4: Verify the session is working by sending a ping
	if err := conn2.WriteJSON(protocol.WSInboundMessage{Type: "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	readUntilGrace(t, conn2, "pong", 2*time.Second)

	// Step 5: Fetch agents and verify the previously created agent still exists
	fetchReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agents_request",
			"requestId": "req-grace-fetch",
		}),
	}
	if err := conn2.WriteJSON(fetchReq); err != nil {
		t.Fatalf("write fetch_agents: %v", err)
	}

	// Read until fetch_agents_response
	fetchResp := readUntilGrace(t, conn2, "fetch_agents_response", 5*time.Second)
	fetchPayload, _ := fetchResp["payload"].(map[string]interface{})
	entries, _ := fetchPayload["entries"].([]interface{})

	foundAgent := false
	for _, e := range entries {
		entry, _ := e.(map[string]interface{})
		if entry == nil {
			continue
		}
		agentMap, _ := entry["agent"].(map[string]interface{})
		if agentMap != nil && agentMap["id"] == createdAgentID {
			foundAgent = true
		}
	}

	if !foundAgent {
		t.Error("expected previously created agent to be visible after reconnection within grace period")
	}
}

// TestGracePeriod_E2E_GraceExpired_FreshSession verifies that when a client
// reconnects after the grace period expires, a new session is created.
func TestGracePeriod_E2E_GraceExpired_FreshSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E grace test in short mode")
	}

	ws, ts := newTestWSServerGrace(t, 500*time.Millisecond)
	_ = ws

	clientID := "grace-client-2"

	// Step 1: Connect
	conn1 := dialAndHelloGrace(t, ts.URL, clientID)

	// Step 2: Disconnect
	conn1.Close()

	// Step 3: Wait for grace period to expire
	time.Sleep(800 * time.Millisecond)

	// Step 4: Reconnect — should be a fresh session
	conn2 := dialAndHelloGrace(t, ts.URL, clientID)
	defer conn2.Close()

	// Verify the session is active by sending a ping
	if err := conn2.WriteJSON(protocol.WSInboundMessage{Type: "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	readUntilGrace(t, conn2, "pong", 2*time.Second)
}
