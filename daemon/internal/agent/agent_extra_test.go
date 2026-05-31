package agent

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestManagedAgentClearAttention(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	// Clear when no attention required
	if agent.ClearAttention() {
		t.Error("expected false when no attention required")
	}

	// Set attention and clear
	agent.SetAttention(true, "test reason")
	if !agent.ClearAttention() {
		t.Error("expected true after clearing attention")
	}
	if agent.Attention.Requires {
		t.Error("expected attention to be cleared")
	}
}

func TestManagedAgentClearAttentionUnlessPermission(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	// Clear when no attention
	if agent.ClearAttentionUnlessPermission() {
		t.Error("expected false when no attention")
	}

	// Set permission attention — should not clear
	agent.SetAttention(true, "permission")
	if agent.ClearAttentionUnlessPermission() {
		t.Error("expected false for permission attention")
	}

	// Set non-permission attention — should clear
	agent.SetAttention(true, "message")
	if !agent.ClearAttentionUnlessPermission() {
		t.Error("expected true for non-permission attention")
	}
}

func TestManagedAgentClearError(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	agent.SetError("something went wrong")
	if agent.LastError == nil || *agent.LastError != "something went wrong" {
		t.Fatal("expected error to be set")
	}

	agent.ClearError()
	if agent.LastError != nil {
		t.Error("expected error to be cleared")
	}
}

func TestManagedAgentSubscribeAndEmit(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	var received AgentEvent
	sub := agent.Subscribe(func(ev AgentEvent) {
		received = ev
	})

	agent.Emit(AgentEvent{Type: EventAgentState, AgentID: "a1"})
	if received.Type != EventAgentState {
		t.Errorf("expected event type state, got %v", received.Type)
	}

	// Unsubscribe
	sub()
	received = AgentEvent{}
	agent.Emit(AgentEvent{Type: EventAgentStream, AgentID: "a1"})
	if received.Type == EventAgentStream {
		t.Error("expected no event after unsubscribe")
	}
}

func TestManagedAgentIsBusy(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	agent.Lifecycle = protocol.AgentIdle
	if agent.IsBusy() {
		t.Error("expected not busy when idle")
	}

	agent.Lifecycle = protocol.AgentInitializing
	if !agent.IsBusy() {
		t.Error("expected busy when initializing")
	}

	agent.Lifecycle = protocol.AgentRunning
	if !agent.IsBusy() {
		t.Error("expected busy when running")
	}
}

func TestManagedAgentSetSession(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	if agent.IsActive() {
		t.Error("expected not active without session")
	}

	sess := &MockAgentSession{}
	agent.SetSession(sess)

	if !agent.IsActive() {
		t.Error("expected active with session")
	}
	if agent.GetSession() != sess {
		t.Error("expected session to match")
	}

	agent.SetSession(nil)
	if agent.IsActive() {
		t.Error("expected not active after clearing session")
	}
}

func TestManagedAgentShortID(t *testing.T) {
	agent := NewManagedAgent("short", "mock", "/tmp", nil, nil)
	if agent.ShortID() != "short" {
		t.Errorf("ShortID: got %q, want short", agent.ShortID())
	}

	agent.ID = "very-long-agent-id-string"
	if agent.ShortID() != "very-lon" {
		t.Errorf("ShortID: got %q, want very-lon", agent.ShortID())
	}
}

func TestManagedAgentDisplayTitle(t *testing.T) {
	// With title via config
	agent := NewManagedAgent("a1", "mock", "/tmp/project", nil, nil)
	agent.Config = &protocol.AgentSessionConfig{Title: strPtr("My Agent")}
	if agent.DisplayTitle() != "My Agent" {
		t.Errorf("DisplayTitle: got %q, want 'My Agent'", agent.DisplayTitle())
	}

	// Without title — falls back to cwd basename
	agent.Config = &protocol.AgentSessionConfig{}
	if agent.DisplayTitle() != "project" {
		t.Errorf("DisplayTitle: got %q, want 'project'", agent.DisplayTitle())
	}

	// Empty cwd — strings.Split("", "/") returns [""]
	agent.Cwd = ""
	if agent.DisplayTitle() != "" {
		t.Errorf("DisplayTitle: got %q, want ''", agent.DisplayTitle())
	}
}

func TestManagedAgentToSnapshotIncludesAttention(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	agent.SetAttention(true, "test_reason")

	snapshot := agent.ToSnapshot()
	if snapshot.AttentionReason == nil || *snapshot.AttentionReason != "test_reason" {
		t.Error("expected attention reason in snapshot")
	}
	if snapshot.AttentionTimestamp == nil {
		t.Error("expected attention timestamp in snapshot")
	}
}

func TestManagedAgentToSnapshotWithNilCollections(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	agent.AvailableModes = nil
	agent.Features = nil
	agent.Labels = nil

	snapshot := agent.ToSnapshot()
	if snapshot.AvailableModes == nil {
		t.Error("expected non-nil availableModes")
	}
	if snapshot.Features == nil {
		t.Error("expected non-nil features")
	}
	if snapshot.Labels == nil {
		t.Error("expected non-nil labels")
	}
}

func TestManagedAgentModelAndTitle(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)

	m := "gpt-4"
	agent.Config = &protocol.AgentSessionConfig{Model: &m}
	if agent.model() == nil || *agent.model() != "gpt-4" {
		t.Error("expected model to be set")
	}

	agent.Config = &protocol.AgentSessionConfig{Title: strPtr("Custom Title")}
	if agent.title() == nil || *agent.title() != "Custom Title" {
		t.Error("expected title to be set")
	}
}


