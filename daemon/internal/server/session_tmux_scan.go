package server

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/WuErPing/solo/protocol"
)

// Subprocess timeout constants to prevent goroutine blocking on hung processes.
const (
	tmuxCommandTimeout = 5 * time.Second
	psTimeout          = 2 * time.Second
	gitTimeout         = 3 * time.Second
)

// unicodeToASCII maps common unicode letters to ASCII equivalents for matching.
var unicodeToASCII = map[rune]string{
	'π': "pi",
	'α': "a",
	'β': "b",
}

func isTmuxAIAgentName(cmd string, agentNames map[string]bool) bool {
	return matchAgentCommand(cmd, agentNames) != ""
}

// matchAgentCommand checks if cmd matches a known AI agent name.
// Returns the matched agent name, or "" if no match.
// Handles version-suffixed commands like "qodercli-1.0.22" → "qodercli".
func matchAgentCommand(cmd string, agentNames map[string]bool) string {
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}
	if agentNames[cmd] {
		return cmd
	}
	for _, name := range agentNamesByLength(agentNames) {
		if !strings.HasPrefix(cmd, name) {
			continue
		}
		rest := cmd[len(name):]
		if len(rest) == 0 {
			return name
		}
		if isDigit(rest[0]) || (rest[0] == '-' && len(rest) > 1 && isDigit(rest[1])) {
			return name
		}
	}
	return ""
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

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

// detectAgentActivity compares pane content hashes between consecutive scans
// to determine if an agent is busy (content changed) or idle (content unchanged).
// When content changes, it also records the timestamp for sorting.
func (s *Session) detectAgentActivity(agents []protocol.TmuxAgentInfo) {
	s.paneContentHashesMu.Lock()
	defer s.paneContentHashesMu.Unlock()

	for i := range agents {
		a := &agents[i]
		if a.Status == "exited" {
			delete(s.paneContentHashes, a.PaneID)
			delete(s.paneLastContentChange, a.PaneID)
			continue
		}

		// Use the hash-based timestamp if available.
		if ts, ok := s.paneLastContentChange[a.PaneID]; ok {
			a.LastContentChange = ts
		}

		content, _, err := capturePaneFunc(a.PaneID, -10, 0)
		if err != nil {
			s.logger.Warn("tmux activity detection: capture failed", "paneID", a.PaneID, "error", err)
			continue
		}

		hash := computeContentHash(content)
		prevHash, exists := s.paneContentHashes[a.PaneID]
		s.paneContentHashes[a.PaneID] = hash

		if !exists {
			a.Activity = ""
			// First scan: initialize timestamp so the card shows a time immediately.
			now := time.Now().Unix()
			s.paneLastContentChange[a.PaneID] = now
			a.LastContentChange = now
		} else if hash != prevHash {
			a.Activity = "busy"
			now := time.Now().Unix()
			s.paneLastContentChange[a.PaneID] = now
			a.LastContentChange = now
		} else {
			a.Activity = "idle"
		}
		a.LastContentChangeHHMM = formatUnixHHMM(a.LastContentChange)
		a.LastContentChangeAgo = formatWindowActivity(a.LastContentChange)
	}
}

// prunePaneActivityState drops hash-map entries for panes that vanished since
// the previous scan (e.g. killed sessions), so they don't leak for the lifetime
// of the WS session. Entries for panes in the current scan are kept.
func (s *Session) prunePaneActivityState(agents []protocol.TmuxAgentInfo, panes []protocol.TmuxPaneInfo) {
	s.paneContentHashesMu.Lock()
	defer s.paneContentHashesMu.Unlock()

	alive := make(map[string]bool, len(agents)+len(panes))
	for _, a := range agents {
		alive[a.PaneID] = true
	}
	for _, p := range panes {
		alive[p.PaneID] = true
	}
	for id := range s.paneContentHashes {
		if !alive[id] {
			delete(s.paneContentHashes, id)
		}
	}
	for id := range s.paneLastContentChange {
		if !alive[id] {
			delete(s.paneLastContentChange, id)
		}
	}
}

// filterWindowActivity deduplicates tmux window_activity noise from status bar redraws.
// When consecutive values differ by < 3 seconds, it keeps the previous timestamp.
func (s *Session) filterWindowActivity(panes []protocol.TmuxPaneInfo) {
	s.paneContentHashesMu.Lock()
	defer s.paneContentHashesMu.Unlock()

	for i := range panes {
		p := &panes[i]
		raw := p.LastContentChange
		if raw == 0 {
			continue
		}
		prev, exists := s.paneLastContentChange[p.PaneID]
		if exists && raw-prev < 3 && raw-prev > -3 {
			// Noise from status bar redraw — keep previous timestamp.
			p.LastContentChange = prev
		} else {
			s.paneLastContentChange[p.PaneID] = raw
		}
		p.LastContentChangeHHMM = formatUnixHHMM(p.LastContentChange)
		p.LastContentChangeAgo = formatWindowActivity(p.LastContentChange)
	}
}

func scanTmuxAgents(agentNames map[string]bool) ([]protocol.TmuxAgentInfo, []protocol.TmuxPaneInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_index}|#{pane_pid}|#{pane_current_command}|#{session_name}|#{window_name}|#{pane_current_path}|#{pane_title}|#{window_activity}")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, friendlyTmuxError(err)
	}
	agents, otherPanes := parseTmuxPaneLines(string(output), agentNames)
	return agents, otherPanes, nil
}

// friendlyTmuxError converts raw exec errors into user-facing messages.
// Without this, a missing or not-running tmux shows as the cryptic "exit status 1".
func friendlyTmuxError(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("tmux is not installed or not in PATH")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return errors.New(stderr)
		}
		return errors.New("tmux server is not running — start a tmux session first")
	}
	return err
}

func extractFirstPrompt(paneID string, paneTitle string, agentNames map[string]bool) string {
	for name := range agentNames {
		prefix := strings.ToUpper(name) + " | "
		if strings.HasPrefix(paneTitle, prefix) {
			return strings.TrimPrefix(paneTitle, prefix)
		}
	}

	content, _, err := captureTmuxPane(paneID, -50, 0)
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

func parseTmuxPaneLines(output string, agentNames map[string]bool) ([]protocol.TmuxAgentInfo, []protocol.TmuxPaneInfo) {
	agents := make([]protocol.TmuxAgentInfo, 0)
	otherPanes := make([]protocol.TmuxPaneInfo, 0)

	// The ps snapshot is loaded lazily and shared across the whole scan — one
	// subprocess per scan instead of one per pane/agent lookup.
	var processTree []processNode
	processTreeLoaded := false
	getProcessTree := func() []processNode {
		if !processTreeLoaded {
			processTreeLoaded = true
			if nodes, err := processListFunc(); err == nil {
				processTree = nodes
			}
		}
		return processTree
	}

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
		var lastContentChange int64
		if len(parts) >= 9 {
			lastContentChange, _ = strconv.ParseInt(parts[8], 10, 64)
		}

		agentName := ""
		status := ""

		// Layer 1: Direct command match (works for claude, opencode, qodercli-1.0.22, etc.)
		if matched := matchAgentCommand(currentCmd, agentNames); matched != "" {
			agentName = matched
		}

		// Layer 2: Title match for non-shell commands (works for pi with node, etc.)
		if agentName == "" && !isShellCommand(currentCmd) && paneTitle != "" {
			agentName = agentNameFromTitle(paneTitle, agentNames)
		}

		// Layer 3: Child process match (works when pane_current_command is node/python/etc.)
		if agentName == "" {
			agentName = agentNameFromChildProcesses(getProcessTree(), panePID, agentNames)
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
				SessionName:       sessionName,
				WindowName:        windowName,
				PaneID:            paneID,
				PaneIndex:         paneIndex,
				PanePID:           panePID,
				CurrentCmd:        currentCmd,
				WorkingDir:        workingDir,
				Title:             paneTitle,
				LastContentChange: lastContentChange,
			})
			continue
		}

		title := ""
		launchCmd := ""
		if status != "exited" {
			title = extractFirstPrompt(paneID, paneTitle, agentNames)
			launchCmd = extractAgentLaunchCmd(getProcessTree(), panePID, agentName, agentNames)
		}

		agents = append(agents, protocol.TmuxAgentInfo{
			SessionName:       sessionName,
			WindowName:        windowName,
			PaneID:            paneID,
			PaneIndex:         paneIndex,
			PanePID:           panePID,
			AgentName:         agentName,
			CurrentCmd:        currentCmd,
			WorkingDir:        workingDir,
			Title:             title,
			Status:            status,
			LaunchCmd:         launchCmd,
			LastContentChange: lastContentChange,
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
// We use "args" instead of "comm" because on macOS "comm" is truncated to 16
// chars when combined with "args"; argv[0] always contains the full path/name.
func listProcessTree() ([]processNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", "pid,ppid,args=").Output()
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
		// argv[0] is the invoked command; derive comm from it to avoid truncation.
		comm := fields[2]
		if idx := strings.LastIndex(comm, "/"); idx >= 0 {
			comm = comm[idx+1:]
		}
		args := strings.Join(fields[2:], " ")
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
// The caller supplies the process snapshot so a scan shares a single ps call.
// Matching order per descendant:
//  1. exact comm match against agentNames
//  2. any agent name token in args (wrappers: "node kimi")
func findAgentDescendant(nodes []processNode, ppid int, agentNames map[string]bool) (string, int, string) {
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
func agentNameFromChildProcesses(nodes []processNode, ppid int, agentNames map[string]bool) string {
	name, _, _ := findAgentDescendant(nodes, ppid, agentNames)
	return name
}

// extractAgentLaunchCmd returns the full command line of the agent process
// from ps output. For wrapper scripts that rewrite argv (e.g. cursor-agent),
// this reports the wrapper-injected args rather than the user's original
// command — that trade-off is documented and accepted.
func extractAgentLaunchCmd(nodes []processNode, panePID int, agentName string, agentNames map[string]bool) string {
	if _, _, a := findAgentDescendant(nodes, panePID, map[string]bool{agentName: true}); a != "" {
		return a
	}
	if _, _, a := findAgentDescendant(nodes, panePID, agentNames); a != "" {
		return a
	}
	return ""
}
