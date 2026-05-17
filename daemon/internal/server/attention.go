package server

import (
	"time"

	"github.com/WuErPing/solo/daemon/internal/push"
)

// broadcastAgentAttention handles agent attention notifications by:
// 1. Building notification payload
// 2. Computing notification plan based on client presence
// 3. Sending push notification if needed
// 4. Broadcasting attention_required event to WebSocket clients
func (s *Session) broadcastAgentAttention(agentID string, reason string) {
	if s.activityTracker == nil {
		return
	}

	// Get the last assistant message for notification preview
	assistantMessage := s.getLastAssistantMessage(agentID)

	// Build notification payload
	notification := push.BuildAttentionNotification(agentID, reason, assistantMessage)

	// Compute notification plan
	states := s.activityTracker.GetAllStates()
	nowMs := time.Now().UnixMilli()
	plan := ComputeNotificationPlan(states, agentID, reason, nowMs)

	// Send push notification if needed
	if plan.ShouldPush && s.pusher != nil && s.pushTokenStore != nil {
		tokens := s.pushTokenStore.GetAll()
		if len(tokens) > 0 {
			go func() {
				if err := s.pusher.Send(tokens, notification); err != nil {
					s.logger.Error("failed to send push notification", "error", err, "agentId", agentID)
				}
			}()
		}
	}

	// Update activity tracker for this session (mark as active)
	if s.clientID != "" {
		s.activityTracker.UpdateActivity(s.clientID, true, agentID)
	}
}

// getLastAssistantMessage retrieves the last assistant message from the timeline store.
func (s *Session) getLastAssistantMessage(agentID string) string {
	if s.timelineStore == nil {
		return ""
	}

	result := s.timelineStore.Fetch(agentID, "tail", nil, 50)
	if result == nil || len(result.Rows) == 0 {
		return ""
	}

	// Search backwards for the last assistant message
	rows := result.Rows
	for i := len(rows) - 1; i >= 0; i-- {
		item := rows[i].Item
		if item.Type == "assistant_message" {
			return item.Text
		}
	}

	return ""
}
