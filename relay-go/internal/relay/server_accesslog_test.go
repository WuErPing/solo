package relay

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
	"github.com/WuErPing/solo/relay/internal/accesslog"
)

// lockedBuffer makes the shared access-log buffer safe to read from the test
// goroutine while server goroutines are still writing.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newTestServerWithAccessLog(t *testing.T) (*httptest.Server, *lockedBuffer) {
	t.Helper()
	logger := slog.Default()
	store := NewSessionStore(200, logger)
	srv := NewServer(store, 200, 0, logger, nil)
	buf := &lockedBuffer{}
	// Must be set before Handler() so the middleware wraps the mux.
	srv.SetAccessLogger(accesslog.New(buf))
	ts := httptest.NewServer(srv.Handler())
	return ts, buf
}

func logEvents(t *testing.T, data string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		if line == "" {
			continue
		}
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatalf("access log line is not valid JSON: %q: %v", line, err)
		}
		events = append(events, v)
	}
	return events
}

func findEvent(events []map[string]any, event string, match func(map[string]any) bool) map[string]any {
	for _, e := range events {
		if e["event"] == event && (match == nil || match(e)) {
			return e
		}
	}
	return nil
}

func TestAccessLogEvents(t *testing.T) {
	ts, buf := newTestServerWithAccessLog(t)
	defer ts.Close()

	// Plain HTTP request: XFF first hop wins, UA recorded.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	req.Header.Set("User-Agent", "accesslog-test/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// WS control connection: one ping (=> msgs == 1), then close.
	control := dialWS(t, ts, protocol.WSEndpoint+"?serverId=alog1&role=server&v=2")
	readJSON(t, control) // consume sync
	if err := control.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	readJSON(t, control) // consume pong
	control.Close()

	// ws_close is logged from the server-side readPump defer; the /ws http_req
	// line follows when handleWebSocket returns. Wait for both.
	deadline := time.Now().Add(3 * time.Second)
	for {
		data := buf.String()
		if strings.Contains(data, `"event":"ws_close"`) && strings.Contains(data, `"path":"/ws"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for ws_close and /ws http_req; log:\n%s", data)
		}
		time.Sleep(10 * time.Millisecond)
	}

	events := logEvents(t, buf.String())

	health := findEvent(events, "http_req", func(e map[string]any) bool { return e["path"] == "/health" })
	if health == nil {
		t.Fatal("missing http_req for /health")
	}
	if health["status"] != float64(200) || health["ip"] != "1.2.3.4" || health["ua"] != "accesslog-test/1.0" {
		t.Fatalf("unexpected /health http_req: %v", health)
	}

	open := findEvent(events, "ws_open", nil)
	if open == nil {
		t.Fatal("missing ws_open")
	}
	if open["server_id"] != "alog1" || open["role"] != "server" || open["conn_id"] != "" || open["version"] != "2" {
		t.Fatalf("unexpected ws_open: %v", open)
	}
	if open["ip"] == "" || open["ua"] == "" {
		t.Fatalf("ws_open missing ip/ua: %v", open)
	}

	closed := findEvent(events, "ws_close", nil)
	if closed == nil {
		t.Fatal("missing ws_close")
	}
	if closed["server_id"] != "alog1" || closed["role"] != "server" || closed["msgs"] != float64(1) {
		t.Fatalf("unexpected ws_close: %v", closed)
	}
	if closed["dur_ms"].(float64) < 0 {
		t.Fatalf("ws_close dur_ms must be >= 0: %v", closed)
	}

	// The upgrade request itself is logged once the handler returns, with
	// status 101 captured through the Hijack passthrough.
	wsReq := findEvent(events, "http_req", func(e map[string]any) bool { return e["path"] == "/ws" })
	if wsReq == nil {
		t.Fatal("missing http_req for /ws")
	}
	if wsReq["status"] != float64(101) {
		t.Fatalf("expected status 101 for /ws upgrade, got %v", wsReq["status"])
	}
}

func TestAccessLogDisabledByDefault(t *testing.T) {
	// Without SetAccessLogger the server behaves exactly as before (no
	// middleware), so existing tests keep passing unchanged.
	srv, _ := newTestServer(t)
	if srv.accessLog != nil {
		t.Fatal("accessLog must default to nil")
	}
}
