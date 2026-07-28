// Package clienttest provides test helpers for clients that connect to the
// Solo daemon over WebSocket. It is kept separate from production code so that
// both the cli/internal/client tests and cli/cmd tests can share the same
// daemon mock without duplicating handshake / message-routing logic.
package clienttest

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// MockDaemon is a lightweight WebSocket server that impersonates the Solo
// daemon for unit tests. It completes the hello handshake, sends a providers
// snapshot, and responds to common session request types.
type MockDaemon struct {
	ServerID          string
	ProvidersSnapshot *protocol.ProvidersSnapshotPayload
	Agents            []protocol.AgentSnapshotPayload

	Upgrader websocket.Upgrader

	mu    sync.Mutex
	conns []*websocket.Conn
}

// NewMockDaemon returns a MockDaemon with sensible defaults:
//   - server id "test-server-id"
//   - providers: openai (ready, gpt-4) and anthropic (loading, claude-3)
//   - two agents: agent-abc123 (running) and agent-idle456 (idle)
func NewMockDaemon() *MockDaemon {
	now := time.Now().Format(time.RFC3339)
	return &MockDaemon{
		ServerID: "test-server-id",
		ProvidersSnapshot: &protocol.ProvidersSnapshotPayload{
			Entries: []protocol.ProviderSnapshotEntry{
				{
					Provider: "openai",
					Status:   protocol.ProviderReady,
					Label:    "OpenAI",
					Models:   []protocol.AgentModelDefinition{{ID: "gpt-4", Label: "GPT-4"}},
				},
				{
					Provider: "anthropic",
					Status:   protocol.ProviderLoading,
					Label:    "Anthropic",
					Models:   []protocol.AgentModelDefinition{{ID: "claude-3", Label: "Claude 3"}},
				},
			},
			GeneratedAt: now,
		},
		Agents: []protocol.AgentSnapshotPayload{
			{
				ID:        "agent-abc123",
				Provider:  "openai",
				Status:    protocol.AgentRunning,
				Cwd:       "/tmp",
				CreatedAt: now,
				Title:     stringPtr("Test Agent"),
			},
			{
				ID:        "agent-idle456",
				Provider:  "openai",
				Status:    protocol.AgentIdle,
				Cwd:       "/tmp",
				CreatedAt: now,
				Title:     stringPtr("Idle Agent"),
			},
		},
		Upgrader: websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }},
	}
}

func stringPtr(s string) *string { return &s }

// ServeHTTP implements http.Handler and should be passed to httptest.NewServer.
func (m *MockDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := m.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	m.mu.Lock()
	m.conns = append(m.conns, conn)
	m.mu.Unlock()
	defer conn.Close()

	var hello protocol.WSInboundMessage
	if err := conn.ReadJSON(&hello); err != nil || hello.Type != "hello" {
		return
	}

	_ = conn.WriteJSON(protocol.WSOutboundMessage{
		Type: "session",
		Message: map[string]interface{}{
			"type":     "server_info",
			"status":   "server_info",
			"serverId": m.ServerID,
			"version":  "0.1.0",
		},
	})

	if m.ProvidersSnapshot != nil {
		_ = conn.WriteJSON(protocol.WSOutboundMessage{
			Type: "session",
			Message: map[string]interface{}{
				"type":    "providers_snapshot_update",
				"payload": m.ProvidersSnapshot,
			},
		})
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var inbound protocol.WSInboundMessage
		if err := json.Unmarshal(data, &inbound); err != nil {
			continue
		}
		if inbound.Type != "session" {
			continue
		}

		var peek struct {
			Type      string `json:"type"`
			RequestID string `json:"requestId"`
			AgentID   string `json:"agentId"`
		}
		_ = json.Unmarshal(inbound.Message, &peek)

		respType, respPayload := m.buildResponse(peek.Type, peek.RequestID, peek.AgentID)
		_ = conn.WriteJSON(protocol.WSOutboundMessage{
			Type:    "session",
			Message: map[string]interface{}{"type": respType, "payload": respPayload},
		})
	}
}

func (m *MockDaemon) buildResponse(reqType, requestID, agentID string) (string, interface{}) {
	switch reqType {
	case "fetch_agents_request":
		entries := make([]map[string]interface{}, 0, len(m.Agents))
		for i := range m.Agents {
			entries = append(entries, map[string]interface{}{"agent": m.Agents[i]})
		}
		return "fetch_agents_response", map[string]interface{}{
			"requestId": requestID,
			"entries":   entries,
			"pageInfo":  map[string]interface{}{"hasMore": false},
		}
	case "fetch_agent_request":
		var agent *protocol.AgentSnapshotPayload
		for i := range m.Agents {
			if m.Agents[i].ID == agentID {
				agent = &m.Agents[i]
				break
			}
		}
		return "fetch_agent_response", map[string]interface{}{
			"requestId": requestID,
			"agent":     agent,
		}
	case "archive_agent_request":
		return "agent_archived", map[string]interface{}{
			"requestId": requestID,
			"agentId":   agentID,
		}
	case "delete_agent_request":
		return "agent_deleted", map[string]interface{}{
			"requestId": requestID,
			"agentId":   agentID,
		}
	case "cancel_agent_request":
		return "cancel_agent_response", map[string]interface{}{
			"requestId": requestID,
			"agentId":   agentID,
		}
	case "send_agent_message_request":
		return "send_agent_message_response", map[string]interface{}{
			"requestId": requestID,
			"agentId":   agentID,
		}
	case "shutdown_server_request":
		return "status", map[string]interface{}{
			"requestId": requestID,
			"type":      "server_shutdown",
		}
	case "restart_server_request":
		return "status", map[string]interface{}{
			"requestId": requestID,
			"type":      "server_restart",
		}
	case "set_agent_mode_request":
		return "set_agent_mode_response", map[string]interface{}{
			"requestId": requestID,
			"agentId":   agentID,
		}
	case "fetch_agent_timeline_request":
		return "fetch_agent_timeline_response", map[string]interface{}{
			"requestId": requestID,
			"entries":   []map[string]interface{}{},
		}
	case "create_agent_request":
		return "agent_created", map[string]interface{}{
			"requestId": requestID,
			"agentId":   "agent-new789",
			"status":    "initializing",
		}
	case "wait_for_finish_request":
		return "wait_for_finish_response", map[string]interface{}{
			"requestId": requestID,
			"status":    "idle",
			"final": map[string]interface{}{
				"id":       "agent-new789",
				"provider": "openai",
				"status":   "idle",
				"cwd":      "/tmp",
			},
		}
	default:
		return "fetch_agents_response", map[string]interface{}{
			"requestId": requestID,
			"entries":   []interface{}{},
			"pageInfo":  map[string]interface{}{"hasMore": false},
		}
	}
}

// CloseConnections closes all server-side WebSocket connections, simulating
// the daemon dying. httptest.Server.CloseClientConnections cannot reach
// hijacked WebSocket connections, so this helper is provided for tests.
func (m *MockDaemon) CloseConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, conn := range m.conns {
		_ = conn.Close()
	}
}
