package server

import (
	"fmt"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
)

func TestDebugScanTmux(t *testing.T) {
	if testing.Short() {
		t.Skip("debug scan")
	}
	cfg := &config.Config{}
	agentNames := cfg.GetTmuxAgentNames()
	fmt.Printf("agentNames: %v\n", agentNames)

	agents, otherPanes, err := scanTmuxAgents(agentNames)
	if err != nil {
		t.Fatalf("scanTmuxAgents error: %v", err)
	}
	nodes, err := listProcessTree()
	if err != nil {
		t.Fatalf("listProcessTree error: %v", err)
	}
	fmt.Printf("agents: %d\n", len(agents))
	for _, a := range agents {
		fmt.Printf("  pane=%s panePID=%d agent=%s currentCmd=%s launchCmd=%q status=%s title=%q\n",
			a.PaneID, a.PanePID, a.AgentName, a.CurrentCmd, a.LaunchCmd, a.Status, a.Title)
		name, pid, args := findAgentDescendant(nodes, a.PanePID, map[string]bool{a.AgentName: true})
		fmt.Printf("    findAgentDescendant: name=%q pid=%d args=%q\n", name, pid, args)
	}
	fmt.Printf("otherPanes: %d\n", len(otherPanes))
	for _, p := range otherPanes {
		fmt.Printf("  pane=%s panePID=%d currentCmd=%s title=%q\n", p.PaneID, p.PanePID, p.CurrentCmd, p.Title)
	}

	fmt.Println("\nprocess tree snapshot:")
	for _, n := range nodes {
		if n.ppid == 1 {
			continue
		}
		fmt.Printf("  pid=%d ppid=%d comm=%q args=%q\n", n.pid, n.ppid, n.comm, n.args)
	}
}
