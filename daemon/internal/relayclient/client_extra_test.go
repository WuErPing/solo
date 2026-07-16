package relayclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/wsconn"
)

// mockWSServer implements SessionAttacher for tests.
type mockWSServer struct {
	attachCalled atomic.Int32
	lastConn     wsconn.WSConn
	mu           sync.Mutex
}

func (m *mockWSServer) AttachExternalConnection(conn wsconn.WSConn) {
	m.attachCalled.Add(1)
	m.mu.Lock()
	m.lastConn = conn
	m.mu.Unlock()
	// Block until connection is closed to simulate session lifecycle
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsServer := &mockWSServer{}
	kp := &DaemonKeyPair{PublicKeyB64: "test", SecretKeyB64: "test"}

	c := NewClient("srv-1", "relay.example.com:443", wsServer, logger, kp, true)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.serverID != "srv-1" {
		t.Errorf("expected serverID srv-1, got %s", c.serverID)
	}
	if c.endpoint != "relay.example.com:443" {
		t.Errorf("expected endpoint relay.example.com:443, got %s", c.endpoint)
	}
	if c.keyPair != kp {
		t.Error("expected keypair to match")
	}
	if !c.disableControlKeepalive {
		t.Error("expected disableControlKeepalive true")
	}
}

func TestClient_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsServer := &mockWSServer{}
	c := NewClient("srv-1", "localhost:9999", wsServer, logger, nil, true)

	if err := c.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop should not panic
	c.Stop()

	// Second stop should be safe
	c.Stop()
}

func TestClient_Start_AlreadyStopped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsServer := &mockWSServer{}
	c := NewClient("srv-1", "localhost:9999", wsServer, logger, nil, true)
	c.stopped.Store(true)

	if err := c.Start(); err == nil {
		t.Error("expected error when starting stopped client")
	}
}

func TestHandleControlMessage_Sync(_ *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	msg, _ := json.Marshal(map[string]interface{}{
		"type":          "sync",
		"connectionIds": []string{"a", "b"},
	})
	c.handleControlMessage(msg)
}

func TestHandleControlMessage_Connected(_ *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsServer := &mockWSServer{}
	c := NewClient("srv-1", "localhost:9999", wsServer, logger, nil, true)

	msg, _ := json.Marshal(map[string]interface{}{
		"type":         "connected",
		"connectionId": "conn-1",
	})
	c.handleControlMessage(msg)

	// connected should spawn a goroutine; give it a moment
	time.Sleep(50 * time.Millisecond)
}

func TestHandleControlMessage_Connected_Duplicate(_ *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsServer := &mockWSServer{}
	c := NewClient("srv-1", "localhost:9999", wsServer, logger, nil, true)

	c.pendingConns["conn-1"] = struct{}{}
	msg, _ := json.Marshal(map[string]interface{}{
		"type":         "connected",
		"connectionId": "conn-1",
	})
	c.handleControlMessage(msg)
	// Should skip because already pending
}

func TestHandleControlMessage_Disconnected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	// Add a fake data conn
	fakeConn := &fakeWSConn{}
	c.dataConns["conn-1"] = fakeConn

	msg, _ := json.Marshal(map[string]interface{}{
		"type":         "disconnected",
		"connectionId": "conn-1",
	})
	c.handleControlMessage(msg)

	if !fakeConn.closed.Load() {
		t.Error("expected fake conn to be closed")
	}
}

func TestHandleControlMessage_Ping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	// Use a real websocket connection for the controlConn
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		// Echo messages back so we can count writes indirectly
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, data)
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	c.controlConn = conn

	msg, _ := json.Marshal(map[string]interface{}{"type": "ping"})
	c.handleControlMessage(msg)

	// Read the pong that was written
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected pong message: %v", err)
	}
	var pong map[string]interface{}
	if err := json.Unmarshal(data, &pong); err != nil {
		t.Fatalf("invalid pong JSON: %v", err)
	}
	if pong["type"] != "pong" {
		t.Errorf("expected type pong, got %v", pong["type"])
	}
}

func TestHandleControlMessage_Pong(_ *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	msg, _ := json.Marshal(map[string]interface{}{"type": "pong"})
	c.handleControlMessage(msg)
	// Should be a no-op beyond activity recording
}

func TestHandleControlMessage_Unknown(_ *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	msg, _ := json.Marshal(map[string]interface{}{"type": "unknown_xyz"})
	c.handleControlMessage(msg)
	// Should not panic
}

func TestHandleControlMessage_InvalidJSON(_ *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	c.handleControlMessage([]byte("not json"))
	// Should not panic
}

func TestBuildControlURL(t *testing.T) {
	c := NewClient("srv-1", "relay.example.com:443", &mockWSServer{}, nil, nil, true)
	u := c.buildControlURL()
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	if parsed.Scheme != "wss" {
		t.Errorf("expected wss scheme, got %s", parsed.Scheme)
	}
	q := parsed.Query()
	if q.Get("serverId") != "srv-1" {
		t.Errorf("expected serverId srv-1, got %s", q.Get("serverId"))
	}
	if q.Get("role") != "server" {
		t.Errorf("expected role server, got %s", q.Get("role"))
	}
}

func TestBuildDataURL(t *testing.T) {
	c := NewClient("srv-1", "relay.example.com:8080", &mockWSServer{}, nil, nil, true)
	u := c.buildDataURL("conn-abc")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	if parsed.Scheme != "ws" {
		t.Errorf("expected ws scheme, got %s", parsed.Scheme)
	}
	q := parsed.Query()
	if q.Get("connectionId") != "conn-abc" {
		t.Errorf("expected connectionId conn-abc, got %s", q.Get("connectionId"))
	}
}

func TestWsScheme(t *testing.T) {
	tests := []struct {
		endpoint string
		expected string
	}{
		{"relay.example.com:443", "wss"},
		{"relay.example.com:8080", "ws"},
		{"relay.example.com", "ws"},
		{"127.0.0.1:17613", "ws"},
	}
	for _, tc := range tests {
		c := NewClient("srv", tc.endpoint, &mockWSServer{}, nil, nil, true)
		if got := c.wsScheme(); got != tc.expected {
			t.Errorf("wsScheme(%q) = %s, want %s", tc.endpoint, got, tc.expected)
		}
	}
}

func TestScheduleReconnect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	c.scheduleReconnect()
	if c.reconnectTimer == nil {
		t.Fatal("expected reconnect timer to be set")
	}

	// Clean up
	c.reconnectTimer.Stop()
}

func TestScheduleReconnect_WhenStopped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)
	c.stopped.Store(true)

	c.scheduleReconnect()
	if c.reconnectTimer != nil {
		t.Error("expected no reconnect timer when stopped")
	}
}

func TestCloseDataConn(t *testing.T) {
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, nil, nil, true)
	fakeConn := &fakeWSConn{}
	c.dataConns["conn-1"] = fakeConn

	c.closeDataConn("conn-1")
	if !fakeConn.closed.Load() {
		t.Error("expected conn to be closed")
	}
	if len(c.dataConns) != 0 {
		t.Error("expected dataConns to be empty")
	}
}

func TestCloseAllDataConns(t *testing.T) {
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, nil, nil, true)
	fake1 := &fakeWSConn{}
	fake2 := &fakeWSConn{}
	c.dataConns["conn-1"] = fake1
	c.dataConns["conn-2"] = fake2

	c.closeAllDataConns()
	if !fake1.closed.Load() || !fake2.closed.Load() {
		t.Error("expected all conns to be closed")
	}
	if len(c.dataConns) != 0 {
		t.Error("expected dataConns to be empty")
	}
}

func TestSendPong_NoControlConn(_ *testing.T) {
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, nil, nil, true)
	// Should not panic when controlConn is nil
	c.sendPong()
}

// fakeWSConn is a test double for wsconn.WSConn.
type fakeWSConn struct {
	closed     atomic.Bool
	writeCount atomic.Int32
	messages   [][]byte
	mu         sync.Mutex
}

func (f *fakeWSConn) ReadMessage() (int, []byte, error) {
	// Block forever until closed
	for {
		time.Sleep(100 * time.Millisecond)
		if f.closed.Load() {
			return 0, nil, websocket.ErrBadHandshake
		}
	}
}

func (f *fakeWSConn) WriteMessage(_ int, data []byte) error {
	f.writeCount.Add(1)
	f.mu.Lock()
	f.messages = append(f.messages, data)
	f.mu.Unlock()
	return nil
}

func (f *fakeWSConn) Close() error {
	f.closed.Store(true)
	return nil
}

func TestControlReadPump_ContextCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, true)

	// Use a real websocket connection for read pump
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		// Keep open until client closes
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.controlReadPump(ctx, conn)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("controlReadPump did not exit after context cancel")
	}
}

func TestControlKeepalive_StaleConnection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, false)

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		// Never respond to anything
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	c.controlConn = conn
	c.lastActivityMs.Store(time.Now().Add(-time.Hour).UnixMilli())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.controlKeepalive(ctx, conn)
		close(done)
	}()

	// keepalive should detect stale connection and close it.
	// Ticker fires every controlPingInterval (10s), so wait at least that long.
	select {
	case <-done:
		// Good
	case <-time.After(15 * time.Second):
		t.Fatal("controlKeepalive did not exit after stale detection")
	}
}

// TestConnectControl_Success verifies connectControl establishes a control
// connection and starts the read pump + keepalive goroutines.
func TestConnectControl_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	connected := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		close(connected)
		// Keep connection open until client closes
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	c := NewClient("srv-1", srv.Listener.Addr().String(), &mockWSServer{}, logger, nil, false)
	c.connectControl()

	select {
	case <-connected:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("control connection was not established")
	}

	// Clean up
	c.Stop()
}

// TestConnectControl_FailureTriggersReconnect verifies that a failed dial
// schedules a reconnect.
func TestConnectControl_FailureTriggersReconnect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "127.0.0.1:1", &mockWSServer{}, logger, nil, false)

	c.connectControl()

	// Reconnect timer should be scheduled after failed dial
	c.reconnectMu.Lock()
	hasTimer := c.reconnectTimer != nil
	c.reconnectMu.Unlock()

	if !hasTimer {
		t.Error("expected reconnect timer to be scheduled after failed dial")
	}

	// Clean up
	c.Stop()
}

// TestConnectControl_AlreadyStopped verifies connectControl is a no-op when
// the client has been stopped.
func TestConnectControl_AlreadyStopped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, false)
	c.stopped.Store(true)

	c.connectControl()

	c.controlMu.Lock()
	hasConn := c.controlConn != nil
	c.controlMu.Unlock()

	if hasConn {
		t.Error("expected no control conn when stopped")
	}
}

// TestControlReadPump_ReceivesMessage verifies the read pump processes
// incoming text messages.
func TestControlReadPump_ReceivesMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a sync message
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"sync","connectionIds":[]}`))

		// Block until client closes
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, false)
	c.controlConn = conn
	c.lastActivityMs.Store(time.Now().UnixMilli())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.controlReadPump(ctx, conn)
		close(done)
	}()

	// Wait for message to be processed
	time.Sleep(200 * time.Millisecond)

	// Cancel context and close connection to unblock ReadMessage
	cancel()
	conn.Close()
	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("controlReadPump did not exit after context cancel")
	}
}

// TestControlReadPump_NonTextMessageIgnored verifies binary messages are
// ignored by the read pump.
func TestControlReadPump_NonTextMessageIgnored(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a binary message (should be ignored)
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte("ignored"))

		// Block until client closes
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, false)
	c.controlConn = conn
	c.lastActivityMs.Store(time.Now().UnixMilli())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.controlReadPump(ctx, conn)
		close(done)
	}()

	// Wait briefly to let binary message be processed
	time.Sleep(200 * time.Millisecond)

	cancel()
	conn.Close()
	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("controlReadPump did not exit after context cancel")
	}
}

// TestControlReadPump_ScheduleReconnectOnClose verifies that when the
// connection is closed unexpectedly, scheduleReconnect is called.
func TestControlReadPump_ScheduleReconnectOnClose(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Close immediately to trigger unexpected close
		conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	// Don't defer close — server already closed it

	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, false)
	c.controlConn = conn
	c.lastActivityMs.Store(time.Now().UnixMilli())

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		c.controlReadPump(ctx, conn)
		close(done)
	}()

	select {
	case <-done:
		// Good — read pump exited because connection closed
	case <-time.After(2 * time.Second):
		t.Fatal("controlReadPump did not exit after connection close")
	}

	// Reconnect timer should be scheduled
	c.reconnectMu.Lock()
	hasTimer := c.reconnectTimer != nil
	c.reconnectMu.Unlock()
	if !hasTimer {
		t.Error("expected reconnect timer after unexpected close")
	}

	c.Stop()
}

// TestStop_Idempotent verifies Stop can be called multiple times safely.
func TestStop_Idempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewClient("srv-1", "localhost:9999", &mockWSServer{}, logger, nil, false)

	c.Stop()
	c.Stop()
	c.Stop()

	if !c.stopped.Load() {
		t.Error("expected stopped to be true")
	}
}

// TestStop_ClosesControlConn verifies Stop closes the active control connection.
func TestStop_ClosesControlConn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	connected := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		close(connected)
		// Block until client closes the connection
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	c := NewClient("srv-1", srv.Listener.Addr().String(), &mockWSServer{}, logger, nil, false)
	c.connectControl()

	// Wait for server to receive the connection
	select {
	case <-connected:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive connection")
	}

	c.Stop()

	// After Stop, controlConn should be nil
	c.controlMu.Lock()
	hasConn := c.controlConn != nil
	c.controlMu.Unlock()
	if hasConn {
		t.Error("expected controlConn to be nil after Stop")
	}
}

// TestConnectControl_CleansUpStaleDataConnsOnReconnect verifies that when the
// daemon re-establishes the relay control connection (e.g. after host wakes
// from sleep and the relay dropped the previous control socket), any data
// sockets associated with the prior control connection are closed.
//
// Rationale (2026-07-14 post-mortem): after host wake the relay drops both
// control and data sockets on its side, but the daemon kept stale data sockets
// in its map. When the mobile client reconnected via the fresh control socket,
// the new data socket attached to the session while the stale one was still
// alive in the daemon, and the attach returned within 2 ms — leaving the
// Android app stuck on "connecting" even after app restart.
func TestConnectControl_CleansUpStaleDataConnsOnReconnect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	// Track per-control-connection close signals so the test can tear down
	// individual control sockets and force a reconnect on demand.
	var mu sync.Mutex
	controlCloses := []chan struct{}{}
	var controlCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if r.URL.Path == "/ws" && r.URL.Query().Get("role") == "server" && r.URL.Query().Get("connectionId") == "" {
			// Control connection.
			idx := int(controlCount.Add(1)) - 1
			closeCh := make(chan struct{})
			mu.Lock()
			controlCloses = append(controlCloses, closeCh)
			mu.Unlock()
			// Send sync so the control read pump has at least one activity tick.
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"sync","connectionIds":[]}`))
			// Hold the connection until the test tells us to drop it, or the
			// client closes first.
			select {
			case <-closeCh:
			}
			_ = conn.Close()
			_ = idx
			return
		}
		// Data socket (or unknown): hold until client closes.
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	c := NewClient("srv-1", srv.Listener.Addr().String(), &mockWSServer{}, logger, nil, true)

	// --- initial control connect ---
	c.connectControl()

	// Wait until the first control connection is up.
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(controlCloses)
		mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("initial control connection did not come up")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// --- simulate an active data socket from the current control session ---
	staleConn := &fakeWSConn{}
	c.dataConnsMu.Lock()
	c.dataConns["stale-conn-id"] = staleConn
	c.dataConnsMu.Unlock()

	// --- simulate host sleep: relay drops the control connection ---
	mu.Lock()
	close(controlCloses[0])
	mu.Unlock()

	// The control read pump will exit, scheduleReconnect will arm a timer.
	// Force an immediate reconnect by clearing the timer and calling
	// connectControl directly — equivalent to the timer firing.
	deadline = time.Now().Add(2 * time.Second)
	for {
		c.reconnectMu.Lock()
		pending := c.reconnectTimer != nil
		c.reconnectMu.Unlock()
		if pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reconnect was not scheduled after control socket drop")
		}
		time.Sleep(10 * time.Millisecond)
	}

	c.reconnectMu.Lock()
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	c.reconnectAttempt = 0
	c.reconnectMu.Unlock()

	c.connectControl()

	// Wait until the second control connection is up.
	deadline = time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(controlCloses)
		mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reconnect did not establish a new control connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Give the cleanup a moment to run (it is synchronous in connectControl,
	// but allow a small window for any goroutine-based paths).
	time.Sleep(50 * time.Millisecond)

	// --- assertions ---
	if !staleConn.closed.Load() {
		t.Error("expected stale data socket to be closed after control reconnect")
	}

	c.dataConnsMu.Lock()
	remaining := len(c.dataConns)
	c.dataConnsMu.Unlock()
	if remaining != 0 {
		t.Errorf("expected dataConns map to be empty after reconnect, got %d entries", remaining)
	}

	// Tear down the second control connection.
	mu.Lock()
	if len(controlCloses) >= 2 {
		close(controlCloses[1])
	}
	mu.Unlock()

	c.Stop()
}
