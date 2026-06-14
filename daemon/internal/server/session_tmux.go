package server

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	agents, otherPanes, err := scanTmuxAgents(agentNames)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxListAgentsResponse(m.RequestID, nil, nil, nil, &errMsg)
		return
	}

	// Persist command history and include it in the response.
	var history []protocol.AgentCommandEntry
	if s.cfg.SoloHome != "" {
		store := NewAgentCommandStore(s.cfg.SoloHome)
		var newEntries []AgentCommandEntry
		for _, a := range agents {
			if a.LaunchCmd != "" {
				newEntries = append(newEntries, AgentCommandEntry{
					AgentName: a.AgentName,
					LaunchCmd: a.LaunchCmd,
				})
			}
		}
		store.Merge(newEntries)
		for _, e := range store.Entries() {
			history = append(history, protocol.AgentCommandEntry{
				AgentName: e.AgentName,
				LaunchCmd: e.LaunchCmd,
				LastSeen:  e.LastSeen,
			})
		}
	}
	s.sendTmuxListAgentsResponse(m.RequestID, agents, otherPanes, history, nil)
}

func (s *Session) sendTmuxListAgentsResponse(requestID string, agents []protocol.TmuxAgentInfo, otherPanes []protocol.TmuxPaneInfo, history []protocol.AgentCommandEntry, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: protocol.TmuxListAgentsResponsePayload{
			RequestID:      requestID,
			Agents:         agents,
			OtherPanes:     otherPanes,
			CommandHistory: history,
			Error:          errMsg,
		},
	}))
}

func scanTmuxAgents(agentNames map[string]bool) ([]protocol.TmuxAgentInfo, []protocol.TmuxPaneInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_index}|#{pane_pid}|#{pane_current_command}|#{session_name}|#{window_name}|#{pane_current_path}|#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}
	agents, otherPanes := parseTmuxPaneLines(string(output), agentNames)
	return agents, otherPanes, nil
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

func parseTmuxPaneLines(output string, agentNames map[string]bool) ([]protocol.TmuxAgentInfo, []protocol.TmuxPaneInfo) {
	agents := make([]protocol.TmuxAgentInfo, 0)
	otherPanes := make([]protocol.TmuxPaneInfo, 0)
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
			otherPanes = append(otherPanes, protocol.TmuxPaneInfo{
				SessionName: sessionName,
				WindowName:  windowName,
				PaneID:      paneID,
				PaneIndex:   paneIndex,
				PanePID:     panePID,
				CurrentCmd:  currentCmd,
				WorkingDir:  workingDir,
				Title:       paneTitle,
			})
			continue
		}

		title := ""
		launchCmd := ""
		if status != "exited" {
			title = extractFirstPrompt(paneID, paneTitle, agentNames)
			launchCmd = extractAgentLaunchCmd(panePID, agentName)
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
			LaunchCmd:   launchCmd,
		})
	}
	return agents, otherPanes
}

// processNode holds a snapshot of a running process.
type processNode struct {
	pid  int
	ppid int
	comm string
	args string
}

// listProcessTree returns a snapshot of all processes with PID, PPID, command
// name, and full argument list. A single ps call avoids N+1 subprocess overhead.
func listProcessTree() ([]processNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", "pid,ppid,comm,args=").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	nodes := make([]processNode, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		comm := fields[2]
		args := ""
		if len(fields) > 3 {
			args = strings.Join(fields[3:], " ")
		}
		nodes = append(nodes, processNode{pid: pid, ppid: ppid, comm: comm, args: args})
	}
	return nodes, nil
}

// descendantProcesses returns all descendant processes of rootPID, ordered by
// depth-first traversal so closer descendants appear first.
func descendantProcesses(rootPID int, nodes []processNode) []processNode {
	children := make(map[int][]processNode, len(nodes))
	for _, n := range nodes {
		children[n.ppid] = append(children[n.ppid], n)
	}
	var out []processNode
	var dfs func(int)
	dfs = func(pid int) {
		for _, child := range children[pid] {
			out = append(out, child)
			dfs(child.pid)
		}
	}
	dfs(rootPID)
	return out
}

// processListFunc is overridable in tests to avoid real ps invocations.
var processListFunc = listProcessTree

// argsContainsAgentName checks whether any whitespace-separated token in args
// has a basename matching a known agent name. This catches wrappers like
// "node /path/to/kimi" or "python -m kimi" where the process comm is not the
// agent name.
func argsContainsAgentName(args string, agentNames map[string]bool) (string, bool) {
	for _, token := range strings.Fields(args) {
		name := token
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if agentNames[name] {
			return name, true
		}
	}
	return "", false
}

// findAgentDescendant returns the agent name, PID, and full command line of the
// first descendant of ppid whose command name matches a known agent name.
// It searches recursively so wrappers (sh -> node -> kimi) are still found.
func findAgentDescendant(ppid int, agentNames map[string]bool) (string, int, string) {
	nodes, err := processListFunc()
	if err != nil {
		return "", 0, ""
	}
	for _, n := range descendantProcesses(ppid, nodes) {
		name := n.comm
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if agentNames[name] {
			return name, n.pid, n.args
		}
		if agentName, ok := argsContainsAgentName(n.args, agentNames); ok {
			return agentName, n.pid, n.args
		}
	}
	return "", 0, ""
}

// agentNameFromChildProcesses checks if any descendant process of the given PID
// matches a known AI agent name. This catches cases where pane_current_command
// shows "node" or "python" but the actual agent is a grandchild process.
func agentNameFromChildProcesses(ppid int, agentNames map[string]bool) string {
	name, _, _ := findAgentDescendant(ppid, agentNames)
	return name
}

// extractAgentLaunchCmd returns the full command line of the agent process.
// It finds the agent's PID among the pane shell's descendants, then reads its args.
func extractAgentLaunchCmd(panePID int, agentName string) string {
	_, _, args := findAgentDescendant(panePID, map[string]bool{agentName: true})
	return args
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

// createTmuxSession creates a new detached tmux session.
func createTmuxSession(name string, workingDir *string, command *string) error {
	args := []string{"new-session", "-d", "-s", name}
	if workingDir != nil {
		args = append(args, "-c", *workingDir)
	}
	if command != nil {
		args = append(args, *command)
	}
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *Session) handleTmuxNewSession(m *protocol.TmuxNewSessionRequest) {
	err := createTmuxSession(m.Name, m.WorkingDir, m.Command)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxNewSessionResponse(m.RequestID, "", &errMsg)
		return
	}
	s.sendTmuxNewSessionResponse(m.RequestID, m.Name, nil)
}

func (s *Session) sendTmuxNewSessionResponse(requestID, sessionName string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxNewSessionResponse{
		Type: "tmux/new_session/response",
		Payload: protocol.TmuxNewSessionResponsePayload{
			RequestID:   requestID,
			SessionName: sessionName,
			Error:       errMsg,
		},
	}))
}

// killTmuxSession kills a tmux session by name.
func killTmuxSession(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *Session) handleTmuxKillSession(m *protocol.TmuxKillSessionRequest) {
	err := killTmuxSession(m.SessionName)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxKillSessionResponse(m.RequestID, &errMsg)
		return
	}
	s.sendTmuxKillSessionResponse(m.RequestID, nil)
}

func (s *Session) sendTmuxKillSessionResponse(requestID string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxKillSessionResponse{
		Type: "tmux/kill_session/response",
		Payload: protocol.TmuxKillSessionResponsePayload{
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

func (s *Session) handleTmuxStatusLine(m *protocol.TmuxStatusLineRequest) {
	left, center, right, err := extractTmuxStatusLine(m.SessionID)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxStatusLineResponse(m.RequestID, "", "", "", &errMsg)
		return
	}
	s.sendTmuxStatusLineResponse(m.RequestID, left, center, right, nil)
}

func (s *Session) sendTmuxStatusLineResponse(requestID, statusLeft, statusCenter, statusRight string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxStatusLineResponse{
		Type: "tmux/status_line/response",
		Payload: protocol.TmuxStatusLineResponsePayload{
			RequestID:    requestID,
			StatusLeft:   statusLeft,
			StatusCenter: statusCenter,
			StatusRight:  statusRight,
			Error:        errMsg,
		},
	}))
}

func extractTmuxStatusLine(sessionID string) (string, string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()

	var left, right, center string
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

	wg.Wait()
	return left, center, right, nil
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
