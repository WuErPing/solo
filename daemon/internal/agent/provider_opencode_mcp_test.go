package agent

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestToOpenCodeMcpConfig_Stdio(t *testing.T) {
	cfg := protocol.McpServerConfig{
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{"KEY": "val"},
	}
	result := toOpenCodeMcpConfig(cfg)
	if result["type"] != "local" {
		t.Errorf("type: got %v, want local", result["type"])
	}
	cmd := result["command"].([]string)
	if len(cmd) != 3 || cmd[0] != "npx" {
		t.Errorf("command: got %v", cmd)
	}
	env := result["environment"].(map[string]string)
	if env["KEY"] != "val" {
		t.Errorf("env: got %v", env)
	}
}

func TestToOpenCodeMcpConfig_Remote(t *testing.T) {
	cfg := protocol.McpServerConfig{
		Type:    "remote",
		URL:     "http://localhost:3000/sse",
		Headers: map[string]string{"Authorization": "Bearer token"},
	}
	result := toOpenCodeMcpConfig(cfg)
	if result["type"] != "remote" {
		t.Errorf("type: got %v, want remote", result["type"])
	}
	if result["url"] != "http://localhost:3000/sse" {
		t.Errorf("url: got %v", result["url"])
	}
	headers := result["headers"].(map[string]string)
	if headers["Authorization"] != "Bearer token" {
		t.Errorf("headers: got %v", headers)
	}
}


