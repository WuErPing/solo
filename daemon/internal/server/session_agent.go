package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

func (s *Session) handleCreateAgent(m *protocol.CreateAgentRequest) {
	// Copy OutputSchema from request to config if present
	if len(m.OutputSchema) > 0 && len(m.Config.OutputSchema) == 0 {
		m.Config.OutputSchema = m.OutputSchema
	}
	_, provisionalTitle := resolveCreateAgentTitles(m.Config.Title, m.InitialPrompt)
	if provisionalTitle != nil {
		m.Config.Title = provisionalTitle
	}
	ag, err := s.agentMgr.CreateAgent(context.Background(), &m.Config, m.Labels)
	if err != nil {
		s.sendRPCError(m.RequestID, "create_agent_request", err.Error(), nil)
		return
	}

	workspaceForAgent, workspaceCreated, err := s.upsertWorkspaceForCwd(ag.Cwd)
	if err != nil {
		s.logger.Warn("failed to ensure workspace for created agent", "agentId", ag.ID, "cwd", ag.Cwd, "error", err)
	}

	// Send agent_created status
	s.sendMessage(protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.AgentCreatedPayload{
			Status:    "agent_created",
			Agent:     ag.ToSnapshot(),
			AgentID:   ag.ID,
			RequestID: m.RequestID,
		},
	}))

	// Broadcast agent_update to all sessions (not just the creator) so that
	// remote clients (e.g. iOS via relay) see the new agent immediately.
	snapshot := ag.ToSnapshot()
	agentUpdateMsg := protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
		Type: "agent_update",
		Payload: protocol.AgentUpdatePayload{
			Kind:    "upsert",
			Agent:   &snapshot,
			Project: s.projectPlacementForAgent(ag),
		},
	})
	if s.broadcast != nil {
		s.broadcast(agentUpdateMsg)
	} else {
		s.sendMessage(agentUpdateMsg)
	}

	if workspaceCreated {
		s.emitWorkspaceUpdate(workspaceForAgent)
	}

	// Handle initial prompt if provided
	if m.InitialPrompt != nil && *m.InitialPrompt != "" {
		if err := s.agentMgr.SendAgentMessage(context.Background(), ag.ID, *m.InitialPrompt, nil, nil, ""); err != nil {
			s.logger.Warn("failed to send initial prompt", "agentId", ag.ID, "error", err)
		}
	}
}

func (s *Session) handleFetchAgents(m *protocol.FetchAgentsRequest) {
	agents := s.agentMgr.ListAgentsWithPersisted()
	entries := s.collectAgentDirectoryEntries(agents, m.Filter, false)

	// If the client requested a subscription, return a subscriptionId so the app
	// knows to process subsequent agent_update broadcasts. Solo already pushes
	// agent_update to every connected session via agentMgr.Subscribe, so no
	// additional tracking is needed — we just need to signal the subscription is active.
	var subscriptionID *string
	if m.Subscribe != nil {
		id := m.Subscribe.SubscriptionID
		if id == nil {
			generated := uuid.New().String()
			id = &generated
		}
		subscriptionID = id
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.FetchAgentsResponse{
		Type: "fetch_agents_response",
		Payload: protocol.FetchAgentsResponsePayload{
			RequestID:      m.RequestID,
			SubscriptionID: subscriptionID,
			Entries:        entries,
			PageInfo:       protocol.FetchPageInfo{},
		},
	}))
}

func (s *Session) handleFetchAgent(m *protocol.FetchAgentRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil {
		s.sendMessage(protocol.NewSessionMessage(&protocol.FetchAgentResponse{
			Type: "fetch_agent_response",
			Payload: protocol.FetchAgentResponsePayload{
				RequestID: m.RequestID,
				Error:     strPtr("agent not found"),
			},
		}))
		return
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.FetchAgentResponse{
		Type: "fetch_agent_response",
		Payload: protocol.FetchAgentResponsePayload{
			RequestID: m.RequestID,
			Agent:     snapshotPtr(ag.ToSnapshot()),
			Project:   s.projectPlacementForAgent(ag),
		},
	}))
}

func (s *Session) handleFetchAgentTimeline(m *protocol.FetchAgentTimelineRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil {
		s.sendRPCError(m.RequestID, "fetch_agent_timeline_request", "agent not found", nil)
		return
	}

	// Hydrate history from provider on first fetch (idempotent).
	// Blocks briefly so the response includes history data.
	if err := s.agentMgr.HydrateTimeline(context.Background(), m.AgentID, s.timelineStore); err != nil {
		s.logger.Warn("HydrateTimeline failed", "agentId", m.AgentID, "error", err)
	}

	direction := "tail"
	if m.Direction != nil {
		direction = *m.Direction
	}

	projection := "projected"
	if m.Projection != nil && *m.Projection != "" {
		projection = *m.Projection
	}

	limit := 0
	if m.Limit != nil {
		limit = *m.Limit
	}

	result := s.timelineStore.Fetch(m.AgentID, direction, m.Cursor, limit)

	// Convert rows to protocol timeline entries
	entries := make([]protocol.FetchAgentTimelineEntry, 0, len(result.Rows))
	for _, row := range result.Rows {
		entry := protocol.FetchAgentTimelineEntry{
			Provider:       ag.Provider,
			Item:           row.Item.ToProtocolMap(),
			Timestamp:      row.Timestamp,
			SeqStart:       row.Seq,
			SeqEnd:         row.Seq,
			SourceSeqRange: []map[string]int{{"startSeq": row.Seq, "endSeq": row.Seq}},
			Collapsed:      []string{},
		}
		entries = append(entries, entry)
	}

	var startCursor, endCursor *protocol.AgentTimelineCursor
	epoch := result.Epoch
	if len(result.Rows) > 0 {
		sc := result.Rows[0].ToProtocolCursor(epoch)
		startCursor = &sc
		ec := result.Rows[len(result.Rows)-1].ToProtocolCursor(epoch)
		endCursor = &ec
	}

	snapshot := ag.ToSnapshot()
	s.sendMessage(protocol.NewSessionMessage(&protocol.FetchAgentTimelineResponse{
		Type: "fetch_agent_timeline_response",
		Payload: protocol.FetchAgentTimelineResponsePayload{
			RequestID:   m.RequestID,
			AgentID:     m.AgentID,
			Agent:       &snapshot,
			Direction:   result.Direction,
			Projection:  projection,
			Epoch:       epoch,
			Reset:       result.Reset,
			StaleCursor: result.StaleCursor,
			Gap:         result.Gap,
			Window: protocol.FetchAgentTimelineWindow{
				MinSeq:  result.Window.MinSeq,
				MaxSeq:  result.Window.MaxSeq,
				NextSeq: result.Window.NextSeq,
			},
			StartCursor: startCursor,
			EndCursor:   endCursor,
			HasOlder:    result.HasOlder,
			HasNewer:    result.HasNewer,
			Entries:     entries,
		},
	}))
}

func (s *Session) handleSendAgentMessage(m *protocol.SendAgentMessageRequest) {
	logger := s.logger.With(
		"requestId", m.RequestID,
		"agentId", m.AgentID,
		"messageId", stringPtrValue(m.MessageID),
		"clientType", s.clientType,
	)
	logger.Info(
		"send_agent_message_request received",
		"textPrefix", textPrefix(m.Text, 80),
		"textLen", len(m.Text),
		"imageCount", len(m.Images),
		"attachmentCount", len(m.Attachments),
	)

	agentID, err := s.resolveAgentIdentifier(m.AgentID)
	if err != nil {
		logger.Warn("send_agent_message_request rejected", "error", err)
		s.sendSendAgentMessageResponse(m.RequestID, m.AgentID, false, err.Error())
		return
	}
	if agentID != m.AgentID {
		logger = logger.With("resolvedAgentId", agentID)
		logger.Info("send_agent_message_request resolved agent identifier")
	}

	if err := s.agentMgr.SendAgentMessage(context.Background(), agentID, m.Text, m.Images, m.Attachments, stringPtrValue(m.MessageID)); err != nil {
		logger.Warn("send_agent_message_request rejected", "error", err)
		s.sendSendAgentMessageResponse(m.RequestID, agentID, false, err.Error())
		return
	}

	logger.Info("send_agent_message_request accepted")
	s.sendSendAgentMessageResponse(m.RequestID, agentID, true, "")
}

func (s *Session) resolveAgentIdentifier(identifier string) (string, error) {
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return "", fmt.Errorf("agent identifier cannot be empty")
	}
	if s.agentMgr.GetAgent(trimmed) != nil {
		return trimmed, nil
	}

	agents := s.agentMgr.ListAgents()
	knownIDs := make([]string, 0, len(agents))
	titleMatches := make([]string, 0, 1)
	for _, ag := range agents {
		if ag == nil || ag.Internal {
			continue
		}
		knownIDs = append(knownIDs, ag.ID)
		snapshot := ag.ToSnapshot()
		if snapshot.Title != nil && *snapshot.Title == trimmed {
			titleMatches = append(titleMatches, ag.ID)
		}
	}

	prefixMatches := make([]string, 0, 1)
	for _, id := range knownIDs {
		if strings.HasPrefix(id, trimmed) {
			prefixMatches = append(prefixMatches, id)
		}
	}
	switch len(prefixMatches) {
	case 1:
		return prefixMatches[0], nil
	case 0:
	default:
		return "", fmt.Errorf("agent identifier %q is ambiguous (%s)", trimmed, summarizeAgentIDMatches(prefixMatches))
	}

	switch len(titleMatches) {
	case 1:
		return titleMatches[0], nil
	case 0:
	default:
		return "", fmt.Errorf("agent title %q is ambiguous (%s)", trimmed, summarizeAgentIDMatches(titleMatches))
	}

	return "", fmt.Errorf("agent not found: %s", trimmed)
}

func (s *Session) sendSendAgentMessageResponse(requestID, agentID string, accepted bool, errMsg string) {
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.SendAgentMessageResponse{
		Type: "send_agent_message_response",
		Payload: protocol.SendAgentMessageResponsePayload{
			RequestID: requestID,
			AgentID:   agentID,
			Accepted:  accepted,
			Error:     errPtr,
		},
	}))
}

func (s *Session) handleClearAgentAttention(m *protocol.ClearAgentAttention) {
	agents := make([]protocol.AgentSnapshotPayload, 0, len(m.AgentID))
	for _, agentID := range m.AgentID {
		snapshot, err := s.agentMgr.ClearAgentAttention(agentID)
		if err != nil {
			s.logger.Debug("clear agent attention skipped", "agentId", agentID, "error", err)
			continue
		}
		agents = append(agents, *snapshot)
	}

	if m.RequestID == nil {
		return
	}

	var agentID interface{}
	if len(m.AgentID) == 1 {
		agentID = m.AgentID[0]
	} else {
		agentID = m.AgentID
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.ClearAgentAttentionResponse{
		Type: "clear_agent_attention_response",
		Payload: protocol.ClearAgentAttentionResponsePayload{
			RequestID: *m.RequestID,
			AgentID:   agentID,
			Agents:    agents,
		},
	}))
}

func (s *Session) handleCancelAgent(m *protocol.CancelAgentRequest) {
	requestID := ""
	if m.RequestID != nil {
		requestID = *m.RequestID
	}

	err := s.agentMgr.CancelAgentRun(context.Background(), m.AgentID)
	if err != nil {
		errMsg := err.Error()
		s.sendMessage(protocol.NewSessionMessage(&protocol.CancelAgentResponse{
			Type: "cancel_agent_response",
			Payload: protocol.CancelAgentResponsePayload{
				RequestID: requestID,
				AgentID:   m.AgentID,
				Error:     &errMsg,
			},
		}))
		return
	}

	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag != nil {
		s.sendMessage(protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
			Type: "agent_update",
			Payload: protocol.AgentUpdatePayload{
				Kind:    "upsert",
				Agent:   snapshotPtr(ag.ToSnapshot()),
				Project: s.projectPlacementForAgent(ag),
			},
		}))
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.CancelAgentResponse{
		Type: "cancel_agent_response",
		Payload: protocol.CancelAgentResponsePayload{
			RequestID: requestID,
			AgentID:   m.AgentID,
		},
	}))
}

func (s *Session) handleDeleteAgent(m *protocol.DeleteAgentRequest) {
	if err := s.agentMgr.DeleteAgent(m.AgentID); err != nil {
		s.sendRPCError(m.RequestID, "delete_agent_request", err.Error(), nil)
		return
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.AgentDeletedMessage{
		Type: "agent_deleted",
		Payload: protocol.AgentDeletedPayload{
			AgentID:   m.AgentID,
			RequestID: m.RequestID,
		},
	}))
}

func (s *Session) handleArchiveAgent(m *protocol.ArchiveAgentRequest) {
	if err := s.agentMgr.ArchiveAgent(m.AgentID); err != nil {
		s.sendRPCError(m.RequestID, "archive_agent_request", err.Error(), nil)
		return
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.AgentArchivedMessage{
		Type: "agent_archived",
		Payload: protocol.AgentArchivedPayload{
			AgentID:    m.AgentID,
			ArchivedAt: time.Now().UTC().Format(time.RFC3339),
			RequestID:  m.RequestID,
		},
	}))
}

func (s *Session) handleResumeAgent(m *protocol.ResumeAgentRequest) {
	ag, err := s.agentMgr.ResumeAgentFromPersistence(context.Background(), &m.Handle, m.Overrides)
	if err != nil {
		s.sendRPCError(m.RequestID, "resume_agent_request", err.Error(), nil)
		return
	}
	snapshot := ag.ToSnapshot()
	s.sendMessage(protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.AgentResumedPayload{
			Status:       "agent_resumed",
			Agent:        snapshot,
			AgentID:      snapshot.ID,
			RequestID:    m.RequestID,
			TimelineSize: s.timelineSize(snapshot.ID),
		},
	}))
}

func (s *Session) handleWaitForFinish(m *protocol.WaitForFinishRequest) {
	status, final, errText, lastMessage := s.resolveWaitForFinishState(m.AgentID)
	if status == "" {
		s.sendRPCError(m.RequestID, "wait_for_finish_request", "agent not found", nil)
		return
	}
	if status != "running" && status != "initializing" {
		s.sendWaitForFinishResponse(m.RequestID, status, final, errText, lastMessage)
		return
	}

	timeout := time.Duration(0)
	if m.TimeoutMs != nil && *m.TimeoutMs > 0 {
		timeout = time.Duration(*m.TimeoutMs) * time.Millisecond
	}
	timer := (*time.Timer)(nil)
	timeoutCh := (<-chan time.Time)(nil)
	if timeout > 0 {
		timer = time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	done := make(chan struct{}, 1)
	var unsubscribe func() = s.agentMgr.Subscribe(func(event agent.AgentEvent) {
		if event.Type != agent.EventAgentState || event.AgentID != m.AgentID || event.Agent == nil {
			return
		}
		snapshot := event.Agent.ToSnapshot()
		if snapshot.Status == protocol.AgentRunning || snapshot.Status == protocol.AgentInitializing {
			return
		}
		select {
		case done <- struct{}{}:
		default:
		}
	})
	defer unsubscribe()

	// Re-check after subscribing to close the race where the terminal update
	// arrives between the first snapshot read and subscription registration.
	status, final, errText, lastMessage = s.resolveWaitForFinishState(m.AgentID)
	if status == "" {
		s.sendRPCError(m.RequestID, "wait_for_finish_request", "agent not found", nil)
		return
	}
	if status != "running" && status != "initializing" {
		s.sendWaitForFinishResponse(m.RequestID, status, final, errText, lastMessage)
		return
	}

	select {
	case <-done:
		status, final, errText, lastMessage = s.resolveWaitForFinishState(m.AgentID)
		if status == "" {
			s.sendRPCError(m.RequestID, "wait_for_finish_request", "agent not found", nil)
			return
		}
		s.sendWaitForFinishResponse(m.RequestID, status, final, errText, lastMessage)
	case <-timeoutCh:
		s.sendWaitForFinishResponse(m.RequestID, "timeout", nil, nil, nil)
	case <-s.done:
		s.sendRPCError(m.RequestID, "wait_for_finish_request", "session closed", nil)
	}
}

func (s *Session) resolveWaitForFinishState(agentID string) (string, *protocol.AgentSnapshotPayload, *string, *string) {
	ag := s.agentMgr.GetAgent(agentID)
	if ag == nil {
		return "", nil, nil, nil
	}

	snapshot := ag.ToSnapshot()
	status := string(snapshot.Status)
	if snapshot.RequiresAttention && snapshot.AttentionReason != nil && *snapshot.AttentionReason == "permission" {
		status = "permission"
	}
	if snapshot.Status == protocol.AgentClosed {
		status = "idle"
	}
	lastMessage := s.lastAssistantMessage(agentID)
	if lastMessage == nil && status != "running" && status != "initializing" {
		lastMessage = s.timelineStore.WaitForAssistantMessage(agentID, 250*time.Millisecond)
	}
	return status, &snapshot, snapshot.LastError, lastMessage
}

func (s *Session) sendWaitForFinishResponse(requestID, status string, final *protocol.AgentSnapshotPayload, errText *string, lastMessage *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.WaitForFinishResponse{
		Type: "wait_for_finish_response",
		Payload: protocol.WaitForFinishResponsePayload{
			RequestID:   requestID,
			Status:      normalizeWaitForFinishStatus(status),
			Final:       final,
			Error:       errText,
			LastMessage: lastMessage,
		},
	}))
}

func (s *Session) lastAssistantMessage(agentID string) *string {
	if s.timelineStore == nil {
		return nil
	}
	s.coalescer.FlushFor(agentID)
	return s.timelineStore.GetLastAssistantMessage(agentID)
}

func (s *Session) handleAgentPermissionResponse(m *protocol.AgentPermissionResponseMessage) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil || ag.Session == nil {
		return
	}
	ag.Session.RespondPermission(m.RequestID, m.Response)
}

func (s *Session) handleSetAgentMode(m *protocol.SetAgentModeRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil || ag.Session == nil {
		s.sendRPCError(m.RequestID, "set_agent_mode_request", "agent not found or no session", nil)
		return
	}
	if err := ag.Session.SetMode(m.ModeID); err != nil {
		s.sendRPCError(m.RequestID, "set_agent_mode_request", err.Error(), nil)
		return
	}
	// Update agent state and broadcast to all clients
	ag.CurrentModeID = &m.ModeID
	ag.TouchUpdatedAt()
	agentUpdateMsg := protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
		Type: "agent_update",
		Payload: protocol.AgentUpdatePayload{
			Kind:    "upsert",
			Agent:   snapshotPtr(ag.ToSnapshot()),
			Project: s.projectPlacementForAgent(ag),
		},
	})
	if s.broadcast != nil {
		s.broadcast(agentUpdateMsg)
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.SetAgentModeResponse{
		Type: "set_agent_mode_response",
		Payload: protocol.SetAgentModeResponsePayload{
			RequestID: m.RequestID,
			AgentID:   m.AgentID,
			Accepted:  true,
		},
	}))
}

func (s *Session) handleSetAgentModel(m *protocol.SetAgentModelRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil || ag.Session == nil {
		s.sendRPCError(m.RequestID, "set_agent_model_request", "agent not found or no session", nil)
		return
	}
	modelID := ""
	if m.ModelID != nil {
		modelID = *m.ModelID
	}
	if err := ag.Session.SetModel(modelID); err != nil {
		s.sendRPCError(m.RequestID, "set_agent_model_request", err.Error(), nil)
		return
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.SetAgentModelResponse{
		Type: "set_agent_model_response",
		Payload: protocol.SetAgentModelResponsePayload{
			RequestID: m.RequestID,
			AgentID:   m.AgentID,
			Accepted:  true,
		},
	}))
}

func (s *Session) handleGetDaemonConfig(m *protocol.GetDaemonConfigRequest) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.GetDaemonConfigResponse{
		Type: "get_daemon_config_response",
		Payload: protocol.GetDaemonConfigResponsePayload{
			RequestID: m.RequestID,
			Config:    map[string]interface{}{},
		},
	}))
}

func (s *Session) handleFetchAgentHistory(m *protocol.FetchAgentHistoryRequest) {
	agents := s.agentMgr.ListAllAgents()
	entries := s.collectAgentDirectoryEntries(agents, m.Filter, true)
	s.sendMessage(protocol.NewSessionMessage(&protocol.FetchAgentHistoryResponse{
		Type: "fetch_agent_history_response",
		Payload: protocol.FetchAgentHistoryResponsePayload{
			RequestID: m.RequestID,
			Entries:   entries,
			PageInfo: protocol.FetchPageInfo{
				NextCursor: nil,
				PrevCursor: nil,
				HasMore:    false,
			},
		},
	}))
}

func (s *Session) handleSetAgentThinking(m *protocol.SetAgentThinkingRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil || ag.Session == nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "agent not found or no session", nil)
		return
	}
	optionID := ""
	if m.ThinkingOptionID != nil {
		optionID = *m.ThinkingOptionID
	}
	if err := ag.Session.SetThinkingOption(optionID); err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), err.Error(), nil)
		return
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.SetAgentModeResponse{
		Type: "set_agent_thinking_response",
		Payload: protocol.SetAgentModeResponsePayload{
			RequestID: m.RequestID,
			AgentID:   m.AgentID,
			Accepted:  true,
		},
	}))
}

func (s *Session) handleSetAgentFeature(m *protocol.SetAgentFeatureRequest) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.SetAgentModeResponse{
		Type: "set_agent_feature_response",
		Payload: protocol.SetAgentModeResponsePayload{
			RequestID: m.RequestID,
			AgentID:   m.AgentID,
			Accepted:  true,
		},
	}))
}

func (s *Session) handleUpdateAgent(m *protocol.UpdateAgentRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "agent not found", nil)
		return
	}
	// Label updates require exported mutex access - send success for now
	s.sendMessage(protocol.NewSessionMessage(&protocol.SetAgentModeResponse{
		Type: "update_agent_response",
		Payload: protocol.SetAgentModeResponsePayload{
			RequestID: m.RequestID,
			AgentID:   m.AgentID,
			Accepted:  true,
		},
	}))
}

func (s *Session) handleRefreshAgent(m *protocol.RefreshAgentRequest) {
	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag == nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "agent not found", nil)
		return
	}
	snapshot := ag.ToSnapshot()
	s.sendMessage(protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.AgentRefreshedPayload{
			Status:       "agent_refreshed",
			AgentID:      m.AgentID,
			RequestID:    m.RequestID,
			TimelineSize: s.timelineSize(snapshot.ID),
		},
	}))
}

func (s *Session) handleCloseItems(m *protocol.CloseItemsRequest) {
	for _, agentID := range m.AgentIDs {
		if err := s.agentMgr.DeleteAgent(agentID); err != nil {
			s.logger.Warn("failed to close agent", "agentId", agentID, "error", err)
		}
	}
	for _, terminalID := range m.TerminalIDs {
		if err := s.terminalMgr.KillTerminalAndWait(terminalID); err != nil {
			s.logger.Warn("failed to close terminal", "terminalId", terminalID, "error", err)
		}
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.SetAgentModeResponse{
		Type: "close_items_response",
		Payload: protocol.SetAgentModeResponsePayload{
			RequestID: m.RequestID,
			Accepted:  true,
		},
	}))
}

func (s *Session) handleListCommands(m *protocol.ListCommandsRequest) {
	var commands []protocol.AgentSlashCommand
	var err error

	ag := s.agentMgr.GetAgent(m.AgentID)
	if ag != nil && ag.Session != nil {
		commands, err = ag.Session.ListCommands(context.Background())
	} else if m.DraftConfig != nil {
		// Fallback: list commands directly from the provider (no active session needed)
		provider := m.DraftConfig.Provider
		if s.registry == nil {
			err = fmt.Errorf("provider registry not available")
		} else {
			client, clientErr := s.registry.Get(provider)
			if clientErr != nil {
				err = clientErr
			} else if client == nil {
				err = fmt.Errorf("provider %q returned nil client", provider)
			} else {
				cwd := m.DraftConfig.Cwd
				commands, err = client.ListClientCommands(context.Background(), cwd)
			}
		}
	} else if ag == nil {
		err = fmt.Errorf("agent not found")
	} else {
		err = fmt.Errorf("agent has no active session")
	}

	payload := protocol.ListCommandsPayload{
		AgentID:   m.AgentID,
		Commands:  commands,
		RequestID: m.RequestID,
	}
	if err != nil {
		errMsg := err.Error()
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ListCommandsResponse{
		Type:    "list_commands_response",
		Payload: payload,
	}))
}

func (s *Session) handleListAvailableEditors(m *protocol.ListAvailableEditorsRequest) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.ListAvailableEditorsResponse{
		Type: "list_available_editors_response",
		Payload: protocol.ListAvailableEditorsPayload{
			RequestID: m.RequestID,
			Editors:   []protocol.EditorTarget{},
		},
	}))
}

func (s *Session) handleListProviderFeatures(m *protocol.ListProviderFeaturesRequest) {
	errMsg := "list_provider_features is not supported by this daemon"
	s.sendMessage(protocol.NewSessionMessage(&protocol.ListProviderFeaturesResponse{
		Type: "list_provider_features_response",
		Payload: protocol.ListProviderFeaturesPayload{
			Provider:  m.DraftConfig.Provider,
			Features:  []protocol.AgentFeature{},
			Error:     &errMsg,
			FetchedAt: time.Now().UTC().Format(time.RFC3339),
			RequestID: m.RequestID,
		},
	}))
}
