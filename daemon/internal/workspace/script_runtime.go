package workspace

import (
	"fmt"
	"net"
	"sync"
)

// ScriptStatus represents the lifecycle state of a running script.
type ScriptStatus string

const (
	ScriptStatusRunning ScriptStatus = "running"
	ScriptStatusStopped ScriptStatus = "stopped"
)

// ScriptRuntime tracks a running service script.
type ScriptRuntime struct {
	WorkspaceID  string       `json:"workspaceId"`
	ScriptName   string       `json:"scriptName"`
	Hostname     string       `json:"hostname"`
	Port         int          `json:"port"`
	TerminalID   string       `json:"terminalId"`
	Status       ScriptStatus `json:"status"`
	ExitCode     *int         `json:"exitCode,omitempty"`
	ProxyURL     string       `json:"proxyUrl"`
}

// ScriptManager manages the lifecycle of workspace service scripts.
type ScriptManager struct {
	mu      sync.RWMutex
	scripts map[string]*ScriptRuntime // key: workspaceID + "/" + scriptName
}

// NewScriptManager creates a new ScriptManager.
func NewScriptManager() *ScriptManager {
	return &ScriptManager{
		scripts: make(map[string]*ScriptRuntime),
	}
}

// Register records a running script runtime.
func (m *ScriptManager) Register(rt *ScriptRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scripts[rt.WorkspaceID+"/"+rt.ScriptName] = rt
}

// Unregister removes a script runtime.
func (m *ScriptManager) Unregister(workspaceID, scriptName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.scripts, workspaceID+"/"+scriptName)
}

// Get retrieves a script runtime.
func (m *ScriptManager) Get(workspaceID, scriptName string) (*ScriptRuntime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.scripts[workspaceID+"/"+scriptName]
	return rt, ok
}

// ListByWorkspace returns all scripts for a workspace.
func (m *ScriptManager) ListByWorkspace(workspaceID string) []*ScriptRuntime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ScriptRuntime
	for _, rt := range m.scripts {
		if rt.WorkspaceID == workspaceID {
			result = append(result, rt)
		}
	}
	return result
}

// MarkStopped updates a script's status to stopped.
func (m *ScriptManager) MarkStopped(workspaceID, scriptName string, exitCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := workspaceID + "/" + scriptName
	if rt, ok := m.scripts[key]; ok {
		rt.Status = ScriptStatusStopped
		rt.ExitCode = &exitCode
	}
}

// BuildHostname constructs the hostname for a service script.
// Format: {projectSlug}.{branchName}.{scriptName}.localhost
func BuildHostname(projectSlug, branchName, scriptName string) string {
	return fmt.Sprintf("%s.%s.%s.localhost", projectSlug, branchName, scriptName)
}

// AllocatePort finds an available port.
func AllocatePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}
