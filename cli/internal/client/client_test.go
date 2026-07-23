package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		subscribers: make(map[string][]*Subscription),
		done:        make(chan struct{}),
	}

	sub := c.Subscribe("test_type")
	if sub == nil || sub.Messages() == nil {
		t.Fatal("expected non-nil subscription channel")
	}

	c.Unsubscribe(sub)

	c.subMu.RLock()
	if len(c.subscribers["test_type"]) != 0 {
		t.Error("expected subscriber to be removed")
	}
	c.subMu.RUnlock()
}

func TestSubscribeAfterDisconnect_ReturnsClosedChannel(t *testing.T) {
	done := make(chan struct{})
	close(done)
	c := &DaemonClient{
		subscribers: make(map[string][]*Subscription),
		done:        done,
	}

	sub := c.Subscribe("test_type")
	select {
	case _, ok := <-sub.Messages():
		if ok {
			t.Fatal("expected closed channel after disconnect")
		}
	default:
		t.Fatal("expected receive on closed channel to proceed immediately")
	}
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

	connMu      sync.Mutex
	connections []*websocket.Conn
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
	m.connMu.Lock()
	m.connections = append(m.connections, conn)
	m.connMu.Unlock()
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

// closeConnections closes all server-side WebSocket connections, simulating
// the daemon dying. (httptest's CloseClientConnections cannot reach hijacked
// WebSocket connections, so we close them explicitly.)
func (m *mockDaemonServer) closeConnections() {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	for _, conn := range m.connections {
		_ = conn.Close()
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

// triggerFloodServer completes the daemon handshake, then waits for any
// session message and floods `count` session messages of msgType (no
// requestId, so they only reach subscribers). With count 0 it never responds,
// which exercises request timeouts.
type triggerFloodServer struct {
	upgrader websocket.Upgrader
	msgType  string
	count    int
}

func (f *triggerFloodServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := f.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var hello protocol.WSInboundMessage
	if err := conn.ReadJSON(&hello); err != nil {
		return
	}

	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type":     "server_info",
			"status":   "server_info",
			"serverId": "flood-server",
		},
	})
	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type:    "session",
		Message: map[string]interface{}{"type": "providers_snapshot_update", "payload": map[string]interface{}{}},
	})

	// Wait for the flood trigger (any session message from the client).
	for {
		var inbound protocol.WSInboundMessage
		if err := conn.ReadJSON(&inbound); err != nil {
			return
		}
		if inbound.Type == "session" {
			break
		}
	}

	for i := 0; i < f.count; i++ {
		err := conn.WriteJSON(protocol.WSOutboundMessage{
			Type:    "session",
			Message: map[string]interface{}{"type": f.msgType, "payload": map[string]interface{}{}},
		})
		if err != nil {
			return
		}
	}

	// Keep the connection alive until the client goes away.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func TestDisconnect_ClosesSubscriptionChannels(t *testing.T) {
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

	sub := c.Subscribe("agent_update")

	// Close the server-side connection to simulate the daemon dying.
	mock.closeConnections()

	select {
	case _, ok := <-sub.Messages():
		if ok {
			t.Fatal("expected subscription channel to be closed on disconnect")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscription channel was not closed after disconnect")
	}
}

func TestSubscription_DropCounting(t *testing.T) {
	const floodCount = 100
	flood := &triggerFloodServer{
		upgrader: websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }},
		msgType:  "flood_event",
		count:    floodCount,
	}
	srv := httptest.NewServer(flood)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewDaemonClient(ctx, srv.Listener.Addr().String(), "test-client")
	if err != nil {
		t.Fatalf("NewDaemonClient failed: %v", err)
	}
	defer c.Close()

	sub := c.Subscribe("flood_event")

	// Trigger the flood; the server never responds, so use a short deadline.
	triggerCtx, triggerCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer triggerCancel()
	_, _ = c.Request(triggerCtx, &protocol.PingMessage{Type: "ping"})

	// The consumer never reads: the 16-slot buffer fills, the rest is dropped.
	wantDropped := floodCount - 16
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && sub.DroppedCount() < int64(wantDropped) {
		time.Sleep(10 * time.Millisecond)
	}
	if sub.DroppedCount() != int64(wantDropped) {
		t.Fatalf("DroppedCount() = %d, want %d", sub.DroppedCount(), wantDropped)
	}
	if buffered := len(sub.Messages()); buffered != 16 {
		t.Fatalf("buffered = %d, want 16", buffered)
	}
}

func TestRequest_DefaultTimeoutApplies(t *testing.T) {
	silent := &triggerFloodServer{
		upgrader: websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }},
		count:    0, // never responds
	}
	srv := httptest.NewServer(silent)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewDaemonClient(ctx, srv.Listener.Addr().String(), "test-client", WithRequestTimeout(200*time.Millisecond))
	if err != nil {
		t.Fatalf("NewDaemonClient failed: %v", err)
	}
	defer c.Close()

	// No deadline on the request context: the client's default must apply.
	start := time.Now()
	_, err = c.Request(context.Background(), &protocol.PingMessage{Type: "ping"})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error for unresponsive server")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Request took %v, expected ~200ms default timeout", elapsed)
	}
}
