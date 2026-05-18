package server

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

type helloThenBlockConn struct {
	hello        []byte
	readOnce     sync.Once
	readErr      chan error
	writeStarted chan struct{}
	writeOnce    sync.Once
	writes       [][]byte
	mu           sync.Mutex
	closed       atomic.Bool
}

func newHelloThenBlockConn(t *testing.T, clientID string) *helloThenBlockConn {
	t.Helper()
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        clientID,
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	data, err := json.Marshal(hello)
	if err != nil {
		t.Fatalf("marshal hello: %v", err)
	}
	return &helloThenBlockConn{
		hello:        data,
		readErr:      make(chan error, 1),
		writeStarted: make(chan struct{}),
	}
}

func (c *helloThenBlockConn) ReadMessage() (int, []byte, error) {
	var data []byte
	var first bool
	c.readOnce.Do(func() {
		data = c.hello
		first = true
	})
	if first {
		return websocket.TextMessage, data, nil
	}
	err := <-c.readErr
	return websocket.TextMessage, nil, err
}

func (c *helloThenBlockConn) WriteMessage(messageType int, data []byte) error {
	c.writeOnce.Do(func() { close(c.writeStarted) })
	c.mu.Lock()
	c.writes = append(c.writes, data)
	c.mu.Unlock()
	return nil
}

func (c *helloThenBlockConn) Close() error {
	c.closed.Store(true)
	return nil
}

func (c *helloThenBlockConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return nil
}

func (c *helloThenBlockConn) SetPongHandler(h func(appData string) error) {}

func (c *helloThenBlockConn) SetReadDeadline(t time.Time) error { return nil }

func (c *helloThenBlockConn) injectReadError(err error) {
	select {
	case c.readErr <- err:
	default:
	}
}

// TestWSServer_ConcurrentReconnect_AttachingSessionReplaced verifies that
// when a session is stuck in the "attaching" state (e.g. read-loop blocked
// on a stale relay data socket), a new connection from the same clientId
// replaces the stuck session instead of being dropped.
//
// Scenario:
//  1. conn A starts AttachSocket (grace cancelled, attachingCount > 0)
//  2. conn B arrives before conn A finishes registering
//  3. handleNewConnection detects IsAttaching() == true → calls
//     shutdownForReplacement on the old session → creates a fresh session
//     for conn B
//
// We verify that:
//   - The old session IS killed (done channel closed)
//   - conn B receives server_info (new session created successfully)
//   - conn A's AttachSocket goroutine exits cleanly (attachingConn closed)
func TestWSServer_ConcurrentReconnect_AttachingSessionReplaced(t *testing.T) {
	ws, _ := newTestWSServer(t)
	clientID := "race-client"

	// Create a session that is already in grace.
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 2*time.Second)
	sess.startBackgroundLoops()
	sess.enterGrace()
	if !sess.IsInGrace() {
		t.Fatal("setup: expected session to be in grace")
	}

	ws.mu.Lock()
	ws.sessions[clientID] = sess
	ws.mu.Unlock()

	// Conn A starts AttachSocket (simulates a slow/stale relay data socket).
	slowConn := newMockConn()
	attachStarted := make(chan struct{})
	attachDone := make(chan struct{})
	go func() {
		defer close(attachDone)
		close(attachStarted)
		sess.AttachSocket(slowConn)
	}()

	// Wait for AttachSocket to start.
	select {
	case <-attachStarted:
	case <-time.After(time.Second):
		t.Fatal("AttachSocket goroutine did not start")
	}
	time.Sleep(20 * time.Millisecond)

	if !sess.IsAttaching() {
		t.Fatal("setup: session should be attaching")
	}

	// Conn B arrives — simulates iOS app reconnecting while old session is stuck.
	connB := newHelloThenBlockConn(t, clientID)
	handleDone := make(chan struct{})
	go func() {
		defer close(handleDone)
		ws.handleNewConnection(connB)
	}()

	select {
	case <-connB.writeStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("connB did not receive server_info")
	}

	// Give handleNewConnection time to process.
	time.Sleep(50 * time.Millisecond)

	// The original session MUST have been killed (replaced by new session).
	select {
	case <-sess.done:
		// Correct: old session was replaced.
	default:
		t.Error("old stuck session was not replaced — new connection should replace it")
	}

	// Verify a new session was created for this clientId.
	ws.mu.RLock()
	current := ws.sessions[clientID]
	ws.mu.RUnlock()
	if current == nil {
		t.Fatal("no session in map after replacement")
	}
	if current == sess {
		t.Fatal("old session still in map, should have been replaced")
	}
	if current.IsAttaching() {
		t.Error("new session should not be in attaching state")
	}

	// Clean up.
	slowConn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	connB.injectReadError(errors.New("test disconnect"))
	select {
	case <-attachDone:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachSocket goroutine did not return")
	}
	select {
	case <-handleDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleNewConnection goroutine did not return")
	}
}

func TestWSServer_ReconnectWithExistingNonGraceSessionCleansUpOldSession(t *testing.T) {
	ws, _ := newTestWSServer(t)
	clientID := "same-client-reconnect"

	oldConn := newMockConn()
	oldSess := newTestSessionGrace(t, oldConn, 5*time.Second)
	var unsubCalled atomic.Bool
	oldSess.unsub = func() {
		unsubCalled.Store(true)
	}
	attachedConn := newMockConn()
	oldSess.sockets = map[string]socketEntry{
		"stale-socket": {
			id:        "stale-socket",
			conn:      attachedConn,
			sendQueue: newSendQueue(),
			writeDone: make(chan struct{}),
			done:      make(chan struct{}),
		},
	}

	ws.mu.Lock()
	ws.sessions[clientID] = oldSess
	ws.mu.Unlock()

	newConn := newHelloThenBlockConn(t, clientID)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ws.handleNewConnection(newConn)
	}()

	select {
	case <-newConn.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("new connection did not receive server_info")
	}

	deadline := time.After(time.Second)
	for {
		ws.mu.RLock()
		current := ws.sessions[clientID]
		ws.mu.RUnlock()
		if current != nil && current != oldSess {
			break
		}
		select {
		case <-deadline:
			t.Fatal("new connection did not replace old session in sessions map")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if !unsubCalled.Load() {
		t.Fatal("old non-grace session was replaced without unsubscribing from agent events")
	}
	oldConn.mu.Lock()
	oldClosed := oldConn.closed
	oldConn.mu.Unlock()
	if !oldClosed {
		t.Fatal("old non-grace session connection was not closed during replacement")
	}
	attachedConn.mu.Lock()
	attachedClosed := attachedConn.closed
	attachedConn.mu.Unlock()
	if !attachedClosed {
		t.Fatal("old non-grace session attached socket was not closed during replacement")
	}

	newConn.injectReadError(errors.New("test disconnect"))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("new connection handler did not return after disconnect")
	}
}
