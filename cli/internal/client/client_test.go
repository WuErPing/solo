package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

func TestGenerateRequestID_Increment(t *testing.T) {
	c := &DaemonClient{}
	id1 := c.GenerateRequestID()
	id2 := c.GenerateRequestID()
	if id1 == id2 {
		t.Error("expected unique request IDs")
	}
	if !strings.HasPrefix(id1, "cli-") {
		t.Errorf("expected prefix cli-, got %q", id1)
	}
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	c := &DaemonClient{
		subscribers: make(map[string][]chan *protocol.WSOutboundMessage),
	}

	ch := c.Subscribe("test_type")
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	c.Unsubscribe("test_type", ch)

	c.subMu.RLock()
	if len(c.subscribers["test_type"]) != 0 {
		t.Error("expected subscriber to be removed")
	}
	c.subMu.RUnlock()
}

func TestProvidersSnapshot(t *testing.T) {
	snapshot := &protocol.ProvidersSnapshotPayload{}
	c := &DaemonClient{providersSnapshot: snapshot}
	if c.ProvidersSnapshot() != snapshot {
		t.Error("expected same snapshot pointer")
	}
}

func TestServerInfo(t *testing.T) {
	info := &protocol.ServerInfoPayload{ServerID: "test"}
	c := &DaemonClient{serverInfo: info}
	if c.ServerInfo() != info {
		t.Error("expected same server info pointer")
	}
}

func TestSetRequestID(t *testing.T) {
	msg := &protocol.PingMessage{Type: "ping"}
	setRequestID(msg, "req-123")
	if msg.RequestID != "req-123" {
		t.Errorf("expected RequestID req-123, got %q", msg.RequestID)
	}
}

func TestMustMarshal(t *testing.T) {
	data := mustMarshal(map[string]string{"key": "value"})
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("mustMarshal produced invalid JSON: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected value, got %q", result["key"])
	}
}

// mockDaemonServer is a test WebSocket server that mimics the Solo daemon handshake.
type mockDaemonServer struct {
	serverID          string
	providersSnapshot *protocol.ProvidersSnapshotPayload
	upgrader          websocket.Upgrader
	connections       []*websocket.Conn
}

func newMockDaemonServer() *mockDaemonServer {
	return &mockDaemonServer{
		serverID: "test-server",
		providersSnapshot: &protocol.ProvidersSnapshotPayload{
			Entries: []protocol.ProviderSnapshotEntry{
				{Provider: "test-provider", Status: protocol.ProviderReady},
			},
			GeneratedAt: time.Now().Format(time.RFC3339),
		},
		upgrader: websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }},
	}
}

func (m *mockDaemonServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	m.connections = append(m.connections, conn)
	defer conn.Close()

	// Read hello
	var hello protocol.WSInboundMessage
	if err := conn.ReadJSON(&hello); err != nil {
		return
	}
	if hello.Type != "hello" {
		return
	}

	// Send server_info
	serverInfo := protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type":     "server_info",
			"status":   "server_info",
			"serverId": m.serverID,
		},
	}
	_ = conn.WriteJSON(serverInfo)

	// Send providers_snapshot_update
	update := protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type":    "providers_snapshot_update",
			"payload": m.providersSnapshot,
		},
	}
	_ = conn.WriteJSON(update)

	// Echo loop: respond to session messages
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var inbound protocol.WSInboundMessage
		if err := json.Unmarshal(data, &inbound); err != nil {
			continue
		}
		if inbound.Type != "session" {
			continue
		}
		// Echo back as outbound with requestId peeked
		var peek struct {
			Type      string `json:"type"`
			RequestID string `json:"requestId"`
		}
		_ = json.Unmarshal(inbound.Message, &peek)

		resp := protocol.WSOutboundMessage{
			Type: "session",
			Message: map[string]interface{}{
				"type": "fetch_agents_response",
				"payload": map[string]interface{}{
					"requestId": peek.RequestID,
					"entries":   []interface{}{},
					"pageInfo": map[string]interface{}{
						"hasMore": false,
					},
				},
			},
		}
		_ = conn.WriteJSON(resp)
	}
}

func TestNewDaemonClient_Handshake(t *testing.T) {
	mock := newMockDaemonServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewDaemonClient(ctx, srv.Listener.Addr().String(), "test-client")
	if err != nil {
		t.Fatalf("NewDaemonClient failed: %v", err)
	}
	defer c.Close()

	if c.ServerInfo() == nil || c.ServerInfo().ServerID != "test-server" {
		t.Error("expected server info to be populated")
	}
	if c.ProvidersSnapshot() == nil || len(c.ProvidersSnapshot().Entries) != 1 {
		t.Error("expected providers snapshot to be populated")
	}
}

func TestNewDaemonClient_InvalidHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewDaemonClient(ctx, "::invalid::", "test-client")
	if err == nil {
		t.Error("expected error for invalid host")
	}
}

func TestRequest_Response(t *testing.T) {
	mock := newMockDaemonServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewDaemonClient(ctx, srv.Listener.Addr().String(), "test-client")
	if err != nil {
		t.Fatalf("NewDaemonClient failed: %v", err)
	}
	defer c.Close()

	req := &protocol.PingMessage{Type: "ping"}
	resp, err := c.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestRequest_ContextCancellation(t *testing.T) {
	mock := newMockDaemonServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewDaemonClient(ctx, srv.Listener.Addr().String(), "test-client")
	if err != nil {
		t.Fatalf("NewDaemonClient failed: %v", err)
	}
	defer c.Close()

	// Cancel context before request
	cancel()
	req := &protocol.PingMessage{Type: "ping"}
	_, err = c.Request(ctx, req)
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}

func TestClose_Idempotent(t *testing.T) {
	mock := newMockDaemonServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewDaemonClient(ctx, srv.Listener.Addr().String(), "test-client")
	if err != nil {
		t.Fatalf("NewDaemonClient failed: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}
