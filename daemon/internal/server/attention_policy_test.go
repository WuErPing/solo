package server

import (
	"testing"
	"time"
)

func TestComputeNotificationPlan_FocusedOnAgent(t *testing.T) {
	now := time.Now().UnixMilli()
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: true, FocusedAgentID: "agent1", LastActivityAtMs: now},
	}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if plan.ShouldPush {
		t.Error("should not push when client is focused on agent")
	}
	if plan.InAppRecipientIndex != nil {
		t.Error("should not notify any client when focused on agent")
	}
}

func TestComputeNotificationPlan_OnlineButNotFocused(t *testing.T) {
	now := time.Now().UnixMilli()
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: true, FocusedAgentID: "agent2", LastActivityAtMs: now},
	}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if plan.ShouldPush {
		t.Error("should not push when client is online but not focused")
	}
	if plan.InAppRecipientIndex == nil || *plan.InAppRecipientIndex != 0 {
		t.Error("should notify the online client via in-app")
	}
}

func TestComputeNotificationPlan_NoClientsOnline(t *testing.T) {
	now := time.Now().UnixMilli()
	oldTime := now - 300000 // 5 minutes ago
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: false, FocusedAgentID: "", LastActivityAtMs: oldTime},
	}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if !plan.ShouldPush {
		t.Error("should push when no clients are online and reason is finished")
	}
	if plan.InAppRecipientIndex != nil {
		t.Error("should not have in-app recipient when no clients online")
	}
}

func TestComputeNotificationPlan_NoClientsOnline_Error(t *testing.T) {
	now := time.Now().UnixMilli()
	oldTime := now - 300000
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: false, FocusedAgentID: "", LastActivityAtMs: oldTime},
	}

	plan := ComputeNotificationPlan(states, "agent1", "error", now)

	if plan.ShouldPush {
		t.Error("should not push for error when no clients online")
	}
}

func TestComputeNotificationPlan_NoClientsOnline_Permission(t *testing.T) {
	now := time.Now().UnixMilli()
	oldTime := now - 300000
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: false, FocusedAgentID: "", LastActivityAtMs: oldTime},
	}

	plan := ComputeNotificationPlan(states, "agent1", "permission", now)

	if !plan.ShouldPush {
		t.Error("should push for permission when no clients online")
	}
}

func TestComputeNotificationPlan_MultipleClients(t *testing.T) {
	now := time.Now().UnixMilli()
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: true, FocusedAgentID: "agent2", LastActivityAtMs: now - 10000},
		{SessionID: "s2", AppVisible: true, FocusedAgentID: "agent1", LastActivityAtMs: now - 5000},
	}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if plan.ShouldPush {
		t.Error("should not push when one client is focused on agent")
	}
	if plan.InAppRecipientIndex != nil {
		t.Error("should not notify any client when one is focused")
	}
}

func TestComputeNotificationPlan_MostRecentWins(t *testing.T) {
	now := time.Now().UnixMilli()
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: true, FocusedAgentID: "agent2", LastActivityAtMs: now - 10000},
		{SessionID: "s2", AppVisible: true, FocusedAgentID: "agent3", LastActivityAtMs: now - 5000},
	}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if plan.ShouldPush {
		t.Error("should not push when clients are online")
	}
	if plan.InAppRecipientIndex == nil {
		t.Fatal("should have an in-app recipient")
	}
	if *plan.InAppRecipientIndex != 1 { // s2 is most recent
		t.Errorf("expected recipient index 1, got %d", *plan.InAppRecipientIndex)
	}
}

func TestComputeNotificationPlan_Empty(t *testing.T) {
	now := time.Now().UnixMilli()
	states := []ClientPresenceState{}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if !plan.ShouldPush {
		t.Error("should push when no clients exist")
	}
	if plan.InAppRecipientIndex != nil {
		t.Error("should not have in-app recipient when no clients")
	}
}

func TestComputeNotificationPlan_InvisibleButRecent_ShouldPush(t *testing.T) {
	now := time.Now().UnixMilli()
	states := []ClientPresenceState{
		{SessionID: "s1", AppVisible: false, FocusedAgentID: "", LastActivityAtMs: now - 5000},
	}

	plan := ComputeNotificationPlan(states, "agent1", "finished", now)

	if !plan.ShouldPush {
		t.Error("should push when client is recent but invisible (background app)")
	}
	if plan.InAppRecipientIndex != nil {
		t.Error("should not have in-app recipient when client is invisible")
	}
}
