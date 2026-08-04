package server

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// TestBranchSuggestionsHandler verifies the daemon answers
// branch_suggestions_request with merged local branches, query filtering and
// branch details (app new-workspace ref picker).
func TestBranchSuggestionsHandler(t *testing.T) {
	repoDir := initTempGitRepo(t, "")
	runGitCmd(t, repoDir, "branch", "dev")

	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-branch-suggestions")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "branch_suggestions_request",
			"requestId": "req-branch-sug-1",
			"cwd":       repoDir,
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write branch_suggestions_request: %v", err)
	}

	resp := readUntilType(t, conn, "branch_suggestions_response")
	payload := decodeSessionPayload[protocol.BranchSuggestionsResponsePayload](t, resp)
	if payload.RequestID != "req-branch-sug-1" {
		t.Errorf("requestId: got %q, want req-branch-sug-1", payload.RequestID)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}

	names := map[string]bool{}
	for _, b := range payload.Branches {
		names[b] = true
	}
	if !names["dev"] {
		t.Errorf("expected dev in branches, got %v", payload.Branches)
	}
	// The default branch (main or master) must also be listed.
	if !names["main"] && !names["master"] {
		t.Errorf("expected default branch in branches, got %v", payload.Branches)
	}

	var devDetail *protocol.BranchSuggestionDetail
	for i := range payload.BranchDetails {
		if payload.BranchDetails[i].Name == "dev" {
			devDetail = &payload.BranchDetails[i]
		}
	}
	if devDetail == nil {
		t.Fatal("expected branchDetails to contain dev")
	}
	if devDetail.HasLocal == nil || !*devDetail.HasLocal {
		t.Errorf("dev hasLocal: got %v, want true", devDetail.HasLocal)
	}
	if devDetail.CommitterDate <= 0 {
		t.Errorf("dev committerDate: got %d, want > 0", devDetail.CommitterDate)
	}
}

// TestBranchSuggestionsHandler_QueryFilter verifies the query parameter
// filters branch names case-insensitively.
func TestBranchSuggestionsHandler_QueryFilter(t *testing.T) {
	repoDir := initTempGitRepo(t, "")
	runGitCmd(t, repoDir, "branch", "dev")

	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-branch-suggestions-query")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "branch_suggestions_request",
			"requestId": "req-branch-sug-2",
			"cwd":       repoDir,
			"query":     "DEV",
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write branch_suggestions_request: %v", err)
	}

	resp := readUntilType(t, conn, "branch_suggestions_response")
	payload := decodeSessionPayload[protocol.BranchSuggestionsResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if len(payload.Branches) != 1 || payload.Branches[0] != "dev" {
		t.Errorf("query filter: got %v, want [dev]", payload.Branches)
	}
}

// TestBranchSuggestionsHandler_NotGitRepo verifies a graceful error payload
// for directories that are not git repositories.
func TestBranchSuggestionsHandler_NotGitRepo(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-branch-suggestions-nogit")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "branch_suggestions_request",
			"requestId": "req-branch-sug-3",
			"cwd":       t.TempDir(),
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write branch_suggestions_request: %v", err)
	}

	resp := readUntilType(t, conn, "branch_suggestions_response")
	payload := decodeSessionPayload[protocol.BranchSuggestionsResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error for non-git directory")
	}
	if len(payload.Branches) != 0 {
		t.Errorf("expected empty branches on error, got %v", payload.Branches)
	}
}

// TestGitHubSearchHandler_Responds verifies the daemon answers
// github_search_request with a well-formed payload. Whether gh is installed
// or authenticated is environment-dependent, so this only asserts the
// response wiring (requestId echo, non-nil items); search behavior itself is
// covered by workspace package tests.
func TestGitHubSearchHandler_Responds(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "test-github-search")
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "github_search_request",
			"requestId": "req-gh-search-1",
			"cwd":       t.TempDir(),
			"query":     "anything",
			"kinds":     []string{"github-pr"},
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write github_search_request: %v", err)
	}

	resp := readUntilType(t, conn, "github_search_response")
	payload := decodeSessionPayload[protocol.GitHubSearchResponsePayload](t, resp)
	if payload.RequestID != "req-gh-search-1" {
		t.Errorf("requestId: got %q, want req-gh-search-1", payload.RequestID)
	}
	if payload.Items == nil {
		t.Error("items must be a non-nil array")
	}
}
