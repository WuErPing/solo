package server

import (
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
)

func TestBuildDaemonConfigResponse_WithMcpAndTmuxAgentNames(t *testing.T) {
	cfg := &config.Config{
		MCPInjectIntoAgents: true,
		TmuxAgentNames:      []string{"aider"},
	}
	result := buildDaemonConfigResponse(cfg)

	mcp, ok := result["mcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'mcp' in config, got: %v", result)
	}
	if mcp["injectIntoAgents"] != true {
		t.Errorf("mcp.injectIntoAgents: got %v, want true", mcp["injectIntoAgents"])
	}

	names, ok := result["tmuxAgentNames"].([]string)
	if !ok {
		t.Fatalf("expected 'tmuxAgentNames' ([]string), got %T: %v", result["tmuxAgentNames"], result["tmuxAgentNames"])
	}
	if len(names) != 1 || names[0] != "aider" {
		t.Errorf("tmuxAgentNames: got %v, want [aider]", names)
	}
}

func TestBuildDaemonConfigResponse_DefaultMcp(t *testing.T) {
	cfg := &config.Config{
		// MCPInjectIntoAgents defaults to false
	}
	result := buildDaemonConfigResponse(cfg)

	mcp, ok := result["mcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'mcp' in config even when not explicitly set, got: %v", result)
	}
	if mcp["injectIntoAgents"] != false {
		t.Errorf("mcp.injectIntoAgents: got %v, want false", mcp["injectIntoAgents"])
	}
}

func TestBuildDaemonConfigResponse_EmptyTmuxAgentNames(t *testing.T) {
	cfg := &config.Config{}
	result := buildDaemonConfigResponse(cfg)

	names, ok := result["tmuxAgentNames"].([]string)
	if !ok {
		t.Fatalf("expected 'tmuxAgentNames' ([]string), got %T", result["tmuxAgentNames"])
	}
	if names != nil {
		t.Errorf("tmuxAgentNames: got %v, want nil", names)
	}
}
