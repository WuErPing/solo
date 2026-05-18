package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// readSessionMessages reads messages from the WS connection until it finds one
// with the specified inner message type, or times out.
func readUntilType(t *testing.T, conn *websocket.Conn, targetType string) protocol.WSOutboundMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read error looking for %q: %v", targetType, err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		if resp.Type == targetType {
			return resp
		}
		// Also check inside session messages
		if resp.Type == "session" {
			msgBytes, _ := json.Marshal(resp.Message)
			var peek struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msgBytes, &peek) == nil && peek.Type == targetType {
				return resp
			}
			// Check payload.status for status messages
			var statusPeek struct {
				Payload struct {
					Status string `json:"status"`
				} `json:"payload"`
			}
			if json.Unmarshal(msgBytes, &statusPeek) == nil && statusPeek.Payload.Status == targetType {
				return resp
			}
		}
	}
}

// drainInitialMessages reads and discards messages sent right after hello.
func drainInitialMessages(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
	conn.SetReadDeadline(time.Time{})
}

// readInitialMessages reads the 2 expected initial messages (server_info + providers_snapshot_update).
func readInitialMessages(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for i := 0; i < 2; i++ {
		_, _, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read initial message %d: %v", i, err)
		}
	}
	conn.SetReadDeadline(time.Time{})
}

// drainMessagesWithTimeout reads all pending messages until timeout.
func drainMessagesWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
	conn.SetReadDeadline(time.Time{})
}

func dialAndHello(t *testing.T, tsURL string, clientID string) *websocket.Conn {
	return dialAndHelloAs(t, tsURL, clientID, protocol.ClientCLI)
}

func dialAndHelloAs(t *testing.T, tsURL string, clientID string, clientType protocol.ClientType) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(tsURL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        clientID,
		ClientType:      clientType,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	return conn
}

func TestCreateAgentViaWebSocket(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-create-agent")
	defer conn.Close()

	// Read server_info + providers_snapshot
	readInitialMessages(t, conn)

	// Create agent
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-1",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      "/tmp/test-project",
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	// Read messages until we get agent_created
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var foundCreated, foundUpdate bool
	for !foundCreated || !foundUpdate {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				Status string `json:"status"`
				Kind   string `json:"kind"`
				Agent  struct {
					ID       string `json:"id"`
					Provider string `json:"provider"`
				} `json:"agent"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) != nil {
			continue
		}

		if peek.Payload.Status == "agent_created" {
			foundCreated = true
			if peek.Payload.Agent.Provider != "mock" {
				t.Errorf("agent provider: got %q, want mock", peek.Payload.Agent.Provider)
			}
			if peek.Payload.Agent.ID == "" {
				t.Error("expected non-empty agent ID")
			}
		}
		if peek.Type == "agent_update" && peek.Payload.Kind == "upsert" {
			foundUpdate = true
		}
	}

	if !foundCreated {
		t.Error("expected agent_created message")
	}
	if !foundUpdate {
		t.Error("expected agent_update message")
	}
}

func TestCreateAgentRegistersWorkspaceForCwd(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-create-agent-workspace")
	defer conn.Close()

	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "test-project")
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-workspace",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	payload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	defer deleteAgentForTest(t, conn, payload.AgentID)

	workspaceUpdate := readUntilType(t, conn, "workspace_update")
	updatePayload := decodeSessionPayload[protocol.WorkspaceUpdatePayload](t, workspaceUpdate)
	if updatePayload.Kind != "upsert" {
		t.Fatalf("workspace update kind: got %q, want upsert", updatePayload.Kind)
	}
	if updatePayload.Workspace == nil {
		t.Fatal("workspace update: got nil workspace")
	}
	if updatePayload.Workspace.ID != cwd {
		t.Fatalf("workspace ID: got %q, want %q", updatePayload.Workspace.ID, cwd)
	}
	if updatePayload.Workspace.WorkspaceDirectory != cwd {
		t.Fatalf("workspace directory: got %q, want %q", updatePayload.Workspace.WorkspaceDirectory, cwd)
	}

	fetchReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_workspaces_request",
			"requestId": "req-fetch-workspaces-after-create",
		}),
	}
	if err := conn.WriteJSON(fetchReq); err != nil {
		t.Fatalf("write fetch_workspaces: %v", err)
	}

	workspaces := readUntilType(t, conn, "fetch_workspaces_response")
	fetchPayload := decodeSessionPayload[protocol.FetchWorkspacesResponsePayload](t, workspaces)
	if fetchPayload.RequestID != "req-fetch-workspaces-after-create" {
		t.Fatalf("fetch request ID: got %q", fetchPayload.RequestID)
	}
	found := false
	for _, entry := range fetchPayload.Entries {
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal workspace entry: %v", err)
		}
		var ws protocol.WorkspaceDescriptor
		if err := json.Unmarshal(raw, &ws); err != nil {
			t.Fatalf("decode workspace entry: %v", err)
		}
		if ws.ID == cwd && ws.WorkspaceDirectory == cwd {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("fetch_workspaces did not include created agent cwd %q", cwd)
	}
}

func TestCreateAgentBroadcastsWorkspaceToExistingSession(t *testing.T) {
	_, ts := newTestWSServer(t)
	webConn := dialAndHello(t, ts.URL, "test-web-workspace-sync")
	defer webConn.Close()
	cliConn := dialAndHello(t, ts.URL, "test-cli-workspace-sync")
	defer cliConn.Close()

	readInitialMessages(t, webConn)
	readInitialMessages(t, cliConn)

	cwd := filepath.Join(t.TempDir(), "cli-created-project")
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-broadcast-workspace",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := cliConn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, cliConn, "agent_created")
	payload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	defer deleteAgentForTest(t, cliConn, payload.AgentID)

	workspaceUpdate := readUntilType(t, webConn, "workspace_update")
	updatePayload := decodeSessionPayload[protocol.WorkspaceUpdatePayload](t, workspaceUpdate)
	if updatePayload.Kind != "upsert" {
		t.Fatalf("workspace update kind: got %q, want upsert", updatePayload.Kind)
	}
	if updatePayload.Workspace == nil {
		t.Fatal("workspace update: got nil workspace")
	}
	if updatePayload.Workspace.ID != cwd {
		t.Fatalf("workspace ID: got %q, want %q", updatePayload.Workspace.ID, cwd)
	}

	fetchReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_workspaces_request",
			"requestId": "req-fetch-web-after-cli-create",
		}),
	}
	if err := webConn.WriteJSON(fetchReq); err != nil {
		t.Fatalf("write fetch_workspaces: %v", err)
	}

	workspaces := readUntilType(t, webConn, "fetch_workspaces_response")
	fetchPayload := decodeSessionPayload[protocol.FetchWorkspacesResponsePayload](t, workspaces)
	found := false
	for _, entry := range fetchPayload.Entries {
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal workspace entry: %v", err)
		}
		var ws protocol.WorkspaceDescriptor
		if err := json.Unmarshal(raw, &ws); err != nil {
			t.Fatalf("decode workspace entry: %v", err)
		}
		if ws.ID == cwd && ws.WorkspaceDirectory == cwd {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("web fetch_workspaces did not include CLI-created cwd %q", cwd)
	}
}

func TestReadProjectConfigReturnsSoloCompatibleMissingResponse(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-read-project-config-missing")
	defer conn.Close()

	readInitialMessages(t, conn)

	repoRoot := t.TempDir()
	canonicalRepoRoot := canonicalizeConfigRoot(repoRoot)
	openReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "open_project_request",
			"requestId": "req-open-config-missing",
			"cwd":       repoRoot,
		}),
	}
	if err := conn.WriteJSON(openReq); err != nil {
		t.Fatalf("write open_project: %v", err)
	}
	readUntilType(t, conn, "workspace_update")
	readUntilType(t, conn, "open_project_response")

	readReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "read_project_config_request",
			"requestId": "req-read-config-missing",
			"repoRoot":  repoRoot,
		}),
	}
	if err := conn.WriteJSON(readReq); err != nil {
		t.Fatalf("write read_project_config: %v", err)
	}

	resp := readUntilType(t, conn, "read_project_config_response")
	payload := decodeSessionPayload[protocol.ReadProjectConfigResponsePayload](t, resp)
	if payload.RequestID != "req-read-config-missing" {
		t.Fatalf("request ID: got %q", payload.RequestID)
	}
	if payload.RepoRoot != canonicalRepoRoot {
		t.Fatalf("repoRoot: got %q, want %q", payload.RepoRoot, canonicalRepoRoot)
	}
	if !payload.OK {
		t.Fatalf("ok: got false, error %#v", payload.Error)
	}
	if payload.Config != nil {
		t.Fatalf("config: got %#v, want nil for missing solo.json", payload.Config)
	}
	if payload.Revision != nil {
		t.Fatalf("revision: got %#v, want nil for missing solo.json", payload.Revision)
	}
}

func TestReadProjectConfigReturnsConfigAndRevision(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-read-project-config-present")
	defer conn.Close()

	readInitialMessages(t, conn)

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "solo.json")
	configJSON := `{"worktree":{"setup":"npm install"},"scripts":{"dev":{"type":"service","command":"npm run dev","port":3000}},"customTopLevel":"preserved"}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("write solo.json: %v", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat solo.json: %v", err)
	}

	openReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "open_project_request",
			"requestId": "req-open-config-present",
			"cwd":       repoRoot,
		}),
	}
	if err := conn.WriteJSON(openReq); err != nil {
		t.Fatalf("write open_project: %v", err)
	}
	readUntilType(t, conn, "workspace_update")
	readUntilType(t, conn, "open_project_response")

	readReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "read_project_config_request",
			"requestId": "req-read-config-present",
			"repoRoot":  repoRoot,
		}),
	}
	if err := conn.WriteJSON(readReq); err != nil {
		t.Fatalf("write read_project_config: %v", err)
	}

	resp := readUntilType(t, conn, "read_project_config_response")
	payload := decodeSessionPayload[protocol.ReadProjectConfigResponsePayload](t, resp)
	if !payload.OK {
		t.Fatalf("ok: got false, error %#v", payload.Error)
	}
	if payload.Config == nil {
		t.Fatal("config: got nil, want parsed solo.json")
	}
	if got := payload.Config["customTopLevel"]; got != "preserved" {
		t.Fatalf("customTopLevel: got %#v", got)
	}
	if payload.Revision == nil {
		t.Fatal("revision: got nil")
	}
	if payload.Revision.Size != info.Size() {
		t.Fatalf("revision size: got %d, want %d", payload.Revision.Size, info.Size())
	}
	if payload.Revision.MtimeMs <= 0 {
		t.Fatalf("revision mtimeMs: got %f, want positive", payload.Revision.MtimeMs)
	}
}

func TestWriteProjectConfigWritesAndDetectsStaleRevision(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-write-project-config")
	defer conn.Close()

	readInitialMessages(t, conn)

	repoRoot := t.TempDir()
	openReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "open_project_request",
			"requestId": "req-open-config-write",
			"cwd":       repoRoot,
		}),
	}
	if err := conn.WriteJSON(openReq); err != nil {
		t.Fatalf("write open_project: %v", err)
	}
	readUntilType(t, conn, "workspace_update")
	readUntilType(t, conn, "open_project_response")

	writeReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":             "write_project_config_request",
			"requestId":        "req-write-config",
			"repoRoot":         repoRoot,
			"config":           map[string]interface{}{"worktree": map[string]interface{}{"setup": []string{"npm install"}}, "customTopLevel": "preserved"},
			"expectedRevision": nil,
		}),
	}
	if err := conn.WriteJSON(writeReq); err != nil {
		t.Fatalf("write write_project_config: %v", err)
	}

	resp := readUntilType(t, conn, "write_project_config_response")
	payload := decodeSessionPayload[protocol.WriteProjectConfigResponsePayload](t, resp)
	if !payload.OK {
		t.Fatalf("ok: got false, error %#v", payload.Error)
	}
	if payload.Config == nil {
		t.Fatal("config: got nil after write")
	}
	if payload.Revision == nil {
		t.Fatal("revision: got nil after write")
	}
	written, err := os.ReadFile(filepath.Join(repoRoot, "solo.json"))
	if err != nil {
		t.Fatalf("read written solo.json: %v", err)
	}
	if !strings.Contains(string(written), `"customTopLevel": "preserved"`) {
		t.Fatalf("written config did not preserve custom key: %s", string(written))
	}

	staleReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "write_project_config_request",
			"requestId": "req-write-config-stale",
			"repoRoot":  repoRoot,
			"config":    map[string]interface{}{"worktree": map[string]interface{}{"setup": "go test ./..."}},
			"expectedRevision": map[string]interface{}{
				"mtimeMs": payload.Revision.MtimeMs - 1,
				"size":    payload.Revision.Size,
			},
		}),
	}
	if err := conn.WriteJSON(staleReq); err != nil {
		t.Fatalf("write stale write_project_config: %v", err)
	}

	staleResp := readUntilType(t, conn, "write_project_config_response")
	stalePayload := decodeSessionPayload[protocol.WriteProjectConfigResponsePayload](t, staleResp)
	if stalePayload.OK {
		t.Fatal("ok: got true for stale write, want false")
	}
	if stalePayload.Error == nil || stalePayload.Error.Code != "stale_project_config" {
		t.Fatalf("error: got %#v, want stale_project_config", stalePayload.Error)
	}
	if stalePayload.Error.CurrentRevision == nil {
		t.Fatal("currentRevision: got nil for stale write")
	}
}

func TestCreateAgentDerivesProvisionalTitleFromInitialPrompt(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-create-agent-title")
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "create_agent_request",
			"requestId":     "req-create-title",
			"initialPrompt": "Implement auth retries with backoff\n\ninclude tests",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      "/tmp/test-title-project",
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	payload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	readUntilType(t, conn, "agent_update")
	deleteAgentForTest(t, conn, payload.AgentID)
	if payload.Agent.Title == nil {
		t.Fatal("title: got nil, want provisional title")
	}
	if *payload.Agent.Title != "Implement auth retries with backoff" {
		t.Fatalf("title: got %q, want provisional title", *payload.Agent.Title)
	}
}

func TestCreateAgentPreservesExplicitTitle(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-create-agent-explicit-title")
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "create_agent_request",
			"requestId":     "req-create-explicit-title",
			"initialPrompt": "Ignored prompt title",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      "/tmp/test-explicit-title-project",
				"title":    "  Keep This Title  ",
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	payload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	readUntilType(t, conn, "agent_update")
	deleteAgentForTest(t, conn, payload.AgentID)
	if payload.Agent.Title == nil {
		t.Fatal("title: got nil, want explicit title")
	}
	if *payload.Agent.Title != "Keep This Title" {
		t.Fatalf("title: got %q, want explicit title", *payload.Agent.Title)
	}
}

func TestCreateAgentClassifiesIdentityPromptTitle(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-create-agent-identity-title")
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "create_agent_request",
			"requestId":     "req-create-identity-title",
			"initialPrompt": "who are you",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      "/tmp/test-identity-title-project",
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	payload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	readUntilType(t, conn, "agent_update")
	deleteAgentForTest(t, conn, payload.AgentID)
	if payload.Agent.Title == nil {
		t.Fatal("title: got nil, want identity title")
	}
	if *payload.Agent.Title != "Identity inquiry" {
		t.Fatalf("title: got %q, want Identity inquiry", *payload.Agent.Title)
	}
}

func TestFetchAgentsViaWebSocket(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-fetch-agents")
	defer conn.Close()

	// Read 2 initial messages: server_info + providers_snapshot_update
	readInitialMessages(t, conn)

	// Fetch agents (empty list expected)
	fetchReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agents_request",
			"requestId": "req-fetch-1",
		}),
	}
	if err := conn.WriteJSON(fetchReq); err != nil {
		t.Fatalf("write fetch_agents: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read fetch response: %v", err)
	}

	var resp protocol.WSOutboundMessage
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "session" {
		t.Errorf("expected session message, got %q", resp.Type)
	}
}

func TestFetchAgentsResponseIncludesProjectPlacement(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-fetch-agents-project")
	defer conn.Close()

	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "project")
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-project",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}
	readUntilType(t, conn, "agent_created")
	readUntilType(t, conn, "agent_update")

	fetchReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agents_request",
			"requestId": "req-fetch-project",
		}),
	}
	if err := conn.WriteJSON(fetchReq); err != nil {
		t.Fatalf("write fetch_agents: %v", err)
	}

	resp := readUntilType(t, conn, "fetch_agents_response")
	payload := decodeSessionPayload[protocol.FetchAgentsResponsePayload](t, resp)
	if len(payload.Entries) != 1 {
		t.Fatalf("entries length: got %d, want 1", len(payload.Entries))
	}
	entry := payload.Entries[0]
	if entry.Project.ProjectKey != cwd {
		t.Fatalf("projectKey: got %q, want %q", entry.Project.ProjectKey, cwd)
	}
	if entry.Project.ProjectName != filepath.Base(cwd) {
		t.Fatalf("projectName: got %q, want %q", entry.Project.ProjectName, filepath.Base(cwd))
	}
	if entry.Project.Checkout.Cwd != cwd {
		t.Fatalf("checkout.cwd: got %q, want %q", entry.Project.Checkout.Cwd, cwd)
	}
	if entry.Project.Checkout.IsGit {
		t.Fatal("checkout.isGit: got true, want false")
	}
	if entry.Project.Checkout.CurrentBranch != nil || entry.Project.Checkout.RemoteURL != nil || entry.Project.Checkout.MainRepoRoot != nil {
		t.Fatal("non-git checkout should expose null git fields")
	}
}

func TestFetchAgentHistoryResponseIncludesProjectPlacement(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-fetch-history-project")
	defer conn.Close()

	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "history-project")
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-history",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}
	readUntilType(t, conn, "agent_created")
	readUntilType(t, conn, "agent_update")

	historyReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agent_history_request",
			"requestId": "req-history-project",
		}),
	}
	if err := conn.WriteJSON(historyReq); err != nil {
		t.Fatalf("write fetch_agent_history: %v", err)
	}

	resp := readUntilType(t, conn, "fetch_agent_history_response")
	payload := decodeSessionPayload[protocol.FetchAgentHistoryResponsePayload](t, resp)
	if len(payload.Entries) != 1 {
		t.Fatalf("entries length: got %d, want 1", len(payload.Entries))
	}
	if payload.Entries[0].Project.ProjectKey != cwd {
		t.Fatalf("projectKey: got %q, want %q", payload.Entries[0].Project.ProjectKey, cwd)
	}
}

func TestRefreshAgentReturnsAgentRefreshedStatus(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-refresh-agent")
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-refresh",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      filepath.Join(t.TempDir(), "refresh-project"),
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}
	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	readUntilType(t, conn, "agent_update")

	refreshReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "refresh_agent_request",
			"requestId": "req-refresh",
			"agentId":   createdPayload.AgentID,
		}),
	}
	if err := conn.WriteJSON(refreshReq); err != nil {
		t.Fatalf("write refresh_agent: %v", err)
	}

	refreshed := readUntilType(t, conn, "agent_refreshed")
	payload := decodeStatusPayload[protocol.AgentRefreshedPayload](t, refreshed)
	if payload.Status != "agent_refreshed" {
		t.Fatalf("status: got %q, want agent_refreshed", payload.Status)
	}
	if payload.AgentID != createdPayload.AgentID {
		t.Fatalf("agentId: got %q, want %q", payload.AgentID, createdPayload.AgentID)
	}
	if payload.RequestID != "req-refresh" {
		t.Fatalf("requestId: got %q, want req-refresh", payload.RequestID)
	}
}

func TestWaitForFinishReturnsCompletedAgentSnapshot(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-wait-for-finish")
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "create_agent_request",
			"requestId":     "req-create-wait",
			"initialPrompt": "Hello from wait test",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      filepath.Join(t.TempDir(), "wait-project"),
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}
	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)

	waitReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "wait_for_finish_request",
			"requestId": "req-wait",
			"agentId":   createdPayload.AgentID,
			"timeoutMs": 5000,
		}),
	}
	if err := conn.WriteJSON(waitReq); err != nil {
		t.Fatalf("write wait_for_finish: %v", err)
	}

	resp := readUntilType(t, conn, "wait_for_finish_response")
	payload := decodeSessionPayload[protocol.WaitForFinishResponsePayload](t, resp)
	if payload.Status != "idle" {
		t.Fatalf("status: got %q, want idle", payload.Status)
	}
	if payload.Final == nil {
		t.Fatal("final: got nil, want agent snapshot")
	}
	if payload.Final.ID != createdPayload.AgentID {
		t.Fatalf("final.id: got %q, want %q", payload.Final.ID, createdPayload.AgentID)
	}
	if payload.LastMessage == nil || *payload.LastMessage != "Mock response to: Hello from wait test" {
		t.Fatalf("lastMessage: got %v, want mock response", payload.LastMessage)
	}
	deleteAgentForTest(t, conn, createdPayload.AgentID)
}

func TestClearAgentAttentionResponseAndUpdate(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-clear-attention")
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "create_agent_request",
			"requestId":     "req-create-clear-attention",
			"initialPrompt": "Please answer quickly",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      filepath.Join(t.TempDir(), "clear-attention-project"),
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}
	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	readUntilAgentUpdateAttention(t, conn, createdPayload.AgentID, true, "finished")

	clearReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "clear_agent_attention",
			"requestId": "req-clear-attention",
			"agentId":   createdPayload.AgentID,
		}),
	}
	if err := conn.WriteJSON(clearReq); err != nil {
		t.Fatalf("write clear_agent_attention: %v", err)
	}

	readUntilAgentUpdateAttention(t, conn, createdPayload.AgentID, false, "")
	resp := readUntilType(t, conn, "clear_agent_attention_response")
	payload := decodeSessionPayload[protocol.ClearAgentAttentionResponsePayload](t, resp)
	if payload.RequestID != "req-clear-attention" {
		t.Fatalf("requestId: got %q, want req-clear-attention", payload.RequestID)
	}
	if payload.AgentID != createdPayload.AgentID {
		t.Fatalf("agentId: got %#v, want %q", payload.AgentID, createdPayload.AgentID)
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents: got %d, want 1", len(payload.Agents))
	}
	if payload.Agents[0].RequiresAttention {
		t.Fatal("response agent still requires attention after clear")
	}
	if payload.Agents[0].AttentionReason != nil {
		t.Fatalf("attentionReason: got %v, want nil", *payload.Agents[0].AttentionReason)
	}

	deleteAgentForTest(t, conn, createdPayload.AgentID)
}

func TestSendAgentMessageViaWebSocket(t *testing.T) {
	// This test creates agents that trigger async goroutines writing to disk.
	// TempDir cleanup may fail if goroutines outlive the test; that's cosmetic.
	if testing.Short() {
		t.Skip("skipping in short mode due to async cleanup")
	}
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-send-msg")
	defer conn.Close()

	// Read 2 initial messages
	readInitialMessages(t, conn)

	// Create agent first
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-send",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      "/mock-project",
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	// Read agent_created to get agent ID
	var agentID string
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for agentID == "" {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var resp protocol.WSOutboundMessage
		json.Unmarshal(msg, &resp)
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Payload struct {
				Status string `json:"status"`
				Agent  struct {
					ID string `json:"id"`
				} `json:"agent"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) == nil && peek.Payload.Status == "agent_created" {
			agentID = peek.Payload.Agent.ID
		}
	}
	conn.SetReadDeadline(time.Time{})

	// Drain remaining messages from create (agent_update, agent_stream events)
	// The mock session runs synchronously, so all events arrive quickly.
	// Read up to 10 messages or until we get an agent_update (which is the last create msg).
	drainUpTo := 10
	for i := 0; i < drainUpTo; i++ {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			// Timeout or error is fine - no more messages
			break
		}
		var resp protocol.WSOutboundMessage
		json.Unmarshal(msg, &resp)
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				Kind string `json:"kind"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) == nil {
			// After agent_update from create, we can stop
			if peek.Type == "agent_update" && peek.Payload.Kind == "upsert" {
				break
			}
		}
	}
	conn.SetReadDeadline(time.Time{})

	// Send message to agent
	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-send-1",
			"agentId":   agentID,
			"text":      "Hello from test",
		}),
	}
	if err := conn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_message: %v", err)
	}

	// Read send_agent_message_response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	foundResponse := false
	for !foundResponse {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var resp protocol.WSOutboundMessage
		json.Unmarshal(msg, &resp)
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				Accepted bool `json:"accepted"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) == nil && peek.Type == "send_agent_message_response" {
			foundResponse = true
			if !peek.Payload.Accepted {
				t.Error("expected accepted=true")
			}
		}
	}

	readUntilAgentUpdateStatus(t, conn, agentID, protocol.AgentIdle)
	deleteReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "delete_agent_request",
			"requestId": "req-delete-send",
			"agentId":   agentID,
		}),
	}
	if err := conn.WriteJSON(deleteReq); err != nil {
		t.Fatalf("write delete_agent: %v", err)
	}
	readUntilType(t, conn, "agent_deleted")
}

func TestMobileSendAgentMessageReceivesAcceptedAndStream(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHelloAs(t, ts.URL, "test-mobile-send-msg", protocol.ClientMobile)
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-mobile-send",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      filepath.Join(t.TempDir(), "mobile-send-project"),
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	defer deleteAgentForTest(t, conn, createdPayload.AgentID)
	readUntilType(t, conn, "agent_update")

	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-mobile-send",
			"agentId":   createdPayload.AgentID,
			"text":      "Hello from mobile",
			"messageId": "mobile-message-1",
		}),
	}
	if err := conn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_message: %v", err)
	}

	response := readUntilType(t, conn, "send_agent_message_response")
	responsePayload := decodeSessionPayload[protocol.SendAgentMessageResponsePayload](t, response)
	if responsePayload.RequestID != "req-mobile-send" {
		t.Fatalf("requestId: got %q, want req-mobile-send", responsePayload.RequestID)
	}
	if responsePayload.AgentID != createdPayload.AgentID {
		t.Fatalf("agentId: got %q, want %q", responsePayload.AgentID, createdPayload.AgentID)
	}
	if !responsePayload.Accepted {
		t.Fatalf("accepted: got false, error=%v", responsePayload.Error)
	}

	waitForTimelineTextOnConn(t, conn, createdPayload.AgentID, "Mock response to: Hello from mobile")
}

func TestSendAgentMessageMissingAgentReturnsRejectedResponse(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHelloAs(t, ts.URL, "test-mobile-missing-agent-send", protocol.ClientMobile)
	defer conn.Close()

	readInitialMessages(t, conn)

	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-missing-agent-send",
			"agentId":   "missing-agent-id",
			"text":      "Hello missing agent",
			"messageId": "missing-agent-message-1",
		}),
	}
	if err := conn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_message: %v", err)
	}

	response := readUntilType(t, conn, "send_agent_message_response")
	responsePayload := decodeSessionPayload[protocol.SendAgentMessageResponsePayload](t, response)
	if responsePayload.RequestID != "req-missing-agent-send" {
		t.Fatalf("requestId: got %q, want req-missing-agent-send", responsePayload.RequestID)
	}
	if responsePayload.AgentID != "missing-agent-id" {
		t.Fatalf("agentId: got %q, want missing-agent-id", responsePayload.AgentID)
	}
	if responsePayload.Accepted {
		t.Fatal("accepted: got true, want false")
	}
	if responsePayload.Error == nil || !strings.Contains(*responsePayload.Error, "not found") {
		t.Fatalf("error: got %v, want not found message", responsePayload.Error)
	}
}

func TestResumeAgentRequestReturnsAgentResumedStatus(t *testing.T) {
	ws, ts := newTestWSServer(t)
	conn := dialAndHelloAs(t, ts.URL, "test-resume-agent", protocol.ClientMobile)
	defer conn.Close()

	readInitialMessages(t, conn)

	cwd := filepath.Join(t.TempDir(), "resume-project")
	handle := protocol.AgentPersistenceHandle{
		Provider:  "mock",
		SessionID: "mock-resume-session-1",
		Metadata: map[string]interface{}{
			"cwd":   cwd,
			"model": "mock-model",
		},
	}
	resumeReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "resume_agent_request",
			"requestId": "req-resume-agent",
			"handle":    handle,
		}),
	}
	if err := conn.WriteJSON(resumeReq); err != nil {
		t.Fatalf("write resume_agent: %v", err)
	}

	resumed := readUntilType(t, conn, "agent_resumed")
	payload := decodeStatusPayload[protocol.AgentResumedPayload](t, resumed)
	defer deleteAgentForTest(t, conn, payload.AgentID)
	if payload.RequestID != "req-resume-agent" {
		t.Fatalf("requestId: got %q, want req-resume-agent", payload.RequestID)
	}
	if payload.Status != "agent_resumed" {
		t.Fatalf("status: got %q, want agent_resumed", payload.Status)
	}
	if payload.Agent.ID == "" || payload.AgentID != payload.Agent.ID {
		t.Fatalf("agent ids not populated: payload=%q agent=%q", payload.AgentID, payload.Agent.ID)
	}
	if payload.Agent.Provider != "mock" {
		t.Fatalf("provider: got %q, want mock", payload.Agent.Provider)
	}
	if payload.Agent.Cwd != cwd {
		t.Fatalf("cwd: got %q, want %q", payload.Agent.Cwd, cwd)
	}
	if ws.agentMgr.GetAgent(payload.AgentID) == nil {
		t.Fatalf("resumed agent %q not registered", payload.AgentID)
	}
}

func TestMobileSendAgentMessageRestoresInactiveAgentSession(t *testing.T) {
	ws, ts := newTestWSServer(t)
	conn := dialAndHelloAs(t, ts.URL, "test-mobile-send-inactive-agent", protocol.ClientMobile)
	defer conn.Close()

	readInitialMessages(t, conn)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-inactive-send",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      filepath.Join(t.TempDir(), "inactive-send-project"),
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	defer deleteAgentForTest(t, conn, createdPayload.AgentID)
	readUntilType(t, conn, "agent_update")

	ag := ws.agentMgr.GetAgent(createdPayload.AgentID)
	if ag == nil {
		t.Fatalf("created agent %q not found", createdPayload.AgentID)
	}
	if ag.Session != nil {
		_ = ag.Session.Close()
		ag.Session = nil
	}

	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-mobile-inactive-send",
			"agentId":   createdPayload.AgentID,
			"text":      "Hello after inactive restore",
			"messageId": "mobile-inactive-message-1",
		}),
	}
	if err := conn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_message: %v", err)
	}

	response := readUntilType(t, conn, "send_agent_message_response")
	responsePayload := decodeSessionPayload[protocol.SendAgentMessageResponsePayload](t, response)
	if !responsePayload.Accepted {
		t.Fatalf("accepted: got false, error=%v", responsePayload.Error)
	}
	waitForTimelineTextOnConn(t, conn, createdPayload.AgentID, "Mock response to: Hello after inactive restore")
	if ws.agentMgr.GetAgent(createdPayload.AgentID).Session == nil {
		t.Fatal("agent session was not restored")
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func deleteAgentForTest(t *testing.T, conn *websocket.Conn, agentID string) {
	t.Helper()
	deleteReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "delete_agent_request",
			"requestId": "req-delete-" + agentID,
			"agentId":   agentID,
		}),
	}
	if err := conn.WriteJSON(deleteReq); err != nil {
		t.Fatalf("write delete_agent: %v", err)
	}
	readUntilType(t, conn, "agent_deleted")
}

func decodeSessionPayload[T any](t *testing.T, msg protocol.WSOutboundMessage) T {
	t.Helper()
	msgBytes, err := json.Marshal(msg.Message)
	if err != nil {
		t.Fatalf("marshal session message: %v", err)
	}
	var wrapper struct {
		Payload T `json:"payload"`
	}
	if err := json.Unmarshal(msgBytes, &wrapper); err != nil {
		t.Fatalf("unmarshal session payload: %v", err)
	}
	return wrapper.Payload
}

func decodeStatusPayload[T any](t *testing.T, msg protocol.WSOutboundMessage) T {
	t.Helper()
	return decodeSessionPayload[T](t, msg)
}

func readUntilAgentUpdateStatus(t *testing.T, conn *websocket.Conn, agentID string, status protocol.AgentLifecycleStatus) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read agent_update %s for %s: %v", status, agentID, err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				Kind  string `json:"kind"`
				Agent struct {
					ID     string                        `json:"id"`
					Status protocol.AgentLifecycleStatus `json:"status"`
				} `json:"agent"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) != nil {
			continue
		}
		if peek.Type == "agent_update" && peek.Payload.Kind == "upsert" && peek.Payload.Agent.ID == agentID && peek.Payload.Agent.Status == status {
			return
		}
	}
}

func waitForTimelineTextOnConn(t *testing.T, conn *websocket.Conn, agentID string, text string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read timeline text %q for %s: %v", text, agentID, err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				AgentID string      `json:"agentId"`
				Event   interface{} `json:"event"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) != nil {
			continue
		}
		if peek.Type == "agent_stream" && peek.Payload.AgentID == agentID {
			if strings.Contains(fmt.Sprint(peek.Payload.Event), text) {
				return
			}
		}
	}
}

func readUntilAgentUpdateAttention(t *testing.T, conn *websocket.Conn, agentID string, requires bool, reason string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read agent_update attention for %s: %v", agentID, err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				Kind  string `json:"kind"`
				Agent struct {
					ID                string  `json:"id"`
					RequiresAttention bool    `json:"requiresAttention"`
					AttentionReason   *string `json:"attentionReason"`
				} `json:"agent"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) != nil {
			continue
		}
		if peek.Type != "agent_update" || peek.Payload.Kind != "upsert" || peek.Payload.Agent.ID != agentID {
			continue
		}
		if peek.Payload.Agent.RequiresAttention != requires {
			continue
		}
		if reason == "" {
			if peek.Payload.Agent.AttentionReason == nil {
				return
			}
			continue
		}
		if peek.Payload.Agent.AttentionReason != nil && *peek.Payload.Agent.AttentionReason == reason {
			return
		}
	}
}
