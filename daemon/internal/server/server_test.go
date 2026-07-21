package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/loop"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/daemon/internal/schedule"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

func newTestWSServer(t *testing.T) (*WSServer, *httptest.Server) {
	t.Helper()
	cfg := &config.Config{
		SoloHome:   t.TempDir(),
		ServerID:   "test-server",
		Version:    "0.1.0",
		AppBaseURL: "https://solo.up2ai.top",
	}
	logger := newTestLogger()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
	agentStorage.Initialize()
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewMockAgentClient())
	agentMgr := agent.NewAgentManager(agentStorage, registry, logger)
	agentMgr.Initialize(context.Background())
	timelineStore := agent.NewInMemoryTimelineStore()
	workspaceStore := NewWorkspaceStore(cfg.SoloHome, logger)
	terminalMgr := terminal.NewTerminalManager(logger)
	projectReg := workspace.NewProjectRegistry(cfg.SoloHome)
	workspaceReg := workspace.NewWorkspaceRegistry(cfg.SoloHome)
	gitSvc := workspace.NewWorkspaceGitService()
	scriptMgr := workspace.NewScriptManager()
	scriptProxy := workspace.NewScriptProxy(logger, scriptMgr)

	pushTokenStore := push.NewInMemoryTokenStore()
	pusher := push.NewExpoPushService("", pushTokenStore, logger)
	activityTracker := NewClientActivityTracker()
	loopStore := loop.NewStore()
	loopEngine := loop.NewEngine(loopStore, agentMgr, logger)
	loopEngine.Start(context.Background())
	t.Cleanup(loopEngine.Stop)
	ws := NewWSServerWithConfig(DaemonConfig{
		Config:          cfg,
		Logger:          logger,
		AgentMgr:        agentMgr,
		TimelineStore:   timelineStore,
		Registry:        registry,
		WorkspaceStore:  workspaceStore,
		TerminalMgr:     terminalMgr,
		ProjectReg:      projectReg,
		WorkspaceReg:    workspaceReg,
		GitSvc:          gitSvc,
		ScriptMgr:       scriptMgr,
		ScriptProxy:     scriptProxy,
		PushTokenStore:  pushTokenStore,
		Pusher:          pusher,
		ActivityTracker: activityTracker,
		ScheduleStore:   schedule.NewStore(schedule.WithDataPath(filepath.Join(cfg.SoloHome, "schedules.json"))),
		LoopStore:       loopStore,
		LoopEngine:      loopEngine,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.HandleWebSocket)
	mux.HandleFunc("/api/health", handleHealth)
	mux.Handle("/metrics", promhttp.Handler())

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ws, ts
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(ioDiscard{}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestWSServer(t)
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", result["status"])
	}
}

func TestWebSocketHello(t *testing.T) {
	_, ts := newTestWSServer(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send hello
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        "test-client-1",
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
		AppVersion:      "0.1.0",
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Read server_info response
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp protocol.WSOutboundMessage
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "session" {
		t.Fatalf("expected session message, got %q", resp.Type)
	}
}

func TestWebSocketPingPong(t *testing.T) {
	_, ts := newTestWSServer(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send hello first
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        "test-client-2",
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Read server_info
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read server_info: %v", err)
	}

	// Read providers_snapshot_update (sent after hello)
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read providers_snapshot: %v", err)
	}
	var snapResp protocol.WSOutboundMessage
	if err := json.Unmarshal(msg, &snapResp); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	// Send WS-level ping
	if err := conn.WriteJSON(protocol.WSInboundMessage{Type: "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	// Read pong
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var resp protocol.WSOutboundMessage
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "pong" {
		t.Fatalf("expected pong, got %q", resp.Type)
	}
}

func TestSessionPingPong(t *testing.T) {
	_, ts := newTestWSServer(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Hello handshake
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        "test-client-3",
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	conn.WriteJSON(hello)
	_, _, _ = conn.ReadMessage() // consume server_info

	// Send session-level ping
	ping := protocol.WSInboundMessage{
		Type:    "session",
		Message: json.RawMessage(`{"type":"ping","requestId":"req-1"}`),
	}
	conn.WriteJSON(ping)

	// Read pong
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var resp protocol.WSOutboundMessage
	json.Unmarshal(msg, &resp)
	if resp.Type != "session" {
		t.Fatalf("expected session message, got %q", resp.Type)
	}
}

func TestHelloTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}
	_, ts := newTestWSServer(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Don't send hello, wait for timeout
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected close due to hello timeout")
	}
}

func TestInvalidProtocolVersion(t *testing.T) {
	_, ts := newTestWSServer(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        "bad-proto",
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: 99,
	}
	conn.WriteJSON(hello)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected close due to incompatible protocol")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	_, ts := newTestWSServer(t)
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("expected text/plain content-type, got %q", ct)
	}
}

func TestMetricsConnectionsTotalIncrements(t *testing.T) {
	_, ts := newTestWSServer(t)

	// Get connection count before
	before := getMetricValue(t, ts, "solo_daemon_connections_total")

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send valid hello
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        "metrics-test-client",
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Wait for server_info response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read server_info: %v", err)
	}

	// Connection counter should have incremented
	after := getMetricValue(t, ts, "solo_daemon_connections_total")
	if after != before+1 {
		t.Fatalf("connections_total: before=%d after=%d, expected after=before+1", before, after)
	}
}

func getMetricValue(t *testing.T, ts *httptest.Server, name string) int {
	t.Helper()
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	// Simple line parser for Prometheus text format
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, name+" ") {
			var val int
			fmt.Sscanf(line, name+" %d", &val)
			return val
		}
	}
	return 0
}
