package server

import (
	"strconv"

	"golang.org/x/sync/singleflight"

	"github.com/WuErPing/solo/protocol"
)

// capturePaneFlight coalesces concurrent capture-pane requests for the same pane+startLine.
var capturePaneFlight singleflight.Group

func (s *Session) handleTmuxListAgents(m *protocol.TmuxListAgentsRequest) {
	agentNames := s.cfg.GetTmuxAgentNames()
	agents, otherPanes, err := scanTmuxAgents(agentNames)
	if err != nil {
		errMsg := err.Error()
		s.logger.Error("tmux list agents failed", "error", errMsg)
		s.sendTmuxListAgentsResponse(m.RequestID, nil, nil, nil, &errMsg)
		return
	}

	s.logger.Info("tmux scan result",
		"agents", len(agents),
		"otherPanes", len(otherPanes),
		"agentNames", agentNames,
	)
	for _, a := range agents {
		s.logger.Info("tmux detected agent",
			"paneID", a.PaneID,
			"agentName", a.AgentName,
			"currentCmd", a.CurrentCmd,
			"launchCmd", a.LaunchCmd,
			"status", a.Status,
		)
	}
	for _, p := range otherPanes {
		s.logger.Info("tmux other pane",
			"paneID", p.PaneID,
			"currentCmd", p.CurrentCmd,
			"title", p.Title,
		)
	}

	// Detect agent activity by comparing pane content hashes between scans.
	s.detectAgentActivity(agents)

	// Filter tmux window_activity noise for non-agent panes.
	s.filterWindowActivity(otherPanes)

	// Drop activity state for panes that vanished (e.g. killed sessions).
	s.prunePaneActivityState(agents, otherPanes)

	// Persist command history and include it in the response.
	var history []protocol.AgentCommandEntry
	if s.cfg.SoloHome != "" {
		store := NewAgentCommandStore(s.cfg.SoloHome)
		var newEntries []AgentCommandEntry
		for _, a := range agents {
			if a.LaunchCmd != "" {
				newEntries = append(newEntries, AgentCommandEntry{
					AgentName: a.AgentName,
					LaunchCmd: a.LaunchCmd,
				})
			} else {
				s.logger.Info("tmux agent skipped due to empty launchCmd", "paneID", a.PaneID, "agentName", a.AgentName)
			}
		}
		// Remove stale entries for currently running agents before merging,
		// so that updated launch commands (e.g. from pane scrollback) replace
		// old ones (e.g. from ps wrapper args) instead of coexisting.
		if len(newEntries) > 0 {
			agentNames := make(map[string]bool, len(newEntries))
			for _, e := range newEntries {
				agentNames[e.AgentName] = true
			}
			store.DeleteByAgentName(agentNames)
		}
		store.Merge(newEntries)
		for _, e := range store.Entries() {
			history = append(history, protocol.AgentCommandEntry{
				AgentName: e.AgentName,
				LaunchCmd: e.LaunchCmd,
				LastSeen:  e.LastSeen,
			})
		}
	}
	s.sendTmuxListAgentsResponse(m.RequestID, agents, otherPanes, history, nil)
}

func (s *Session) sendTmuxListAgentsResponse(requestID string, agents []protocol.TmuxAgentInfo, otherPanes []protocol.TmuxPaneInfo, history []protocol.AgentCommandEntry, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: protocol.TmuxListAgentsResponsePayload{
			RequestID:      requestID,
			Agents:         agents,
			OtherPanes:     otherPanes,
			CommandHistory: history,
			Error:          errMsg,
		},
	}))
}

func (s *Session) handleTmuxCapturePane(m *protocol.TmuxCapturePaneRequest) {
	startLine := -200
	if m.StartLine != nil {
		startLine = *m.StartLine
	}

	cols := 0
	if m.Cols != nil {
		cols = *m.Cols
	}

	// Coalesce concurrent requests for the same pane+startLine+cols into a single tmux call.
	key := m.PaneID + ":" + strconv.Itoa(startLine) + ":" + strconv.Itoa(cols)
	type captureResult struct {
		content  string
		paneCols int
	}
	result, err, _ := capturePaneFlight.Do(key, func() (any, error) {
		content, paneCols, err := capturePaneFunc(m.PaneID, startLine, cols)
		if err != nil {
			return nil, err
		}
		return captureResult{content: content, paneCols: paneCols}, nil
	})
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxCapturePaneResponse(m.RequestID, "", nil, nil, nil, &errMsg)
		return
	}
	cr := result.(captureResult)
	hash := computeContentHash(cr.content)
	if m.LastContentHash != nil && *m.LastContentHash == hash {
		changed := false
		s.sendTmuxCapturePaneResponse(m.RequestID, "", &changed, &hash, &cr.paneCols, nil)
		return
	}
	changed := true
	s.sendTmuxCapturePaneResponse(m.RequestID, cr.content, &changed, &hash, &cr.paneCols, nil)
}

func (s *Session) sendTmuxCapturePaneResponse(requestID string, content string, changed *bool, contentHash *string, paneCols *int, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxCapturePaneResponse{
		Type: "tmux/capture_pane/response",
		Payload: protocol.TmuxCapturePaneResponsePayload{
			RequestID:   requestID,
			Content:     content,
			Changed:     changed,
			ContentHash: contentHash,
			PaneCols:    paneCols,
			Error:       errMsg,
		},
	}))
}

func (s *Session) handleTmuxSendKeys(m *protocol.TmuxSendKeysRequest) {
	sendEnter := m.SendEnter == nil || *m.SendEnter
	err := sendKeysToTmuxPane(m.PaneID, m.Keys, sendEnter)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxSendKeysResponse(m.RequestID, &errMsg)
		return
	}
	s.sendTmuxSendKeysResponse(m.RequestID, nil)
}

func (s *Session) sendTmuxSendKeysResponse(requestID string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxSendKeysResponse{
		Type: "tmux/send_keys/response",
		Payload: protocol.TmuxSendKeysResponsePayload{
			RequestID: requestID,
			Error:     errMsg,
		},
	}))
}

func (s *Session) handleTmuxNewSession(m *protocol.TmuxNewSessionRequest) {
	err := createTmuxSession(m.Name, m.WorkingDir, m.Command)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxNewSessionResponse(m.RequestID, "", &errMsg)
		return
	}
	s.sendTmuxNewSessionResponse(m.RequestID, m.Name, nil)
}

func (s *Session) sendTmuxNewSessionResponse(requestID, sessionName string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxNewSessionResponse{
		Type: "tmux/new_session/response",
		Payload: protocol.TmuxNewSessionResponsePayload{
			RequestID:   requestID,
			SessionName: sessionName,
			Error:       errMsg,
		},
	}))
}

func (s *Session) handleTmuxKillSession(m *protocol.TmuxKillSessionRequest) {
	err := killTmuxSession(m.SessionName)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxKillSessionResponse(m.RequestID, &errMsg)
		return
	}
	s.sendTmuxKillSessionResponse(m.RequestID, nil)
}

func (s *Session) sendTmuxKillSessionResponse(requestID string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxKillSessionResponse{
		Type: "tmux/kill_session/response",
		Payload: protocol.TmuxKillSessionResponsePayload{
			RequestID: requestID,
			Error:     errMsg,
		},
	}))
}

func (s *Session) handleTmuxDeleteCommandHistory(m *protocol.TmuxDeleteCommandHistoryRequest) {
	if s.cfg.SoloHome == "" {
		errMsg := "SoloHome not configured"
		s.sendTmuxDeleteCommandHistoryResponse(m.RequestID, &errMsg)
		return
	}
	store := NewAgentCommandStore(s.cfg.SoloHome)
	store.Delete(m.LaunchCmd)
	s.sendTmuxDeleteCommandHistoryResponse(m.RequestID, nil)
}

func (s *Session) sendTmuxDeleteCommandHistoryResponse(requestID string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxDeleteCommandHistoryResponse{
		Type: "tmux/delete_command_history/response",
		Payload: protocol.TmuxDeleteCommandHistoryResponsePayload{
			RequestID: requestID,
			Error:     errMsg,
		},
	}))
}

func (s *Session) handleTmuxStatusLine(m *protocol.TmuxStatusLineRequest) {
	left, center, right, err := extractTmuxStatusLine(m.SessionID)
	if err != nil {
		errMsg := err.Error()
		s.sendTmuxStatusLineResponse(m.RequestID, "", "", "", &errMsg)
		return
	}
	s.sendTmuxStatusLineResponse(m.RequestID, left, center, right, nil)
}

func (s *Session) sendTmuxStatusLineResponse(requestID, statusLeft, statusCenter, statusRight string, errMsg *string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.TmuxStatusLineResponse{
		Type: "tmux/status_line/response",
		Payload: protocol.TmuxStatusLineResponsePayload{
			RequestID:    requestID,
			StatusLeft:   statusLeft,
			StatusCenter: statusCenter,
			StatusRight:  statusRight,
			Error:        errMsg,
		},
	}))
}
