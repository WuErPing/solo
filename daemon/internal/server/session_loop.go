package server

import (
	"context"
	"sort"

	"github.com/WuErPing/solo/daemon/internal/loop"
	"github.com/WuErPing/solo/protocol"
)

func (s *Session) handleLoopRun(m *protocol.LoopRunRequest) {
	if s.loopStore == nil {
		s.sendLoopRunResponse(m.RequestID, nil, "loop module not initialized")
		return
	}

	var record *protocol.LoopRecord
	var err error

	// If TemplateID is provided, create an instance from existing template
	if m.TemplateID != "" {
		record, err = s.loopStore.CreateInstance(m.TemplateID, *m, s.defaultLoopProvider)
	} else {
		record, err = s.loopStore.Create(*m, s.defaultLoopProvider)
	}

	if err != nil {
		s.sendLoopRunResponse(m.RequestID, nil, err.Error())
		return
	}

	// Start the loop engine in the background.
	engine := loop.NewEngine(s.loopStore, s.agentMgr, s.logger)
	engine.Start(context.Background(), record.ID)

	s.sendLoopRunResponse(m.RequestID, record, "")
}

func (s *Session) handleLoopList(m *protocol.LoopListRequest) {
	if s.loopStore == nil {
		s.sendLoopListResponse(m.RequestID, nil, "loop module not initialized")
		return
	}
	list := s.loopStore.List()
	s.sendLoopListResponse(m.RequestID, list, "")
}

func (s *Session) handleLoopInspect(m *protocol.LoopInspectRequest) {
	if s.loopStore == nil {
		s.sendLoopInspectResponse(m.RequestID, nil, "loop module not initialized")
		return
	}
	record, ok := s.loopStore.Get(m.ID)
	if !ok {
		s.sendLoopInspectResponse(m.RequestID, nil, "loop not found")
		return
	}
	s.sendLoopInspectResponse(m.RequestID, record, "")
}

func (s *Session) handleLoopLogs(m *protocol.LoopLogsRequest) {
	if s.loopStore == nil {
		s.sendLoopLogsResponse(m.RequestID, nil, nil, 0, "loop module not initialized")
		return
	}
	record, ok := s.loopStore.Get(m.ID)
	if !ok {
		s.sendLoopLogsResponse(m.RequestID, nil, nil, 0, "loop not found")
		return
	}

	entries := record.Logs
	afterSeq := 0
	if m.AfterSeq != nil {
		afterSeq = *m.AfterSeq
		filtered := make([]protocol.LoopLogEntry, 0, len(entries))
		for _, e := range entries {
			if e.Seq > afterSeq {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	nextCursor := record.NextLogSeq
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		if last.Seq >= nextCursor {
			nextCursor = last.Seq + 1
		}
	}

	s.sendLoopLogsResponse(m.RequestID, record, entries, nextCursor, "")
}

func (s *Session) handleLoopStop(m *protocol.LoopStopRequest) {
	if s.loopStore == nil {
		s.sendLoopStopResponse(m.RequestID, nil, "loop module not initialized")
		return
	}
	record, err := s.loopStore.Stop(m.ID)
	if err != nil {
		s.sendLoopStopResponse(m.RequestID, nil, err.Error())
		return
	}
	s.sendLoopStopResponse(m.RequestID, record, "")
}

func (s *Session) handleLoopUpdate(m *protocol.LoopUpdateRequest) {
	if s.loopStore == nil {
		s.sendLoopUpdateResponse(m.RequestID, nil, "loop module not initialized")
		return
	}
	record, err := s.loopStore.Update(m.ID, loop.UpdateInput{
		Name:                  m.Name,
		Archive:               m.Archive,
		Prompt:                m.Prompt,
		Cwd:                   m.Cwd,
		VerifyChecks:          m.VerifyChecks,
		MaxIterations:         m.MaxIterations,
		AgentTemplate:         m.AgentTemplate,
		WorkerAgentTemplate:   m.WorkerAgentTemplate,
		VerifierAgentTemplate: m.VerifierAgentTemplate,
	})
	if err != nil {
		s.sendLoopUpdateResponse(m.RequestID, nil, err.Error())
		return
	}
	s.sendLoopUpdateResponse(m.RequestID, record, "")
}

func (s *Session) handleLoopDelete(m *protocol.LoopDeleteRequest) {
	if s.loopStore == nil {
		s.sendLoopDeleteResponse(m.RequestID, m.ID, "loop module not initialized")
		return
	}
	if err := s.loopStore.Delete(m.ID); err != nil {
		s.sendLoopDeleteResponse(m.RequestID, m.ID, err.Error())
		return
	}
	s.sendLoopDeleteResponse(m.RequestID, m.ID, "")
}

func (s *Session) handleLoopTemplateList(m *protocol.LoopTemplateListRequest) {
	if s.loopStore == nil {
		s.sendLoopTemplateListResponse(m.RequestID, nil, "loop module not initialized")
		return
	}
	templates := s.loopStore.ListTemplates()
	s.sendLoopTemplateListResponse(m.RequestID, templates, "")
}

func (s *Session) handleLoopTemplateGet(m *protocol.LoopTemplateGetRequest) {
	if s.loopStore == nil {
		s.sendLoopTemplateGetResponse(m.RequestID, nil, nil, nil, "loop module not initialized")
		return
	}
	template, err := s.loopStore.GetTemplate(m.TemplateID)
	if err != nil {
		s.sendLoopTemplateGetResponse(m.RequestID, nil, nil, nil, err.Error())
		return
	}
	instances := s.loopStore.ListInstances(m.TemplateID)
	var latestRecord *protocol.LoopRecord
	if len(instances) > 0 {
		latestRecord, _ = s.loopStore.Get(instances[0].ID)
	}
	s.sendLoopTemplateGetResponse(m.RequestID, template, instances, latestRecord, "")
}

func (s *Session) handleLoopTemplateDelete(m *protocol.LoopTemplateDeleteRequest) {
	if s.loopStore == nil {
		s.sendLoopTemplateDeleteResponse(m.RequestID, "", "loop module not initialized")
		return
	}
	if err := s.loopStore.DeleteTemplate(m.TemplateID); err != nil {
		s.sendLoopTemplateDeleteResponse(m.RequestID, m.TemplateID, err.Error())
		return
	}
	s.sendLoopTemplateDeleteResponse(m.RequestID, m.TemplateID, "")
}

// defaultLoopProvider returns the first currently available provider.
// In test/E2E environments that enable the mock provider, prefer it so loops
// run quickly and without external API calls.
func (s *Session) defaultLoopProvider() (string, error) {
	available := s.registry.ListAvailable()
	providers := make([]string, 0, len(available))
	for name, err := range available {
		if err == nil {
			providers = append(providers, name)
		}
	}
	if len(providers) == 0 {
		return "", loop.ErrNoProviderAvailable
	}
	if containsString(providers, "mock") {
		return "mock", nil
	}
	sort.Strings(providers)
	return providers[0], nil
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// --- Send helpers ---

func (s *Session) sendLoopRunResponse(requestID string, record *protocol.LoopRecord, errMsg string) {
	payload := protocol.LoopRunResponsePayload{RequestID: requestID, Loop: record}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopRunResponse{Type: "loop/run/response", Payload: payload}))
}

func (s *Session) sendLoopListResponse(requestID string, loops []protocol.LoopListItem, errMsg string) {
	payload := protocol.LoopListResponsePayload{RequestID: requestID, Loops: loops}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopListResponse{Type: "loop/list/response", Payload: payload}))
}

func (s *Session) sendLoopInspectResponse(requestID string, record *protocol.LoopRecord, errMsg string) {
	payload := protocol.LoopInspectResponsePayload{RequestID: requestID, Loop: record}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopInspectResponse{Type: "loop/inspect/response", Payload: payload}))
}

func (s *Session) sendLoopLogsResponse(requestID string, record *protocol.LoopRecord, entries []protocol.LoopLogEntry, nextCursor int, errMsg string) {
	payload := protocol.LoopLogsResponsePayload{RequestID: requestID, Loop: record, Entries: entries, NextCursor: nextCursor}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopLogsResponse{Type: "loop/logs/response", Payload: payload}))
}

func (s *Session) sendLoopStopResponse(requestID string, record *protocol.LoopRecord, errMsg string) {
	payload := protocol.LoopStopResponsePayload{RequestID: requestID, Loop: record}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopStopResponse{Type: "loop/stop/response", Payload: payload}))
}

func (s *Session) sendLoopUpdateResponse(requestID string, record *protocol.LoopRecord, errMsg string) {
	payload := protocol.LoopUpdateResponsePayload{RequestID: requestID, Loop: record}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopUpdateResponse{Type: "loop/update/response", Payload: payload}))
}

func (s *Session) sendLoopDeleteResponse(requestID, id, errMsg string) {
	payload := protocol.LoopDeleteResponsePayload{RequestID: requestID, ID: id}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopDeleteResponse{Type: "loop/delete/response", Payload: payload}))
}

func (s *Session) sendLoopTemplateListResponse(requestID string, templates []protocol.LoopTemplateSummary, errMsg string) {
	payload := protocol.LoopTemplateListResponsePayload{RequestID: requestID, Templates: templates}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopTemplateListResponse{Type: "loop/template/list/response", Payload: payload}))
}

func (s *Session) sendLoopTemplateGetResponse(requestID string, template *protocol.LoopTemplateSummary, instances []protocol.LoopListItem, latestRecord *protocol.LoopRecord, errMsg string) {
	payload := protocol.LoopTemplateGetResponsePayload{RequestID: requestID, Template: template, Instances: instances, LatestRecord: latestRecord}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopTemplateGetResponse{Type: "loop/template/get/response", Payload: payload}))
}

func (s *Session) sendLoopTemplateDeleteResponse(requestID string, templateID string, errMsg string) {
	payload := protocol.LoopTemplateDeleteResponsePayload{RequestID: requestID, TemplateID: templateID}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.LoopTemplateDeleteResponse{Type: "loop/template/delete/response", Payload: payload}))
}
