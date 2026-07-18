package server

import (
	"context"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/llm"
	"github.com/WuErPing/solo/daemon/internal/schedule"
	"github.com/WuErPing/solo/protocol"
)

// getScheduleAssistant lazily builds the per-session schedule Assistant.
// A per-session instance gives per-connection rate limiting for free.
func (s *Session) getScheduleAssistant() *schedule.Assistant {
	s.scheduleAssistOnce.Do(func() {
		s.scheduleAssist = schedule.NewAssistant(schedule.AssistantConfig{
			Store:          s.scheduleStore,
			AgentsFn:       s.listScheduleAssistAgents,
			LLMProvidersFn: func() []config.LLMProviderConfig { return s.cfg.LLMProviders },
			LLMClient:      llm.NewClient(nil),
			Logger:         s.logger,
		})
	})
	return s.scheduleAssist
}

// listScheduleAssistAgents projects all agents (live + persisted) into the
// read-only view the Assistant renders into its prompt context block.
func (s *Session) listScheduleAssistAgents() []schedule.AgentInfo {
	managed := s.agentMgr.ListAgentsWithPersisted()
	infos := make([]schedule.AgentInfo, 0, len(managed))
	for _, m := range managed {
		snap := m.ToSnapshot()
		title := ""
		if snap.Title != nil {
			title = *snap.Title
		}
		infos = append(infos, schedule.AgentInfo{
			ID:       snap.ID,
			Title:    title,
			Provider: snap.Provider,
			Cwd:      snap.Cwd,
			Status:   string(snap.Status),
		})
	}
	return infos
}

func (s *Session) handleScheduleAssist(m *protocol.ScheduleAssistRequest) {
	payload, err := s.getScheduleAssistant().Assist(context.Background(), *m)
	if err != nil {
		// Domain errors are carried in the payload; a Go error is a wiring bug.
		s.logger.Error("schedule assist failed", "error", err)
		errMsg := err.Error()
		payload = &protocol.ScheduleAssistResponsePayload{
			RequestID: m.RequestID,
			Kind:      "error",
			Message:   "Internal error while processing the request.",
			Error:     &errMsg,
		}
	}
	s.sendScheduleAssistResponse(m.RequestID, payload)
}

func (s *Session) sendScheduleAssistResponse(requestID string, payload *protocol.ScheduleAssistResponsePayload) {
	payload.RequestID = requestID
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleAssistResponse{Type: "schedule/assist/response", Payload: *payload}))
}
