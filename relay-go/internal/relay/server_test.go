package relay

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	logger := slog.Default()
	store := NewSessionStore(200, logger)
	srv := NewServer(store, 200, logger, nil)
	ts := httptest.NewServer(srv.Handler())
	return srv, ts
}

func dialWS(t *testing.T, ts *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return conn
}

func readJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(msg, &v); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return v
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
	// Verify enhanced fields exist
	if _, ok := body["sessions"]; !ok {
		t.Fatal("health response missing sessions field")
	}
	if _, ok := body["connections"]; !ok {
		t.Fatal("health response missing connections field")
	}
	if _, ok := body["version"]; !ok {
		t.Fatal("health response missing version field")
	}
}

func TestHealthEndpointWithActiveConnections(t *testing.T) {
	srv, ts := newTestServer(t)
	defer ts.Close()

	// Verify initial state: no sessions, no connections
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	if body["sessions"] != float64(0) {
		t.Fatalf("expected 0 sessions initially, got %v", body["sessions"])
	}
	if body["connections"] != float64(0) {
		t.Fatalf("expected 0 connections initially, got %v", body["connections"])
	}

	// Create a control socket
	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=health1&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	// Verify 1 session, 1 connection
	resp, err = http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	if body["sessions"] != float64(1) {
		t.Fatalf("expected 1 session, got %v", body["sessions"])
	}
	if body["connections"] != float64(1) {
		t.Fatalf("expected 1 connection, got %v", body["connections"])
	}

	// Create a client socket
	client := dialWS(t, ts, protocol.WSEndpoint+"?serverId=health1&role=client&v=2&connectionId=c1")
	defer client.Close()
	readJSON(t, control) // consume connected

	// Verify 1 session, 2 connections
	resp, err = http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	if body["sessions"] != float64(1) {
		t.Fatalf("expected 1 session, got %v", body["sessions"])
	}
	if body["connections"] != float64(2) {
		t.Fatalf("expected 2 connections, got %v", body["connections"])
	}

	_ = srv // suppress unused warning
}

func TestV2ServerControlConnect(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test1&role=server&v=2")
	defer control.Close()

	msg := readJSON(t, control)
	if msg["type"] != "sync" {
		t.Fatalf("expected sync message, got %v", msg)
	}
}

func TestV2ClientServerRelay(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test2&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	client := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test2&role=client&v=2&connectionId=abc")
	defer client.Close()

	msg := readJSON(t, control)
	if msg["type"] != "connected" || msg["connectionId"] != "abc" {
		t.Fatalf("expected connected(abc), got %v", msg)
	}

	serverData := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test2&role=server&v=2&connectionId=abc")
	defer serverData.Close()

	testPayload := `{"encrypted":"hello"}`
	if err := client.WriteMessage(websocket.TextMessage, []byte(testPayload)); err != nil {
		t.Fatal(err)
	}
	serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, received, err := serverData.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(received) != testPayload {
		t.Fatalf("expected %s, got %s", testPayload, string(received))
	}

	serverPayload := `{"encrypted":"reply"}`
	if err := serverData.WriteMessage(websocket.TextMessage, []byte(serverPayload)); err != nil {
		t.Fatal(err)
	}
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, clientMsg, err := client.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(clientMsg) != serverPayload {
		t.Fatalf("expected %s, got %s", serverPayload, string(clientMsg))
	}
}

func TestV2PingPong(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test3&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	ping := `{"type":"ping"}`
	control.WriteMessage(websocket.TextMessage, []byte(ping))

	msg := readJSON(t, control)
	if msg["type"] != "pong" {
		t.Fatalf("expected pong, got %v", msg)
	}
	if _, ok := msg["ts"]; !ok {
		t.Fatal("pong response missing ts field")
	}
}

func TestV2ControlChannelIgnoresNonPingJson(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=ignore_nonping&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	control.WriteMessage(websocket.TextMessage, []byte(`{"type":"something_else"}`))

	// The server should NOT respond to non-ping messages on control channel.
	// Verify by sending a ping afterward and checking we get the pong (not the
	// non-ping response or some error).
	control.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))

	msg := readJSON(t, control)
	if msg["type"] != "pong" {
		t.Fatalf("expected pong after non-ping message, got %v", msg)
	}
}

func TestV2ControlChannelIgnoresMalformedJson(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=malformed&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	control.WriteMessage(websocket.TextMessage, []byte(`not json`))

	// Should be silently ignored. Verify channel is still healthy with a real ping.
	control.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))

	msg := readJSON(t, control)
	if msg["type"] != "pong" {
		t.Fatalf("expected pong after malformed JSON, got %v", msg)
	}
}

func TestV2ControlChannelIgnoresBinaryPing(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=binping&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	// Send ping as binary then as text back-to-back.
	// If binary ping is incorrectly handled, two pongs arrive.
	control.WriteMessage(websocket.BinaryMessage, []byte(`{"type":"ping"}`))
	control.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))

	// Read first pong (from text ping, or from binary ping if buggy).
	msg := readJSON(t, control)
	if msg["type"] != "pong" {
		t.Fatalf("expected pong, got %v", msg)
	}

	// Check for a second pong with a short deadline. If binary ping was
	// incorrectly handled, a second pong arrives; the test must fail.
	control.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := control.ReadMessage()
	if err == nil {
		t.Fatal("server incorrectly responded to binary ping (extra pong received)")
	}
}

func TestV2PingOnlyOnControlChannel(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=ping_ctrl&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	// Connect a client (has connectionId)
	client := dialWS(t, ts, protocol.WSEndpoint+"?serverId=ping_ctrl&role=client&v=2&connectionId=abc")
	defer client.Close()
	readJSON(t, control) // consume connected

	serverData := dialWS(t, ts, protocol.WSEndpoint+"?serverId=ping_ctrl&role=server&v=2&connectionId=abc")
	defer serverData.Close()

	// Send ping on the data channel (has connectionId). It should be relayed, not
	// intercepted as a control ping.
	client.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))

	serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := serverData.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != `{"type":"ping"}` {
		t.Fatalf("expected ping to be relayed on data channel, got %s", string(msg))
	}
}

func TestV2ClientAutoConnectionId(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test4&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	client := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test4&role=client&v=2")
	defer client.Close()

	msg := readJSON(t, control)
	if msg["type"] != "connected" {
		t.Fatalf("expected connected, got %v", msg)
	}
	cid, ok := msg["connectionId"].(string)
	if !ok || !strings.HasPrefix(cid, "conn_") {
		t.Fatalf("expected auto-assigned conn_ id, got %v", msg)
	}
}

func TestV2RejectV1(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	u := "ws" + strings.TrimPrefix(ts.URL, "http") + protocol.WSEndpoint + "?serverId=test5&role=server&v=1"
	_, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err == nil {
		t.Fatal("expected v1 to be rejected")
	}
}

func TestV2FrameBuffering(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test6&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	client := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test6&role=client&v=2&connectionId=buf")
	defer client.Close()
	readJSON(t, control) // consume connected

	for i := 0; i < 3; i++ {
		client.WriteMessage(websocket.TextMessage, []byte(`{"msg":"buffered"}`))
	}
	time.Sleep(100 * time.Millisecond)

	serverData := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test6&role=server&v=2&connectionId=buf")
	defer serverData.Close()

	serverData.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := serverData.ReadMessage()
	if err != nil {
		t.Fatalf("expected buffered message, got error: %v", err)
	}
	if string(msg) != `{"msg":"buffered"}` {
		t.Fatalf("unexpected message: %s", string(msg))
	}
}

func TestV2ClientDisconnectNotifiesControl(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test7&role=server&v=2")
	defer control.Close()
	readJSON(t, control) // consume sync

	client := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test7&role=client&v=2&connectionId=dc")
	readJSON(t, control) // consume connected

	serverData := dialWS(t, ts, protocol.WSEndpoint+"?serverId=test7&role=server&v=2&connectionId=dc")
	defer serverData.Close()
	readJSON(t, control) // consume connected (server data socket join with existing client)

	client.Close()

	msg := readJSON(t, control)
	if msg["type"] != "disconnected" || msg["connectionId"] != "dc" {
		t.Fatalf("expected disconnected(dc), got %v", msg)
	}
}
