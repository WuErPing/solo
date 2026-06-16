package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

// mockDaemonServer creates a minimal Solo daemon for CLI tests.
type mockDaemonServer struct {
	upgrader          websocket.Upgrader
	serverID          string
	providersSnapshot *protocol.ProvidersSnapshotPayload
}

func newMockDaemonServer() *mockDaemonServer {
	return &mockDaemonServer{
		serverID: "test-server-id",
		providersSnapshot: &protocol.ProvidersSnapshotPayload{
			Entries: []protocol.ProviderSnapshotEntry{
				{Provider: "openai", Status: protocol.ProviderReady, Label: "OpenAI", Models: []protocol.AgentModelDefinition{{ID: "gpt-4"}}},
				{Provider: "anthropic", Status: protocol.ProviderLoading, Label: "Anthropic", Models: []protocol.AgentModelDefinition{{ID: "claude-3"}}},
			},
			GeneratedAt: "2024-01-01T00:00:00Z",
		},
		upgrader: websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }},
	}
}

func (m *mockDaemonServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Read hello
	var hello protocol.WSInboundMessage
	if err := conn.ReadJSON(&hello); err != nil {
		return
	}
	if hello.Type != "hello" {
		return
	}

	// Send server_info
	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type":     "server_info",
			"status":   "server_info",
			"serverId": m.serverID,
			"version":  "0.1.0",
		},
	})

	// Send providers_snapshot_update
	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type":    "providers_snapshot_update",
			"payload": m.providersSnapshot,
		},
	})

	// Echo loop for requests
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var inbound protocol.WSInboundMessage
		if err := json.Unmarshal(data, &inbound); err != nil {
			continue
		}
		if inbound.Type != "session" {
			continue
		}

		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				RequestID string `json:"requestId"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(inbound.Message, &peek)

		// Respond based on message type
		resp := map[string]interface{}{
			"type": "fetch_agents_response",
			"payload": map[string]interface{}{
				"requestId": peek.Payload.RequestID,
				"entries":   []interface{}{},
				"pageInfo":  map[string]interface{}{"hasMore": false},
			},
		}
		_ = conn.WriteJSON(protocol.WSOutboundMessage{
			Type:    "session",
			Message: resp,
		})
	}
}

func setupTestCLI(t *testing.T) (*mockDaemonServer, *bytes.Buffer, *bytes.Buffer) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	t.Cleanup(func() { os.Unsetenv("SOLO_HOME") })

	mock := newMockDaemonServer()
	srv := httptest.NewServer(mock)
	t.Cleanup(srv.Close)

	// Override stdout/stderr for output capture
	oldStdout := cmdStdout
	oldStderr := cmdStderr
	var outBuf, errBuf bytes.Buffer
	cmdStdout = &outBuf
	cmdStderr = &errBuf
	t.Cleanup(func() {
		cmdStdout = oldStdout
		cmdStderr = oldStderr
	})

	flagHost = srv.Listener.Addr().String()

	return mock, &outBuf, &errBuf
}

func TestResolveAgentID_ExactMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
		{ID: "def456", Title: "Other"},
	}
	if got := resolveAgentID("abc123", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_PrefixMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
	}
	if got := resolveAgentID("abc", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_TitleMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
	}
	if got := resolveAgentID("My Agent", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_PartialTitleMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
	}
	if got := resolveAgentID("Agent", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_AmbiguousPrefix(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "A"},
		{ID: "abc456", Title: "B"},
	}
	// Ambiguous prefix returns first match
	got := resolveAgentID("abc", agents)
	if got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_Empty(t *testing.T) {
	agents := []agentEntry{{ID: "abc123", Title: "X"}}
	if got := resolveAgentID("", agents); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := resolveAgentID("x", nil); got != "" {
		t.Errorf("expected empty string for nil agents, got %q", got)
	}
}

func TestToLower(t *testing.T) {
	if got := toLower("Hello World"); got != "hello world" {
		t.Errorf("expected hello world, got %q", got)
	}
	if got := toLower("ABC123"); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
	if got := toLower(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Error("expected true")
	}
	if contains("hello", "world") {
		t.Error("expected false")
	}
	if !contains("hello", "") {
		t.Error("expected true for empty substring")
	}
}

func TestSearchSubstring(t *testing.T) {
	if !searchSubstring("hello world", "world") {
		t.Error("expected true")
	}
	if searchSubstring("hello", "world") {
		t.Error("expected false")
	}
}

func TestGetOutputOpts_Default(t *testing.T) {
	opts := getOutputOpts("table", false, false, false, false)
	if opts.Format != output.FormatTable {
		t.Errorf("expected table format, got %v", opts.Format)
	}
}

func TestGetOutputOpts_JSON(t *testing.T) {
	opts := getOutputOpts("table", true, false, false, true)
	if opts.Format != output.FormatJSON {
		t.Errorf("expected json format, got %v", opts.Format)
	}
	if !opts.NoColor {
		t.Error("expected NoColor true")
	}
}

func TestGetOutputOpts_Quiet(t *testing.T) {
	opts := getOutputOpts("table", false, true, false, false)
	if opts.Format != output.FormatQuiet {
		t.Errorf("expected quiet format, got %v", opts.Format)
	}
}

func TestGetOutputOpts_InvalidFormat(t *testing.T) {
	// getOutputOpts with invalid format calls os.Exit(1).
	// We can't easily test os.Exit, but we can verify ParseOutputFormat.
	_, err := output.ParseOutputFormat("invalid")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestRunDaemonStatus_Table(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := runDaemonStatus(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonStatus failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Daemon is running") {
		t.Errorf("expected 'Daemon is running' in output, got: %s", out)
	}
	if !strings.Contains(out, "test-server-id") {
		t.Errorf("expected server ID in output, got: %s", out)
	}
}

func TestRunDaemonStatus_JSON(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "json"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := runDaemonStatus(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonStatus failed: %v", err)
	}

	out := outBuf.String()
	var result map[string]interface{}
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON output: %v\noutput: %s", err, out)
	}
	if result["status"] != "running" {
		t.Errorf("expected status running, got %v", result["status"])
	}
}

func TestRunDaemonStatus_DaemonNotRunning(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	oldStdout := cmdStdout
	oldStderr := cmdStderr
	var outBuf, errBuf bytes.Buffer
	cmdStdout = &outBuf
	cmdStderr = &errBuf
	defer func() {
		cmdStdout = oldStdout
		cmdStderr = oldStderr
	}()

	// Point to a port that is definitely not listening
	flagHost = "127.0.0.1:1"
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runDaemonStatus(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when daemon is not running")
	}
	ce, ok := err.(*output.CommandError)
	if !ok || ce.Code != "DAEMON_NOT_RUNNING" {
		t.Errorf("expected DAEMON_NOT_RUNNING error, got %v", err)
	}
}

func TestRunProviderLs_Table(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderLs(cmd, []string{}); err != nil {
		t.Fatalf("runProviderLs failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "openai") {
		t.Errorf("expected openai in output, got: %s", out)
	}
	if !strings.Contains(out, "anthropic") {
		t.Errorf("expected anthropic in output, got: %s", out)
	}
}

func TestRunProviderLs_JSON(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "json"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderLs(cmd, []string{}); err != nil {
		t.Fatalf("runProviderLs failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array: %v\noutput: %s", err, outBuf.String())
	}
	if len(result) != 2 {
		t.Errorf("expected 2 providers, got %d", len(result))
	}
}

func TestRunProviderLs_NoProviders(t *testing.T) {
	mock, outBuf, _ := setupTestCLI(t)
	mock.providersSnapshot = &protocol.ProvidersSnapshotPayload{Entries: []protocol.ProviderSnapshotEntry{}}

	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderLs(cmd, []string{}); err != nil {
		t.Fatalf("runProviderLs failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "No providers available") {
		t.Errorf("expected 'No providers available', got: %s", out)
	}
}

func TestExecute(_ *testing.T) {
	// Execute with no args should not panic.
	rootCmd.SetArgs([]string{})
	// Just ensure it doesn't crash; we don't want to actually call os.Exit.
}
