package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// --- File Explorer Tests ---

func TestFileExplorerListDirectory(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-file-explorer-list")
	defer conn.Close()
	readInitialMessages(t, conn)

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "data.bin"), []byte{0x00, 0x01, 0x02}, 0644)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "file_explorer_request",
			"requestId": "req-fe-list-1",
			"cwd":       tmpDir,
			"mode":      "list",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write file_explorer: %v", err)
	}

	resp := readUntilType(t, conn, "file_explorer_response")
	payload := decodeSessionPayload[protocol.FileExplorerResponsePayload](t, resp)
	if payload.RequestID != "req-fe-list-1" {
		t.Fatalf("request ID: got %q, want req-fe-list-1", payload.RequestID)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Directory == nil {
		t.Fatal("expected directory in response")
	}

	names := make(map[string]bool)
	for _, e := range payload.Directory.Entries {
		names[e.Name] = true
		if e.Kind != "file" && e.Kind != "directory" {
			t.Errorf("entry %q: unexpected kind %q", e.Name, e.Kind)
		}
	}
	if !names["hello.txt"] {
		t.Error("expected hello.txt in directory listing")
	}
	if !names["subdir"] {
		t.Error("expected subdir in directory listing")
	}
	if !names["data.bin"] {
		t.Error("expected data.bin in directory listing")
	}
}

func TestFileExplorerReadTextFile(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-file-explorer-read")
	defer conn.Close()
	readInitialMessages(t, conn)

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0644)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "file_explorer_request",
			"requestId": "req-fe-file-1",
			"cwd":       tmpDir,
			"path":      filepath.Join(tmpDir, "test.txt"),
			"mode":      "file",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write file_explorer: %v", err)
	}

	resp := readUntilType(t, conn, "file_explorer_response")
	payload := decodeSessionPayload[protocol.FileExplorerResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.File == nil {
		t.Fatal("expected file in response")
	}
	if payload.File.Kind != "text" {
		t.Errorf("file kind: got %q, want text", payload.File.Kind)
	}
	if payload.File.Content == nil || *payload.File.Content != "hello world" {
		t.Errorf("file content: got %v, want 'hello world'", payload.File.Content)
	}
}

func TestFileExplorerMissingCwdReturnsError(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-file-explorer-no-cwd")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "file_explorer_request",
			"requestId": "req-fe-no-cwd",
			"cwd":       "",
			"mode":      "list",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write file_explorer: %v", err)
	}

	resp := readUntilType(t, conn, "file_explorer_response")
	payload := decodeSessionPayload[protocol.FileExplorerResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error for missing cwd")
	}
}

// --- Terminal Tests ---

func TestListTerminalsReturnsEmpty(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-list-terminals")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "list_terminals_request",
			"requestId": "req-lt-1",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write list_terminals: %v", err)
	}

	resp := readUntilType(t, conn, "list_terminals_response")
	payload := decodeSessionPayload[protocol.ListTerminalsPayload](t, resp)
	if payload.RequestID != "req-lt-1" {
		t.Fatalf("request ID: got %q, want req-lt-1", payload.RequestID)
	}
}

func TestCreateAndKillTerminal(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-create-kill-terminal")
	defer conn.Close()
	readInitialMessages(t, conn)

	tmpDir := t.TempDir()

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_terminal_request",
			"requestId": "req-ct-1",
			"cwd":       tmpDir,
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_terminal: %v", err)
	}

	resp := readUntilType(t, conn, "create_terminal_response")
	payload := decodeSessionPayload[protocol.CreateTerminalPayload](t, resp)
	if payload.RequestID != "req-ct-1" {
		t.Fatalf("request ID: got %q, want req-ct-1", payload.RequestID)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Terminal == nil {
		t.Fatal("expected terminal in response")
	}
	if payload.Terminal.ID == "" {
		t.Error("expected non-empty terminal ID")
	}
	terminalID := payload.Terminal.ID

	killReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "kill_terminal_request",
			"requestId":  "req-kt-1",
			"terminalId": terminalID,
		}),
	}
	if err := conn.WriteJSON(killReq); err != nil {
		t.Fatalf("write kill_terminal: %v", err)
	}
	drainMessagesWithTimeout(t, conn, 500*time.Millisecond)
}

// --- Daemon Config Test ---

func TestGetDaemonConfig(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-get-config")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "get_daemon_config_request",
			"requestId": "req-gdc-1",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write get_daemon_config: %v", err)
	}

	resp := readUntilType(t, conn, "get_daemon_config_response")
	payload := decodeSessionPayload[protocol.GetDaemonConfigResponsePayload](t, resp)
	if payload.RequestID != "req-gdc-1" {
		t.Fatalf("request ID: got %q, want req-gdc-1", payload.RequestID)
	}
	if payload.Config == nil {
		t.Error("expected non-nil config")
	}
}

// --- ListAvailableEditors Test ---

func TestListAvailableEditors(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-list-editors")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "list_available_editors_request",
			"requestId": "req-lae-1",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write list_available_editors: %v", err)
	}

	resp := readUntilType(t, conn, "list_available_editors_response")
	var raw struct {
		Payload struct {
			RequestID string        `json:"requestId"`
			Editors   []interface{} `json:"editors"`
		} `json:"payload"`
	}
	msgBytes, _ := json.Marshal(resp.Message)
	if err := json.Unmarshal(msgBytes, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw.Payload.RequestID != "req-lae-1" {
		t.Fatalf("request ID: got %q, want req-lae-1", raw.Payload.RequestID)
	}
}

// --- DirectorySuggestions Test ---

func TestDirectorySuggestions(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-dir-suggestions")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "directory_suggestions_request",
			"requestId": "req-ds-1",
			"query":     "/tmp",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write directory_suggestions: %v", err)
	}

	resp := readUntilType(t, conn, "directory_suggestions_response")
	var raw struct {
		Payload struct {
			RequestID   string   `json:"requestId"`
			Directories []string `json:"directories"`
		} `json:"payload"`
	}
	msgBytes, _ := json.Marshal(resp.Message)
	if err := json.Unmarshal(msgBytes, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw.Payload.RequestID != "req-ds-1" {
		t.Fatalf("request ID: got %q, want req-ds-1", raw.Payload.RequestID)
	}
}

// --- Agent Attribute Tests ---
// We inline the create agent flow (no drain) to avoid consuming response messages.

func TestSetAgentMode(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-set-mode")
	defer conn.Close()
	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "test-project")
	// Create agent inline to avoid drain consuming responses
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-set-mode",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create: %v", err)
	}
	createdResp := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, createdResp)
	agentID := createdPayload.AgentID

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "set_agent_mode_request",
			"requestId": "req-sam-1",
			"agentId":   agentID,
			"modeId":    "plan",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write set_agent_mode: %v", err)
	}

	resp := readUntilType(t, conn, "set_agent_mode_response")
	payload := decodeSessionPayload[protocol.SetAgentModeResponsePayload](t, resp)
	if payload.RequestID != "req-sam-1" {
		t.Fatalf("request ID: got %q, want req-sam-1", payload.RequestID)
	}
	if !payload.Accepted {
		t.Error("expected accepted=true")
	}
}

func TestCancelAgent(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-cancel-agent")
	defer conn.Close()
	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "test-project")
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-cancel",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create: %v", err)
	}
	createdResp := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, createdResp)
	agentID := createdPayload.AgentID

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "cancel_agent_request",
			"requestId": "req-ca-1",
			"agentId":   agentID,
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write cancel_agent: %v", err)
	}

	resp := readUntilType(t, conn, "cancel_agent_response")
	payload := decodeSessionPayload[protocol.CancelAgentResponsePayload](t, resp)
	if payload.RequestID != "req-ca-1" {
		t.Fatalf("request ID: got %q, want req-ca-1", payload.RequestID)
	}
}

func TestArchiveAgent(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-archive-agent")
	defer conn.Close()
	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "test-project")
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-archive",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create: %v", err)
	}
	createdResp := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, createdResp)
	agentID := createdPayload.AgentID

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "archive_agent_request",
			"requestId": "req-aa-1",
			"agentId":   agentID,
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write archive_agent: %v", err)
	}

	readUntilType(t, conn, "agent_archived")
}
