package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

// === Pure function tests ===

func TestShortenID(t *testing.T) {
	if got := shortenID("abc123def456"); got != "abc123de" {
		t.Errorf("expected abc123de, got %q", got)
	}
	if got := shortenID("short"); got != "short" {
		t.Errorf("expected short, got %q", got)
	}
	if got := shortenID(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestIsCwdMatch(t *testing.T) {
	if !isCwdMatch("/home/user", "/home/user") {
		t.Error("expected exact match")
	}
	if !isCwdMatch("/home/user", "/home/user/project") {
		t.Error("expected descendant match")
	}
	if isCwdMatch("/home/user", "/home/user2") {
		t.Error("expected false for sibling")
	}
	if isCwdMatch("/home/user", "/home/user-project") {
		t.Error("expected false for prefix without slash")
	}
}

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"image.png", "image/png"},
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"image.gif", "image/gif"},
		{"image.webp", "image/webp"},
		{"image.svg", "image/svg+xml"},
		{"file.bin", "application/octet-stream"},
		{"file", "application/octet-stream"},
	}
	for _, tc := range tests {
		if got := detectMIME(tc.path); got != tc.expected {
			t.Errorf("detectMIME(%q) = %q, want %q", tc.path, got, tc.expected)
		}
	}
}

func TestIsRPCError(t *testing.T) {
	errMsg := protocol.WSOutboundMessage{
		Type:    "session",
		Message: map[string]interface{}{"type": "rpc_error", "payload": map[string]interface{}{"error": "boom"}},
	}
	if !isRPCError(&errMsg) {
		t.Error("expected rpc_error")
	}
	okMsg := protocol.WSOutboundMessage{
		Type:    "session",
		Message: map[string]interface{}{"type": "fetch_agents_response"},
	}
	if isRPCError(&okMsg) {
		t.Error("expected non-error")
	}
}

func TestExtractRPCError(t *testing.T) {
	msg := protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type": "rpc_error",
			"payload": map[string]interface{}{
				"error": "something failed",
			},
		},
	}
	if got := extractRPCError(&msg); got != "something failed" {
		t.Errorf("expected 'something failed', got %q", got)
	}
}

func TestFormatProvider(t *testing.T) {
	a := protocol.AgentSnapshotPayload{Provider: "openai"}
	if got := formatProvider(a); got != "openai" {
		t.Errorf("expected openai, got %q", got)
	}

	model := "gpt-4"
	a.Model = &model
	if got := formatProvider(a); got != "openai/gpt-4" {
		t.Errorf("expected openai/gpt-4, got %q", got)
	}

	defaultModel := "default"
	a.Model = &defaultModel
	if got := formatProvider(a); got != "openai" {
		t.Errorf("expected openai (default stripped), got %q", got)
	}
}

func TestShortenPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home != "" {
		longPath := home + "/projects"
		if got := shortenPath(longPath); got != "~/projects" {
			t.Errorf("expected ~/projects, got %q", got)
		}
	}
	if got := shortenPath("/tmp"); got != "/tmp" {
		t.Errorf("expected /tmp, got %q", got)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now().Format(time.RFC3339)
	got := relativeTime(now)
	if got != "just now" {
		t.Errorf("expected 'just now', got %q", got)
	}

	fewMin := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	got = relativeTime(fewMin)
	if !strings.Contains(got, "m ago") {
		t.Errorf("expected Xm ago, got %q", got)
	}

	fewHour := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	got = relativeTime(fewHour)
	if !strings.Contains(got, "h ago") {
		t.Errorf("expected Xh ago, got %q", got)
	}

	fewDay := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	got = relativeTime(fewDay)
	if !strings.Contains(got, "d ago") {
		t.Errorf("expected Xd ago, got %q", got)
	}

	invalid := "not-a-time"
	if got := relativeTime(invalid); got != invalid {
		t.Errorf("expected passthrough for invalid, got %q", got)
	}
}

func TestParseLabels(t *testing.T) {
	result := parseLabels([]string{"env=prod", "team=backend"})
	if result["env"] != "prod" {
		t.Errorf("expected prod, got %q", result["env"])
	}
	if result["team"] != "backend" {
		t.Errorf("expected backend, got %q", result["team"])
	}

	result = parseLabels([]string{"no-equals"})
	if len(result) != 0 {
		t.Errorf("expected empty map for invalid label, got %v", result)
	}

	result = parseLabels(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil, got %v", result)
	}
}

func TestIndexByte(t *testing.T) {
	if got := indexByte("hello", 'e'); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
	if got := indexByte("hello", 'z'); got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
}

func TestLoadImages(t *testing.T) {
	tmp := t.TempDir()
	imgPath := filepath.Join(tmp, "test.png")
	_ = os.WriteFile(imgPath, []byte("fake-image-data"), 0644)

	images, err := loadImages([]string{imgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].MimeType != "image/png" {
		t.Errorf("expected image/png, got %q", images[0].MimeType)
	}
	if images[0].Data == "" {
		t.Error("expected non-empty base64 data")
	}
}

func TestLoadImages_Missing(t *testing.T) {
	_, err := loadImages([]string{"/nonexistent/file.png"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// === Enhanced mock daemon server ===

type enhancedMockDaemon struct{}

func newEnhancedMockDaemon() *enhancedMockDaemon {
	return &enhancedMockDaemon{}
}

func (m *enhancedMockDaemon) handler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Read hello
	var hello protocol.WSInboundMessage
	if err := conn.ReadJSON(&hello); err != nil || hello.Type != "hello" {
		return
	}

	// Send server_info
	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type:    "session",
		Message: map[string]interface{}{"type": "server_info", "status": "server_info", "serverId": "test-srv"},
	})

	// Send providers_snapshot_update
	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type": "providers_snapshot_update",
			"payload": map[string]interface{}{
				"entries": []map[string]interface{}{
					{"provider": "openai", "status": "ready", "label": "OpenAI", "models": []map[string]interface{}{{"id": "gpt-4", "label": "GPT-4"}}},
				},
				"generatedAt": time.Now().Format(time.RFC3339),
			},
		},
	})

	agents := []map[string]interface{}{
		{"id": "agent-abc123", "provider": "openai", "status": "running", "cwd": "/tmp", "createdAt": time.Now().Format(time.RFC3339), "title": "Test Agent"},
		{"id": "agent-idle456", "provider": "openai", "status": "idle", "cwd": "/tmp", "createdAt": time.Now().Format(time.RFC3339), "title": "Idle Agent"},
	}

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

		var reqPeek struct {
			Type      string `json:"type"`
			RequestID string `json:"requestId"`
			AgentID   string `json:"agentId"`
		}
		_ = json.Unmarshal(inbound.Message, &reqPeek)

		var resp map[string]interface{}
		switch reqPeek.Type {
		case "fetch_agents_request":
			resp = map[string]interface{}{
				"type": "fetch_agents_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"entries": []map[string]interface{}{
						{"agent": agents[0]},
					},
					"pageInfo": map[string]interface{}{"hasMore": false},
				},
			}
		case "fetch_agent_request":
			var agent map[string]interface{}
			for _, a := range agents {
				if a["id"] == reqPeek.AgentID {
					agent = a
					break
				}
			}
			resp = map[string]interface{}{
				"type": "fetch_agent_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agent":     agent,
				},
			}
		case "archive_agent_request":
			resp = map[string]interface{}{
				"type": "agent_archived",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agentId":   reqPeek.AgentID,
				},
			}
		case "delete_agent_request":
			resp = map[string]interface{}{
				"type": "agent_deleted",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agentId":   reqPeek.AgentID,
				},
			}
		case "cancel_agent_request":
			resp = map[string]interface{}{
				"type": "cancel_agent_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agentId":   reqPeek.AgentID,
				},
			}
		case "send_agent_message_request":
			resp = map[string]interface{}{
				"type": "send_agent_message_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agentId":   reqPeek.AgentID,
				},
			}
		case "shutdown_server_request":
			resp = map[string]interface{}{
				"type":    "status",
				"payload": map[string]interface{}{"requestId": reqPeek.RequestID, "type": "server_shutdown"},
			}
		case "restart_server_request":
			resp = map[string]interface{}{
				"type":    "status",
				"payload": map[string]interface{}{"requestId": reqPeek.RequestID, "type": "server_restart"},
			}
		case "set_agent_mode_request":
			resp = map[string]interface{}{
				"type": "set_agent_mode_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agentId":   reqPeek.AgentID,
				},
			}
		case "fetch_agent_timeline_request":
			resp = map[string]interface{}{
				"type": "fetch_agent_timeline_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"entries":   []map[string]interface{}{},
				},
			}
		case "create_agent_request":
			resp = map[string]interface{}{
				"type": "agent_created",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"agentId":   "agent-new789",
					"status":    "initializing",
				},
			}
		case "wait_for_finish_request":
			resp = map[string]interface{}{
				"type": "wait_for_finish_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"status":    "idle",
					"final": map[string]interface{}{
						"id":       "agent-new789",
						"provider": "openai",
						"status":   "idle",
						"cwd":      "/tmp",
					},
				},
			}
		default:
			resp = map[string]interface{}{
				"type": "fetch_agents_response",
				"payload": map[string]interface{}{
					"requestId": reqPeek.RequestID,
					"entries":   []map[string]interface{}{},
					"pageInfo":  map[string]interface{}{"hasMore": false},
				},
			}
		}

		_ = conn.WriteJSON(protocol.WSOutboundMessage{
			Type:    "session",
			Message: resp,
		})
	}
}

func setupEnhancedCLI(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	t.Cleanup(func() { os.Unsetenv("SOLO_HOME") })

	oldStdout := cmdStdout
	oldStderr := cmdStderr
	var outBuf, errBuf bytes.Buffer
	cmdStdout = &outBuf
	cmdStderr = &errBuf
	t.Cleanup(func() {
		cmdStdout = oldStdout
		cmdStderr = oldStderr
	})

	mock := newEnhancedMockDaemon()
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	t.Cleanup(srv.Close)

	flagHost = srv.Listener.Addr().String()
	return &outBuf, &errBuf
}

func TestRunAgentArchive(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentArchiveForce = false

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentArchive(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentArchive failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "archived") {
		t.Errorf("expected 'archived' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentDelete_Single(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentDeleteAll = false
	agentDeleteCwd = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentDelete(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentDelete failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "deleted") {
		t.Errorf("expected 'deleted' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentDelete_MissingID(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	agentDeleteAll = false
	agentDeleteCwd = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runAgentDelete(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
	ce, ok := err.(*output.CommandError)
	if !ok || ce.Code != "MISSING_ID" {
		t.Errorf("expected MISSING_ID, got %v", err)
	}
}

func TestRunAgentDelete_All(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentDeleteAll = true
	agentDeleteCwd = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentDelete(cmd, []string{}); err != nil {
		t.Fatalf("runAgentDelete --all failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "Deleted") {
		t.Errorf("expected 'Deleted' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentStop_Single(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentStopAll = false
	agentStopCwd = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentStop(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentStop failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentStop_MissingID(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	agentStopAll = false
	agentStopCwd = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runAgentStop(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestRunAgentInspect(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentInspect(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentInspect failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "ID") {
		t.Errorf("expected 'ID' in output, got: %q", outBuf.String())
	}
}

func TestRunDaemonStop(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runDaemonStop(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonStop failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %q", outBuf.String())
	}
}

func TestRunDaemonRestart(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runDaemonRestart(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonRestart failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "restarting") {
		t.Errorf("expected 'restarting' in output, got: %q", outBuf.String())
	}
}

func TestRunProviderModels(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderModels(cmd, []string{"openai"}); err != nil {
		t.Fatalf("runProviderModels failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "gpt-4") {
		t.Errorf("expected 'gpt-4' in output, got: %q", outBuf.String())
	}
}

func TestRunProviderModels_NotFound(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runProviderModels(cmd, []string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestRunProviderModels_JSON(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "json"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderModels(cmd, []string{"openai"}); err != nil {
		t.Fatalf("runProviderModels failed: %v", err)
	}
	var result []interface{}
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON array: %v\n%s", err, outBuf.String())
	}
}

func TestRunAgentSend(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentSendNoWait = false
	agentSendImage = nil

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentSend(cmd, []string{"agent-abc", "hello"}); err != nil {
		t.Fatalf("runAgentSend failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "sent") {
		t.Errorf("expected 'sent' in output, got: %q", outBuf.String())
	}
}

func TestMakeAgentShortcut(t *testing.T) {
	sc := makeAgentShortcut("ls", "List agents", agentLsCmd)
	if sc == nil {
		t.Fatal("expected non-nil shortcut")
	}
	if sc.Use != agentLsCmd.Use {
		t.Error("expected Use to match source")
	}
	if sc.Short != "List agents" {
		t.Errorf("expected short description, got %q", sc.Short)
	}
}

func TestMakeDaemonShortcut(t *testing.T) {
	sc := makeDaemonShortcut("status", "Show status", daemonStatusCmd)
	if sc == nil {
		t.Fatal("expected non-nil shortcut")
	}
	if sc.Use != daemonStatusCmd.Use {
		t.Error("expected Use to match source")
	}
}


func TestMatchesLogFilter(t *testing.T) {
	if !matchesLogFilter("tool_call", "tools") {
		t.Error("expected tool_call to match tools filter")
	}
	if !matchesLogFilter("user_message", "text") {
		t.Error("expected user_message to match text filter")
	}
	if !matchesLogFilter("assistant_message", "text") {
		t.Error("expected assistant_message to match text filter")
	}
	if !matchesLogFilter("reasoning", "text") {
		t.Error("expected reasoning to match text filter")
	}
	if !matchesLogFilter("error", "errors") {
		t.Error("expected error to match errors filter")
	}
	if matchesLogFilter("tool_call", "errors") {
		t.Error("expected tool_call not to match errors filter")
	}
	if !matchesLogFilter("anything", "") {
		t.Error("expected empty filter to match everything")
	}
	if !matchesLogFilter("custom_type", "custom_type") {
		t.Error("expected exact type match")
	}
}

func TestPrintLogEntry(t *testing.T) {
	buf := captureOutput(t)
	printLogEntry("assistant_message", "hello", "")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected 'hello', got %q", buf.String())
	}

	buf.Reset()
	printLogEntry("reasoning", "thinking", "")
	if !strings.Contains(buf.String(), "[Reasoning]") {
		t.Errorf("expected [Reasoning], got %q", buf.String())
	}

	buf.Reset()
	printLogEntry("tool_call", "", "search")
	if !strings.Contains(buf.String(), "[Tool: search]") {
		t.Errorf("expected [Tool: search], got %q", buf.String())
	}

	buf.Reset()
	printLogEntry("error", "oops", "")
	if !strings.Contains(buf.String(), "[Error]") {
		t.Errorf("expected [Error], got %q", buf.String())
	}

	buf.Reset()
	printLogEntry("user_message", "hi", "")
	if !strings.Contains(buf.String(), "[User]") {
		t.Errorf("expected [User], got %q", buf.String())
	}

	buf.Reset()
	printLogEntry("unknown", "x", "")
	if !strings.Contains(buf.String(), "[unknown]") {
		t.Errorf("expected [unknown], got %q", buf.String())
	}
}

func TestPrintWaitResult(t *testing.T) {
	buf := captureOutput(t)
	flagFormat = "table"
	printWaitResult("agent-123", "idle")
	if !strings.Contains(buf.String(), "idle") {
		t.Errorf("expected 'idle', got %q", buf.String())
	}
}

func TestPrintTimelineItem(t *testing.T) {
	buf := captureOutput(t)
	printTimelineItem("assistant_message", "hello", "")
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}

	buf.Reset()
	printTimelineItem("tool_call", "", "search")
	if !strings.Contains(buf.String(), "[Tool: search]") {
		t.Errorf("expected [Tool: search], got %q", buf.String())
	}

	buf.Reset()
	printTimelineItem("error", "oops", "")
	if !strings.Contains(buf.String(), "[Error]") {
		t.Errorf("expected [Error], got %q", buf.String())
	}
}

func TestPrintStreamEvent(t *testing.T) {
	buf := captureOutput(t)
	printStreamEvent(map[string]interface{}{"type": "timeline", "item": map[string]interface{}{"type": "assistant_message", "text": "hi"}})
	if buf.String() != "hi" {
		t.Errorf("expected 'hi', got %q", buf.String())
	}

	buf.Reset()
	printStreamEvent(map[string]interface{}{"type": "permission_requested"})
	if !strings.Contains(buf.String(), "[Permission Required]") {
		t.Errorf("expected permission required, got %q", buf.String())
	}

	buf.Reset()
	printStreamEvent(map[string]interface{}{"type": "turn_failed"})
	if !strings.Contains(buf.String(), "[Turn Failed]") {
		t.Errorf("expected turn failed, got %q", buf.String())
	}

	buf.Reset()
	printStreamEvent(map[string]interface{}{"type": "attention_required"})
	if !strings.Contains(buf.String(), "[Attention Required]") {
		t.Errorf("expected attention required, got %q", buf.String())
	}
}

func TestRunAgentLogs(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLogsFollow = false
	agentLogsTail = 0
	agentLogsFilter = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLogs(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentLogs failed: %v", err)
	}
	// Timeline is empty in mock, so should print "No activity to display."
	if !strings.Contains(outBuf.String(), "No activity") {
		t.Errorf("expected 'No activity' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentLogs_WithFilter(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLogsFollow = false
	agentLogsTail = 0
	agentLogsFilter = "tools"

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLogs(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentLogs failed: %v", err)
	}
}

func TestStopMultipleAgents(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentStopAll = true
	agentStopCwd = ""

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}
	defer c.Close()

	if err := stopMultipleAgents(ctx, c); err != nil {
		t.Fatalf("stopMultipleAgents failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "Stopped") {
		t.Errorf("expected 'Stopped' in output, got: %q", outBuf.String())
	}
}

func captureOutput(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	oldStdout := cmdStdout
	oldStderr := cmdStderr
	cmdStdout = &buf
	cmdStderr = &buf
	t.Cleanup(func() {
		cmdStdout = oldStdout
		cmdStderr = oldStderr
	})
	return &buf
}


func TestRunAgentWait_AlreadyIdle(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentWaitTimeout = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentWait(cmd, []string{"agent-idle456"}); err != nil {
		t.Fatalf("runAgentWait failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "idle") {
		t.Errorf("expected 'idle' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentWait_WithTimeout(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	agentWaitTimeout = "1s"

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	// agent-abc123 is running, so it will wait and then timeout
	err := runAgentWait(cmd, []string{"agent-abc123"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	ce, ok := err.(*output.CommandError)
	if !ok || ce.Code != "TIMEOUT" {
		t.Errorf("expected TIMEOUT error, got %v", err)
	}
}

func TestParseAgentCreatedResponse_Success(t *testing.T) {
	resp := &protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type": "agent_created",
			"payload": map[string]interface{}{
				"agentId": "agent-new",
				"status":  "initializing",
			},
		},
	}
	id, err := parseAgentCreatedResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "agent-new" {
		t.Errorf("expected agent-new, got %q", id)
	}
}

func TestParseAgentCreatedResponse_RPCError(t *testing.T) {
	resp := &protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type": "rpc_error",
			"payload": map[string]interface{}{
				"error": "provider not ready",
			},
		},
	}
	_, err := parseAgentCreatedResponse(resp)
	if err == nil {
		t.Fatal("expected error for rpc_error")
	}
}

func TestParseAgentCreatedResponse_Unexpected(t *testing.T) {
	resp := &protocol.WSOutboundMessage{
		Type:    "session",
		Message: map[string]interface{}{"type": "something_else"},
	}
	_, err := parseAgentCreatedResponse(resp)
	if err == nil {
		t.Fatal("expected error for unexpected response")
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Error("expected nil for empty string")
	}
	p := strPtr("hello")
	if p == nil || *p != "hello" {
		t.Error("expected pointer to hello")
	}
}

func TestIntPtr(t *testing.T) {
	p := intPtr(42)
	if p == nil || *p != 42 {
		t.Error("expected pointer to 42")
	}
}

func TestAgentCountRequest_MsgType(t *testing.T) {
	req := &agentCountRequest{}
	if req.MsgType() != "fetch_agents_request" {
		t.Errorf("expected fetch_agents_request, got %q", req.MsgType())
	}
}

func TestRunDaemonPair(t *testing.T) {
	outBuf := captureOutput(t)

	// Set up a temp home with server-id and config
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "server-id"), []byte("test-server-id\n"), 0644)
	cfg := map[string]interface{}{
		"daemon": map[string]interface{}{
			"relay": map[string]interface{}{
				"enabled": true,
				"endpoint": "wss://relay.example.com",
				"publicEndpoint": "wss://relay.example.com",
			},
		},
		"app": map[string]interface{}{
			"baseUrl": "https://app.example.com",
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runDaemonPair(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonPair failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "Scan to pair") {
		t.Errorf("expected 'Scan to pair' in output, got: %q", outBuf.String())
	}
}

func TestRunDaemonPair_JSON(t *testing.T) {
	outBuf := captureOutput(t)

	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "server-id"), []byte("test-server-id\n"), 0644)
	cfg := map[string]interface{}{}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	flagFormat = "json"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runDaemonPair(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonPair failed: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON output: %v\n%s", err, outBuf.String())
	}
}

func TestRunDaemonPair_RelayDisabled(t *testing.T) {
	outBuf := captureOutput(t)

	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "server-id"), []byte("test-server-id\n"), 0644)
	cfg := map[string]interface{}{
		"daemon": map[string]interface{}{
			"relay": map[string]interface{}{
				"enabled": false,
			},
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runDaemonPair(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when relay disabled")
	}
	if !strings.Contains(outBuf.String(), "") {
		// error is returned, stdout may be empty
	}
}


func TestRunAgentMode_List(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentModeList = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentMode(cmd, []string{"agent-abc"}); err != nil {
		t.Fatalf("runAgentMode --list failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "No modes available") {
		t.Errorf("expected 'No modes available' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentMode_Set(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentModeList = false

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentMode(cmd, []string{"agent-abc", "code"}); err != nil {
		t.Fatalf("runAgentMode set failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "mode set") {
		t.Errorf("expected 'mode set' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentMode_MissingMode(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	agentModeList = false

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runAgentMode(cmd, []string{"agent-abc"})
	if err == nil {
		t.Fatal("expected error for missing mode")
	}
}

func TestRunAgentLs(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLsAll = false
	agentLsStatus = ""
	agentLsCwd = ""
	agentLsLabel = nil
	agentLsThinking = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLs(cmd, []string{}); err != nil {
		t.Fatalf("runAgentLs failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "running") {
		t.Errorf("expected 'running' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentLs_All(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLsAll = true
	agentLsStatus = ""
	agentLsCwd = ""
	agentLsLabel = nil
	agentLsThinking = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLs(cmd, []string{}); err != nil {
		t.Fatalf("runAgentLs failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "running") {
		t.Errorf("expected output, got: %q", outBuf.String())
	}
}

func TestRunAgentLs_WithCwdFilter(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLsAll = false
	agentLsStatus = ""
	agentLsCwd = "/tmp"
	agentLsLabel = nil
	agentLsThinking = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLs(cmd, []string{}); err != nil {
		t.Fatalf("runAgentLs failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "running") {
		t.Errorf("expected output, got: %q", outBuf.String())
	}
}

func TestRunAgentLs_WithStatusFilter(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLsAll = false
	agentLsStatus = "running"
	agentLsCwd = ""
	agentLsLabel = nil
	agentLsThinking = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLs(cmd, []string{}); err != nil {
		t.Fatalf("runAgentLs failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "running") {
		t.Errorf("expected output, got: %q", outBuf.String())
	}
}

func TestRunAgentLs_Quiet(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "quiet"
	flagJSON = false
	flagQuiet = true
	flagNoHeaders = false
	flagNoColor = true
	agentLsAll = false
	agentLsStatus = ""
	agentLsCwd = ""
	agentLsLabel = nil
	agentLsThinking = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentLs(cmd, []string{}); err != nil {
		t.Fatalf("runAgentLs failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) == 0 {
		t.Errorf("expected IDs in quiet output, got: %q", outBuf.String())
	}
}


func TestRunAgentRun_Detach(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentRunDetach = true
	agentRunProvider = "openai"
	agentRunModel = ""
	agentRunMode = ""
	agentRunTitle = ""
	agentRunCwd = ""
	agentRunLabel = nil
	agentRunTimeout = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentRun(cmd, []string{"write a test"}); err != nil {
		t.Fatalf("runAgentRun detached failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "created") {
		t.Errorf("expected 'created' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentRun_Foreground(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentRunDetach = false
	agentRunProvider = "openai"
	agentRunModel = ""
	agentRunMode = ""
	agentRunTitle = ""
	agentRunCwd = ""
	agentRunLabel = nil
	agentRunTimeout = ""

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runAgentRun(cmd, []string{"write a test"}); err != nil {
		t.Fatalf("runAgentRun foreground failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "completed") {
		t.Errorf("expected 'completed' in output, got: %q", outBuf.String())
	}
}

func TestRunAgentRun_MissingPrompt(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	agentRunDetach = true
	agentRunProvider = "openai"

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runAgentRun(cmd, []string{""})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestMapRunStatus(t *testing.T) {
	if got := mapRunStatus("idle"); got != "completed" {
		t.Errorf("expected completed, got %q", got)
	}
	if got := mapRunStatus("error"); got != "error" {
		t.Errorf("expected error, got %q", got)
	}
}

func TestRenderRunResult(t *testing.T) {
	buf := captureOutput(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	agent := &protocol.AgentSnapshotPayload{
		ID:       "agent-123",
		Provider: "openai",
		Status:   protocol.AgentIdle,
		Cwd:      "/tmp",
	}
	if err := renderRunResult(agent, "completed"); err != nil {
		t.Fatalf("renderRunResult failed: %v", err)
	}
	if !strings.Contains(buf.String(), "agent-123") {
		t.Errorf("expected agent ID in output, got: %q", buf.String())
	}
}

func TestRenderRunResult_NilAgent(t *testing.T) {
	err := renderRunResult(nil, "completed")
	if err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestRequestWaitForFinish(t *testing.T) {
	_, _ = setupEnhancedCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}
	defer c.Close()

	req := &protocol.WaitForFinishRequest{
		Type:    "wait_for_finish_request",
		AgentID: "agent-new789",
	}
	result := requestWaitForFinish(ctx, c, req)
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.status != "idle" {
		t.Errorf("expected idle status, got %q", result.status)
	}
}

func TestWaitForAgentFinish_InvalidTimeout(t *testing.T) {
	agentRunTimeout = "invalid"
	result := waitForAgentFinish(t.Context(), nil, "agent-123")
	if result.err == nil {
		t.Fatal("expected error for invalid timeout")
	}
	agentRunTimeout = ""
}


func TestResolveDaemonHost(t *testing.T) {
	daemonStartPort = "9090"
	if got := resolveDaemonHost(); got != "127.0.0.1:9090" {
		t.Errorf("expected 127.0.0.1:9090, got %q", got)
	}
	daemonStartPort = ""
}

func TestWaitForDaemon_Timeout(t *testing.T) {
	err := waitForDaemon("127.0.0.1:1", 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestFindDaemonBinary_NotFound(t *testing.T) {
	// Save and clear PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", oldPath)

	_, err := findDaemonBinary()
	if err == nil {
		t.Error("expected error when binary not found")
	}
}

func TestRunAgentAttach(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	err := runAgentAttach(cmd, []string{"agent-abc"})
	// Should return ctx.Err() when context times out during stream loop
	if err == nil {
		t.Logf("runAgentAttach returned nil, output: %q", outBuf.String())
	}
}

func TestRunAgentLogs_Follow(t *testing.T) {
	outBuf, _ := setupEnhancedCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true
	agentLogsFollow = true
	agentLogsTail = 0
	agentLogsFilter = ""

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	err := runAgentLogs(cmd, []string{"agent-abc"})
	if err == nil {
		t.Logf("runAgentLogs follow returned nil, output: %q", outBuf.String())
	}
}
