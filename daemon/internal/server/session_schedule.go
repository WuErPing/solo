package server

import (
	"github.com/WuErPing/solo/protocol"
)

func (s *Session) handleScheduleCreate(m *protocol.ScheduleCreateRequest) {
	sched, err := s.scheduleStore.Create(*m)
	if err != nil {
		s.sendScheduleCreateResponse(m.RequestID, nil, err.Error())
		return
	}
	summary := toScheduleSummary(sched)
	s.sendScheduleCreateResponse(m.RequestID, &summary, "")
}

func (s *Session) handleScheduleList(m *protocol.ScheduleListRequest) {
	list := s.scheduleStore.List()
	s.sendScheduleListResponse(m.RequestID, list, "")
}

func (s *Session) handleScheduleInspect(m *protocol.ScheduleInspectRequest) {
	sched, ok := s.scheduleStore.Get(m.ScheduleID)
	if !ok {
		s.sendScheduleInspectResponse(m.RequestID, nil, "schedule not found")
		return
	}
	s.sendScheduleInspectResponse(m.RequestID, sched, "")
}

func (s *Session) handleScheduleLogs(m *protocol.ScheduleLogsRequest) {
	sched, ok := s.scheduleStore.Get(m.ScheduleID)
	if !ok {
		s.sendScheduleLogsResponse(m.RequestID, nil, "schedule not found")
		return
	}
	s.sendScheduleLogsResponse(m.RequestID, sched.Runs, "")
}

func (s *Session) handleSchedulePause(m *protocol.SchedulePauseRequest) {
	sched, err := s.scheduleStore.Pause(m.ScheduleID)
	if err != nil {
		s.sendSchedulePauseResponse(m.RequestID, nil, err.Error())
		return
	}
	summary := toScheduleSummary(sched)
	s.sendSchedulePauseResponse(m.RequestID, &summary, "")
}

func (s *Session) handleScheduleResume(m *protocol.ScheduleResumeRequest) {
	sched, err := s.scheduleStore.Resume(m.ScheduleID)
	if err != nil {
		s.sendScheduleResumeResponse(m.RequestID, nil, err.Error())
		return
	}
	summary := toScheduleSummary(sched)
	s.sendScheduleResumeResponse(m.RequestID, &summary, "")
}

func (s *Session) handleScheduleDelete(m *protocol.ScheduleDeleteRequest) {
	if err := s.scheduleStore.Delete(m.ScheduleID); err != nil {
		s.sendScheduleDeleteResponse(m.RequestID, m.ScheduleID, err.Error())
		return
	}
	s.sendScheduleDeleteResponse(m.RequestID, m.ScheduleID, "")
}

func (s *Session) handleScheduleUpdate(m *protocol.ScheduleUpdateRequest) {
	sched, err := s.scheduleStore.Update(*m)
	if err != nil {
		s.sendScheduleUpdateResponse(m.RequestID, m.ScheduleID, nil, err.Error())
		return
	}
	summary := toScheduleSummary(sched)
	s.sendScheduleUpdateResponse(m.RequestID, m.ScheduleID, &summary, "")
}

// --- Send helpers ---

func (s *Session) sendScheduleCreateResponse(requestID string, schedule *protocol.ScheduleSummary, errMsg string) {
	payload := protocol.ScheduleCreateResponsePayload{RequestID: requestID, Schedule: schedule}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleCreateResponse{Type: "schedule/create/response", Payload: payload}))
}

func (s *Session) sendScheduleListResponse(requestID string, schedules []protocol.ScheduleSummary, errMsg string) {
	payload := protocol.ScheduleListResponsePayload{RequestID: requestID, Schedules: schedules}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleListResponse{Type: "schedule/list/response", Payload: payload}))
}

func (s *Session) sendScheduleInspectResponse(requestID string, schedule *protocol.StoredSchedule, errMsg string) {
	payload := protocol.ScheduleInspectResponsePayload{RequestID: requestID, Schedule: schedule}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleInspectResponse{Type: "schedule/inspect/response", Payload: payload}))
}

func (s *Session) sendScheduleLogsResponse(requestID string, runs []protocol.ScheduleRun, errMsg string) {
	payload := protocol.ScheduleLogsResponsePayload{RequestID: requestID, Runs: runs}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleLogsResponse{Type: "schedule/logs/response", Payload: payload}))
}

func (s *Session) sendSchedulePauseResponse(requestID string, schedule *protocol.ScheduleSummary, errMsg string) {
	payload := protocol.SchedulePauseResponsePayload{RequestID: requestID, Schedule: schedule}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.SchedulePauseResponse{Type: "schedule/pause/response", Payload: payload}))
}

func (s *Session) sendScheduleResumeResponse(requestID string, schedule *protocol.ScheduleSummary, errMsg string) {
	payload := protocol.ScheduleResumeResponsePayload{RequestID: requestID, Schedule: schedule}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleResumeResponse{Type: "schedule/resume/response", Payload: payload}))
}

func (s *Session) sendScheduleDeleteResponse(requestID, scheduleID, errMsg string) {
	payload := protocol.ScheduleDeleteResponsePayload{RequestID: requestID, ScheduleID: scheduleID}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleDeleteResponse{Type: "schedule/delete/response", Payload: payload}))
}

func (s *Session) sendScheduleUpdateResponse(requestID, scheduleID string, schedule *protocol.ScheduleSummary, errMsg string) {
	payload := protocol.ScheduleUpdateResponsePayload{RequestID: requestID, ScheduleID: scheduleID, Schedule: schedule}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ScheduleUpdateResponse{Type: "schedule/update/response", Payload: payload}))
}

func toScheduleSummary(s *protocol.StoredSchedule) protocol.ScheduleSummary {
	return protocol.ScheduleSummary{
		ID:        s.ID,
		Name:      s.Name,
		Prompt:    s.Prompt,
		Cadence:   s.Cadence,
		Target:    s.Target,
		Cwd:       s.Cwd,
		Status:    s.Status,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		NextRunAt: s.NextRunAt,
		LastRunAt: s.LastRunAt,
		PausedAt:  s.PausedAt,
		ExpiresAt: s.ExpiresAt,
		MaxRuns:   s.MaxRuns,
	}
}
