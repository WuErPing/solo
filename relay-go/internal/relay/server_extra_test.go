package relay

// Additional tests ported from solo/packages/relay test suite.
// Coverage: nudge/reset timer, multi-client, server-data disconnect, control disconnect,
// binary frame relay, opaque byte forwarding.

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// newTestServerFast creates a server with very short nudge delays for timer-based tests.
func newTestServerFast(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	logger := slog.Default()
	store := NewSessionStore(200, logger)
	srv := NewServer(store, 200, 0, logger, nil)
	srv.NudgeSyncDelay = 80 * time.Millisecond
	srv.NudgeResetDelay = 40 * time.Millisecond
	ts := httptest.NewServer(srv.Handler())
	return srv, ts
}

// dialWSExtra dials a WebSocket, failing the test on error.
func dialWSExtra(t *testing.T, ts *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial %s failed: %v", path, err)
	}
	return conn
}

// readJSONExtra reads one text message and unmarshals it, with a 3s deadline.
func readJSONExtra(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(msg, &v); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return v
}

// --- Nudge / Reset timer tests -----------------------------------------------

// TestV2NudgeSendsSync verifies that when a client is connected but the server-data
// socket has not arrived, the relay sends a sync message to the control socket
// after nudgeSyncDelay.
func TestV2NudgeSendsSync(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge1&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	// Client connects — no server-data socket follows.
	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge1&role=client&v=2&connectionId=c1")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	// After nudgeSyncDelay the relay should send another sync.
	control.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := control.ReadMessage()
	if err != nil {
		t.Fatalf("expected sync nudge, got error: %v", err)
	}
	var v map[string]any
	json.Unmarshal(msg, &v)
	if v["type"] != "sync" {
		t.Fatalf("expected sync, got %v", v)
	}
}

// TestV2NudgeResetsControlWhenUnresponsive verifies that when the nudge sync is
// sent but still no server-data socket appears, the control socket is closed
// after nudgeSyncDelay + nudgeResetDelay.
func TestV2NudgeResetsControlWhenUnresponsive(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge2&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge2&role=client&v=2&connectionId=c2")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	// After nudgeSyncDelay: sync arrives.
	control.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	control.ReadMessage() // consume sync nudge

	// After another nudgeResetDelay: control should be force-closed.
	control.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("expected control socket to be closed by relay, but read succeeded")
	}
}

// TestV2NudgeCancelledWhenServerDataArrives verifies that if the server-data
// socket connects before the nudge fires, the control socket is NOT nudged.
func TestV2NudgeCancelledWhenServerDataArrives(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge3&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge3&role=client&v=2&connectionId=c3")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	// Server-data socket arrives before the nudge deadline.
	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge3&role=server&v=2&connectionId=c3")
	defer serverData.Close()
	readJSONExtra(t, control) // consume "connected" notification for server-data socket

	// Wait beyond both nudge delays; control must NOT receive a sync.
	control.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("control received an unexpected message after server-data connected")
	}
}

// TestV2NudgeCancelledWhenClientDisconnects verifies that if the client disconnects
// before the nudge fires, the control socket is NOT nudged.
func TestV2NudgeCancelledWhenClientDisconnects(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge4&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=nudge4&role=client&v=2&connectionId=c4")
	readJSONExtra(t, control) // consume connected

	// Client disconnects immediately.
	client.Close()
	readJSONExtra(t, control) // consume disconnected

	// Wait beyond both nudge delays; control must NOT receive a sync.
	control.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("control received an unexpected message after client disconnected")
	}
}

// --- Multi-client tests -------------------------------------------------------

// TestV2MultipleClientsOnSameConnection verifies that multiple client sockets can
// share the same connectionId without displacing each other.
func TestV2MultipleClientsOnSameConnection(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi1&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client1 := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi1&role=client&v=2&connectionId=shared")
	defer client1.Close()
	readJSONExtra(t, control) // consume connected

	// Second client on same connectionId — must succeed without closing client1.
	client2 := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi1&role=client&v=2&connectionId=shared")
	defer client2.Close()
	// The relay sends "connected" again for the same connectionId.
	readJSONExtra(t, control)

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi1&role=server&v=2&connectionId=shared")
	defer serverData.Close()

	// Both clients must receive a broadcast from server.
	payload := `{"msg":"broadcast"}`
	serverData.WriteMessage(websocket.TextMessage, []byte(payload))

	for i, c := range []*websocket.Conn{client1, client2} {
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, got, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("client%d read failed: %v", i+1, err)
		}
		if string(got) != payload {
			t.Fatalf("client%d expected %s, got %s", i+1, payload, got)
		}
	}
}

// TestV2OneClientDisconnectKeepsServerDataSocket verifies that when one of two
// clients on the same connectionId disconnects, the server-data socket stays open.
func TestV2OneClientDisconnectKeepsServerDataSocket(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi2&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client1 := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi2&role=client&v=2&connectionId=shared2")
	defer client1.Close()
	readJSONExtra(t, control) // consume connected

	client2 := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi2&role=client&v=2&connectionId=shared2")
	defer client2.Close()
	readJSONExtra(t, control) // consume second connected

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=multi2&role=server&v=2&connectionId=shared2")
	defer serverData.Close()
	readJSONExtra(t, control) // consume "connected" notification for server-data socket join

	// client1 disconnects.
	client1.Close()

	// Control must NOT receive a "disconnected" message (client2 still alive).
	control.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("control received unexpected message after one of two clients disconnected")
	}

	// server-data can still write to client2.
	payload := `{"msg":"still alive"}`
	if err := serverData.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		t.Fatalf("server-data write failed after one client disconnected: %v", err)
	}
	client2.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, got, err := client2.ReadMessage()
	if err != nil {
		t.Fatalf("client2 read failed: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("expected %s, got %s", payload, got)
	}
}

// --- Disconnect propagation tests --------------------------------------------

// TestV2ServerDataDisconnectClosesAllClients verifies that when the server-data
// socket closes, all client sockets on that connectionId are closed.
func TestV2ServerDataDisconnectClosesAllClients(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdclose1&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdclose1&role=client&v=2&connectionId=sd1")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdclose1&role=server&v=2&connectionId=sd1")
	readJSONExtra(t, control) // consume second connected (after server data joins)

	// Close server-data socket.
	serverData.Close()

	// Client should receive a close frame.
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := client.ReadMessage()
	if err == nil {
		t.Fatal("expected client to be closed after server-data disconnected")
	}

	// Control receives disconnected notification.
	msg := readJSONExtra(t, control)
	if msg["type"] != "disconnected" || msg["connectionId"] != "sd1" {
		t.Fatalf("expected disconnected(sd1), got %v", msg)
	}
}

// TestV2ControlDisconnectClosesServerDataSockets verifies that when the control
// socket closes, all server-data sockets for that serverId are closed.
func TestV2ControlDisconnectClosesServerDataSockets(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=ctrlclose1&role=server&v=2")
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=ctrlclose1&role=client&v=2&connectionId=cd1")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=ctrlclose1&role=server&v=2&connectionId=cd1")
	defer serverData.Close()

	// Close control socket.
	control.Close()

	// Server-data socket should be closed.
	serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := serverData.ReadMessage()
	if err == nil {
		t.Fatal("expected server-data socket to be closed after control disconnected")
	}
}

// --- Binary frame tests -------------------------------------------------------

// TestV2BinaryFrameRelay verifies that binary frames are forwarded unchanged
// from client to server-data and back.
func TestV2BinaryFrameRelay(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=bin1&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=bin1&role=client&v=2&connectionId=b1")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=bin1&role=server&v=2&connectionId=b1")
	defer serverData.Close()

	// Client → server (binary).
	binaryPayload := []byte{0x01, 0x02, 0x03, 0xDE, 0xAD, 0xBE, 0xEF}
	client.WriteMessage(websocket.BinaryMessage, binaryPayload)

	serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
	msgType, got, err := serverData.ReadMessage()
	if err != nil {
		t.Fatalf("server-data read failed: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected binary message type, got %d", msgType)
	}
	if string(got) != string(binaryPayload) {
		t.Fatalf("binary payload mismatch: got %v", got)
	}

	// Server → client (binary).
	serverData.WriteMessage(websocket.BinaryMessage, binaryPayload)

	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	msgType, got, err = client.ReadMessage()
	if err != nil {
		t.Fatalf("client read failed: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected binary message type, got %d", msgType)
	}
	if string(got) != string(binaryPayload) {
		t.Fatalf("binary payload mismatch: got %v", got)
	}
}

// TestV2OpaqueByteForwarding verifies that arbitrary opaque bytes (as would be
// produced by E2EE) are forwarded byte-for-byte without the relay reading or
// modifying them. Mirrors the "relay only sees opaque bytes" test in Solo.
func TestV2OpaqueByteForwarding(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=opaque1&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=opaque1&role=client&v=2&connectionId=op1")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=opaque1&role=server&v=2&connectionId=op1")
	defer serverData.Close()

	// Simulate what an encrypted payload looks like: random nonce + ciphertext.
	// The plaintext ("secret") must NOT appear in the forwarded bytes.
	secret := "This is a secret that relay cannot read"
	// Fake "encrypted" blob: just nonce-like prefix + NOT the plaintext bytes.
	fakeEncrypted := make([]byte, 24+len(secret))
	for i := 0; i < 24; i++ {
		fakeEncrypted[i] = byte(i + 1) // fake nonce
	}
	for i, b := range []byte(secret) {
		fakeEncrypted[24+i] = b ^ 0xFF // XOR so raw bytes differ from plaintext
	}

	client.WriteMessage(websocket.BinaryMessage, fakeEncrypted)

	serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, received, err := serverData.ReadMessage()
	if err != nil {
		t.Fatalf("server-data read failed: %v", err)
	}

	// The raw forwarded bytes must not contain the plaintext secret.
	if strings.Contains(string(received), secret) {
		t.Fatal("relay forwarded plaintext that should be opaque ciphertext")
	}

	// But they must be identical to what was sent (relay must not alter bytes).
	if string(received) != string(fakeEncrypted) {
		t.Fatal("relay modified the bytes during forwarding")
	}
}

// TestV2BufferedFramesFlushedOnServerDataConnect verifies that frames buffered
// while no server-data socket exists are all flushed once it connects.
func TestV2BufferedFramesFlushedOnServerDataConnect(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=buf2&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=buf2&role=client&v=2&connectionId=bf2")
	defer client.Close()
	readJSONExtra(t, control) // consume connected

	// Send several frames before server-data connects.
	const numFrames = 5
	for i := 0; i < numFrames; i++ {
		client.WriteMessage(websocket.TextMessage, []byte(`{"seq":"`+string(rune('0'+i))+`"}`))
	}
	time.Sleep(50 * time.Millisecond)

	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=buf2&role=server&v=2&connectionId=bf2")
	defer serverData.Close()

	// All buffered frames must arrive.
	for i := 0; i < numFrames; i++ {
		serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, err := serverData.ReadMessage()
		if err != nil {
			t.Fatalf("expected buffered frame %d, got error: %v", i, err)
		}
	}
}

// --- Server-data nudge tests ------------------------------------------------

// TestV2ServerDataNudgeSendsSync verifies that when a server-data socket connects
// but no client is present, the relay sends a sync message to the control socket
// after nudgeSyncDelay.
func TestV2ServerDataNudgeSendsSync(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge1&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	// Server-data socket connects — no client follows.
	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge1&role=server&v=2&connectionId=sd1")
	defer serverData.Close()

	// After nudgeSyncDelay the relay should send a sync nudge.
	control.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := control.ReadMessage()
	if err != nil {
		t.Fatalf("expected sync nudge after server-data connect, got error: %v", err)
	}
	var v map[string]any
	json.Unmarshal(msg, &v)
	if v["type"] != "sync" {
		t.Fatalf("expected sync, got %v", v)
	}
}

// TestV2ServerDataNudgeCancelledWhenClientArrives verifies that if a client
// connects before the server-data nudge fires, the nudge is cancelled.
func TestV2ServerDataNudgeCancelledWhenClientArrives(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge2&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	// Server-data socket connects — no client yet.
	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge2&role=server&v=2&connectionId=sd2")
	defer serverData.Close()

	// Client connects before nudge deadline.
	client := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge2&role=client&v=2&connectionId=sd2")
	defer client.Close()
	readJSONExtra(t, control) // consume "connected"

	// Wait beyond both nudge delays; control must NOT receive a sync.
	control.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("control received an unexpected sync after client connected")
	}
}

// TestV2ServerDataNudgeResetsControlWhenUnresponsive verifies that when the
// server-data nudge sync is sent but still no client appears, the control socket
// is closed after nudgeSyncDelay + nudgeResetDelay.
func TestV2ServerDataNudgeResetsControlWhenUnresponsive(t *testing.T) {
	_, ts := newTestServerFast(t)
	defer ts.Close()

	control := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge3&role=server&v=2")
	defer control.Close()
	readJSONExtra(t, control) // consume initial sync

	// Server-data socket connects — no client follows.
	serverData := dialWSExtra(t, ts, protocol.WSEndpoint+"?serverId=sdnudge3&role=server&v=2&connectionId=sd3")
	defer serverData.Close()

	// After nudgeSyncDelay: sync arrives.
	control.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	control.ReadMessage() // consume sync nudge

	// After another nudgeResetDelay: control should be force-closed.
	control.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("expected control socket to be closed by relay, but read succeeded")
	}
}
