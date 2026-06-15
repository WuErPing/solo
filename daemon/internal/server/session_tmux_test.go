package server

import (
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

var testAgentNames = map[string]bool{
	"claude": true, "opencode": true, "qodercli": true,
	"pi": true, "cursor": true, "kimi": true, "kimi-cli": true, "codex": true,
}

func TestIsTmuxAIAgentName(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"claude", "claude", true},
		{"opencode", "opencode", true},
		{"qodercli", "qodercli", true},
		{"pi", "pi", true},
		{"cursor", "cursor", true},
		{"kimi", "kimi", true},
		{"kimi-cli", "kimi-cli", true},
		{"/usr/local/bin/pi", "/usr/local/bin/pi", true},
		{"/usr/local/bin/kimi-cli", "/usr/local/bin/kimi-cli", true},
		{"bash", "bash", false},
		{"zsh", "zsh", false},
		{"node", "node", false},
		{"", "", false},
		{"pi-agent", "pi-agent", false},
		{"PI", "PI", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTmuxAIAgentName(tt.cmd, testAgentNames)
			if got != tt.want {
				t.Errorf("isTmuxAIAgentName(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestIsTmuxAIAgentName_CustomAgent(t *testing.T) {
	custom := map[string]bool{"aider": true, "cody": true}
	if !isTmuxAIAgentName("aider", custom) {
		t.Error("expected 'aider' to match custom agent names")
	}
	if isTmuxAIAgentName("claude", custom) {
		t.Error("expected 'claude' NOT to match custom-only agent names")
	}
}

func TestParseTmuxPaneLines(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantNames []string
	}{
		{
			name: "single pi agent",
			input: "%0|0|1000|bash|session1|window1|/home/user\n" +
				"%1|1|2000|pi|session1|window1|/home/user/project\n",
			wantCount: 1,
			wantNames: []string{"pi"},
		},
		{
			name: "multiple agents",
			input: "%0|0|1000|claude|s1|w1|/tmp\n" +
				"%1|1|2000|pi|s1|w1|/tmp/a\n" +
				"%2|0|3000|bash|s2|w1|/tmp/b\n" +
				"%3|1|4000|kimi-cli|s2|w1|/tmp/c\n",
			wantCount: 3,
			wantNames: []string{"claude", "pi", "kimi-cli"},
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name: "no agents",
			input: "%0|0|1000|bash|s1|w1|/tmp\n" +
				"%1|1|2000|zsh|s1|w1|/tmp\n",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name: "malformed lines skipped",
			input: "%0|0|1000\n" +
				"%1|1|2000|pi|s1|w1|/tmp\n",
			wantCount: 1,
			wantNames: []string{"pi"},
		},
		{
			name:      "path prefix stripped",
			input:     "%0|0|1000|/usr/local/bin/claude|s1|w1|/tmp\n",
			wantCount: 1,
			wantNames: []string{"claude"},
		},
		{
			name: "all eight agents",
			input: "%0|0|1000|claude|s1|w1|/a\n" +
				"%1|1|2000|opencode|s1|w1|/b\n" +
				"%2|2|3000|qodercli|s1|w1|/c\n" +
				"%3|0|4000|pi|s2|w1|/d\n" +
				"%4|1|5000|cursor|s2|w1|/e\n" +
				"%5|2|6000|kimi|s2|w1|/f\n" +
				"%6|0|7000|kimi-cli|s3|w1|/g\n" +
				"%7|1|8000|codex|s3|w1|/h\n",
			wantCount: 8,
			wantNames: []string{"claude", "opencode", "qodercli", "pi", "cursor", "kimi", "kimi-cli", "codex"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents, _ := parseTmuxPaneLines(tt.input, testAgentNames)
			if len(agents) != tt.wantCount {
				t.Fatalf("got %d agents, want %d", len(agents), tt.wantCount)
			}
			for i, want := range tt.wantNames {
				if agents[i].AgentName != want {
					t.Errorf("agent[%d].AgentName = %q, want %q", i, agents[i].AgentName, want)
				}
			}
		})
	}
}

func TestAgentNameFromTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"pi unicode title", "π - solo", "pi"},
		{"opencode title", "OpenCode", "opencode"},
		{"claude title", "Claude Code - my-project", "claude"},
		{"kimi title", "Kimi AI Assistant", "kimi"},
		{"no agent in title", "zsh - bash", ""},
		{"empty title", "", ""},
		{"node title no agent", "node - my-app", ""},
		{"pi word boundary start", "pi tool", "pi"},
		{"pi word boundary end", "the pi", "pi"},
		{"pi not substring", "pixel", ""},
		{"kimi-cli in title", "kimi-cli running", "kimi-cli"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentNameFromTitle(tt.title, testAgentNames)
			if got != tt.want {
				t.Errorf("agentNameFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestParseTmuxPaneLinesTitleDetection(t *testing.T) {
	// pi detected via pane_title (pane_current_command is "node")
	input := "%0|0|86415|node|0|node|/home/user|π - solo\n"
	agents, _ := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.AgentName != "pi" {
		t.Errorf("AgentName = %q, want %q", a.AgentName, "pi")
	}
	if a.CurrentCmd != "node" {
		t.Errorf("CurrentCmd = %q, want %q", a.CurrentCmd, "node")
	}
}

func TestParseTmuxPaneLinesTitleDetectionMultiple(t *testing.T) {
	// Mix: claude via command, pi via title, opencode via title
	input := "%0|0|1000|claude|s1|w1|/a|Claude Code\n" +
		"%1|1|2000|node|s1|w1|/b|π - solo\n" +
		"%2|0|3000|bash|s2|w1|/c|bash\n" +
		"%3|1|4000|opencode|s2|w1|/d|OpenCode\n"
	agents, _ := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 3 {
		t.Fatalf("got %d agents, want 3", len(agents))
	}
	if agents[0].AgentName != "claude" {
		t.Errorf("agent[0] = %q, want %q", agents[0].AgentName, "claude")
	}
	if agents[1].AgentName != "pi" {
		t.Errorf("agent[1] = %q, want %q", agents[1].AgentName, "pi")
	}
	if agents[2].AgentName != "opencode" {
		t.Errorf("agent[2] = %q, want %q", agents[2].AgentName, "opencode")
	}
}

func TestParseTmuxPaneLines_CustomAgent(t *testing.T) {
	custom := map[string]bool{"aider": true, "cody": true}
	input := "%0|0|1000|aider|s1|w1|/a\n" +
		"%1|1|2000|cody|s1|w1|/b\n" +
		"%2|2|3000|claude|s1|w1|/c\n" +
		"%3|3|4000|bash|s1|w1|/d\n"
	agents, _ := parseTmuxPaneLines(input, custom)
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}
	if agents[0].AgentName != "aider" {
		t.Errorf("agent[0] = %q, want %q", agents[0].AgentName, "aider")
	}
	if agents[1].AgentName != "cody" {
		t.Errorf("agent[1] = %q, want %q", agents[1].AgentName, "cody")
	}
}

func TestParseTmuxPaneLinesMetadata(t *testing.T) {
	input := "%5|2|9876|pi|my-session|my-window|/Users/me/code\n"
	agents, _ := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.PaneID != "%5" {
		t.Errorf("PaneID = %q, want %q", a.PaneID, "%5")
	}
	if a.PaneIndex != 2 {
		t.Errorf("PaneIndex = %d, want %d", a.PaneIndex, 2)
	}
	if a.PanePID != 9876 {
		t.Errorf("PanePID = %d, want %d", a.PanePID, 9876)
	}
	if a.SessionName != "my-session" {
		t.Errorf("SessionName = %q, want %q", a.SessionName, "my-session")
	}
	if a.WindowName != "my-window" {
		t.Errorf("WindowName = %q, want %q", a.WindowName, "my-window")
	}
	if a.WorkingDir != "/Users/me/code" {
		t.Errorf("WorkingDir = %q, want %q", a.WorkingDir, "/Users/me/code")
	}
	if a.CurrentCmd != "pi" {
		t.Errorf("CurrentCmd = %q, want %q", a.CurrentCmd, "pi")
	}
}

func TestParseTmuxPaneLinesExitedAgent(t *testing.T) {
	// Agent exited: command is bash, but title still contains agent name
	input := "%0|0|5000|bash|s1|w1|/home/user|π - solo\n"
	agents, _ := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.AgentName != "pi" {
		t.Errorf("AgentName = %q, want %q", a.AgentName, "pi")
	}
	if a.Status != "exited" {
		t.Errorf("Status = %q, want %q", a.Status, "exited")
	}
	if a.CurrentCmd != "bash" {
		t.Errorf("CurrentCmd = %q, want %q", a.CurrentCmd, "bash")
	}
}

func TestParseTmuxPaneLinesExitedAgentClaude(t *testing.T) {
	// Claude exited: command is zsh, title contains "Claude Code"
	input := "%1|1|6000|zsh|s1|w1|/tmp|Claude Code - my-project\n"
	agents, _ := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", a.AgentName, "claude")
	}
	if a.Status != "exited" {
		t.Errorf("Status = %q, want %q", a.Status, "exited")
	}
}

func TestParseTmuxPaneLinesExitedAgentMixed(t *testing.T) {
	// Mix: active claude via command, exited pi via title, active kimi-cli via command
	input := "%0|0|1000|claude|s1|w1|/a\n" +
		"%1|1|5000|bash|s1|w1|/b|π - solo\n" +
		"%2|0|7000|kimi-cli|s2|w1|/c\n"
	agents, _ := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 3 {
		t.Fatalf("got %d agents, want 3", len(agents))
	}
	// Active agents have empty status
	if agents[0].AgentName != "claude" {
		t.Errorf("agent[0].AgentName = %q, want %q", agents[0].AgentName, "claude")
	}
	if agents[0].Status != "" {
		t.Errorf("agent[0].Status = %q, want empty", agents[0].Status)
	}
	// Exited agent
	if agents[1].AgentName != "pi" {
		t.Errorf("agent[1].AgentName = %q, want %q", agents[1].AgentName, "pi")
	}
	if agents[1].Status != "exited" {
		t.Errorf("agent[1].Status = %q, want %q", agents[1].Status, "exited")
	}
	// Active agent
	if agents[2].AgentName != "kimi-cli" {
		t.Errorf("agent[2].AgentName = %q, want %q", agents[2].AgentName, "kimi-cli")
	}
	if agents[2].Status != "" {
		t.Errorf("agent[2].Status = %q, want empty", agents[2].Status)
	}
}

func TestParseTmuxPaneLinesNoAgentStillSkipped(t *testing.T) {
	// No agent name in title either — agents list empty, otherPanes populated
	input := "%0|0|1000|bash|s1|w1|/tmp|bash\n" +
		"%1|1|2000|zsh|s1|w1|/tmp|zsh - terminal\n"
	agents, otherPanes := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 0 {
		t.Fatalf("got %d agents, want 0", len(agents))
	}
	if len(otherPanes) != 2 {
		t.Fatalf("got %d otherPanes, want 2", len(otherPanes))
	}
	if otherPanes[0].CurrentCmd != "bash" {
		t.Errorf("otherPanes[0].CurrentCmd = %q, want %q", otherPanes[0].CurrentCmd, "bash")
	}
	if otherPanes[1].CurrentCmd != "zsh" {
		t.Errorf("otherPanes[1].CurrentCmd = %q, want %q", otherPanes[1].CurrentCmd, "zsh")
	}
}

func TestParseTmuxPaneLinesOtherPanes(t *testing.T) {
	// Mix of agent and non-agent panes
	input := "%0|0|1000|claude|s1|w1|/a\n" +
		"%1|1|2000|bash|s1|w1|/tmp\n" +
		"%2|2|3000|node|s1|w1|/app\n" +
		"%3|0|4000|pi|s2|w1|/b\n"
	agents, otherPanes := parseTmuxPaneLines(input, testAgentNames)
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}
	if len(otherPanes) != 2 {
		t.Fatalf("got %d otherPanes, want 2", len(otherPanes))
	}
	if otherPanes[0].CurrentCmd != "bash" {
		t.Errorf("otherPanes[0].CurrentCmd = %q, want %q", otherPanes[0].CurrentCmd, "bash")
	}
	if otherPanes[0].SessionName != "s1" {
		t.Errorf("otherPanes[0].SessionName = %q, want %q", otherPanes[0].SessionName, "s1")
	}
	if otherPanes[1].CurrentCmd != "node" {
		t.Errorf("otherPanes[1].CurrentCmd = %q, want %q", otherPanes[1].CurrentCmd, "node")
	}
}

func TestIsShellCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"bash", true},
		{"zsh", true},
		{"sh", true},
		{"fish", true},
		{"dash", true},
		{"/bin/bash", true},
		{"/usr/bin/zsh", true},
		{"claude", false},
		{"node", false},
		{"python", false},
		{"pi", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := isShellCommand(tt.cmd); got != tt.want {
				t.Errorf("isShellCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestCaptureTmuxPaneInvalidID(t *testing.T) {
	_, err := captureTmuxPane("%99999", -200)
	if err == nil {
		t.Fatal("expected error for invalid pane ID, got nil")
	}
}

func TestCaptureTmuxPaneReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Capture any available pane
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id}").Output()
	if err != nil {
		t.Skip("tmux not available")
	}
	paneID := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if paneID == "" {
		t.Skip("no tmux panes available")
	}

	content, err := captureTmuxPane(paneID, -200)
	if err != nil {
		t.Fatalf("captureTmuxPane(%q) error: %v", paneID, err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty content from capture-pane")
	}
}

func TestCaptureTmuxPaneWithStartLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id}").Output()
	if err != nil {
		t.Skip("tmux not available")
	}
	paneID := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if paneID == "" {
		t.Skip("no tmux panes available")
	}

	// Default start line (-200)
	contentDefault, err := captureTmuxPane(paneID, -200)
	if err != nil {
		t.Fatalf("captureTmuxPane default error: %v", err)
	}

	// Larger start line (more history)
	contentLarge, err := captureTmuxPane(paneID, -400)
	if err != nil {
		t.Fatalf("captureTmuxPane large error: %v", err)
	}

	// The larger capture should contain at least as much content
	if len(contentLarge) < len(contentDefault) {
		t.Errorf("expected larger capture to have more content, got %d vs %d", len(contentLarge), len(contentDefault))
	}
}

func TestSendKeysToTmuxPaneInvalidID(t *testing.T) {
	err := sendKeysToTmuxPane("%99999", "echo hello", true)
	if err == nil {
		t.Fatal("expected error for invalid pane ID, got nil")
	}
}

func TestExtractTmuxStatusLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Skip("tmux not available")
	}
	sessionID := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if sessionID == "" {
		t.Skip("no tmux sessions available")
	}

	left, center, right, err := extractTmuxStatusLine(sessionID)
	if err != nil {
		t.Fatalf("extractTmuxStatusLine(%q) error: %v", sessionID, err)
	}
	// At least one of them should be non-empty (tmux always has a status line)
	if left == "" && right == "" {
		t.Error("expected at least one of statusLeft or statusRight to be non-empty")
	}
	t.Logf("statusLeft=%q statusCenter=%q statusRight=%q", left, center, right)
}

func TestExtractTmuxStatusLineExpanded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Skip("tmux not available")
	}
	sessionID := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if sessionID == "" {
		t.Skip("no tmux sessions available")
	}

	left, _, right, err := extractTmuxStatusLine(sessionID)
	if err != nil {
		t.Fatalf("extractTmuxStatusLine(%q) error: %v", sessionID, err)
	}
	// The right side should contain a time pattern (HH:MM) if the default status-right is used
	// This verifies that format strings are actually expanded, not returned raw
	if right != "" && strings.Contains(right, "#{") {
		t.Errorf("statusRight contains unexpanded format specifiers: %q", right)
	}
	if left != "" && strings.Contains(left, "#{") {
		t.Errorf("statusLeft contains unexpanded format specifiers: %q", left)
	}
	t.Logf("statusLeft=%q statusRight=%q", left, right)
}

func TestExtractTmuxStatusLineInvalidSession(t *testing.T) {
	// tmux display-message may succeed even for invalid sessions (falls back to current),
	// so we just verify it doesn't panic and returns some result.
	left, _, right, err := extractTmuxStatusLine("nonexistent-session-99999")
	if err != nil {
		// Some tmux versions do error - that's fine too.
		return
	}
	t.Logf("invalid session returned: left=%q right=%q", left, right)
}

func TestCreateTmuxSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sessionName := "solo-test-new-session-" + t.Name()
	defer func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	}()

	err := createTmuxSession(sessionName, nil, nil)
	if err != nil {
		t.Fatalf("createTmuxSession(%q) error: %v", sessionName, err)
	}

	// Verify the session exists
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("list-sessions error: %v", err)
	}
	if !strings.Contains(string(out), sessionName) {
		t.Errorf("session %q not found in tmux sessions:\n%s", sessionName, string(out))
	}
}

func TestCreateTmuxSessionWithWorkingDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sessionName := "solo-test-cwd-" + t.Name()
	defer func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	}()

	cwd := "/tmp"
	err := createTmuxSession(sessionName, &cwd, nil)
	if err != nil {
		t.Fatalf("createTmuxSession with cwd error: %v", err)
	}

	// Verify the session's current path
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_current_path}").Output()
	if err != nil {
		t.Fatalf("list-panes error: %v", err)
	}
	gotPath := strings.TrimSpace(string(out))
	if gotPath != "/private/tmp" && gotPath != "/tmp" {
		t.Errorf("expected working dir /tmp or /private/tmp, got %q", gotPath)
	}
}

func TestCreateTmuxSessionWithCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sessionName := "solo-test-cmd-" + t.Name()
	defer func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	}()

	// Use a long-running command so the session stays alive
	cmd := "sleep 60"
	err := createTmuxSession(sessionName, nil, &cmd)
	if err != nil {
		t.Fatalf("createTmuxSession with command error: %v", err)
	}

	// Verify the session exists
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("list-sessions error: %v", err)
	}
	if !strings.Contains(string(out), sessionName) {
		t.Errorf("session %q not found in tmux sessions:\n%s", sessionName, string(out))
	}
}

func TestCreateTmuxSessionDuplicateName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sessionName := "solo-test-dup-" + t.Name()
	defer func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	}()

	err := createTmuxSession(sessionName, nil, nil)
	if err != nil {
		t.Fatalf("first createTmuxSession error: %v", err)
	}

	err = createTmuxSession(sessionName, nil, nil)
	if err == nil {
		t.Fatal("expected error for duplicate session name, got nil")
	}
}

func TestKillTmuxSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sessionName := "solo-test-kill-" + t.Name()

	// Create a session first
	err := createTmuxSession(sessionName, nil, nil)
	if err != nil {
		t.Fatalf("createTmuxSession(%q) error: %v", sessionName, err)
	}

	// Kill it
	err = killTmuxSession(sessionName)
	if err != nil {
		t.Fatalf("killTmuxSession(%q) error: %v", sessionName, err)
	}

	// Verify the session no longer exists
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		// tmux list-sessions exits non-zero when no sessions exist
		return
	}
	if strings.Contains(string(out), sessionName) {
		t.Errorf("session %q still exists after kill:\n%s", sessionName, string(out))
	}
}

func TestKillTmuxSessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := killTmuxSession("nonexistent-session-solo-test")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
}

func TestDescendantProcesses(t *testing.T) {
	nodes := []processNode{
		{pid: 1000, ppid: 1, comm: "tmux"},
		{pid: 2000, ppid: 1000, comm: "bash"},
		{pid: 3000, ppid: 2000, comm: "node"},
		{pid: 4000, ppid: 3000, comm: "kimi"},
		{pid: 5000, ppid: 1000, comm: "zsh"},
	}
	desc := descendantProcesses(1000, nodes)
	if len(desc) != 4 {
		t.Fatalf("got %d descendants, want 4: %+v", len(desc), desc)
	}
	wantPIDs := []int{2000, 3000, 4000, 5000}
	for i, want := range wantPIDs {
		if desc[i].pid != want {
			t.Errorf("desc[%d].pid = %d, want %d", i, desc[i].pid, want)
		}
	}
}

func TestFindAgentDescendantDirectChild(t *testing.T) {
	orig := processListFunc
	defer func() { processListFunc = orig }()
	processListFunc = func() ([]processNode, error) {
		return []processNode{
			{pid: 1000, ppid: 1, comm: "tmux"},
			{pid: 2000, ppid: 1000, comm: "kimi", args: "kimi /home/user/project"},
		}, nil
	}

	name, pid, args := findAgentDescendant(1000, map[string]bool{"kimi": true})
	if name != "kimi" {
		t.Errorf("name = %q, want kimi", name)
	}
	if pid != 2000 {
		t.Errorf("pid = %d, want 2000", pid)
	}
	if args != "kimi /home/user/project" {
		t.Errorf("args = %q, want 'kimi /home/user/project'", args)
	}
}

func TestFindAgentDescendantGrandchild(t *testing.T) {
	orig := processListFunc
	defer func() { processListFunc = orig }()
	processListFunc = func() ([]processNode, error) {
		return []processNode{
			{pid: 1000, ppid: 1, comm: "tmux"},
			{pid: 2000, ppid: 1000, comm: "bash"},
			{pid: 3000, ppid: 2000, comm: "node"},
			{pid: 4000, ppid: 3000, comm: "kimi", args: "node /usr/local/bin/kimi --cwd /home/user/project"},
		}, nil
	}

	name, pid, args := findAgentDescendant(1000, map[string]bool{"kimi": true})
	if name != "kimi" {
		t.Errorf("name = %q, want kimi", name)
	}
	if pid != 4000 {
		t.Errorf("pid = %d, want 4000", pid)
	}
	if args != "node /usr/local/bin/kimi --cwd /home/user/project" {
		t.Errorf("args = %q, want matching launch cmd", args)
	}
}

func TestFindAgentDescendantNoMatch(t *testing.T) {
	orig := processListFunc
	defer func() { processListFunc = orig }()
	processListFunc = func() ([]processNode, error) {
		return []processNode{
			{pid: 1000, ppid: 1, comm: "tmux"},
			{pid: 2000, ppid: 1000, comm: "bash"},
			{pid: 3000, ppid: 2000, comm: "vim"},
		}, nil
	}

	name, pid, args := findAgentDescendant(1000, map[string]bool{"kimi": true})
	if name != "" || pid != 0 || args != "" {
		t.Errorf("expected no match, got name=%q pid=%d args=%q", name, pid, args)
	}
}

func TestExtractAgentLaunchCmdFromGrandchild(t *testing.T) {
	orig := processListFunc
	defer func() { processListFunc = orig }()
	processListFunc = func() ([]processNode, error) {
		return []processNode{
			{pid: 1000, ppid: 1, comm: "tmux"},
			{pid: 2000, ppid: 1000, comm: "bash"},
			{pid: 3000, ppid: 2000, comm: "node"},
			{pid: 4000, ppid: 3000, comm: "kimi", args: "kimi --permission-mode=bypass_permissions"},
		}, nil
	}

	cmd := extractAgentLaunchCmd(1000, "kimi", map[string]bool{"kimi": true})
	if cmd != "kimi --permission-mode=bypass_permissions" {
		t.Errorf("cmd = %q, want 'kimi --permission-mode=bypass_permissions'", cmd)
	}
}

func TestArgsContainsAgentName(t *testing.T) {
	tests := []struct {
		args string
		want string
	}{
		{"node /usr/local/bin/kimi --cwd /home/user/project", "kimi"},
		{"python /opt/kimi-cli", "kimi-cli"},
		{"/bin/bash /home/user/scripts/claude", "claude"},
		{"npx opencode", "opencode"},
		{"node server.js", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			got, ok := argsContainsAgentName(tt.args, testAgentNames)
			if tt.want == "" {
				if ok {
					t.Errorf("argsContainsAgentName(%q) = %q, want no match", tt.args, got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("argsContainsAgentName(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestFindAgentDescendantByArgs(t *testing.T) {
	orig := processListFunc
	defer func() { processListFunc = orig }()
	processListFunc = func() ([]processNode, error) {
		return []processNode{
			{pid: 1000, ppid: 1, comm: "tmux"},
			{pid: 2000, ppid: 1000, comm: "bash"},
			{pid: 3000, ppid: 2000, comm: "node", args: "node /usr/local/bin/kimi /home/user/project"},
		}, nil
	}

	name, pid, args := findAgentDescendant(1000, map[string]bool{"kimi": true})
	if name != "kimi" {
		t.Errorf("name = %q, want kimi", name)
	}
	if pid != 3000 {
		t.Errorf("pid = %d, want 3000", pid)
	}
	if args != "node /usr/local/bin/kimi /home/user/project" {
		t.Errorf("args = %q, want matching launch cmd", args)
	}
}

func TestComputeContentHash(t *testing.T) {
	t.Run("deterministic for same input", func(t *testing.T) {
		h1 := computeContentHash("hello world")
		h2 := computeContentHash("hello world")
		if h1 != h2 {
			t.Errorf("same input produced different hashes: %q vs %q", h1, h2)
		}
	})

	t.Run("different for different input", func(t *testing.T) {
		h1 := computeContentHash("hello")
		h2 := computeContentHash("world")
		if h1 == h2 {
			t.Errorf("different inputs produced same hash: %q", h1)
		}
	})

	t.Run("empty string produces deterministic hash", func(t *testing.T) {
		h1 := computeContentHash("")
		h2 := computeContentHash("")
		if h1 != h2 {
			t.Errorf("empty string produced different hashes: %q vs %q", h1, h2)
		}
		if len(h1) != 16 {
			t.Errorf("expected 16-char hash, got %d chars: %q", len(h1), h1)
		}
	})

	t.Run("hash is 16 hex chars", func(t *testing.T) {
		h := computeContentHash("some terminal content with ANSI \x1b[31mcolors\x1b[0m")
		if len(h) != 16 {
			t.Errorf("expected 16-char hash, got %d chars: %q", len(h), h)
		}
	})
}

func TestDetectAgentActivity(t *testing.T) {
	orig := capturePaneFunc
	defer func() { capturePaneFunc = orig }()

	// Mock capturePaneFunc to return controllable content.
	var mu sync.Mutex
	paneContents := map[string]string{
		"%0": "line 1\nline 2\nline 3",
		"%1": "output A\noutput B",
	}
	capturePaneFunc = func(paneID string, _ int) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		return paneContents[paneID], nil
	}

	s := &Session{
		paneContentHashes: make(map[string]string),
	}

	t.Run("first scan sets unknown activity", func(t *testing.T) {
		agents := []protocol.TmuxAgentInfo{
			{PaneID: "%0", AgentName: "claude"},
			{PaneID: "%1", AgentName: "pi"},
		}
		s.detectAgentActivity(agents)
		if agents[0].Activity != "" {
			t.Errorf("agent[0].Activity = %q, want empty (first scan)", agents[0].Activity)
		}
		if agents[1].Activity != "" {
			t.Errorf("agent[1].Activity = %q, want empty (first scan)", agents[1].Activity)
		}
	})

	t.Run("unchanged content sets idle", func(t *testing.T) {
		// Same content — should be idle.
		agents := []protocol.TmuxAgentInfo{
			{PaneID: "%0", AgentName: "claude"},
			{PaneID: "%1", AgentName: "pi"},
		}
		s.detectAgentActivity(agents)
		if agents[0].Activity != "idle" {
			t.Errorf("agent[0].Activity = %q, want idle", agents[0].Activity)
		}
		if agents[1].Activity != "idle" {
			t.Errorf("agent[1].Activity = %q, want idle", agents[1].Activity)
		}
	})

	t.Run("changed content sets busy", func(t *testing.T) {
		// Change one pane's content.
		mu.Lock()
		paneContents["%0"] = "line 1\nline 2\nline 3\nnew output"
		mu.Unlock()

		agents := []protocol.TmuxAgentInfo{
			{PaneID: "%0", AgentName: "claude"},
			{PaneID: "%1", AgentName: "pi"},
		}
		s.detectAgentActivity(agents)
		if agents[0].Activity != "busy" {
			t.Errorf("agent[0].Activity = %q, want busy (content changed)", agents[0].Activity)
		}
		if agents[1].Activity != "idle" {
			t.Errorf("agent[1].Activity = %q, want idle (unchanged)", agents[1].Activity)
		}
	})

	t.Run("exited agent cleans up hash", func(t *testing.T) {
		agents := []protocol.TmuxAgentInfo{
			{PaneID: "%0", AgentName: "claude", Status: "exited"},
		}
		s.detectAgentActivity(agents)
		if agents[0].Activity != "" {
			t.Errorf("exited agent Activity = %q, want empty", agents[0].Activity)
		}
		// Hash should be cleaned up.
		s.paneContentHashesMu.RLock()
		_, exists := s.paneContentHashes["%0"]
		s.paneContentHashesMu.RUnlock()
		if exists {
			t.Error("expected hash for exited agent to be cleaned up")
		}
	})

	t.Run("new pane after cleanup starts fresh", func(t *testing.T) {
		// After cleanup, a new agent with same paneID should get unknown activity.
		agents := []protocol.TmuxAgentInfo{
			{PaneID: "%0", AgentName: "claude"},
		}
		s.detectAgentActivity(agents)
		if agents[0].Activity != "" {
			t.Errorf("agent[0].Activity = %q, want empty (fresh start after cleanup)", agents[0].Activity)
		}
	})
}
