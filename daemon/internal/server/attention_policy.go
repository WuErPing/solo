package server

// NotificationPlan determines how to route agent attention notifications.
type NotificationPlan struct {
	ShouldPush          bool
	InAppRecipientIndex *int
}

// ComputeNotificationPlan calculates the notification strategy based on client presence.
//
// Rules (matching Solo behavior):
//   - If any client is visible AND focused on the agent: no notification
//   - If any client is online (visible, recent activity): in-app notification to most recent
//   - If no clients are online:
//   - reason != "error": push notification
//   - reason == "error": no push (avoid spam)
func ComputeNotificationPlan(states []ClientPresenceState, agentID string, reason string, nowMs int64) NotificationPlan {
	var mostRecentIndex *int
	var mostRecentTime int64 = -1

	const presenceThresholdMs = 180_000 // 3 minutes, matching Solo

	for i, state := range states {
		// Check if client is "present" (has recent activity)
		isPresent := state.LastActivityAtMs > 0 && nowMs-state.LastActivityAtMs <= presenceThresholdMs

		if !isPresent {
			continue
		}

		// If any client is visible and focused on this agent, suppress all notifications
		if state.AppVisible && state.FocusedAgentID == agentID {
			return NotificationPlan{
				ShouldPush:          false,
				InAppRecipientIndex: nil,
			}
		}

		// Track most recent visible client for in-app notification.
		// Only visible clients can display in-app notifications;
		// backgrounded clients should fall through to push.
		if state.AppVisible && state.LastActivityAtMs > mostRecentTime {
			idx := i
			mostRecentIndex = &idx
			mostRecentTime = state.LastActivityAtMs
		}
	}

	// If there's a recent online client, send in-app notification only
	if mostRecentIndex != nil {
		return NotificationPlan{
			ShouldPush:          false,
			InAppRecipientIndex: mostRecentIndex,
		}
	}

	// No clients online - push unless it's an error
	return NotificationPlan{
		ShouldPush:          reason != "error",
		InAppRecipientIndex: nil,
	}
}
