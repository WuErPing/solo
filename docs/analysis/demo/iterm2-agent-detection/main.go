package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// AI_AGENT_NAMES matches the set defined in iterm2-agent-observation.md
var AI_AGENT_NAMES = map[string]bool{
	"claude":    true,
	"opencode":  true,
	"qoder":     true,
	"pi":        true,
	"cursor":    true,
	"kimi":      true,
	"kimi-cli":  true,
}

// AgentInfo holds detected agent information from a tmux pane.
type AgentInfo struct {
	PaneID        string
	WindowName    string
	SessionName   string
	CurrentCommand string
	PID           string
}

func main() {
	fmt.Println("=== iTerm2 Agent Detection Demo ===")
	fmt.Println()

	// 1. Print registered agent names
	fmt.Println("[1] Registered AI Agent Names:")
	for name := range AI_AGENT_NAMES {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println()

	// 2. Detect agents via tmux
	fmt.Println("[2] Scanning tmux panes for AI agents...")
	agents, err := detectAgents()
	if err != nil {
		fmt.Printf("  Error: tmux not available or not running: %v\n", err)
		fmt.Println("  Skipping tmux-based detection.")
		fmt.Println()
		demoFallbackDetection()
		return
	}

	if len(agents) == 0 {
		fmt.Println("  No AI agents detected in any tmux pane.")
		fmt.Println()
		fmt.Println("  To test, start an agent in a tmux session:")
		fmt.Println("    tmux new-session -d -s test 'claude'")
		fmt.Println("    tmux new-session -d -s test2 'opencode'")
		fmt.Println("  Then re-run this demo.")
		return
	}

	// 3. Print detected agents
	fmt.Printf("  Found %d AI agent(s):\n\n", len(agents))
	for i, a := range agents {
		fmt.Printf("  [%d] Agent: %s\n", i+1, a.CurrentCommand)
		fmt.Printf("      Pane:    %s\n", a.PaneID)
		fmt.Printf("      Window:  %s\n", a.WindowName)
		fmt.Printf("      Session: %s\n", a.SessionName)
		fmt.Printf("      PID:     %s\n", a.PID)
		fmt.Println()
	}

	// 4. Demonstrate process tree inspection
	fmt.Println("[3] Process tree for detected agents:")
	for _, a := range agents {
		fmt.Printf("\n  --- %s (PID: %s) ---\n", a.CurrentCommand, a.PID)
		printProcessTree(a.PID, 2)
	}

	// 5. Demonstrate sending keys (commented out for safety)
	fmt.Println()
	fmt.Println("[4] Interactive control example (dry run):")
	for _, a := range agents {
		fmt.Printf("  Would send '2' + Enter to pane %s (%s)\n", a.PaneID, a.CurrentCommand)
		// Uncomment to actually send:
		// sendKeys(a.PaneID, "2")
		// sendKeys(a.PaneID, "Enter")
	}

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
}

// detectAgents scans all tmux panes and returns those running known AI agents.
func detectAgents() ([]AgentInfo, error) {
	out, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{pane_id}\t#{pane_current_command}\t#{pane_pid}\t#{window_name}\t#{session_name}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes failed: %w", err)
	}
	return parseTmuxOutput(string(out)), nil
}

// parseTmuxOutput parses tmux list-panes tab-delimited output and returns detected agents.
func parseTmuxOutput(output string) []AgentInfo {
	var agents []AgentInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 5 {
			continue
		}
		paneID, cmd, pid, window, session := parts[0], parts[1], parts[2], parts[3], parts[4]
		if isAIAgent(cmd) {
			agents = append(agents, AgentInfo{
				PaneID:        paneID,
				WindowName:    window,
				SessionName:   session,
				CurrentCommand: cmd,
				PID:           pid,
			})
		}
	}
	return agents
}

// isAIAgent checks if a command name matches a known AI agent.
func isAIAgent(cmd string) bool {
	// Strip path prefix if present (e.g., /usr/local/bin/claude -> claude)
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}
	return AI_AGENT_NAMES[cmd]
}

// sendKeys sends keys to a tmux pane.
func sendKeys(paneID, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", paneID, keys, "Enter").Run()
}

// printProcessTree prints a simple process tree for a given PID.
func printProcessTree(pid string, indent int) {
	fmt.Print(getProcessTree(pid, indent))
}

// getProcessTree returns a formatted process tree string for a given PID.
func getProcessTree(pid string, indent int) string {
	prefix := strings.Repeat(" ", indent)
	out, err := exec.Command("ps", "-o", "pid,comm", "-p", pid).Output()
	if err != nil {
		return fmt.Sprintf("%s(ps failed for PID %s)\n", prefix, pid)
	}

	var sb strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		sb.WriteString(prefix)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	childrenOut, err := exec.Command("pgrep", "-P", pid).Output()
	if err != nil {
		return sb.String()
	}
	children := strings.Fields(strings.TrimSpace(string(childrenOut)))
	for _, child := range children {
		sb.WriteString(getProcessTree(child, indent+2))
	}
	return sb.String()
}

// demoFallbackDetection shows shell-based detection when tmux is not running.
func demoFallbackDetection() {
	fmt.Println("[2b] Fallback: Shell-based detection demo")
	fmt.Println()
	fmt.Println("  Equivalent shell commands from iterm2-agent-observation.md:")
	fmt.Println()
	fmt.Println("  # List all panes and filter by current command:")
	fmt.Println(`  tmux list-panes -a -F "#{pane_id} #{pane_current_command} #{pane_pid} #{window_name}" | \`)
	fmt.Println(`    awk '$2 ~ /^(claude|opencode|qoder|pi|cursor|kimi|kimi-cli)$/ {print $0}'`)
	fmt.Println()
	fmt.Println("  # Recursively inspect process tree:")
	fmt.Println(`  tmux list-panes -a -F "#{pane_pid} #{pane_current_command}" | \`)
	fmt.Println(`    awk '$2 ~ /^(claude|opencode|qoder|pi|cursor|kimi|kimi-cli)$/ {print $1}' | \`)
	fmt.Println(`    while IFS= read -r pid; do`)
	fmt.Println(`      echo "=== Agent process tree (PID: $pid) ==="`)
	fmt.Println(`      ps -o pid=,comm= -p "$pid"`)
	fmt.Println(`    done`)
}
