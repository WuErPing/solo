package relayclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/wsconn"
)

// blockingSessionAttacher blocks forever in AttachExternalConnection,
// simulating a session that never completes the hello handshake.
type blockingSessionAttacher struct {
	attached chan struct{}
}

func (b *blockingSessionAttacher) AttachExternalConnection(conn wsconn.WSConn) {
	close(b.attached)
	// Block until the connection is closed (simulates hello timeout hang)
	_, _, _ = conn.ReadMessage()
}

// newMockRelayServer creates a minimal httptest WebSocket server that accepts
// connections and silently blocks (never sends anything), simulating a relay
// that accepted the data socket connection but the daemon-side hello never fires.
func newMockRelayServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Block until the client closes the connection
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
}

// TestOpenDataSocket_TerminatesOnOpenTimeout verifies that openDataSocket
// exits within dataSocketOpenTimeout when AttachExternalConnection blocks
// indefinitely (simulating a hung hello handshake on the WSServer side).
func TestOpenDataSocket_TerminatesOnOpenTimeout(t *testing.T) {
	srv := newMockRelayServer(t)
	defer srv.Close()

	// Convert http://... to host:port for the relay client endpoint
	host := srv.Listener.Addr().String()

	attacher := &blockingSessionAttacher{attached: make(chan struct{})}
	client := NewClient("server-id", host, attacher, nil, nil, false)

	// Override timeout to something short for test speed
	origTimeout := dataSocketOpenTimeout
	dataSocketOpenTimeout = 200 * time.Millisecond
	defer func() { dataSocketOpenTimeout = origTimeout }()

	// Manually build the data URL using the test server (ws not wss)
	u := "ws://" + host + "/ws?serverId=server-id&role=server&v=2&connectionId=test-conn-1"
	_ = u

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Inject the connectionId directly so we don't need control socket
		client.openDataSocketURL("test-conn-1", u)
	}()

	// Attacher should be called promptly
	select {
	case <-attacher.attached:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachExternalConnection was never called")
	}

	// After dataSocketOpenTimeout, the goroutine should exit
	select {
	case <-done:
		// goroutine exited — correct
	case <-time.After(2 * time.Second):
		t.Fatal("openDataSocket did not terminate after open timeout")
	}
}

// TestOpenDataSocket_NoTimeoutOnNormalPath verifies that when
// AttachExternalConnection returns promptly, the timer is cancelled and
// there is no spurious close.
func TestOpenDataSocket_NoTimeoutOnNormalPath(t *testing.T) {
	srv := newMockRelayServer(t)
	defer srv.Close()

	host := srv.Listener.Addr().String()

	// Fast attacher that returns immediately
	fastAttacher := &fastSessionAttacher{}
	client := NewClient("server-id", host, fastAttacher, nil, nil, false)

	origTimeout := dataSocketOpenTimeout
	dataSocketOpenTimeout = 500 * time.Millisecond
	defer func() { dataSocketOpenTimeout = origTimeout }()

	u := "ws://" + host + "/ws?serverId=server-id&role=server&v=2&connectionId=test-conn-2"

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.openDataSocketURL("test-conn-2", u)
	}()

	select {
	case <-done:
		// exited promptly — good
	case <-time.After(2 * time.Second):
		t.Fatal("openDataSocket did not return on fast path")
	}
}

type fastSessionAttacher struct{}

func (f *fastSessionAttacher) AttachExternalConnection(conn wsconn.WSConn) {
	// Return immediately — simulates quick session end
}

// TestDataSocketOpenTimeout_IsAtLeast60s verifies that the default
// dataSocketOpenTimeout is long enough to survive a slow AttachSocket
// initialisation (e.g. when an agent is in a long thinking phase).
// Prior to the fix the value was 15 s, which was too short and caused
// relay reconnect loops on slow agents (see analysis:
// 2026-05-08_relay-data-socket-timeout-opencode-thinking-freeze.md).
func TestDataSocketOpenTimeout_IsAtLeast60s(t *testing.T) {
	if dataSocketOpenTimeout < 60*time.Second {
		t.Errorf("dataSocketOpenTimeout is %v, want >= 60s; a short timeout causes relay reconnect loops when AttachSocket is slow during agent thinking", dataSocketOpenTimeout)
	}
}

// helloProcessedAttacher blocks in AttachExternalConnection simulating a
// running session, but signals hello-processed so the openTimer should be
// cancelled and the connection must survive beyond the timeout.
type helloProcessedAttacher struct {
	attached       chan struct{}
	helloProcessed func()
	capturedConn   wsconn.WSConn

	// helloNotifier stores the callback registered via OnHelloProcessed.
	helloNotifier func()
}

func (h *helloProcessedAttacher) AttachExternalConnection(conn wsconn.WSConn) {
	h.capturedConn = conn
	close(h.attached)
	// Signal hello-processed through the notifier if registered.
	if h.helloNotifier != nil {
		h.helloNotifier()
	}
	// Also signal via the legacy channel for test assertions.
	if h.helloProcessed != nil {
		h.helloProcessed()
	}
	// Block until the connection is closed (simulates session running).
	_, _, _ = conn.ReadMessage()
}

func (h *helloProcessedAttacher) OnHelloProcessed(fn func()) {
	h.helloNotifier = fn
}

// TestOpenDataSocket_HelloProcessedCancelsTimer verifies that when the
// WSServer signals that the hello handshake completed successfully, the
// openTimer is cancelled and does NOT kill an active session.
//
// Before the fix, the openTimer fired unconditionally after
// dataSocketOpenTimeout and killed every relay data socket — including
// healthy sessions. This caused the iOS "second message no response" bug
// when the relay session lasted longer than the timeout (previously 15s,
// later raised to 60s).
func TestOpenDataSocket_HelloProcessedCancelsTimer(t *testing.T) {
	srv := newMockRelayServer(t)
	defer srv.Close()

	host := srv.Listener.Addr().String()

	helloProcessedCh := make(chan struct{})
	attacher := &helloProcessedAttacher{
		attached: make(chan struct{}),
		helloProcessed: func() {
			close(helloProcessedCh)
		},
	}
	client := NewClient("server-id", host, attacher, nil, nil, false)

	origTimeout := dataSocketOpenTimeout
	dataSocketOpenTimeout = 200 * time.Millisecond
	defer func() { dataSocketOpenTimeout = origTimeout }()

	u := "ws://" + host + "/ws?serverId=server-id&role=server&v=2&connectionId=test-conn-3"

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.openDataSocketURL("test-conn-3", u)
	}()

	// Wait for hello-processed signal
	select {
	case <-helloProcessedCh:
		// Hello processed — the openTimer should now be cancelled.
	case <-time.After(2 * time.Second):
		t.Fatal("hello processed callback was never invoked")
	}

	// Wait significantly longer than the timeout to verify the timer did NOT fire.
	// The connection should still be alive because the timer was cancelled.
	select {
	case <-done:
		t.Fatal("openDataSocketURL returned before connection was closed — openTimer fired and killed a healthy session")
	case <-time.After(2 * dataSocketOpenTimeout):
		// Connection survived beyond the timeout window — timer was cancelled correctly.
	}

	// Clean up: close the underlying connection to unblock the goroutine.
	if attacher.capturedConn != nil {
		attacher.capturedConn.Close()
	}

	select {
	case <-done:
		// Clean exit.
	case <-time.After(2 * time.Second):
		t.Fatal("openDataSocketURL did not return after connection was closed")
	}
}
