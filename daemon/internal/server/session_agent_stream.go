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
	// Fast path: typed StreamEvent implementations (new code path).
	switch e := evt.Event.(type) {
	case protocol.TimelineStreamEvent:
		if !s.coalescer.Handle(evt.AgentID, e.StreamEventType(), e.Item, e.Provider, e.TurnID) {
			row := s.timelineStore.Append(evt.AgentID, e.Item)
			epoch := s.timelineStore.GetEpoch(evt.AgentID)
			seq := row.Seq
			s.sendAgentStream(evt.AgentID, e, time.Now(), &seq, &epoch)
		}
		return

	case protocol.FlushSignalStreamEvent:
		// content_block_stop signals the end of a thinking/text block.
		// Flush any buffered reasoning entries for this agent immediately
		// rather than waiting for the full 2s extended coalescer window.
		s.coalescer.FlushFor(evt.AgentID)
		return

	case protocol.ThreadStartedStreamEvent:
		s.timelineStore.Initialize(evt.AgentID)
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.TurnCompletedStreamEvent:
		s.coalescer.FlushFor(evt.AgentID)
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.TurnFailedStreamEvent:
		s.coalescer.FlushFor(evt.AgentID)
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.TurnCanceledStreamEvent:
		s.coalescer.FlushFor(evt.AgentID)
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.PermissionRequestedStreamEvent:
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.PermissionResolvedStreamEvent:
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.AttentionRequiredStreamEvent:
		// Enrich notification with assistant message from timeline and compute shouldNotify
		e.Timestamp = evt.Timestamp.UTC().Format(time.RFC3339)
		if s.activityTracker != nil {
			assistantMessage := s.getLastAssistantMessage(evt.AgentID)
			notification := push.BuildAttentionNotificationWithServerID(evt.AgentID, e.Reason, assistantMessage, s.cfg.ServerID)
			states := s.activityTracker.GetAllStates()
			nowMs := time.Now().UnixMilli()
			plan := ComputeNotificationPlan(states, evt.AgentID, e.Reason, nowMs)

			e.Notification = map[string]interface{}{
				"title": notification.Title,
				"body":  notification.Body,
				"data": map[string]interface{}{
					"agentId":  notification.Data.AgentID,
					"reason":   notification.Data.Reason,
					"serverId": notification.Data.ServerID,
				},
			}
			e.ShouldNotify = plan.InAppRecipientIndex != nil
		}
		s.broadcastAgentAttention(evt.AgentID, e.Reason)
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return

	case protocol.SessionClosedStreamEvent:
		s.sendAgentStream(evt.AgentID, e, evt.Timestamp, nil, nil)
		return
	}
}

func (s *Session) handleCoalescedFlush(p agent.FlushPayload) {
	row := s.timelineStore.Append(p.AgentID, p.Item)

	epoch := s.timelineStore.GetEpoch(p.AgentID)
	seq := row.Seq
	evt := protocol.TimelineStreamEvent{
		Item:     p.Item,
		Provider: p.Provider,
		TurnID:   p.TurnID,
	}
	s.sendAgentStream(p.AgentID, evt, time.Now(), &seq, &epoch)
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
