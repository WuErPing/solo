package relay

// Regression tests for the socket-replacement race in handleClose
// (observed 2026-09-09): when a new control or server-data socket replaces
// an existing one, the old socket's readPump exits afterwards and its
// handleClose must not clear the new registration. Before the fix, the
// stale close orphaned the live control socket — keepalive ping/pong kept
// flowing while notifyControl silently dropped every client ConnectedMessage,
// leaving mobile clients in a connect/timeout retry loop.

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// waitForCloseProcessing gives the relay a moment to run handleClose for a
// socket closed by the test, so subsequent assertions observe the settled
// session state.
func waitForCloseProcessing() {
	time.Sleep(200 * time.Millisecond)
}

// TestV2ControlReplacementKeepsNewRegistration verifies that closing the
// replaced control socket does not unregister the new one: a client
// connecting afterwards must still trigger a "connected" notification on the
// new control socket.
func TestV2ControlReplacementKeepsNewRegistration(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	controlA := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl1&role=server&v=2")
	readJSONExtra(t, controlA) // consume initial sync

	// Control B replaces A; the relay closes A.
	controlB := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl1&role=server&v=2")
	defer controlB.Close()
	readJSONExtra(t, controlB) // consume initial sync

	// A observes the forced close, then its readPump on the relay exits and
	// runs handleClose — this must not clear B's registration.
	controlA.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := controlA.ReadMessage(); err != nil {
			break
		}
	}
	controlA.Close()
	waitForCloseProcessing()

	// A client connecting now must be signalled to control B.
	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl1&role=client&v=2&connectionId=cr1")
	defer client.Close()

	msg := readJSONExtra(t, controlB)
	if msg["type"] != "connected" || msg["connectionId"] != "cr1" {
		t.Fatalf("expected connected(cr1) on replacement control socket, got %v", msg)
	}
}

// TestV2StaleDataSocketCloseKeepsReplacement verifies that closing a replaced
// server-data socket does not clear the new data socket, close the clients,
// or emit a spurious "disconnected" notification.
func TestV2StaleDataSocketCloseKeepsReplacement(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl2&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl2&role=client&v=2&connectionId=dr1")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	dataD1 := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl2&role=server&v=2&connectionId=dr1")
	readJSONExtra(t, control) // consume second connected (after server data joins)

	// D2 replaces D1; the relay closes D1.
	dataD2 := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=repl2&role=server&v=2&connectionId=dr1")
	defer dataD2.Close()
	readJSONExtra(t, control) // consume third connected (replacement)

	dataD1.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := dataD1.ReadMessage(); err != nil {
			break
		}
	}
	dataD1.Close()
	waitForCloseProcessing()

	// Client frames must still reach the daemon through D2.
	if err := client.WriteMessage(websocket.TextMessage, []byte("hello-daemon")); err != nil {
		t.Fatalf("client write failed: %v", err)
	}
	dataD2.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := dataD2.ReadMessage()
	if err != nil {
		t.Fatalf("replacement data socket must receive client frames: %v", err)
	}
	if string(msg) != "hello-daemon" {
		t.Fatalf("unexpected payload on replacement data socket: %q", msg)
	}

	// And no spurious "disconnected" was sent to the control socket.
	control.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, msg, err := control.ReadMessage(); err == nil {
		t.Fatalf("expected no control message after stale data close, got %q", msg)
	}

	// Client socket must still be open.
	if err := client.WriteMessage(websocket.TextMessage, []byte("still-here")); err != nil {
		t.Fatalf("client socket was closed by stale data socket close: %v", err)
	}
}
