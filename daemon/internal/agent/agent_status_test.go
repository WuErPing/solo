package agent

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestManagedAgentUsesProtocolAgentStatus(t *testing.T) {
	agent := NewManagedAgent("test", "claude", "/tmp", nil, nil)
	if agent.Lifecycle != protocol.AgentInitializing {
		t.Errorf("initial lifecycle = %q, want initializing", agent.Lifecycle)
	}
}

func TestManagedAgentSetLifecycle(t *testing.T) {
	agent := NewManagedAgent("test", "claude", "/tmp", nil, nil)
	agent.SetLifecycle(protocol.AgentIdle)
	if agent.Lifecycle != protocol.AgentIdle {
		t.Errorf("lifecycle = %q, want idle", agent.Lifecycle)
	}
}

func TestManagedAgentIsBusy_StatusRefactor(t *testing.T) {
	agent := NewManagedAgent("test", "claude", "/tmp", nil, nil)
	if !agent.IsBusy() {
		t.Error("initializing should be busy")
	}
	agent.SetLifecycle(protocol.AgentIdle)
	if agent.IsBusy() {
		t.Error("idle should not be busy")
	}
	agent.SetLifecycle(protocol.AgentRunning)
	if !agent.IsBusy() {
		t.Error("running should be busy")
	}
	agent.SetLifecycle(protocol.AgentError)
	if agent.IsBusy() {
		t.Error("error should not be busy")
	}
	agent.SetLifecycle(protocol.AgentClosed)
	if agent.IsBusy() {
		t.Error("closed should not be busy")
	}
}

func TestManagedAgentSetError(t *testing.T) {
	agent := NewManagedAgent("test", "claude", "/tmp", nil, nil)
	agent.SetError("something went wrong")
	if agent.Lifecycle != protocol.AgentError {
		t.Errorf("lifecycle = %q, want error", agent.Lifecycle)
	}
	if agent.LastError == nil || *agent.LastError != "something went wrong" {
		t.Error("last error not set correctly")
	}
}

func TestManagedAgentToSnapshotStatus(t *testing.T) {
	agent := NewManagedAgent("test", "claude", "/tmp", nil, nil)
	snapshot := agent.ToSnapshot()
	if snapshot.Status != protocol.AgentLifecycleStatus(protocol.AgentInitializing) {
		t.Errorf("snapshot status = %q, want initializing", snapshot.Status)
	}
}
