package server

import (
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/protocol"
)

func (s *Session) timelineSize(agentID string) int {
	if s.timelineStore == nil {
		return 0
	}
	result := s.timelineStore.Fetch(agentID, "tail", nil, 0)
	if result == nil {
		return 0
	}
	return result.Window.NextSeq
}

func (s *Session) handleAgentEvent(event agent.AgentEvent) {
	switch event.Type {
	case agent.EventAgentState:
		if event.Agent == nil {
			return
		}
		snapshot := event.Agent.ToSnapshot()
		s.sendMessage(protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
			Type: "agent_update",
			Payload: protocol.AgentUpdatePayload{
				Kind:    "upsert",
				Agent:   &snapshot,
				Project: s.projectPlacementForAgent(event.Agent),
			},
		}))

	case agent.EventAgentStream:
		if event.Stream == nil {
			return
		}
		event.Stream.AgentID = event.AgentID
		s.handleStreamEvent(*event.Stream)
	}
}

func (s *Session) handleStreamEvent(evt agent.AgentStreamEvent) {
	payload, ok := evt.Event.(map[string]interface{})
	if !ok {
		return
	}

	evtType, _ := payload["type"].(string)
	provider, _ := payload["provider"].(string)

	switch evtType {
	case "timeline":
		item := extractTimelineItem(payload["item"])
		turnID, _ := payload["turnId"].(string)
		if !s.coalescer.Handle(evt.AgentID, evtType, item, provider, turnID) {
			// Non-coalescable types (e.g. user_message) are handled immediately
			row := s.timelineStore.Append(evt.AgentID, item)
			epoch := s.timelineStore.GetEpoch(evt.AgentID)
			seq := row.Seq
			streamPayload := map[string]interface{}{
				"type":     "timeline",
				"item":     item.ToProtocolMap(),
				"provider": provider,
			}
			if turnID != "" {
				streamPayload["turnId"] = turnID
			}
			s.sendAgentStream(evt.AgentID, streamPayload, time.Now(), &seq, &epoch)
		}

	case "flush_signal":
		// content_block_stop signals the end of a thinking/text block.
		// Flush any buffered reasoning entries for this agent immediately
		// rather than waiting for the full 2s extended coalescer window.
		s.coalescer.FlushFor(evt.AgentID)

	case "thread_started":
		s.timelineStore.Initialize(evt.AgentID)
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)

	case "turn_completed":
		usage, _ := payload["usage"].(*protocol.AgentUsage)
		_ = usage
		s.coalescer.FlushFor(evt.AgentID)
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)

	case "turn_failed", "turn_canceled":
		s.coalescer.FlushFor(evt.AgentID)
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)

	case "permission_requested":
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)

	case "permission_resolved":
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)

	case "attention_required":
		reason, _ := payload["reason"].(string)
		// Enrich notification with assistant message from timeline and compute shouldNotify
		if s.activityTracker != nil {
			assistantMessage := s.getLastAssistantMessage(evt.AgentID)
			notification := push.BuildAttentionNotificationWithServerID(evt.AgentID, reason, assistantMessage, s.cfg.ServerID)
			states := s.activityTracker.GetAllStates()
			nowMs := time.Now().UnixMilli()
			plan := ComputeNotificationPlan(states, evt.AgentID, reason, nowMs)

			payload["notification"] = map[string]interface{}{
				"title": notification.Title,
				"body":  notification.Body,
				"data": map[string]interface{}{
					"agentId":  notification.Data.AgentID,
					"reason":   notification.Data.Reason,
					"serverId": notification.Data.ServerID,
				},
			}
			payload["shouldNotify"] = plan.InAppRecipientIndex != nil
		}
		s.broadcastAgentAttention(evt.AgentID, reason)
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)

	default:
		s.sendAgentStream(evt.AgentID, payload, evt.Timestamp, nil, nil)
	}
}

func (s *Session) handleCoalescedFlush(p agent.FlushPayload) {
	row := s.timelineStore.Append(p.AgentID, p.Item)

	epoch := s.timelineStore.GetEpoch(p.AgentID)
	seq := row.Seq
	payload := map[string]interface{}{
		"type":     "timeline",
		"item":     p.Item.ToProtocolMap(),
		"provider": p.Provider,
	}
	if p.TurnID != "" {
		payload["turnId"] = p.TurnID
	}
	s.sendAgentStream(p.AgentID, payload, time.Now(), &seq, &epoch)
}

func (s *Session) pushActiveAgents() {
	for _, ag := range s.agentMgr.ListAgentsWithPersisted() {
		snapshot := ag.ToSnapshot()
		s.sendMessage(protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
			Type: "agent_update",
			Payload: protocol.AgentUpdatePayload{
				Kind:    "upsert",
				Agent:   &snapshot,
				Project: s.projectPlacementForAgent(ag),
			},
		}))
	}
}

func (s *Session) sendAgentStream(agentID string, event interface{}, timestamp time.Time, seq *int, epoch *string) {
	s.maybeRecordAssistantTurn(agentID, event)
	s.sendMessage(protocol.NewSessionMessage(&protocol.AgentStreamMessage{
		Type: "agent_stream",
		Payload: protocol.AgentStreamPayload{
			AgentID:   agentID,
			Event:     event,
			Timestamp: timestamp.UTC().Format(time.RFC3339),
			Seq:       seq,
			Epoch:     epoch,
		},
	}))
}

func (s *Session) sendProviderSnapshot() {
	entries := s.registry.ToProviderSnapshotEntries()
	s.sendMessage(protocol.NewSessionMessage(&protocol.ProvidersSnapshotUpdate{
		Type: "providers_snapshot_update",
		Payload: protocol.ProvidersSnapshotPayload{
			Entries:     entries,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}))
}

func (s *Session) handleGetProvidersSnapshot(m *protocol.GetProvidersSnapshotRequest) {
	entries := s.registry.ToProviderSnapshotEntries()
	s.sendMessage(protocol.NewSessionMessage(&protocol.GetProvidersSnapshotResponse{
		Type: "get_providers_snapshot_response",
		Payload: protocol.GetProvidersSnapshotResponsePayload{
			RequestID:   m.RequestID,
			Entries:     entries,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}))
}
