package server

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/sync/singleflight"

	"github.com/WuErPing/solo/protocol"
)

// unicodeToASCII maps common unicode letters to ASCII equivalents for matching.
var unicodeToASCII = map[rune]string{
	'π': "pi",
	'α': "a",
	'β': "b",
}

// Subprocess timeout constants to prevent goroutine blocking on hung processes.
const (
	tmuxCommandTimeout = 5 * time.Second
	pgrepTimeout       = 2 * time.Second
	psTimeout          = 2 * time.Second
	gitTimeout         = 3 * time.Second
)

// capturePaneFlight coalesces concurrent capture-pane requests for the same pane+startLine.
var capturePaneFlight singleflight.Group

func isTmuxAIAgentName(cmd string, agentNames map[string]bool) bool {
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}
	return agentNames[cmd]
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

// agentNamesByLength returns agent names sorted longest-first so "kimi-cli" matches before "kimi".
func agentNamesByLength(agentNames map[string]bool) []string {
	names := make([]string, 0, len(agentNames))
	for name := range agentNames {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})
	return names
}

// agentNameFromTitle checks if a pane title contains a known AI agent name.
func agentNameFromTitle(title string, agentNames map[string]bool) string {
	normalized := normalizeTitleToLower(title)
	for _, name := range agentNamesByLength(agentNames) {
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

var shellCommands = map[string]bool{
	"bash": true, "zsh": true, "sh": true, "fish": true, "dash": true,
}

func isShellCommand(cmd string) bool {
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}
	return shellCommands[cmd]
}

func (s *Session) handleTmuxListAgents(m *protocol.TmuxListAgentsRequest) {
	agentNames := s.cfg.GetTmuxAgentNames()
	agents, err := scanTmuxAgents(agentNames)
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

func scanTmuxAgents(agentNames map[string]bool) ([]protocol.TmuxAgentInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_index}|#{pane_pid}|#{pane_current_command}|#{session_name}|#{window_name}|#{pane_current_path}|#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseTmuxPaneLines(string(output), agentNames), nil
}

func extractFirstPrompt(paneID string, paneTitle string, agentNames map[string]bool) string {
	for name := range agentNames {
		prefix := strings.ToUpper(name) + " | "
		if strings.HasPrefix(paneTitle, prefix) {
			return strings.TrimPrefix(paneTitle, prefix)
		}
	}

	content, err := captureTmuxPane(paneID, -50)
	if err != nil {
		return ""
	}
	return extractLastMeaningfulLine(content)
}

func extractLastMeaningfulLine(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && len(line) > 3 {
			lastLine = line
		}
	}
	if len(lastLine) > 80 {
		lastLine = lastLine[:77] + "..."
	}
	return lastLine
}

func getGitCommitHash(workingDir string) string {
	if workingDir == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", workingDir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseTmuxPaneLines(output string, agentNames map[string]bool) []protocol.TmuxAgentInfo {
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
		status := ""

		// Layer 1: Direct command match (works for claude, opencode, etc.)
		if isTmuxAIAgentName(currentCmd, agentNames) {
			agentName = currentCmd
			if idx := strings.LastIndex(agentName, "/"); idx >= 0 {
				agentName = agentName[idx+1:]
			}
		}

		// Layer 2: Title match for non-shell commands (works for pi with node, etc.)
		if agentName == "" && !isShellCommand(currentCmd) && paneTitle != "" {
			agentName = agentNameFromTitle(paneTitle, agentNames)
		}

		// Layer 3: Child process match (works when pane_current_command is node/python/etc.)
		if agentName == "" {
			agentName = agentNameFromChildProcesses(panePID, agentNames)
		}

		// Layer 4: Title-only match when command is a shell — agent exited, shell returned
		if agentName == "" && isShellCommand(currentCmd) && paneTitle != "" {
			agentName = agentNameFromTitle(paneTitle, agentNames)
			if agentName != "" {
				status = "exited"
			}
		}

		if agentName == "" {
			continue
		}

		title := ""
		if status != "exited" {
			title = extractFirstPrompt(paneID, paneTitle, agentNames)
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
			Title:       title,
			Status:      status,
		})
	}
	return agents
}

// agentNameFromChildProcesses checks if any child process of the given PID
// matches a known AI agent name. This catches cases where pane_current_command
// shows "node" or "python" but the actual agent is a child process.
// Uses batched ps call to avoid N+1 subprocess invocations.
func agentNameFromChildProcesses(ppid int, agentNames map[string]bool) string {
	ctx, cancel := context.WithTimeout(context.Background(), pgrepTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "pgrep", "-P", strconv.Itoa(ppid)).Output()
	if err != nil {
		return ""
	}
	pids := strings.Fields(strings.TrimSpace(string(out)))
	if len(pids) == 0 {
		return ""
	}

	// Query all child PIDs in a single ps call instead of one per PID.
	psCtx, psCancel := context.WithTimeout(context.Background(), psTimeout)
	defer psCancel()
	commOut, err := exec.CommandContext(psCtx, "ps", "-o", "comm=", "-p", strings.Join(pids, ",")).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(commOut)), "\n") {
		name := strings.TrimSpace(line)
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if agentNames[name] {
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

	// Coalesce concurrent requests for the same pane+startLine into a single tmux call.
	key := m.PaneID + ":" + strconv.Itoa(startLine)
	result, err, _ := capturePaneFlight.Do(key, func() (any, error) {
		content, err := captureTmuxPane(m.PaneID, startLine)
		if err != nil {
			return nil, err
		}
		return content, nil
	})
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxCapturePaneResponse(m.RequestID, "", nil, nil, &errMsg)
		return
	}
	content := result.(string)
	hash := computeContentHash(content)
	if m.LastContentHash != nil && *m.LastContentHash == hash {
		changed := false
		s.sendTmuxCapturePaneResponse(m.RequestID, "", &changed, &hash, nil)
		return
	}
	changed := true
	s.sendTmuxCapturePaneResponse(m.RequestID, content, &changed, &hash, nil)
}

func (s *Session) sendTmuxCapturePaneResponse(requestID string, content string, changed *bool, contentHash *string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxCapturePaneResponse{
		Type: "tmux/capture_pane/response",
		Payload: protocol.TmuxCapturePaneResponsePayload{
			RequestID:   requestID,
			Content:     content,
			Changed:     changed,
			ContentHash: contentHash,
			Error:       errMsg,
		},
	}))
}

// computeContentHash returns a 16-char hex SHA-256 prefix for content dedup.
func computeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8])
}

func captureTmuxPane(paneID string, startLine int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-t", paneID, "-p", "-e", "-S", strconv.Itoa(startLine)).Output()
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
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	if sendEnter {
		return exec.CommandContext(ctx, "tmux", "send-keys", "-t", paneID, keys, "Enter").Run()
	}
	return exec.CommandContext(ctx, "tmux", "send-keys", "-t", paneID, keys).Run()
}

func parseTmuxThemeOutput(options map[string]string) protocol.TmuxThemeColors {
	theme := protocol.TmuxThemeColors{}

	// Window colors: prefer active window style, fall back to default window style
	if style, ok := options["window-active-style"]; ok {
		bg, fg := parseTmuxStatusStyle(style)
		theme.Background = bg
		theme.Foreground = fg
	} else if style, ok := options["window-style"]; ok {
		bg, fg := parseTmuxStatusStyle(style)
		theme.Background = bg
		theme.Foreground = fg
	}

	// Status bar colors
	if style, ok := options["status-style"]; ok {
		bg, fg := parseTmuxStatusStyle(style)
		theme.StatusBackground = bg
		theme.StatusForeground = fg
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

func (s *Session) handleTmuxStatusLine(m *protocol.TmuxStatusLineRequest) {
	left, center, right, paneBg, paneFg, err := extractTmuxStatusLine(m.SessionID)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxStatusLineResponse(m.RequestID, "", "", "", "", "", &errMsg)
		return
	}
	s.sendTmuxStatusLineResponse(m.RequestID, left, center, right, paneBg, paneFg, nil)
}

func (s *Session) sendTmuxStatusLineResponse(requestID, statusLeft, statusCenter, statusRight, paneBg, paneFg string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxStatusLineResponse{
		Type: "tmux/status_line/response",
		Payload: protocol.TmuxStatusLineResponsePayload{
			RequestID:      requestID,
			StatusLeft:     statusLeft,
			StatusCenter:   statusCenter,
			StatusRight:    statusRight,
			PaneBackground: paneBg,
			PaneForeground: paneFg,
			Error:          errMsg,
		},
	}))
}

func extractTmuxStatusLine(sessionID string) (string, string, string, string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()

	var left, right, center, paneBg, paneFg string
	var wg sync.WaitGroup

	// Fetch and expand status-left in its own goroutine (2 sequential tmux calls)
	wg.Go(func() {
		leftFmt, err := exec.CommandContext(ctx, "tmux", "show-options", "-gv", "-t", sessionID, "status-left").Output()
		if err != nil {
			return
		}
		fmt := strings.TrimSpace(string(leftFmt))
		if fmt == "" {
			return
		}
		out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", sessionID, fmt).Output()
		if err == nil {
			left = strings.TrimSpace(string(out))
		}
	})

	// Fetch and expand status-right in its own goroutine (2 sequential tmux calls)
	wg.Go(func() {
		rightFmt, err := exec.CommandContext(ctx, "tmux", "show-options", "-gv", "-t", sessionID, "status-right").Output()
		if err != nil {
			return
		}
		fmt := strings.TrimSpace(string(rightFmt))
		if fmt == "" {
			return
		}
		out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", sessionID, fmt).Output()
		if err == nil {
			right = strings.TrimSpace(string(out))
		}
	})

	// Window list is independent — run in parallel
	wg.Go(func() {
		center = extractWindowList(sessionID)
	})

	// Pane colors are independent — run in parallel
	wg.Go(func() {
		paneBg, paneFg = extractPaneColors(sessionID)
	})

	wg.Wait()
	return left, center, right, paneBg, paneFg, nil
}

func extractWindowList(sessionID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-t", sessionID, "-F", "#{window_index}:#{window_name}#{window_flags}").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.Join(lines, " ")
}

func extractPaneColors(sessionID string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "show-options", "-gv", "-t", sessionID, "window-active-style").Output()
	if err != nil {
		return "", ""
	}
	style := strings.TrimSpace(string(out))
	bg, fg := parseTmuxStatusStyle(style)
	return bg, fg
}

func extractTmuxTheme(sessionID string) (protocol.TmuxThemeColors, error) {
	options := []string{
		"window-active-style",
		"window-style",
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
		ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
		out, err := exec.CommandContext(ctx, "tmux", "show-options", "-gv", "-t", sessionID, opt).Output()
		cancel()
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
