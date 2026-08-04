package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// TestStartWorkspaceScriptResponseIncludesWorkspaceID is a regression test for
// the e2e gap where the daemon's start_workspace_script_response payload had no
// workspaceId while the app-bridge zod schema requires it, causing the client
// to discard the response and time out the RPC.
func TestStartWorkspaceScriptResponseIncludesWorkspaceID(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-start-script-wsid")
	defer conn.Close()
	readInitialMessages(t, conn)

	tmpDir := t.TempDir()
	soloCfg := `{"scripts":{"editor":{"command":"echo editor ready"}}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "solo.json"), []byte(soloCfg), 0644); err != nil {
		t.Fatalf("write solo.json: %v", err)
	}

	// Register the workspace via open_project_request.
	openReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "open_project_request",
			"requestId": "req-open-for-script",
			"cwd":       tmpDir,
		}),
	}
	if err := conn.WriteJSON(openReq); err != nil {
		t.Fatalf("write open_project_request: %v", err)
	}
	openResp := readUntilType(t, conn, "open_project_response")
	openPayload := decodeSessionPayload[protocol.OpenProjectResponsePayload](t, openResp)
	if openPayload.Error != nil {
		t.Fatalf("open project failed: %s", *openPayload.Error)
	}
	if openPayload.Workspace == nil {
		t.Fatal("expected workspace in open_project_response")
	}

	// Start the script through the explicit daemon request.
	startReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":        "start_workspace_script_request",
			"requestId":   "req-start-script-1",
			"workspaceId": tmpDir,
			"scriptName":  "editor",
		}),
	}
	if err := conn.WriteJSON(startReq); err != nil {
		t.Fatalf("write start_workspace_script_request: %v", err)
	}

	resp := readUntilType(t, conn, "start_workspace_script_response")
	payload := decodeSessionPayload[protocol.StartWorkspaceScriptResponsePayload](t, resp)
	if payload.RequestID != "req-start-script-1" {
		t.Errorf("requestId: got %q, want req-start-script-1", payload.RequestID)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.WorkspaceID != tmpDir {
		t.Errorf("workspaceId: got %q, want %q", payload.WorkspaceID, tmpDir)
	}
	if payload.ScriptName != "editor" {
		t.Errorf("scriptName: got %q, want editor", payload.ScriptName)
	}
}
