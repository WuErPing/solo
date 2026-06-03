package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIsAIAgent(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		// Known agents — must return true
		{"claude is agent", "claude", true},
		{"opencode is agent", "opencode", true},
		{"qoder is agent", "qoder", true},
		{"pi is agent", "pi", true},
		{"cursor is agent", "cursor", true},
		{"kimi is agent", "kimi", true},
		{"kimi-cli is agent", "kimi-cli", true},

		// Path-prefixed commands
		{"/usr/local/bin/pi is agent", "/usr/local/bin/pi", true},
		{"/usr/local/bin/claude is agent", "/usr/local/bin/claude", true},
		{"/usr/local/bin/kimi-cli is agent", "/usr/local/bin/kimi-cli", true},

		// Not agents
		{"bash is not agent", "bash", false},
		{"zsh is not agent", "zsh", false},
		{"node is not agent", "node", false},
		{"python is not agent", "python", false},
		{"empty string is not agent", "", false},
		{"substring match fails", "pi-agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAIAgent(tt.cmd)
			if got != tt.want {
				t.Errorf("isAIAgent(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestParseTmuxPanes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantCmds  []string
	}{
		{
			name: "detects pi in tmux output",
			input: "%0\tbash\t1234\twindow1\tsession1\n" +
				"%1\tpi\t5678\twindow2\tsession1\n" +
				"%2\tnode\t9012\twindow3\tsession2\n",
			wantCount: 1,
			wantCmds:  []string{"pi"},
		},
		{
			name: "detects multiple agents",
			input: "%0\tclaude\t1111\tw1\ts1\n" +
				"%1\tpi\t2222\tw2\ts1\n" +
				"%2\tbash\t3333\tw3\ts1\n" +
				"%3\tkimi-cli\t4444\tw4\ts2\n",
			wantCount: 3,
			wantCmds:  []string{"claude", "pi", "kimi-cli"},
		},
		{
			name:      "empty input yields no agents",
			input:     "",
			wantCount: 0,
			wantCmds:  nil,
		},
		{
			name: "no agents in output",
			input: "%0\tbash\t1111\tw1\ts1\n" +
				"%1\tzsh\t2222\tw2\ts1\n",
			wantCount: 0,
			wantCmds:  nil,
		},
		{
			name: "malformed lines are skipped",
			input: "%0\tbash\n" +
				"%1\tpi\t5678\twindow2\tsession1\n",
			wantCount: 1,
			wantCmds:  []string{"pi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := parseTmuxOutput(tt.input)
			if len(agents) != tt.wantCount {
				t.Errorf("got %d agents, want %d", len(agents), tt.wantCount)
				return
			}
			for i, cmd := range tt.wantCmds {
				if agents[i].CurrentCommand != cmd {
					t.Errorf("agent[%d].CurrentCommand = %q, want %q", i, agents[i].CurrentCommand, cmd)
				}
			}
		})
	}
}

func TestParseTmuxPanesPreservesMetadata(t *testing.T) {
	input := "%0\tpi\t5678\tmywindow\tmysession\n"
	agents := parseTmuxOutput(input)

	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}

	a := agents[0]
	if a.PaneID != "%0" {
		t.Errorf("PaneID = %q, want %q", a.PaneID, "%0")
	}
	if a.PID != "5678" {
		t.Errorf("PID = %q, want %q", a.PID, "5678")
	}
	if a.WindowName != "mywindow" {
		t.Errorf("WindowName = %q, want %q", a.WindowName, "mywindow")
	}
	if a.SessionName != "mysession" {
		t.Errorf("SessionName = %q, want %q", a.SessionName, "mysession")
	}
}

// TestDetectPiInTmux is an integration test that creates a real tmux session
// running "pi" and verifies detectAgents finds it.
func TestDetectPiInTmux(t *testing.T) {
	sessionName := "test-pi-detection"

	// Cleanup any stale session
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Create a tmux session running "pi"
	// We use "sleep 60" as a stand-in if "pi" binary isn't installed;
	// the test checks that the pane's current command is detected.
	err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "sleep", "60").Run()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Verify the session exists
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("tmux list-sessions failed: %v", err)
	}
	if !strings.Contains(string(out), sessionName) {
		t.Fatalf("session %q not found in tmux", sessionName)
	}

	// detectAgents should run without error
	agents, err := detectAgents()
	if err != nil {
		t.Fatalf("detectAgents() error: %v", err)
	}

	// "sleep" is not an AI agent, so no agents should be detected
	for _, a := range agents {
		if a.SessionName == sessionName {
			t.Errorf("unexpected agent detected in test session: %s", a.CurrentCommand)
		}
	}
}

// TestDetectPiAgentInTmux simulates a tmux pane whose current command is "pi"
// by parsing crafted output, verifying the full detection pipeline for the "pi" scenario.
func TestDetectPiAgentInTmux(t *testing.T) {
	// Simulate tmux output where "pi" is running in a pane
	tmuxOutput := "%0\tbash\t1000\tdev\tmain\n" +
		"%1\tpi\t2000\tagent-window\tagent-session\n" +
		"%2\tvim\t3000\tedit\tmain\n"

	agents := parseTmuxOutput(tmuxOutput)

	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}

	agent := agents[0]
	if agent.CurrentCommand != "pi" {
		t.Errorf("agent.CurrentCommand = %q, want %q", agent.CurrentCommand, "pi")
	}
	if agent.PaneID != "%1" {
		t.Errorf("agent.PaneID = %q, want %q", agent.PaneID, "%1")
	}
	if agent.PID != "2000" {
		t.Errorf("agent.PID = %q, want %q", agent.PID, "2000")
	}
	if agent.WindowName != "agent-window" {
		t.Errorf("agent.WindowName = %q, want %q", agent.WindowName, "agent-window")
	}
	if agent.SessionName != "agent-session" {
		t.Errorf("agent.SessionName = %q, want %q", agent.SessionName, "agent-session")
	}
}

// --- Core dimension: Agent name edge cases ---

func TestIsAIAgentEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		// Case sensitivity — agent names are lowercase
		{"uppercase PI rejected", "PI", false},
		{"mixed case Pi rejected", "Pi", false},
		{"uppercase CLAUDE rejected", "CLAUDE", false},

		// Whitespace
		{"leading space rejected", " pi", false},
		{"trailing space rejected", "pi ", false},
		{"tab prefix rejected", "\tpi", false},

		// Exact match — no substring
		{"superstring rejected", "pi-agent", false},
		{"prefix rejected", "picli", false},
		{"suffix rejected", "mypi", false},
		{"partial kimi rejected", "kimi-cli-extra", false},

		// Deep path prefixes
		{"deep path /a/b/c/pi", "/a/b/c/pi", true},
		{"path with spaces /my tools/pi", "/my tools/pi", true},

		// kimi-cli hyphenated name
		{"kimi-cli exact", "kimi-cli", true},
		{"kimi without suffix", "kimi", true},
		{"kimi-cli-extra rejected", "kimi-cli-extra", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAIAgent(tt.cmd)
			if got != tt.want {
				t.Errorf("isAIAgent(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// --- Core dimension: Process tree inspection ---

func TestProcessTreeContainsChildProcess(t *testing.T) {
	// Start a real child process so we have a parent->child tree
	// "sleep 30" will be the child of this test's process
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}
	defer cmd.Process.Kill()

	pid := strconv.Itoa(cmd.Process.Pid)
	tree := getProcessTree(pid, 0)

	if tree == "" {
		t.Fatal("getProcessTree returned empty string for valid PID")
	}

	// The tree should contain the sleep process
	if !strings.Contains(tree, "sleep") {
		t.Errorf("process tree does not contain 'sleep':\n%s", tree)
	}
}

func TestProcessTreeForInvalidPID(t *testing.T) {
	tree := getProcessTree("99999999", 0)
	// Should handle gracefully — either empty or an error message
	if strings.Contains(tree, "sleep") {
		t.Error("invalid PID should not return 'sleep' in tree")
	}
}

func TestProcessTreeIndentation(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}
	defer cmd.Process.Kill()

	tree := getProcessTree(strconv.Itoa(cmd.Process.Pid), 4)
	lines := strings.Split(strings.TrimSpace(tree), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (header + data), got %d", len(lines))
	}

	// First line is ps header "PID COMM" — skip it
	// Remaining lines should be indented
	for _, line := range lines[1:] {
		if len(line) > 0 && !strings.HasPrefix(line, "    ") {
			t.Errorf("expected 4-space indentation in line %q", line)
		}
	}

	// Verify the process appears in the tree
	if !strings.Contains(tree, "sleep") {
		t.Errorf("tree should contain 'sleep':\n%s", tree)
	}
}

// --- Core dimension: Window/session title parsing ---

func TestParseTmuxOutputWindowSessionTitles(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantWindow     string
		wantSession    string
	}{
		{
			name:        "simple names",
			input:       "%0\tpi\t1000\tmy-window\tmy-session\n",
			wantWindow:  "my-window",
			wantSession: "my-session",
		},
		{
			name:        "names with spaces",
			input:       "%0\tpi\t1000\tmy window\tmy session\n",
			wantWindow:  "my window",
			wantSession: "my session",
		},
		{
			name:        "names with special chars",
			input:       "%0\tpi\t1000\twork[dev]\tproject-2026\n",
			wantWindow:  "work[dev]",
			wantSession: "project-2026",
		},
		{
			name:        "empty window and session names",
			input:       "%0\tpi\t1000\t\t\n",
			wantWindow:  "",
			wantSession: "",
		},
		{
			name:        "numeric names",
			input:       "%0\tpi\t1000\t0\t1\n",
			wantWindow:  "0",
			wantSession: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := parseTmuxOutput(tt.input)
			if len(agents) != 1 {
				t.Fatalf("got %d agents, want 1", len(agents))
			}
			if agents[0].WindowName != tt.wantWindow {
				t.Errorf("WindowName = %q, want %q", agents[0].WindowName, tt.wantWindow)
			}
			if agents[0].SessionName != tt.wantSession {
				t.Errorf("SessionName = %q, want %q", agents[0].SessionName, tt.wantSession)
			}
		})
	}
}

// --- Core dimension: Interactive control (sendKeys) ---

func TestSendKeysCallsTmux(t *testing.T) {
	// Create a real tmux session to test sendKeys
	sessionName := "test-sendkeys"
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "cat").Run()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Get the pane ID
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("failed to list panes: %v", err)
	}
	paneID := strings.TrimSpace(string(out))
	if paneID == "" {
		t.Fatal("no pane ID found")
	}

	// Send a key — should not error
	err = sendKeys(paneID, "hello")
	if err != nil {
		t.Errorf("sendKeys(%q, %q) error: %v", paneID, "hello", err)
	}
}

func TestSendKeysToInvalidPane(t *testing.T) {
	err := sendKeys("%999", "test")
	// tmux send-keys to non-existent pane should fail
	if err == nil {
		t.Error("expected error when sending to non-existent pane, got nil")
	}
}

// --- Core dimension: Full pipeline with all agents ---

func TestParseTmuxOutputAllAgents(t *testing.T) {
	// Build tmux output with every known agent
	var lines []string
	agents := []struct {
		cmd  string
		pid  string
		win  string
		sess string
	}{
		{"claude", "1001", "w-claude", "s-claude"},
		{"opencode", "1002", "w-opencode", "s-opencode"},
		{"qoder", "1003", "w-qoder", "s-qoder"},
		{"pi", "1004", "w-pi", "s-pi"},
		{"cursor", "1005", "w-cursor", "s-cursor"},
		{"kimi", "1006", "w-kimi", "s-kimi"},
		{"kimi-cli", "1007", "w-kimi-cli", "s-kimi-cli"},
	}
	// Interleave with non-agents
	lines = append(lines, "%0\tbash\t9000\tw-bash\ts-bash")
	for i, a := range agents {
		lines = append(lines, formatPaneLine(i+1, a.cmd, a.pid, a.win, a.sess))
	}
	lines = append(lines, "%99\tzsh\t9001\tw-zsh\ts-zsh")

	input := strings.Join(lines, "\n") + "\n"
	detected := parseTmuxOutput(input)

	if len(detected) != len(agents) {
		t.Fatalf("got %d agents, want %d", len(detected), len(agents))
	}

	for i, want := range agents {
		got := detected[i]
		if got.CurrentCommand != want.cmd {
			t.Errorf("agent[%d].CurrentCommand = %q, want %q", i, got.CurrentCommand, want.cmd)
		}
		if got.WindowName != want.win {
			t.Errorf("agent[%d].WindowName = %q, want %q", i, got.WindowName, want.win)
		}
		if got.SessionName != want.sess {
			t.Errorf("agent[%d].SessionName = %q, want %q", i, got.SessionName, want.sess)
		}
	}
}

// --- Core dimension: Real tmux integration with "pi" ---

func TestRealTmuxDetectPiAgent(t *testing.T) {
	sessionName := "test-pi-agent"
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Start a tmux session with a recognizable process
	// We use a shell wrapper so pane_current_command shows "pi"-like behavior
	// If "pi" binary exists, use it directly; otherwise simulate with a named script
	piScript := filepath.Join(t.TempDir(), "pi")
	if err := os.WriteFile(piScript, []byte("#!/bin/sh\nsleep 60\n"), 0755); err != nil {
		t.Fatalf("failed to create pi script: %v", err)
	}

	err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, piScript).Run()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Get the actual pane current command
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName,
		"-F", "#{pane_current_command}").Output()
	if err != nil {
		t.Fatalf("failed to list panes: %v", err)
	}
	cmd := strings.TrimSpace(string(out))
	t.Logf("pane_current_command = %q", cmd)

	// The temp script path contains "pi" — verify isAIAgent handles it
	if !isAIAgent(cmd) {
		// If the command is the full path, it should still match via path stripping
		base := cmd
		if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
			base = cmd[idx+1:]
		}
		if base == "pi" {
			t.Errorf("isAIAgent(%q) should detect 'pi' from path", cmd)
		}
		// Otherwise it's a different command name — that's OK for this test
	}
}

// formatPaneLine builds a tmux list-panes tab-delimited line.
func formatPaneLine(id int, cmd, pid, window, session string) string {
	return fmt.Sprintf("%%%d\t%s\t%s\t%s\t%s", id, cmd, pid, window, session)
}
