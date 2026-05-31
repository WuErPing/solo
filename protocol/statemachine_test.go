package protocol

import "testing"

func TestStateMachineValidTransition(t *testing.T) {
	sm := NewStateMachine(AgentInitializing).
		Allow(AgentInitializing, AgentIdle).
		Allow(AgentIdle, AgentRunning).
		Allow(AgentRunning, AgentIdle)

	if !sm.CanTransition(AgentIdle) {
		t.Error("initializing -> idle should be allowed")
	}
	if err := sm.Transition(AgentIdle); err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if sm.Current() != AgentIdle {
		t.Errorf("current = %q, want idle", sm.Current())
	}

	if err := sm.Transition(AgentRunning); err != nil {
		t.Fatalf("idle -> running failed: %v", err)
	}
	if sm.Current() != AgentRunning {
		t.Errorf("current = %q, want running", sm.Current())
	}
}

func TestStateMachineInvalidTransition(t *testing.T) {
	sm := NewStateMachine(AgentInitializing).
		Allow(AgentInitializing, AgentIdle)

	if sm.CanTransition(AgentRunning) {
		t.Error("initializing -> running should not be allowed")
	}
	if err := sm.Transition(AgentRunning); err == nil {
		t.Error("invalid transition should return error")
	}
	if sm.Current() != AgentInitializing {
		t.Error("state should not change on invalid transition")
	}
}

func TestStateMachineTerminalStateNoTransition(t *testing.T) {
	sm := NewStateMachine(AgentClosed)
	if sm.CanTransition(AgentIdle) {
		t.Error("closed -> idle should not be allowed")
	}
	if err := sm.Transition(AgentIdle); err == nil {
		t.Error("transition from terminal state should return error")
	}
}

func TestStateMachineHooks(t *testing.T) {
	var enteredFrom, exitedTo AgentStatus
	sm := NewStateMachine(AgentInitializing).
		Allow(AgentInitializing, AgentIdle).
		OnEnter(AgentIdle, func(from AgentStatus) { enteredFrom = from }).
		OnExit(AgentInitializing, func(to AgentStatus) { exitedTo = to })

	if err := sm.Transition(AgentIdle); err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if enteredFrom != AgentInitializing {
		t.Errorf("OnEnter called with from=%q, want initializing", enteredFrom)
	}
	if exitedTo != AgentIdle {
		t.Errorf("OnExit called with to=%q, want idle", exitedTo)
	}
}

func TestStateMachineMultipleTransitions(t *testing.T) {
	sm := NewStateMachine(AgentInitializing).
		Allow(AgentInitializing, AgentIdle).
		Allow(AgentIdle, AgentRunning).
		Allow(AgentRunning, AgentIdle).
		Allow(AgentIdle, AgentError).
		Allow(AgentError, AgentClosed)

	transitions := []AgentStatus{AgentIdle, AgentRunning, AgentIdle, AgentError, AgentClosed}
	for _, to := range transitions {
		if err := sm.Transition(to); err != nil {
			t.Fatalf("transition to %q failed: %v", to, err)
		}
	}
	if sm.Current() != AgentClosed {
		t.Errorf("final state = %q, want closed", sm.Current())
	}
}
