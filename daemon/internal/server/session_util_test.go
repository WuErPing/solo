package server

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

func TestDeriveInitialAgentTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Fix the login bug", "Fix the login bug"},
		{"\n\n  Refactor database layer  \n", "Refactor database layer"},
		{"who are you", "Identity inquiry"},
		{"What are you?", "Identity inquiry"},
		{"a very long prompt that exceeds the sixty character limit for initial titles", "a very long prompt that exceeds the sixty character limit fo"},
	}

	for _, tc := range cases {
		got := deriveInitialAgentTitle(tc.input)
		if got == nil {
			if tc.expected != "" {
				t.Errorf("deriveInitialAgentTitle(%q): got nil, want %q", tc.input, tc.expected)
			}
			continue
		}
		if *got != tc.expected {
			t.Errorf("deriveInitialAgentTitle(%q): got %q, want %q", tc.input, *got, tc.expected)
		}
	}
}

func TestResolveCreateAgentTitles(t *testing.T) {
	t.Run("explicit title", func(t *testing.T) {
		explicit, provisional := resolveCreateAgentTitles(strPtr("My Agent"), strPtr("some prompt"))
		if explicit == nil || *explicit != "My Agent" {
			t.Errorf("explicit: got %v", explicit)
		}
		if provisional == nil || *provisional != "My Agent" {
			t.Errorf("provisional: got %v", provisional)
		}
	})

	t.Run("derived from prompt", func(t *testing.T) {
		explicit, provisional := resolveCreateAgentTitles(nil, strPtr("fix bug"))
		if explicit != nil {
			t.Error("expected nil explicit")
		}
		if provisional == nil || *provisional != "fix bug" {
			t.Errorf("provisional: got %v", provisional)
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		explicit, provisional := resolveCreateAgentTitles(nil, strPtr("  "))
		if explicit != nil || provisional != nil {
			t.Error("expected both nil for empty prompt")
		}
	})
}

func TestRelPath(t *testing.T) {
	base := "/project"
	target := "/project/src/main.go"
	got := relPath(base, target)
	if got != "src/main.go" {
		t.Errorf("relPath: got %q, want src/main.go", got)
	}

	got = relPath(base, base)
	if got != "." {
		t.Errorf("relPath same dir: got %q, want .", got)
	}
}

func TestIsTextContent(t *testing.T) {
	if !isTextContent([]byte("hello world")) {
		t.Error("expected text content")
	}
	if !isTextContent([]byte{}) {
		t.Error("expected empty data to be text")
	}
	if isTextContent([]byte("hello\x00world")) {
		t.Error("expected binary content with null byte")
	}
}

func TestIsTextMimeType(t *testing.T) {
	if !isTextMimeType("text/plain") {
		t.Error("expected text/plain to be text")
	}
	if !isTextMimeType("application/json") {
		t.Error("expected application/json to be text")
	}
	if isTextMimeType("application/octet-stream") {
		t.Error("expected application/octet-stream to not be text")
	}
}

func TestSummarizeAgentIDMatches(t *testing.T) {
	ids := []string{"agent-one-123", "agent-two-456", "agent-three-789"}
	got := summarizeAgentIDMatches(ids)
	if !strings.Contains(got, "agent-on") {
		t.Errorf("expected summary to contain truncated IDs, got %q", got)
	}

	many := make([]string, 10)
	for i := range many {
		many[i] = "agent-" + string(rune('a'+i))
	}
	got = summarizeAgentIDMatches(many)
	if !strings.Contains(got, "...") {
		t.Error("expected ellipsis for many matches")
	}
}

func TestNormalizeWaitForFinishStatus(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"error", "error"},
		{"permission", "permission"},
		{"timeout", "timeout"},
		{"idle", "idle"},
		{"running", "idle"},
		{"", "idle"},
	}
	for _, tc := range cases {
		got := normalizeWaitForFinishStatus(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeWaitForFinishStatus(%q): got %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestStringPtrValue(t *testing.T) {
	if stringPtrValue(nil) != "" {
		t.Error("expected empty string for nil")
	}
	if stringPtrValue(strPtr("hello")) != "hello" {
		t.Error("expected 'hello'")
	}
}

func TestTextPrefix(t *testing.T) {
	if textPrefix("hello", 10) != "hello" {
		t.Error("expected full string when under limit")
	}
	if textPrefix("hello world", 5) != "hello" {
		t.Errorf("expected prefix, got %q", textPrefix("hello world", 5))
	}
}

func TestStrPtrOrNil(t *testing.T) {
	if strPtrOrNil("") != nil {
		t.Error("expected nil for empty")
	}
	if strPtrOrNil("x") == nil {
		t.Error("expected non-nil for non-empty")
	}
}

func TestNormalizeProjectCwd(t *testing.T) {
	got := normalizeProjectCwd("  ")
	if got != "." {
		t.Errorf("empty cwd: got %q, want .", got)
	}
	got = normalizeProjectCwd("/tmp/project")
	if !strings.HasSuffix(got, "project") {
		t.Errorf("got %q", got)
	}
}

func TestDeriveRemoteProjectKey(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"https://github.com/owner/repo.git", "remote:github.com/owner/repo"},
		{"git@github.com:owner/repo.git", "remote:github.com/owner/repo"},
		{"https://gitlab.com/group/project.git", "remote:gitlab.com/group/project"},
		{"not-a-url", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := deriveRemoteProjectKey(&tc.url)
		if got != tc.expected {
			t.Errorf("deriveRemoteProjectKey(%q): got %q, want %q", tc.url, got, tc.expected)
		}
	}
}

func TestDeriveProjectGroupingKey(t *testing.T) {
	remote := "https://github.com/owner/repo.git"
	got := deriveProjectGroupingKey("/cwd", &remote, nil)
	if got != "remote:github.com/owner/repo" {
		t.Errorf("got %q", got)
	}

	mainRepo := "/main/repo"
	got = deriveProjectGroupingKey("/cwd", nil, &mainRepo)
	if !strings.HasSuffix(got, "repo") {
		t.Errorf("got %q", got)
	}
}

func TestDeriveProjectGroupingName(t *testing.T) {
	if deriveProjectGroupingName("remote:github.com/owner/repo") != "owner/repo" {
		t.Errorf("github remote: got %q", deriveProjectGroupingName("remote:github.com/owner/repo"))
	}
	if deriveProjectGroupingName("/path/to/project") != "project" {
		t.Errorf("path: got %q", deriveProjectGroupingName("/path/to/project"))
	}
}

func TestMatchesFetchAgentsFilter(t *testing.T) {
	agent := protocol.AgentSnapshotPayload{Status: "idle", Labels: map[string]string{"team": "backend"}}
	project := protocol.ProjectPlacementPayload{ProjectKey: "proj1"}

	if !matchesFetchAgentsFilter(agent, project, nil, false) {
		t.Error("expected match with nil filter")
	}

	filter := &protocol.FetchAgentsFilter{Labels: map[string]string{"team": "backend"}}
	if !matchesFetchAgentsFilter(agent, project, filter, false) {
		t.Error("expected match by label")
	}

	filter = &protocol.FetchAgentsFilter{Labels: map[string]string{"team": "frontend"}}
	if matchesFetchAgentsFilter(agent, project, filter, false) {
		t.Error("expected no match by wrong label")
	}

	filter = &protocol.FetchAgentsFilter{ProjectKeys: []string{"proj1"}}
	if !matchesFetchAgentsFilter(agent, project, filter, false) {
		t.Error("expected match by project key")
	}

	filter = &protocol.FetchAgentsFilter{Statuses: []protocol.AgentLifecycleStatus{"idle"}}
	if !matchesFetchAgentsFilter(agent, project, filter, false) {
		t.Error("expected match by status")
	}

	filter = &protocol.FetchAgentsFilter{RequiresAttention: boolPtr(false)}
	if !matchesFetchAgentsFilter(agent, project, filter, false) {
		t.Error("expected match by requiresAttention")
	}

	// Archived exclusion
	archivedAgent := protocol.AgentSnapshotPayload{Status: "idle", ArchivedAt: strPtr("2024-01-01")}
	if matchesFetchAgentsFilter(archivedAgent, project, nil, false) {
		t.Error("expected archived agent to be excluded by default")
	}
	if !matchesFetchAgentsFilter(archivedAgent, project, nil, true) {
		t.Error("expected archived agent to be included when defaultIncludeArchived is true")
	}
}

func TestExtractTimelineItem(t *testing.T) {
	item := agent.TimelineItem{Type: "text", Text: "hello"}
	got := extractTimelineItem(item)
	if got.Type != "text" || got.Text != "hello" {
		t.Errorf("got %+v", got)
	}

	m := map[string]interface{}{
		"type":    "tool_call",
		"name":    "git",
		"message": "msg",
	}
	got = extractTimelineItem(m)
	if got.Type != "tool_call" || got.Name != "git" || got.Message != "msg" {
		t.Errorf("got %+v", got)
	}

	got = extractTimelineItem("invalid")
	if got.Type != "" {
		t.Error("expected empty for invalid input")
	}
}

func TestCanonicalizeConfigRoot(t *testing.T) {
	got := canonicalizeConfigRoot("  ")
	if got != "" {
		t.Errorf("empty: got %q", got)
	}
	got = canonicalizeConfigRoot("/tmp")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestStripTrailingPathSeparators(t *testing.T) {
	if stripTrailingPathSeparators("/tmp/") != "/tmp" {
		t.Errorf("got %q", stripTrailingPathSeparators("/tmp/"))
	}
	if stripTrailingPathSeparators("/") != "/" {
		t.Errorf("root: got %q", stripTrailingPathSeparators("/"))
	}
}

func TestProtocolRevisionFromWorkspace(t *testing.T) {
	if protocolRevisionFromWorkspace(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestWorkspaceRevisionFromProtocol(t *testing.T) {
	if workspaceRevisionFromProtocol(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestBuildSetupLog(t *testing.T) {
	exitCode := 0
	duration := 100
	commands := []workspace.SetupCommandSnapshot{
		{Index: 1, Command: "echo hello", Status: "completed", ExitCode: &exitCode, DurationMs: &duration},
	}
	log := buildSetupLog(commands)
	if !strings.Contains(log, "echo hello") {
		t.Errorf("expected command in log, got %q", log)
	}
}

func TestConvertCommandSnapshots(t *testing.T) {
	commands := []workspace.SetupCommandSnapshot{
		{Index: 1, Command: "echo hello", Status: "completed"},
	}
	result := convertCommandSnapshots(commands)
	if len(result) != 1 || result[0].Command != "echo hello" {
		t.Errorf("got %+v", result)
	}
}

func boolPtr(b bool) *bool { return &b }
