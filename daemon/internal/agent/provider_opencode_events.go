package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func (s *openCodeSession) consumeSSE(ctx context.Context, turnID string) (*AgentRunResult, error) {
	// Use /global/event which stays open and delivers turn events with heartbeats.
	// The /event endpoint in OpenCode 1.14.x only sends server.connected then closes,
	// making it unusable for turn event delivery.
	url := s.baseURL + "/global/event"

	timeout := s.sseReadIdleTimeout
	if timeout <= 0 {
		timeout = opencodeSSEReadIdleTimeout
	}

	var result *AgentRunResult
	var resultErr error
	terminalReached := false

	// Per-connection idle watchdog
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel() // stop the watchdog when consumeSSE returns
	var lastEventTime atomic.Int64
	lastEventTime.Store(time.Now().UnixNano())

	go func() {
		// Tick frequently enough to catch soft-timeout reliably even with
		// small test timeouts, but cap at 5s in production.
		tickInterval := timeout / 6
		if tickInterval < 500*time.Millisecond {
			tickInterval = 500 * time.Millisecond
		}
		if tickInterval > 5*time.Second {
			tickInterval = 5 * time.Second
		}
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		softTimeout := timeout / 2
		if softTimeout <= 0 {
			softTimeout = timeout
		}
		for {
			select {
			case <-ticker.C:
				last := time.Unix(0, lastEventTime.Load())
				idle := time.Since(last)
				if idle > timeout {
					s.base.Logger().Warn("SSE idle timeout — no events received, closing connection",
						"timeout", timeout, "turnID", turnID)
					connCancel()
					return
				}
				if idle >= softTimeout {
					if s.pingServer(ctx) {
						s.base.Logger().Debug("SSE heartbeat succeeded, resetting idle timer",
							"turnID", turnID)
						lastEventTime.Store(time.Now().UnixNano())
					}
				}
			case <-connCtx.Done():
				return
			}
		}
	}()

	req, err := http.NewRequestWithContext(connCtx, "GET", url, nil)
	if err != nil {
		connCancel()
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		connCancel()
		return nil, err
	}
	defer resp.Body.Close()

	s.base.Logger().Info("SSE connected to /global/event", "url", url, "status", resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		lastEventTime.Store(time.Now().UnixNano())
		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		// /global/event wraps events as:
		// {"directory":"...","project":"...","payload":{"id":"...","type":"...","properties":{...}}}
		// Server-level events (heartbeat, connected) have no directory/project wrapper:
		// {"payload":{"id":"...","type":"server.connected","properties":{}}}
		var rawEvent map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &rawEvent); err != nil {
			s.base.Logger().Warn("SSE unmarshal error", "error", err, "line", data)
			continue
		}

		// Extract payload (the actual event)
		payloadBytes, hasPayload := rawEvent["payload"]
		if !hasPayload {
			s.base.Logger().Warn("SSE event missing payload", "line", data)
			continue
		}

		var payload map[string]json.RawMessage
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			s.base.Logger().Warn("SSE payload unmarshal error", "error", err)
			continue
		}

		evtTypeBytes, ok := payload["type"]
		if !ok {
			s.base.Logger().Warn("SSE payload missing type")
			continue
		}
		var eventType string
		if err := json.Unmarshal(evtTypeBytes, &eventType); err != nil {
			s.base.Logger().Warn("SSE type unmarshal error", "error", err)
			continue
		}

		// Skip server-level events (heartbeat, connected)
		if strings.HasPrefix(eventType, "server.") {
			continue
		}

		// Filter by session ID: events include a sessionID in properties
		if propsBytes, ok := payload["properties"]; ok {
			var props map[string]interface{}
			if json.Unmarshal(propsBytes, &props) == nil {
				if sid, ok := props["sessionID"].(string); ok && sid != "" && sid != s.base.SessionID() {
					continue // Event for a different session
				}
			}
		}

		s.base.Logger().Info("SSE event received", "type", eventType)
		events := s.translateEvent(eventType, payload)
		for _, evt := range events {
			e := evt.Event
			switch se := e.(type) {
			case protocol.TurnCompletedStreamEvent:
				s.base.Logger().Info("SSE translated event", "type", se.StreamEventType())
				result = &AgentRunResult{SessionID: s.base.SessionID(), Canceled: false}
				resultErr = nil
				terminalReached = true
			case protocol.TurnFailedStreamEvent:
				s.base.Logger().Info("SSE translated event", "type", se.StreamEventType())
				result = &AgentRunResult{SessionID: s.base.SessionID(), Canceled: false}
				resultErr = fmt.Errorf("%v", se.Error)
				terminalReached = true
			case protocol.TurnCanceledStreamEvent:
				s.base.Logger().Info("SSE translated event", "type", se.StreamEventType())
				result = &AgentRunResult{SessionID: s.base.SessionID(), Canceled: true}
				resultErr = nil
				terminalReached = true
			case map[string]interface{}:
				s.base.Logger().Info("SSE translated event", "type", se["type"])
				switch se["type"] {
				case "turn_completed":
					result = &AgentRunResult{SessionID: s.base.SessionID(), Canceled: false}
					resultErr = nil
					terminalReached = true
				case "turn_failed":
					result = &AgentRunResult{SessionID: s.base.SessionID(), Canceled: false}
					resultErr = fmt.Errorf("%v", se["error"])
					terminalReached = true
				case "turn_canceled":
					result = &AgentRunResult{SessionID: s.base.SessionID(), Canceled: true}
					resultErr = nil
					terminalReached = true
				}
			}
			if terminalReached {
				s.finishForegroundTurn(evt, turnID)
				return result, resultErr
			}
			s.notifySubscribers(evt)
		}
	}
	scanErr := scanner.Err()
	s.base.Logger().Info("SSE connection closed", "scanErr", scanErr, "terminated", terminalReached)

	if !terminalReached {
		s.finishForegroundTurn(AgentStreamEvent{
			Event: protocol.TurnFailedStreamEvent{
				Provider: opencodeProviderName,
				Error:    "OpenCode event stream ended before the turn reached a terminal state",
			},
			Timestamp: time.Now(),
		}, turnID)
		return &AgentRunResult{SessionID: s.base.SessionID()}, fmt.Errorf("OpenCode event stream ended before the turn reached a terminal state")
	}

	return result, resultErr
}

// pingServer sends a lightweight HTTP request to the OpenCode server to verify
// it is still alive. Used by the SSE idle watchdog to avoid false-positive
// timeouts when the server is processing a long-running tool call.
func (s *openCodeSession) pingServer(ctx context.Context) bool {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(pingCtx, "GET", s.baseURL+"/command", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// finishForegroundTurn guards against duplicate turn completion and synthesizes
// failed tool calls on cancel/fail (matches Solo's finishForegroundTurn).
func (s *openCodeSession) finishForegroundTurn(evt AgentStreamEvent, turnID string) {
	s.mu.Lock()

	if s.activeForegroundTurnID != turnID {
		s.mu.Unlock()
		return
	}

	evtType := ""
	switch e := evt.Event.(type) {
	case protocol.TurnCanceledStreamEvent:
		evtType = e.StreamEventType()
	case protocol.TurnFailedStreamEvent:
		evtType = e.StreamEventType()
	case map[string]interface{}:
		evtType, _ = e["type"].(string)
	}

	var notifyEvents []AgentStreamEvent
	if evtType == "turn_canceled" || evtType == "turn_failed" {
		// Synthesize failed status for running tool calls
		for callID, item := range s.runningToolCalls {
			notifyEvents = append(notifyEvents, AgentStreamEvent{
				Event: protocol.TimelineStreamEvent{
					Item:     TimelineItem{Type: "tool_call", CallID: callID, Name: item.Name, Status: "failed", Error: &protocol.ToolError{Message: "Tool execution aborted"}},
					Provider: opencodeProviderName,
				},
				Timestamp: time.Now(),
			})
		}
	}

	s.activeForegroundTurnID = ""
	s.streamedPartKeys = make(map[string]bool)
	s.partTypes = make(map[string]string)
	s.runningToolCalls = make(map[string]timelineItem)

	s.mu.Unlock()

	for _, ne := range notifyEvents {
		s.dispatcher.Emit(ne)
	}
	if evtType != "" {
		s.dispatcher.Emit(evt)
	}
}

// --- Event Translation (matches Solo's translateOpenCodeEvent) ---

func (s *openCodeSession) translateEvent(eventType string, raw map[string]json.RawMessage) []AgentStreamEvent {
	var events []AgentStreamEvent
	now := time.Now()

	emit := func(e interface{}) {
		events = append(events, AgentStreamEvent{Event: e, Timestamp: now})
	}

	switch eventType {
	case "session.created", "session.updated":
		s.translateSessionCreatedOrUpdated(raw, emit)

	case "message.updated":
		s.translateMessageUpdated(raw, emit)

	case "message.part.delta":
		s.translateMessagePartDelta(raw, emit)

	case "message.part.updated":
		s.translateMessagePartUpdated(raw, emit)

	case "permission.asked":
		s.translatePermissionAsked(raw, emit)

	case "question.asked":
		s.translateQuestionAsked(raw, emit)

	case "session.idle":
		// Primary turn-completion signal from OpenCode (matches Solo's case "session.idle").
		// This fires when the session becomes idle after a turn finishes.
		var props struct {
			SessionID string `json:"sessionID"`
		}
		if raw, ok := raw["properties"]; ok {
			json.Unmarshal(raw, &props)
		}
		if props.SessionID == s.base.SessionID() {
			evt := protocol.TurnCompletedStreamEvent{
				Provider: opencodeProviderName,
			}
			if s.hasUsage() {
				evt.Usage = s.buildUsagePayload()
			}
			emit(evt)
		}

	case "session.error":
		var props struct {
			SessionID string      `json:"sessionID"`
			Error     interface{} `json:"error"`
		}
		if raw, ok := raw["properties"]; ok {
			json.Unmarshal(raw, &props)
		}
		if props.SessionID == s.base.SessionID() {
			emit(protocol.TurnFailedStreamEvent{
				Provider: opencodeProviderName,
				Error:    normalizeError(props.Error),
			})
		}

	case "session.status":
		s.translateSessionStatus(raw, emit)

	case "todo.updated":
		var props struct {
			SessionID string `json:"sessionID"`
			Todos     []struct {
				Content string `json:"content"`
				Status  string `json:"status"`
			} `json:"todos"`
		}
		if raw, ok := raw["properties"]; ok {
			json.Unmarshal(raw, &props)
		}
		if props.SessionID == s.base.SessionID() {
			var items []TodoItem
			for _, t := range props.Todos {
				if t.Content != "" {
					items = append(items, TodoItem{
						Text:      t.Content,
						Completed: t.Status == "completed",
					})
				}
			}
			if len(items) > 0 {
				emit(protocol.TimelineStreamEvent{
					Item:     TimelineItem{Type: "todo", TodoItems: items},
					Provider: opencodeProviderName,
				})
			}
		}

	case "session.compacted":
		var props struct {
			SessionID string `json:"sessionID"`
		}
		if raw, ok := raw["properties"]; ok {
			json.Unmarshal(raw, &props)
		}
		if props.SessionID == s.base.SessionID() {
			emit(protocol.TimelineStreamEvent{
				Item:     TimelineItem{Type: "compaction", CompactionStatus: "completed"},
				Provider: opencodeProviderName,
			})
		}
	}

	return events
}

func (s *openCodeSession) translateSessionCreatedOrUpdated(raw map[string]json.RawMessage, emit func(interface{})) {
	var props struct {
		Info struct {
			ID string `json:"id"`
		} `json:"info"`
		SessionID string `json:"sessionID"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &props)
	}
	sid := props.Info.ID
	if sid == "" {
		sid = props.SessionID
	}
	if sid == s.base.SessionID() {
		emit(protocol.ThreadStartedStreamEvent{
			Provider:  opencodeProviderName,
			SessionID: s.base.SessionID(),
		})
	}
}

// translateMessageUpdated tracks message roles and emits structured messages (gap #2 enhancement).
func (s *openCodeSession) translateMessageUpdated(raw map[string]json.RawMessage, emit func(interface{})) {
	var props struct {
		Info struct {
			ID         string      `json:"id"`
			SessionID  string      `json:"sessionID"`
			Role       string      `json:"role"`
			Structured interface{} `json:"structured"`
			ProviderID string      `json:"providerID"`
			ModelID    string      `json:"modelID"`
			Model      string      `json:"model"`
			Time       struct {
				Completed interface{} `json:"completed"`
			} `json:"time"`
		} `json:"info"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &props)
	}
	if props.Info.SessionID != s.base.SessionID() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.messageRoles[props.Info.ID] = props.Info.Role

	// Context window tracking: build lookup key from providerID/modelID (matches Solo)
	if props.Info.Role == "assistant" {
		modelKey := ""
		if props.Info.ProviderID != "" && props.Info.ModelID != "" {
			modelKey = props.Info.ProviderID + "/" + props.Info.ModelID
		} else if props.Info.Model != "" {
			modelKey = props.Info.Model
		}
		if modelKey != "" {
			if cw, ok := s.modelContextWindows[modelKey]; ok && cw > 0 {
				s.accumulatedUsage.ContextWindowMaxTokens = &cw
				s.selectedModelContextWindowMaxTokens = cw
			}
		}
	}

	// Emit structured assistant message if not already streamed via deltas
	if props.Info.Role == "assistant" && props.Info.Time.Completed != nil && !s.emittedStructuredMsgIDs[props.Info.ID] {
		if text := stringifyStructuredMessage(props.Info.Structured); text != "" {
			s.emittedStructuredMsgIDs[props.Info.ID] = true
			emit(protocol.TimelineStreamEvent{
				Item:     TimelineItem{Type: "assistant_message", Text: text},
				Provider: opencodeProviderName,
			})
		}
	}
}

func (s *openCodeSession) translateMessagePartDelta(raw map[string]json.RawMessage, emit func(interface{})) {
	var delta struct {
		SessionID string `json:"sessionID"`
		MessageID string `json:"messageID"`
		PartID    string `json:"partID"`
		Field     string `json:"field"`
		Delta     string `json:"delta"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &delta)
	}
	if delta.SessionID != s.base.SessionID() {
		return
	}
	if delta.Delta == "" || delta.Field == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	knownPartType := s.partTypes[delta.PartID]
	isReasoning := knownPartType == "reasoning" || delta.Field == "reasoning"

	if isReasoning {
		if delta.PartID != "" {
			s.streamedPartKeys["reasoning:"+delta.PartID] = true
		}
		emit(protocol.TimelineStreamEvent{
			Item:     TimelineItem{Type: "reasoning", Text: delta.Delta},
			Provider: opencodeProviderName,
		})
		return
	}

	if delta.Field != "text" {
		return
	}
	role := s.messageRoles[delta.MessageID]
	if role == "user" {
		return
	}
	if delta.PartID != "" {
		s.streamedPartKeys["text:"+delta.PartID] = true
	}
	emit(protocol.TimelineStreamEvent{
		Item:     TimelineItem{Type: "assistant_message", Text: delta.Delta},
		Provider: opencodeProviderName,
	})
}

// translateMessagePartUpdated handles completed parts with tool call detail mapping (gap #1) and usage tracking (gap #5).
func (s *openCodeSession) translateMessagePartUpdated(raw map[string]json.RawMessage, emit func(interface{})) {
	var props struct {
		Part struct {
			SessionID string          `json:"sessionID"`
			MessageID string          `json:"messageID"`
			ID        string          `json:"id"`
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			Tool      string          `json:"tool"`
			CallID    string          `json:"callID"`
			State     json.RawMessage `json:"state"`
			Auto      bool            `json:"auto"`
			Time      struct {
				End interface{} `json:"end"`
			} `json:"time"`
			// step-finish usage fields
			Cost   interface{} `json:"cost"`
			Tokens *struct {
				Input     interface{} `json:"input"`
				Output    interface{} `json:"output"`
				Reasoning interface{} `json:"reasoning"`
				Total     interface{} `json:"total"`
				Cache     *struct {
					Read  interface{} `json:"read"`
					Write interface{} `json:"write"`
				} `json:"cache"`
			} `json:"tokens"`
		} `json:"part"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &props)
	}
	part := props.Part
	if part.SessionID != s.base.SessionID() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if part.ID != "" {
		s.partTypes[part.ID] = part.Type
	}

	switch part.Type {
	case "text":
		role := s.messageRoles[part.MessageID]
		if role == "user" {
			return
		}
		if part.Time.End == nil {
			return
		}
		partKey := "text:" + part.ID
		if s.streamedPartKeys[partKey] {
			delete(s.streamedPartKeys, partKey)
			s.emittedStructuredMsgIDs[part.MessageID] = true
			return
		}
		if part.Text != "" {
			emit(protocol.TimelineStreamEvent{
				Item:     TimelineItem{Type: "assistant_message", Text: part.Text},
				Provider: opencodeProviderName,
			})
		}

	case "reasoning":
		if part.Time.End == nil {
			return
		}
		partKey := "reasoning:" + part.ID
		if s.streamedPartKeys[partKey] {
			delete(s.streamedPartKeys, partKey)
			return
		}
		if part.Text != "" {
			emit(protocol.TimelineStreamEvent{
				Item:     TimelineItem{Type: "reasoning", Text: part.Text},
				Provider: opencodeProviderName,
			})
		}

	case "tool":
		callID := part.CallID
		if callID == "" {
			callID = part.ID
		}

		// Parse tool state for detail mapping (gap #1)
		var toolStatus string
		var toolInput, toolOutput, toolError interface{}
		if part.State != nil {
			var state struct {
				Status string      `json:"status"`
				Input  interface{} `json:"input"`
				Output interface{} `json:"output"`
				Error  interface{} `json:"error"`
			}
			json.Unmarshal(part.State, &state)
			toolStatus = state.Status
			toolInput = state.Input
			toolOutput = state.Output
			toolError = state.Error
		}
		if toolStatus == "" {
			toolStatus = "running"
		}

		// Normalize status (matches Solo: output-present implies completed)
		normalizedStatus := normalizeToolStatus(toolStatus)
		if normalizedStatus == "running" && toolOutput != nil {
			normalizedStatus = "completed"
		}

		item := timelineItem{CallID: callID, Name: part.Tool, Status: normalizedStatus, Input: toolInput, Output: toolOutput, Error: toolError}
		if normalizedStatus == "running" {
			s.runningToolCalls[callID] = item
		} else {
			delete(s.runningToolCalls, callID)
		}

		emit(protocol.TimelineStreamEvent{
			Item:     buildToolCallTimelineItem(callID, part.Tool, normalizedStatus, toolInput, toolOutput, toolError),
			Provider: opencodeProviderName,
		})

	case "compaction":
		trigger := "manual"
		if part.Auto {
			trigger = "auto"
		}
		emit(protocol.TimelineStreamEvent{
			Item:     TimelineItem{Type: "compaction", CompactionStatus: "loading", Trigger: trigger},
			Provider: opencodeProviderName,
		})

	case "step-finish":
		// Usage tracking from step-finish (gap #5)
		s.mergeStepFinishUsage(part.Cost, part.Tokens)
		if s.hasUsage() {
			emit(protocol.UsageUpdatedStreamEvent{
				Provider: opencodeProviderName,
				Usage:    s.buildUsagePayload(),
			})
		}
	}
}

// mergeStepFinishUsage accumulates token usage from step-finish events.
func (s *openCodeSession) mergeStepFinishUsage(cost interface{}, tokens *struct {
	Input     interface{} `json:"input"`
	Output    interface{} `json:"output"`
	Reasoning interface{} `json:"reasoning"`
	Total     interface{} `json:"total"`
	Cache     *struct {
		Read  interface{} `json:"read"`
		Write interface{} `json:"write"`
	} `json:"cache"`
}) {
	if tokens != nil {
		if v := readPositiveFloat(tokens.Input); v != nil {
			s.accumulatedUsage.InputTokens = v
		}
		if v := readPositiveFloat(tokens.Output); v != nil {
			s.accumulatedUsage.OutputTokens = v
		}
		if tokens.Cache != nil {
			if v := readPositiveFloat(tokens.Cache.Read); v != nil {
				s.accumulatedUsage.CachedInputTokens = v
			}
		}
		// Calculate context window used
		total := 0.0
		if v := readPositiveFloat(tokens.Input); v != nil {
			total += *v
		}
		if v := readPositiveFloat(tokens.Output); v != nil {
			total += *v
		}
		if v := readPositiveFloat(tokens.Reasoning); v != nil {
			total += *v
		}
		if total > 0 {
			t := int(total)
			s.accumulatedUsage.ContextWindowUsedTokens = &t
		}
	}
	if v := readPositiveFloat(cost); v != nil {
		if s.accumulatedUsage.TotalCostUSD != nil {
			*s.accumulatedUsage.TotalCostUSD += *v
		} else {
			s.accumulatedUsage.TotalCostUSD = v
		}
	}
}

func (s *openCodeSession) hasUsage() bool {
	u := s.accumulatedUsage
	return u.InputTokens != nil || u.OutputTokens != nil || u.CachedInputTokens != nil || u.TotalCostUSD != nil || u.ContextWindowUsedTokens != nil
}

func (s *openCodeSession) buildUsagePayload() *protocol.AgentUsage {
	u := s.accumulatedUsage
	usage := &protocol.AgentUsage{}
	if u.InputTokens != nil {
		v := *u.InputTokens
		usage.InputTokens = &v
	}
	if u.OutputTokens != nil {
		v := *u.OutputTokens
		usage.OutputTokens = &v
	}
	if u.CachedInputTokens != nil {
		v := *u.CachedInputTokens
		usage.CachedInputTokens = &v
	}
	if u.TotalCostUSD != nil {
		v := *u.TotalCostUSD
		usage.TotalCostUSD = &v
	}
	if u.ContextWindowMaxTokens != nil {
		v := float64(*u.ContextWindowMaxTokens)
		usage.ContextWindowMaxTokens = &v
	}
	if u.ContextWindowUsedTokens != nil {
		v := float64(*u.ContextWindowUsedTokens)
		usage.ContextWindowUsedTokens = &v
	}
	return usage
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// --- Permission Handling (gap #11, #12) ---

func (s *openCodeSession) translatePermissionAsked(raw map[string]json.RawMessage, emit func(interface{})) {
	var props struct {
		SessionID  string          `json:"sessionID"`
		ID         string          `json:"id"`
		Permission string          `json:"permission"`
		Metadata   json.RawMessage `json:"metadata"`
		Tool       json.RawMessage `json:"tool"`
		Patterns   []string        `json:"patterns"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &props)
	}
	if props.SessionID != s.base.SessionID() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingPerms[props.ID] = pendingPermission{
		kind:  "tool",
		input: map[string]interface{}{},
	}

	command := extractPermissionField(props.Metadata, []string{"command", "cmd", "shellCommand"})
	cwd := extractPermissionField(props.Metadata, []string{"cwd", "directory", "path", "workdir"})
	reason := extractPermissionField(props.Metadata, []string{"reason", "purpose", "description", "message"})

	input := map[string]interface{}{}
	if len(props.Patterns) > 0 {
		input["patterns"] = props.Patterns
	}
	if props.Metadata != nil {
		var metadataObj interface{}
		json.Unmarshal(props.Metadata, &metadataObj)
		if metadataObj != nil {
			input["metadata"] = metadataObj
		}
	}
	if props.Tool != nil {
		var toolObj interface{}
		json.Unmarshal(props.Tool, &toolObj)
		if toolObj != nil {
			input["tool"] = toolObj
		}
	}
	if command != "" {
		input["command"] = command
	}

	var descParts []string
	if reason != "" {
		descParts = append(descParts, reason)
	}
	if len(props.Patterns) > 0 {
		descParts = append(descParts, "Scope: "+strings.Join(props.Patterns, ", "))
	}
	description := ""
	if len(descParts) > 0 {
		description = strings.Join(descParts, " - ")
	}

	detail := map[string]interface{}{}
	if command != "" {
		detail["type"] = "shell"
		detail["command"] = command
		if cwd != "" {
			detail["cwd"] = cwd
		}
	} else {
		detail["type"] = "unknown"
		detail["input"] = input
	}

	request := protocol.PermissionRequest{
		ID:          props.ID,
		Provider:    opencodeProviderName,
		Name:        props.Permission,
		Kind:        "tool",
		Title:       humanReadablePermission(props.Permission),
		Input:       input,
		Detail:      detail,
		Description: description,
	}

	emit(protocol.PermissionRequestedStreamEvent{
		Provider: opencodeProviderName,
		Request:  request,
	})
}

// translateQuestionAsked handles question.asked with multi-option support (gap #11).
func (s *openCodeSession) translateQuestionAsked(raw map[string]json.RawMessage, emit func(interface{})) {
	var props struct {
		SessionID string `json:"sessionID"`
		ID        string `json:"id"`
		Questions []struct {
			Question string `json:"question"`
			Header   string `json:"header"`
			Options  []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
			Multiple bool `json:"multiple"`
		} `json:"questions"`
		Tool json.RawMessage `json:"tool"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &props)
	}
	if props.SessionID != s.base.SessionID() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var questions []map[string]interface{}
	for _, q := range props.Questions {
		if q.Question == "" || q.Header == "" {
			continue
		}
		var options []map[string]interface{}
		for _, o := range q.Options {
			opt := map[string]interface{}{"label": o.Label}
			if o.Description != "" {
				opt["description"] = o.Description
			}
			options = append(options, opt)
		}
		question := map[string]interface{}{
			"question": q.Question,
			"header":   q.Header,
			"options":  options,
		}
		if q.Multiple {
			question["multiSelect"] = true
		}
		questions = append(questions, question)
	}

	if len(questions) == 0 {
		return
	}

	input := map[string]interface{}{
		"questions": questions,
	}

	s.pendingPerms[props.ID] = pendingPermission{
		kind:  "question",
		input: input,
	}

	request := protocol.PermissionRequest{
		ID:       props.ID,
		Provider: opencodeProviderName,
		Name:     "question",
		Kind:     "question",
		Title:    "Question",
		Input:    input,
	}
	if props.Tool != nil {
		var toolObj interface{}
		json.Unmarshal(props.Tool, &toolObj)
		if toolObj != nil {
			if request.Input == nil {
				request.Input = map[string]interface{}{}
			}
			request.Input["metadata"] = map[string]interface{}{"source": "opencode_question", "tool": toolObj}
		}
	}

	emit(protocol.PermissionRequestedStreamEvent{
		Provider: opencodeProviderName,
		Request:  request,
	})
}

func (s *openCodeSession) translateSessionStatus(raw map[string]json.RawMessage, emit func(interface{})) {
	var props struct {
		SessionID string `json:"sessionID"`
		Status    struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"status"`
	}
	if raw, ok := raw["properties"]; ok {
		json.Unmarshal(raw, &props)
	}
	if props.SessionID != s.base.SessionID() {
		return
	}

	switch props.Status.Type {
	case "idle":
		evt := protocol.TurnCompletedStreamEvent{
			Provider: opencodeProviderName,
		}
		if s.hasUsage() {
			evt.Usage = s.buildUsagePayload()
		}
		emit(evt)
	case "retry":
		if isFatalRetryMessage(props.Status.Message) {
			emit(protocol.TurnFailedStreamEvent{
				Provider: opencodeProviderName,
				Error:    props.Status.Message,
			})
		}
	}
}
