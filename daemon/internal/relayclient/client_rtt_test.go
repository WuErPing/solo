package relayclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
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
	t.Cleanup(func() { controlPingInterval.Store(origInterval) })

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

// TestRelayLegRTT_ResetOnReconnect verifies that establishing a fresh
// control socket invalidates the RTT measured on the previous one, by
// driving the real connectControl reconnect path against a fake relay.
func TestRelayLegRTT_ResetOnReconnect(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

	// Fake relay: answers ping with pong on every control socket and lets
	// the test drop individual connections to force a reconnect.
	var mu sync.Mutex
	var conns []*websocket.Conn
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		conns = append(conns, conn)
		mu.Unlock()
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
	defer srv.Close()

	origInterval := controlPingInterval.Load()
	controlPingInterval.Store(int64(50 * time.Millisecond))
	t.Cleanup(func() { controlPingInterval.Store(origInterval) })

	host := srv.Listener.Addr().String()
	client := NewClient("server-id", host, &fastSessionAttacher{}, testLogger(), nil, false)
	if err := client.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Stop()

	// Wait for the first ping/pong exchange to produce an RTT measurement.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, _, ok := client.RelayLegRTT(); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no RTT measurement within 3s of keepalive")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Stretch the ping interval so the new keepalive cannot re-measure
	// before the reset assertion below.
	controlPingInterval.Store(int64(time.Hour))

	// Drop the control socket server-side; the read pump exits and
	// schedules a reconnect.
	mu.Lock()
	first := conns[0]
	mu.Unlock()
	_ = first.Close()

	deadline = time.Now().Add(3 * time.Second)
	for {
		client.reconnectMu.Lock()
		pending := client.reconnectTimer != nil
		client.reconnectMu.Unlock()
		if pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reconnect was not scheduled after control socket drop")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Fire the reconnect immediately — equivalent to the timer expiring.
	client.reconnectMu.Lock()
	if client.reconnectTimer != nil {
		client.reconnectTimer.Stop()
		client.reconnectTimer = nil
	}
	client.reconnectAttempt = 0
	client.reconnectMu.Unlock()

	client.connectControl()

	// Wait until the second control connection is up server-side.
	deadline = time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		n := len(conns)
		mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reconnect did not establish a new control connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The fresh control socket must have invalidated the old measurement.
	if _, _, ok := client.RelayLegRTT(); ok {
		t.Fatal("expected ok=false after reconnect reset the RTT measurement")
	}
}
