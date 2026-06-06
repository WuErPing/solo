package server

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

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
			got := isTmuxAIAgentName(tt.cmd)
			if got != tt.want {
				t.Errorf("isTmuxAIAgentName(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
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
			name: "path prefix stripped",
			input: "%0|0|1000|/usr/local/bin/claude|s1|w1|/tmp\n",
			wantCount: 1,
			wantNames: []string{"claude"},
		},
		{
			name: "all seven agents",
			input: "%0|0|1000|claude|s1|w1|/a\n" +
				"%1|1|2000|opencode|s1|w1|/b\n" +
				"%2|2|3000|qodercli|s1|w1|/c\n" +
				"%3|0|4000|pi|s2|w1|/d\n" +
				"%4|1|5000|cursor|s2|w1|/e\n" +
				"%5|2|6000|kimi|s2|w1|/f\n" +
				"%6|0|7000|kimi-cli|s3|w1|/g\n",
			wantCount: 7,
			wantNames: []string{"claude", "opencode", "qodercli", "pi", "cursor", "kimi", "kimi-cli"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := parseTmuxPaneLines(tt.input)
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
			got := agentNameFromTitle(tt.title)
			if got != tt.want {
				t.Errorf("agentNameFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestParseTmuxPaneLinesTitleDetection(t *testing.T) {
	// pi detected via pane_title (pane_current_command is "node")
	input := "%0|0|86415|node|0|node|/home/user|π - solo\n"
	agents := parseTmuxPaneLines(input)
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
	agents := parseTmuxPaneLines(input)
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

func TestParseTmuxPaneLinesMetadata(t *testing.T) {
	input := "%5|2|9876|pi|my-session|my-window|/Users/me/code\n"
	agents := parseTmuxPaneLines(input)
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

func TestParseTmuxThemeOutput(t *testing.T) {
	tests := []struct {
		name   string
		output map[string]string
		want   protocol.TmuxThemeColors
	}{
		{
			name: "full theme",
			output: map[string]string{
				"message-bg":              "#1e1e2e",
				"message-fg":              "#cdd6f4",
				"pane-active-border-style": "#89b4fa",
				"pane-border-style":       "#45475a",
				"status-style":            "bg=#181825,fg=#cdd6f4",
				"message-command-bg":      "#1e1e2e",
				"message-command-fg":      "#cdd6f4",
				"window-status-current-bg": "#585b70",
				"window-status-current-fg": "#cdd6f4",
			},
			want: protocol.TmuxThemeColors{
				Background:            "#181825",
				Foreground:            "#cdd6f4",
				MessageBackground:     "#1e1e2e",
				MessageForeground:     "#cdd6f4",
				PaneActiveBorder:      "#89b4fa",
				PaneInactiveBorder:    "#45475a",
				StatusBackground:      "#181825",
				StatusForeground:      "#cdd6f4",
				WindowStatusCurrentBg: "#585b70",
				WindowStatusCurrentFg: "#cdd6f4",
			},
		},
		{
			name: "minimal theme with hex colors",
			output: map[string]string{
				"status-style": "bg=#000000,fg=#ffffff",
			},
			want: protocol.TmuxThemeColors{
				Background:       "#000000",
				Foreground:       "#ffffff",
				StatusBackground: "#000000",
				StatusForeground: "#ffffff",
			},
		},
		{
			name: "theme with named colors",
			output: map[string]string{
				"status-style": "bg=black,fg=white",
			},
			want: protocol.TmuxThemeColors{
				Background:       "black",
				Foreground:       "white",
				StatusBackground: "black",
				StatusForeground: "white",
			},
		},
		{
			name:   "empty output",
			output: map[string]string{},
			want:   protocol.TmuxThemeColors{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTmuxThemeOutput(tt.output)
			if got != tt.want {
				t.Errorf("parseTmuxThemeOutput() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseTmuxStatusStyle(t *testing.T) {
	tests := []struct {
		name      string
		style     string
		wantBg    string
		wantFg    string
	}{
		{
			name:   "bg and fg",
			style:  "bg=#181825,fg=#cdd6f4",
			wantBg: "#181825",
			wantFg: "#cdd6f4",
		},
		{
			name:   "fg only",
			style:  "fg=#ffffff",
			wantBg: "",
			wantFg: "#ffffff",
		},
		{
			name:   "bg only",
			style:  "bg=#000000",
			wantBg: "#000000",
			wantFg: "",
		},
		{
			name:   "with spaces",
			style:  "bg=#181825, fg=#cdd6f4",
			wantBg: "#181825",
			wantFg: "#cdd6f4",
		},
		{
			name:   "empty",
			style:  "",
			wantBg: "",
			wantFg: "",
		},
		{
			name:   "plain color",
			style:  "bg=black,fg=white",
			wantBg: "black",
			wantFg: "white",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBg, gotFg := parseTmuxStatusStyle(tt.style)
			if gotBg != tt.wantBg {
				t.Errorf("parseTmuxStatusStyle(%q) bg = %q, want %q", tt.style, gotBg, tt.wantBg)
			}
			if gotFg != tt.wantFg {
				t.Errorf("parseTmuxStatusStyle(%q) fg = %q, want %q", tt.style, gotFg, tt.wantFg)
			}
		})
	}
}
