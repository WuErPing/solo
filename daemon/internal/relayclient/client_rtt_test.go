package relayclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newPingPongRelayServer creates a mock relay that answers every control
// "ping" with a "pong", like the real relay does.
func newPingPongRelayServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var base struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &base); err != nil {
				continue
			}
			if base.Type == "ping" {
				pong := struct {
					Type string `json:"type"`
					Ts   int64  `json:"ts"`
				}{Type: "pong", Ts: time.Now().UnixMilli()}
				out, _ := json.Marshal(pong)
				if err := conn.WriteMessage(websocket.TextMessage, out); err != nil {
					return
				}
			}
		}
	}))
}

// TestRelayLegRTT_NotMeasured verifies the getter reports ok=false before
// any control ping/pong exchange and while no control socket is up.
func TestRelayLegRTT_NotMeasured(t *testing.T) {
	client := NewClient("server-id", "127.0.0.1:1", &fastSessionAttacher{}, testLogger(), nil, false)
	if rtt, _, ok := client.RelayLegRTT(); ok {
		t.Fatalf("expected ok=false on fresh client, got ok=true rtt=%d", rtt)
	}
}

// TestControlKeepalive_MeasuresRelayRTT verifies the keepalive ping/pong
// exchange on the control socket produces an RTT measurement.
func TestControlKeepalive_MeasuresRelayRTT(t *testing.T) {
	srv := newPingPongRelayServer(t)
	defer srv.Close()

	origInterval := controlPingInterval.Load()
	controlPingInterval.Store(int64(50 * time.Millisecond))
	defer controlPingInterval.Store(origInterval)

	host := srv.Listener.Addr().String()
	client := NewClient("server-id", host, &fastSessionAttacher{}, testLogger(), nil, false)
	if err := client.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Stop()

	deadline := time.Now().Add(3 * time.Second)
	for {
		rtt, measuredAt, ok := client.RelayLegRTT()
		if ok {
			if rtt < 0 {
				t.Fatalf("negative RTT: %d", rtt)
			}
			if time.Since(measuredAt) > 5*time.Second {
				t.Fatalf("measuredAt too old: %v", measuredAt)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no RTT measurement within 3s of keepalive")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestRelayLegRTT_ResetOnReconnect verifies that a new control socket
// invalidates the RTT measured on the previous one.
func TestRelayLegRTT_ResetOnReconnect(t *testing.T) {
	client := NewClient("server-id", "127.0.0.1:1", &fastSessionAttacher{}, testLogger(), nil, false)

	// Simulate a completed measurement on a live control socket.
	client.controlMu.Lock()
	client.controlConn = &websocket.Conn{}
	client.controlMu.Unlock()
	client.rttMu.Lock()
	client.lastPingSentAt = time.Now()
	client.lastRttMs = 12
	client.rttMeasuredAt = time.Now()
	client.rttMeasured = true
	client.rttMu.Unlock()

	if _, _, ok := client.RelayLegRTT(); !ok {
		t.Fatal("expected ok=true after simulated measurement")
	}

	// Same reset as connectControl performs for a fresh control socket.
	client.rttMu.Lock()
	client.lastPingSentAt = time.Time{}
	client.rttMeasured = false
	client.rttMu.Unlock()
	client.controlMu.Lock()
	client.controlConn = nil
	client.controlMu.Unlock()

	if _, _, ok := client.RelayLegRTT(); ok {
		t.Fatal("expected ok=false after control socket reset")
	}
}
