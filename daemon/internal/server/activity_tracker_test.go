package server

import (
	"testing"
	"time"
)

func TestClientActivityTracker_UpdateAndGetAll(t *testing.T) {
	tracker := NewClientActivityTracker()

	tracker.UpdateActivity("session1", true, "agent1")
	tracker.UpdateActivity("session2", false, "agent2")

	states := tracker.GetAllStates()
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	// Find session1 state
	var session1State *ClientPresenceState
	for i := range states {
		if states[i].SessionID == "session1" {
			session1State = &states[i]
			break
		}
	}

	if session1State == nil {
		t.Fatal("session1 state not found")
	}
	if !session1State.AppVisible {
		t.Error("expected session1 to be visible")
	}
	if session1State.FocusedAgentID != "agent1" {
		t.Errorf("expected focused agent1, got %s", session1State.FocusedAgentID)
	}
	if session1State.LastActivityAtMs == 0 {
		t.Error("expected LastActivityAtMs to be set")
	}
}

func TestClientActivityTracker_UpdateExisting(t *testing.T) {
	tracker := NewClientActivityTracker()

	tracker.UpdateActivity("session1", true, "agent1")
	oldTime := time.Now().UnixMilli()
	time.Sleep(10 * time.Millisecond)
	tracker.UpdateActivity("session1", false, "agent2")

	states := tracker.GetAllStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}

	state := states[0]
	if state.AppVisible {
		t.Error("expected session to be not visible after update")
	}
	if state.FocusedAgentID != "agent2" {
		t.Errorf("expected focused agent2, got %s", state.FocusedAgentID)
	}
	if state.LastActivityAtMs <= oldTime {
		t.Error("expected LastActivityAtMs to be updated")
	}
}

func TestClientActivityTracker_Remove(t *testing.T) {
	tracker := NewClientActivityTracker()

	tracker.UpdateActivity("session1", true, "agent1")
	tracker.UpdateActivity("session2", false, "agent2")
	tracker.Remove("session1")

	states := tracker.GetAllStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 state after removal, got %d", len(states))
	}
	if states[0].SessionID != "session2" {
		t.Errorf("expected session2, got %s", states[0].SessionID)
	}
}

func TestClientActivityTracker_RemoveNonExistent(t *testing.T) {
	tracker := NewClientActivityTracker()

	tracker.UpdateActivity("session1", true, "agent1")
	tracker.Remove("nonexistent")

	states := tracker.GetAllStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
}

func TestClientActivityTracker_Empty(t *testing.T) {
	tracker := NewClientActivityTracker()

	states := tracker.GetAllStates()
	if len(states) != 0 {
		t.Fatalf("expected 0 states, got %d", len(states))
	}
}

func TestClientActivityTracker_ClearFocusedAgent(t *testing.T) {
	tracker := NewClientActivityTracker()

	tracker.UpdateActivity("session1", true, "agent1")
	tracker.UpdateActivity("session1", true, "")

	states := tracker.GetAllStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].FocusedAgentID != "" {
		t.Errorf("expected empty focused agent, got %s", states[0].FocusedAgentID)
	}
}
