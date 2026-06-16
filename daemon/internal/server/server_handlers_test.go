package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// createTestAgent creates an agent via WebSocket and returns its ID.
func createTestAgent(t *testing.T, tsURL, clientID, provider, cwd string) string {
	t.Helper()
	conn := dialAndHello(t, tsURL, clientID)
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create",
			"config": map[string]interface{}{
				"provider": provider,
				"cwd":      cwd,
			},
			"labels": map[string]string{},
		}),
	})

	// Read until agent_created status message
	for {
		resp := readUntilType(t, conn, "agent_created")
		var inner struct {
			Payload struct {
				Status string `json:"status"`
				Agent  struct {
					ID string `json:"id"`
				} `json:"agent"`
			} `json:"payload"`
		}
		b, _ := json.Marshal(resp.Message)
		json.Unmarshal(b, &inner)
		if inner.Payload.Agent.ID != "" {
			return inner.Payload.Agent.ID
		}
	}
}

// ---- handleFetchAgent ----

func TestHandleFetchAgent_Found(t *testing.T) {
	_, ts := newTestWSServer(t)
	agentID := createTestAgent(t, ts.URL, "client-fetch-create", "mock", t.TempDir())

	conn := dialAndHello(t, ts.URL, "client-fetch")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agent_request",
			"requestId": "req-fetch-1",
			"agentId":   agentID,
		}),
	})

	resp := readUntilType(t, conn, "fetch_agent_response")
	payload := decodeSessionPayload[protocol.FetchAgentResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Agent == nil {
		t.Fatal("expected agent in response")
	}
	if payload.Agent.ID != agentID {
		t.Errorf("expected agentID %q, got %q", agentID, payload.Agent.ID)
	}
}

func TestHandleFetchAgent_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-fetch-notfound")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agent_request",
			"requestId": "req-fetch-nf",
			"agentId":   "no-such-agent",
		}),
	})

	resp := readUntilType(t, conn, "fetch_agent_response")
	payload := decodeSessionPayload[protocol.FetchAgentResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error for missing agent")
	}
}

// ---- handleFetchAgentTimeline ----

func TestHandleFetchAgentTimeline_Found(t *testing.T) {
	_, ts := newTestWSServer(t)
	agentID := createTestAgent(t, ts.URL, "client-tl-create", "mock", t.TempDir())

	conn := dialAndHello(t, ts.URL, "client-tl-fetch")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agent_timeline_request",
			"requestId": "req-tl-1",
			"agentId":   agentID,
		}),
	})

	resp := readUntilType(t, conn, "fetch_agent_timeline_response")
	payload := decodeSessionPayload[protocol.FetchAgentTimelineResponsePayload](t, resp)
	if payload.AgentID != agentID {
		t.Errorf("expected agentID %q, got %q", agentID, payload.AgentID)
	}
}

func TestHandleFetchAgentTimeline_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-tl-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agent_timeline_request",
			"requestId": "req-tl-nf",
			"agentId":   "no-such-agent",
		}),
	})

	resp := readUntilType(t, conn, "rpc_error")
	_ = resp // just ensure we got an error response
}

// ---- handleGetDaemonConfig ----

func TestHandleGetDaemonConfig(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-daemon-cfg")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "get_daemon_config_request",
			"requestId": "req-dcfg-1",
		}),
	})

	resp := readUntilType(t, conn, "get_daemon_config_response")
	payload := decodeSessionPayload[protocol.GetDaemonConfigResponsePayload](t, resp)
	if payload.RequestID != "req-dcfg-1" {
		t.Errorf("requestId: got %q, want req-dcfg-1", payload.RequestID)
	}
}

// ---- handleFetchAgentHistory ----

func TestHandleFetchAgentHistory(t *testing.T) {
	_, ts := newTestWSServer(t)
	createTestAgent(t, ts.URL, "client-hist-create", "mock", t.TempDir())

	conn := dialAndHello(t, ts.URL, "client-hist")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "fetch_agent_history_request",
			"requestId": "req-hist-1",
		}),
	})

	resp := readUntilType(t, conn, "fetch_agent_history_response")
	payload := decodeSessionPayload[protocol.FetchAgentHistoryResponsePayload](t, resp)
	if payload.RequestID != "req-hist-1" {
		t.Errorf("requestId: got %q, want req-hist-1", payload.RequestID)
	}
	if payload.Entries == nil {
		t.Error("expected non-nil entries slice")
	}
}

// ---- handleRefreshAgent ----

func TestHandleRefreshAgent_Found(t *testing.T) {
	_, ts := newTestWSServer(t)
	agentID := createTestAgent(t, ts.URL, "client-refresh-create", "mock", t.TempDir())

	conn := dialAndHello(t, ts.URL, "client-refresh")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "refresh_agent_request",
			"requestId": "req-refresh-1",
			"agentId":   agentID,
		}),
	})

	resp := readUntilType(t, conn, "agent_refreshed")
	_ = resp
}

func TestHandleRefreshAgent_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-refresh-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "refresh_agent_request",
			"requestId": "req-refresh-nf",
			"agentId":   "no-such-agent",
		}),
	})

	resp := readUntilType(t, conn, "rpc_error")
	_ = resp
}

// ---- handleSetAgentModel ----

func TestHandleSetAgentModel_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-model-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "set_agent_model_request",
			"requestId": "req-model-nf",
			"agentId":   "no-such-agent",
			"modelId":   "claude-3",
		}),
	})

	resp := readUntilType(t, conn, "rpc_error")
	_ = resp
}

// ---- handleSetAgentThinking ----

func TestHandleSetAgentThinking_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-thinking-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":             "set_agent_thinking_request",
			"requestId":        "req-thinking-nf",
			"agentId":          "no-such-agent",
			"thinkingOptionId": "fast",
		}),
	})

	resp := readUntilType(t, conn, "rpc_error")
	_ = resp
}

// ---- handleSetAgentFeature ----

func TestHandleSetAgentFeature(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-feature")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "set_agent_feature_request",
			"requestId": "req-feature-1",
			"agentId":   "any-agent",
			"featureId": "web-search",
			"value":     true,
		}),
	})

	resp := readUntilType(t, conn, "set_agent_feature_response")
	var inner struct {
		Type    string `json:"type"`
		Payload struct {
			Accepted bool `json:"accepted"`
		} `json:"payload"`
	}
	b, _ := json.Marshal(resp.Message)
	json.Unmarshal(b, &inner)
	if !inner.Payload.Accepted {
		t.Error("expected accepted=true")
	}
}

// ---- handleUpdateAgent ----

func TestHandleUpdateAgent_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-update-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "update_agent_request",
			"requestId": "req-update-nf",
			"agentId":   "no-such-agent",
		}),
	})

	resp := readUntilType(t, conn, "rpc_error")
	_ = resp
}

func TestHandleUpdateAgent_Found(t *testing.T) {
	_, ts := newTestWSServer(t)
	agentID := createTestAgent(t, ts.URL, "client-update-create", "mock", t.TempDir())

	conn := dialAndHello(t, ts.URL, "client-update")
	defer conn.Close()
	readInitialMessages(t, conn)

	name := "new-name"
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "update_agent_request",
			"requestId": "req-update-1",
			"agentId":   agentID,
			"name":      name,
		}),
	})

	resp := readUntilType(t, conn, "update_agent_response")
	var inner struct {
		Type    string `json:"type"`
		Payload struct {
			Accepted bool `json:"accepted"`
		} `json:"payload"`
	}
	b, _ := json.Marshal(resp.Message)
	json.Unmarshal(b, &inner)
	if !inner.Payload.Accepted {
		t.Error("expected accepted=true")
	}
}

// ---- handleCloseItems ----

func TestHandleCloseItems(t *testing.T) {
	_, ts := newTestWSServer(t)
	agentID := createTestAgent(t, ts.URL, "client-close-create", "mock", t.TempDir())

	conn := dialAndHello(t, ts.URL, "client-close")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":        "close_items_request",
			"requestId":   "req-close-1",
			"agentIds":    []string{agentID},
			"terminalIds": []string{},
		}),
	})

	// close_items produces agent_update messages; just drain briefly
	drainMessagesWithTimeout(t, conn, 500)
}

// ---- handleListCommands (no agent) ----

func TestHandleListCommands_NoAgent(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-cmds-noagent")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "list_commands_request",
			"requestId": "req-cmds-1",
			"agentId":   "no-such-agent",
		}),
	})

	resp := readUntilType(t, conn, "list_commands_response")
	payload := decodeSessionPayload[protocol.ListCommandsPayload](t, resp)
	if payload.Error == nil {
		t.Error("expected error for missing agent")
	}
}

func TestHandleListCommands_WithDraftConfig(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-cmds-draft")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "list_commands_request",
			"requestId": "req-cmds-draft",
			"agentId":   "",
			"draftConfig": map[string]interface{}{
				"provider": "mock",
				"cwd":      t.TempDir(),
			},
		}),
	})

	resp := readUntilType(t, conn, "list_commands_response")
	payload := decodeSessionPayload[protocol.ListCommandsPayload](t, resp)
	// mock provider returns no error for ListClientCommands
	if payload.RequestID != "req-cmds-draft" {
		t.Errorf("requestId: got %q", payload.RequestID)
	}
}

// ---- handleListProviderFeatures ----

func TestHandleListProviderFeatures(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-features")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "list_provider_features_request",
			"requestId": "req-feat-1",
			"draftConfig": map[string]interface{}{
				"provider": "mock",
				"cwd":      t.TempDir(),
			},
		}),
	})

	resp := readUntilType(t, conn, "list_provider_features_response")
	payload := decodeSessionPayload[protocol.ListProviderFeaturesPayload](t, resp)
	if payload.RequestID != "req-feat-1" {
		t.Errorf("requestId: got %q", payload.RequestID)
	}
	if payload.Error == nil {
		t.Error("expected unsupported error in list_provider_features")
	}
}

// ---- handleStatus HTTP endpoint ----

func TestHandleStatusEndpoint(t *testing.T) {
	_, ts := newTestWSServer(t)
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---- handleGetProvidersSnapshot ----

func TestHandleGetProvidersSnapshot(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-providers")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "get_providers_snapshot_request",
			"requestId": "req-providers-1",
		}),
	})

	resp := readUntilType(t, conn, "get_providers_snapshot_response")
	var inner struct {
		Payload struct {
			RequestID string `json:"requestId"`
		} `json:"payload"`
	}
	b, _ := json.Marshal(resp.Message)
	json.Unmarshal(b, &inner)
	if inner.Payload.RequestID != "req-providers-1" {
		t.Errorf("requestId: got %q, want req-providers-1", inner.Payload.RequestID)
	}
}

// ---- handler_registry HasHandler / unhandled message ----

func TestHandlerRegistryHasHandler(t *testing.T) {
	r := newMessageHandlerRegistry()
	r.Register("test_msg", func(_ *Session, _ protocol.SessionInboundMessage) {})
	if !r.HasHandler("test_msg") {
		t.Error("expected HasHandler to return true for registered type")
	}
	if r.HasHandler("no_such_type") {
		t.Error("expected HasHandler to return false for unknown type")
	}
}

func TestUnhandledSessionMessageReturnsRPCError(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-unknown-msg")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "totally_unknown_message_type_xyz",
			"requestId": "req-unknown-1",
		}),
	})

	resp := readUntilType(t, conn, "rpc_error")
	_ = resp
}
