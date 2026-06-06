package server

import (
	"bufio"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/WuErPing/solo/protocol"
)

var tmuxAIAgentNames = map[string]bool{
	"claude":   true,
	"opencode": true,
	"qodercli": true,
	"pi":       true,
	"cursor":   true,
	"kimi":     true,
	"kimi-cli": true,
}

// unicodeToASCII maps common unicode letters to ASCII equivalents for matching.
var unicodeToASCII = map[rune]string{
	'π': "pi",
	'α': "a",
	'β': "b",
}

func isTmuxAIAgentName(cmd string) bool {
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}
	return tmuxAIAgentNames[cmd]
}

// normalizeTitleToLower strips non-alphanumeric chars and lowercases for matching.
func normalizeTitleToLower(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if replacement, ok := unicodeToASCII[r]; ok {
			b.WriteString(replacement)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// tmuxAIAgentNamesByLength is sorted longest-first so "kimi-cli" matches before "kimi".
var tmuxAIAgentNamesByLength []string

func init() {
	for name := range tmuxAIAgentNames {
		tmuxAIAgentNamesByLength = append(tmuxAIAgentNamesByLength, name)
	}
	sort.Slice(tmuxAIAgentNamesByLength, func(i, j int) bool {
		return len(tmuxAIAgentNamesByLength[i]) > len(tmuxAIAgentNamesByLength[j])
	})
}

// agentNameFromTitle checks if a pane title contains a known AI agent name.
func agentNameFromTitle(title string) string {
	normalized := normalizeTitleToLower(title)
	for _, name := range tmuxAIAgentNamesByLength {
		// Match whole word: check that name appears surrounded by non-alphanum
		idx := strings.Index(normalized, name)
		if idx < 0 {
			continue
		}
		// Check word boundary: char before and after must not be letter/digit
		beforeOK := idx == 0 || !isAlnum(rune(normalized[idx-1]))
		afterIdx := idx + len(name)
		afterOK := afterIdx >= len(normalized) || !isAlnum(rune(normalized[afterIdx]))
		if beforeOK && afterOK {
			return name
		}
	}
	return ""
}

func isAlnum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (s *Session) handleTmuxListAgents(m *protocol.TmuxListAgentsRequest) {
	agents, err := scanTmuxAgents()
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxListAgentsResponse(m.RequestID, nil, &errMsg)
		return
	}
	s.sendTmuxListAgentsResponse(m.RequestID, agents, nil)
}

func (s *Session) sendTmuxListAgentsResponse(requestID string, agents []protocol.TmuxAgentInfo, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: protocol.TmuxListAgentsResponsePayload{
			RequestID: requestID,
			Agents:    agents,
			Error:     errMsg,
		},
	}))
}

func scanTmuxAgents() ([]protocol.TmuxAgentInfo, error) {
	cmd := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_index}|#{pane_pid}|#{pane_current_command}|#{session_name}|#{window_name}|#{pane_current_path}|#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseTmuxPaneLines(string(output)), nil
}

func parseTmuxPaneLines(output string) []protocol.TmuxAgentInfo {
	var agents []protocol.TmuxAgentInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "|")
		if len(parts) < 7 {
			continue
		}
		paneID := parts[0]
		paneIndex, _ := strconv.Atoi(parts[1])
		panePID, _ := strconv.Atoi(parts[2])
		currentCmd := parts[3]
		sessionName := parts[4]
		windowName := parts[5]
		workingDir := parts[6]
		paneTitle := ""
		if len(parts) >= 8 {
			paneTitle = parts[7]
		}

		agentName := ""

		// Layer 1: Direct command match (works for claude, opencode, etc.)
		if isTmuxAIAgentName(currentCmd) {
			agentName = currentCmd
			if idx := strings.LastIndex(agentName, "/"); idx >= 0 {
				agentName = agentName[idx+1:]
			}
		}

		// Layer 2: Title match (works for pi, which sets title to "π - solo")
		if agentName == "" && paneTitle != "" {
			agentName = agentNameFromTitle(paneTitle)
		}

		// Layer 3: Child process match (works when pane_current_command is node/python/etc.)
		if agentName == "" {
			agentName = agentNameFromChildProcesses(panePID)
		}

		if agentName == "" {
			continue
		}

		agents = append(agents, protocol.TmuxAgentInfo{
			SessionName: sessionName,
			WindowName:  windowName,
			PaneID:      paneID,
			PaneIndex:   paneIndex,
			PanePID:     panePID,
			AgentName:   agentName,
			CurrentCmd:  currentCmd,
			WorkingDir:  workingDir,
		})
	}
	return agents
}

// agentNameFromChildProcesses checks if any child process of the given PID
// matches a known AI agent name. This catches cases where pane_current_command
// shows "node" or "python" but the actual agent is a child process.
func agentNameFromChildProcesses(ppid int) string {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(ppid)).Output()
	if err != nil {
		return ""
	}
	for _, pidStr := range strings.Fields(strings.TrimSpace(string(out))) {
		comm, err := exec.Command("ps", "-o", "comm=", "-p", pidStr).Output()
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(comm))
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if tmuxAIAgentNames[name] {
			return name
		}
	}
	return ""
}

func (s *Session) handleTmuxCapturePane(m *protocol.TmuxCapturePaneRequest) {
	startLine := -200
	if m.StartLine != nil {
		startLine = *m.StartLine
	}
	content, err := captureTmuxPane(m.PaneID, startLine)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxCapturePaneResponse(m.RequestID, "", &errMsg)
		return
	}
	s.sendTmuxCapturePaneResponse(m.RequestID, content, nil)
}

func (s *Session) sendTmuxCapturePaneResponse(requestID string, content string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxCapturePaneResponse{
		Type: "tmux/capture_pane/response",
		Payload: protocol.TmuxCapturePaneResponsePayload{
			RequestID: requestID,
			Content:   content,
			Error:     errMsg,
		},
	}))
}

func captureTmuxPane(paneID string, startLine int) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", paneID, "-p", "-e", "-S", strconv.Itoa(startLine)).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s *Session) handleTmuxSendKeys(m *protocol.TmuxSendKeysRequest) {
	sendEnter := m.SendEnter == nil || *m.SendEnter
	err := sendKeysToTmuxPane(m.PaneID, m.Keys, sendEnter)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxSendKeysResponse(m.RequestID, &errMsg)
		return
	}
	s.sendTmuxSendKeysResponse(m.RequestID, nil)
}

func (s *Session) sendTmuxSendKeysResponse(requestID string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxSendKeysResponse{
		Type: "tmux/send_keys/response",
		Payload: protocol.TmuxSendKeysResponsePayload{
			RequestID: requestID,
			Error:     errMsg,
		},
	}))
}

func sendKeysToTmuxPane(paneID, keys string, sendEnter bool) error {
	if sendEnter {
		return exec.Command("tmux", "send-keys", "-t", paneID, keys, "Enter").Run()
	}
	return exec.Command("tmux", "send-keys", "-t", paneID, keys).Run()
}

func parseTmuxThemeOutput(options map[string]string) protocol.TmuxThemeColors {
	theme := protocol.TmuxThemeColors{}

	if style, ok := options["status-style"]; ok {
		bg, fg := parseTmuxStatusStyle(style)
		theme.StatusBackground = bg
		theme.StatusForeground = fg
		theme.Background = bg
		theme.Foreground = fg
	}

	if v, ok := options["message-bg"]; ok && v != "" {
		theme.MessageBackground = v
	}
	if v, ok := options["message-fg"]; ok && v != "" {
		theme.MessageForeground = v
	}
	if v, ok := options["message-command-bg"]; ok && v != "" && theme.MessageBackground == "" {
		theme.MessageBackground = v
	}
	if v, ok := options["message-command-fg"]; ok && v != "" && theme.MessageForeground == "" {
		theme.MessageForeground = v
	}

	if v, ok := options["pane-active-border-style"]; ok && v != "" {
		theme.PaneActiveBorder = v
	}
	if v, ok := options["pane-border-style"]; ok && v != "" {
		theme.PaneInactiveBorder = v
	}

	if v, ok := options["window-status-current-bg"]; ok && v != "" {
		theme.WindowStatusCurrentBg = v
	}
	if v, ok := options["window-status-current-fg"]; ok && v != "" {
		theme.WindowStatusCurrentFg = v
	}

	return theme
}

func parseTmuxStatusStyle(style string) (bg, fg string) {
	parts := strings.Split(style, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "bg=") {
			bg = strings.TrimPrefix(part, "bg=")
		} else if strings.HasPrefix(part, "fg=") {
			fg = strings.TrimPrefix(part, "fg=")
		}
	}
	return bg, fg
}

func (s *Session) handleTmuxGetTheme(m *protocol.TmuxGetThemeRequest) {
	theme, err := extractTmuxTheme(m.SessionID)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxGetThemeResponse(m.RequestID, protocol.TmuxThemeColors{}, &errMsg)
		return
	}
	s.sendTmuxGetThemeResponse(m.RequestID, theme, nil)
}

func (s *Session) sendTmuxGetThemeResponse(requestID string, theme protocol.TmuxThemeColors, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxGetThemeResponse{
		Type: "tmux/get_theme/response",
		Payload: protocol.TmuxGetThemeResponsePayload{
			RequestID: requestID,
			Theme:     theme,
			Error:     errMsg,
		},
	}))
}

func extractTmuxTheme(sessionID string) (protocol.TmuxThemeColors, error) {
	options := []string{
		"status-style",
		"message-bg",
		"message-fg",
		"message-command-bg",
		"message-command-fg",
		"pane-active-border-style",
		"pane-border-style",
		"window-status-current-bg",
		"window-status-current-fg",
	}

	result := make(map[string]string)
	for _, opt := range options {
		out, err := exec.Command("tmux", "show-options", "-gv", "-t", sessionID, opt).Output()
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(out))
		if val != "" {
			result[opt] = val
		}
	}

	return parseTmuxThemeOutput(result), nil
}
