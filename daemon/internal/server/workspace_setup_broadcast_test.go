package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// initTempGitRepo creates a real git repository with one commit and the given
// solo.json content (empty string means no solo.json). Skips if git is missing.
func initTempGitRepo(t *testing.T, soloJSON string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")
	runGitCmd(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if soloJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "solo.json"), []byte(soloJSON), 0644); err != nil {
			t.Fatalf("write solo.json: %v", err)
		}
	}
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "init")
	return dir
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// readUntilSessionType reads messages until a session message with the given
// type arrives, returning its raw payload.
func readUntilSessionType(t *testing.T, conn *websocket.Conn, targetType string) json.RawMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read %s: %v", targetType, err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		if resp.Type != "session" {
			continue
		}
		msgBytes, err := json.Marshal(resp.Message)
		if err != nil {
			continue
		}
		var peek struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) != nil {
			continue
		}
		if peek.Type == targetType {
			return peek.Payload
		}
	}
}

// TestWorkspaceSetupProgressBroadcastToOtherSessions verifies that worktree
// setup progress is broadcast to ALL sessions, not just the one that created
// the worktree. Regression test for the e2e gap where a browser session never
// saw setup progress for a workspace created by another client.
func TestWorkspaceSetupProgressBroadcastToOtherSessions(t *testing.T) {
	repoDir := initTempGitRepo(t, `{"worktree":{"setup":["echo setup-done"]}}`)

	_, ts := newTestWSServer(t)
	connA := dialAndHello(t, ts.URL, "test-setup-bcast-creator")
	defer connA.Close()
	connB := dialAndHello(t, ts.URL, "test-setup-bcast-observer")
	defer connB.Close()
	readInitialMessages(t, connA)
	readInitialMessages(t, connB)

	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":         "create_solo_worktree_request",
			"requestId":    "req-create-wt-bcast",
			"cwd":          repoDir,
			"worktreeSlug": "wt-bcast",
		}),
	}
	if err := connA.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_solo_worktree_request: %v", err)
	}

	// The creator gets the RPC response.
	createResp := readUntilType(t, connA, "create_solo_worktree_response")
	createPayload := decodeSessionPayload[protocol.CreateSoloWorktreeResponsePayload](t, createResp)
	if createPayload.Error != nil {
		t.Fatalf("create worktree failed: %s", *createPayload.Error)
	}
	if createPayload.Workspace == nil {
		t.Fatal("expected workspace in create_solo_worktree_response")
	}
	workspaceID := createPayload.Workspace.ID

	// The observer session must receive workspace_setup_progress too.
	progressPayload := readUntilSessionType(t, connB, "workspace_setup_progress")
	var progress protocol.WorkspaceSetupProgressPayload
	if err := json.Unmarshal(progressPayload, &progress); err != nil {
		t.Fatalf("unmarshal progress payload: %v", err)
	}
	if progress.WorkspaceID != workspaceID {
		t.Errorf("workspaceId: got %q, want %q", progress.WorkspaceID, workspaceID)
	}
	if progress.Status != "running" && progress.Status != "completed" {
		t.Errorf("unexpected status %q", progress.Status)
	}
}
