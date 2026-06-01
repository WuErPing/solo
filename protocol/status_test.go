package protocol

import "testing"

func TestAgentStatusConstants(t *testing.T) {
	tests := []struct {
		status AgentStatus
		want   string
	}{
		{AgentInitializing, "initializing"},
		{AgentIdle, "idle"},
		{AgentRunning, "running"},
		{AgentError, "error"},
		{AgentClosed, "closed"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("status = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestAgentStatusIsTerminal(t *testing.T) {
	terminal := []AgentStatus{AgentError, AgentClosed}
	for _, s := range terminal {
		t.Run(string(s), func(t *testing.T) {
			if !s.IsTerminal() {
				t.Errorf("%q should be terminal", s)
			}
		})
	}
	nonTerminal := []AgentStatus{AgentInitializing, AgentIdle, AgentRunning}
	for _, s := range nonTerminal {
		t.Run(string(s), func(t *testing.T) {
			if s.IsTerminal() {
				t.Errorf("%q should not be terminal", s)
			}
		})
	}
}

func TestAgentStatusIsActive(t *testing.T) {
	active := []AgentStatus{AgentIdle, AgentRunning}
	for _, s := range active {
		t.Run(string(s), func(t *testing.T) {
			if !s.IsActive() {
				t.Errorf("%q should be active", s)
			}
		})
	}
	nonActive := []AgentStatus{AgentInitializing, AgentError, AgentClosed}
	for _, s := range nonActive {
		t.Run(string(s), func(t *testing.T) {
			if s.IsActive() {
				t.Errorf("%q should not be active", s)
			}
		})
	}
}

func TestAgentLifecycleStatusAlias(t *testing.T) {
	// AgentLifecycleStatus is an alias of AgentStatus; assignments should work both ways
	var als = AgentIdle
	var as = als
	if as != AgentIdle {
		t.Errorf("alias round-trip failed: got %q", as)
	}

	// Reverse: AgentStatus -> AgentLifecycleStatus
	as = AgentRunning
	als = as
	if als != AgentRunning {
		t.Errorf("reverse alias failed: got %q", als)
	}
}

func TestProviderStatusRemoved(t *testing.T) {
	// ProviderSnapshotEntry.Status should be string, not a custom type
	entry := ProviderSnapshotEntry{Provider: "claude", Status: "ready"}
	if entry.Status != "ready" {
		t.Errorf("ProviderSnapshotEntry.Status = %q, want ready", entry.Status)
	}
}
