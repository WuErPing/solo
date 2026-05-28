package server

// MemoryBridge is the minimal contract the server uses to forward user
// and assistant turns to a memory recorder. Implementations must be safe
// for concurrent use. A nil MemoryBridge disables recording.
//
// The canonical implementation lives in daemon/internal/memory/bridge.
type MemoryBridge interface {
	OnUserTurn(sessionID, agentID, content string)
	// OnAssistantTurn records a one-shot assistant message immediately.
	OnAssistantTurn(sessionID, agentID, content string)
	// OnAssistantChunk accumulates a streaming-assistant fragment; the
	// bridge coalesces fragments and persists a single turn when
	// OnAssistantTurnEnd fires for the same agentID.
	OnAssistantChunk(agentID, sessionID, fragment string)
	// OnAssistantTurnEnd flushes any buffered chunks for agentID as one
	// assistant turn. No-op when there is nothing buffered.
	OnAssistantTurnEnd(agentID, sessionID string)
	OnSystemTurn(sessionID, agentID, content string)
	// Close flushes any pending streaming buffers (e.g. turns that did
	// not see a turn_completed before shutdown) and releases resources.
	// Must be idempotent.
	Close() error
}

// SetMemoryBridge installs a MemoryBridge on this session. Passing nil
// disables recording.
func (s *Session) SetMemoryBridge(b MemoryBridge) {
	s.memoryBridge = b
}

// maybeRecordAssistantTurn inspects a stream event and routes it to the
// memory bridge:
//
//   - "timeline" events carrying an "assistant_message" item → OnAssistantChunk
//     (accumulate; the bridge decides when to persist).
//   - "turn_completed" / "turn_failed" / "turn_canceled" → OnAssistantTurnEnd
//     (flush buffered chunks as one assistant turn).
//
// Everything else is a silent no-op.
func (s *Session) maybeRecordAssistantTurn(agentID string, event interface{}) {
	if s.memoryBridge == nil {
		return
	}
	payload, ok := event.(map[string]interface{})
	if !ok {
		return
	}
	evtType, _ := payload["type"].(string)

	switch evtType {
	case "timeline":
		item, ok := payload["item"].(map[string]interface{})
		if !ok {
			return
		}
		if itemType, _ := item["type"].(string); itemType != "assistant_message" {
			return
		}
		text, _ := item["text"].(string)
		s.memoryBridge.OnAssistantChunk(agentID, s.clientID, text)

	case "turn_completed", "turn_failed", "turn_canceled":
		s.memoryBridge.OnAssistantTurnEnd(agentID, s.clientID)
	}
}
