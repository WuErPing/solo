package protocol

import "testing"

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
