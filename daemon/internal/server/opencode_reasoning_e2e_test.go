//go:build !short

package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
)

// newTestWSServerWithOpenCode creates a test WS server that registers both mock
// and opencode providers. It returns the server, httptest.Server, availability flag,
// a general model ID, a reasoning-capable model ID, a model with thinking variants,
// and that model's default thinking option ID. Callers should skip
// the test when opencodeAvailable is false or modelID is empty.
func newTestWSServerWithOpenCode(t *testing.T) (*WSServer, *httptest.Server, bool, string, string, string, string) {
	t.Helper()
	cfg := &config.Config{
		SoloHome:   t.TempDir(),
		ServerID:   "test-server",
		Version:    "0.1.0",
		AppBaseURL: "https://app.solo.sh",
	}
	logger := newTestLogger()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
	agentStorage.Initialize()
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewMockAgentClient())

	opencodeClient := agent.NewOpenCodeAgentClient("", logger)
	opencodeAvailable := true
	modelID := ""
	reasoningModelID := ""
	thinkingModelID := ""
	thinkingOptionID := ""
	if err := opencodeClient.IsAvailable(context.Background()); err != nil {
		opencodeAvailable = false
	} else {
		registry.Register(opencodeClient)
		// Pick models so tests use real connected models.
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if models, err := opencodeClient.ListModels(ctx, ""); err == nil && len(models) > 0 {
			modelID = models[0].ID
			// Find a reasoning-capable model and a model with thinking variants
			for _, m := range models {
				if supportsReasoning, ok := m.Metadata["supportsReasoning"].(bool); ok && supportsReasoning {
					reasoningModelID = m.ID
				}
				if m.DefaultThinkingOptionID != "" && thinkingModelID == "" {
					thinkingModelID = m.ID
					thinkingOptionID = m.DefaultThinkingOptionID
				}
			}
			if reasoningModelID == "" {
				reasoningModelID = modelID // fall back to general model
			}
			// If no model has thinking variants, fall back to general model
			if thinkingModelID == "" {
				thinkingModelID = modelID
			}
		}
	}

	agentMgr := agent.NewAgentManager(agentStorage, registry, logger)
	agentMgr.Initialize(context.Background())
	timelineStore := agent.NewInMemoryTimelineStore()
	workspaceStore := NewWorkspaceStore(cfg.SoloHome, logger)
	terminalMgr := terminal.NewTerminalManager(logger)
	projectReg := workspace.NewProjectRegistry(cfg.SoloHome)
	workspaceReg := workspace.NewWorkspaceRegistry(cfg.SoloHome)
	gitSvc := workspace.NewWorkspaceGitService()
	scriptMgr := workspace.NewScriptManager()
	scriptProxy := workspace.NewScriptProxy(logger, scriptMgr)

	pushTokenStore := push.NewInMemoryTokenStore()
	pusher := push.NewExpoPushService("", pushTokenStore, logger)
	activityTracker := NewClientActivityTracker()
	ws := NewWSServer(cfg, logger, agentMgr, timelineStore, registry, workspaceStore, terminalMgr, projectReg, workspaceReg, gitSvc, scriptMgr, scriptProxy, pushTokenStore, pusher, activityTracker)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.HandleWebSocket)
	mux.HandleFunc("/api/health", handleHealth)

	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		ts.Close()
		if opencodeAvailable {
			agent.ShutdownOpenCodeServerManager()
		}
	})
	return ws, ts, opencodeAvailable, modelID, reasoningModelID, thinkingModelID, thinkingOptionID
}

// timelineItemFromStream parses an agent_stream WS message and extracts the
// timeline item if the inner event type is "timeline".
func timelineItemFromStream(msg protocol.WSOutboundMessage) (item agent.TimelineItem, ok bool) {
	msgBytes, err := json.Marshal(msg.Message)
	if err != nil {
		return
	}
	var peek struct {
		Type    string `json:"type"`
		Payload struct {
			Event struct {
				Type string          `json:"type"`
				Item json.RawMessage `json:"item"`
			} `json:"event"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(msgBytes, &peek); err != nil {
		return
	}
	if peek.Type != "agent_stream" || peek.Payload.Event.Type != "timeline" {
		return
	}
	if err := json.Unmarshal(peek.Payload.Event.Item, &item); err != nil {
		return
	}
	return item, true
}

// collectTimelineEvents reads WS messages for agentID until the agent returns
// to idle status, collecting all timeline items from agent_stream events.
func collectTimelineEvents(t *testing.T, conn *websocket.Conn, agentID string, timeout time.Duration) []agent.TimelineItem {
	t.Helper()
	var items []agent.TimelineItem
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)
	defer conn.SetReadDeadline(time.Time{})

	for time.Now().Before(deadline) {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
				break
			}
			t.Fatalf("read: %v", err)
		}

		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(raw, &resp); err != nil {
			continue
		}

		// Check for agent returning to idle
		msgBytes, _ := json.Marshal(resp.Message)
		var statusPeek struct {
			Type    string `json:"type"`
			Payload struct {
				Kind  string `json:"kind"`
				Agent struct {
					ID     string                      `json:"id"`
					Status protocol.AgentLifecycleStatus `json:"status"`
				} `json:"agent"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &statusPeek) == nil &&
			statusPeek.Type == "agent_update" &&
			statusPeek.Payload.Kind == "upsert" &&
			statusPeek.Payload.Agent.ID == agentID &&
			statusPeek.Payload.Agent.Status == protocol.AgentIdle {
			break
		}

		// Collect timeline items
		if item, ok := timelineItemFromStream(resp); ok {
			items = append(items, item)
		}
	}
	return items
}

func TestOpenCodeReasoningEventsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	_, ts, opencodeAvailable, modelID, _, _, _ := newTestWSServerWithOpenCode(t)
	if !opencodeAvailable {
		t.Skip("opencode binary not found, skipping")
	}
	if modelID == "" {
		t.Skip("no connected opencode model available, skipping")
	}
	t.Logf("using model: %s", modelID)

	conn := dialAndHello(t, ts.URL, "test-opencode-reasoning")
	defer conn.Close()
	readInitialMessages(t, conn)

	cwd := t.TempDir()

	// Create agent with opencode provider
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-opencode-reasoning",
			"config": map[string]interface{}{
				"provider": "opencode",
				"cwd":      cwd,
				"model":    modelID,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	agentID := createdPayload.AgentID
	if agentID == "" {
		t.Fatal("expected non-empty agent ID")
	}
	defer deleteOpencodeAgentForTest(t, conn, agentID)

	// Wait for agent to be ready (idle after initialization)
	readUntilAgentUpdateStatus(t, conn, agentID, protocol.AgentIdle)

	// Send a message that should trigger reasoning
	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-send-reasoning",
			"agentId":   agentID,
			"text":      "What is 2+2? Think step by step.",
		}),
	}
	if err := conn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_agent_message: %v", err)
	}

	// Read send response
	readUntilType(t, conn, "send_agent_message_response")

	// Collect timeline events until agent returns to idle
	items := collectTimelineEvents(t, conn, agentID, 120*time.Second)

	// Assert: received some events
	if len(items) == 0 {
		t.Fatal("expected at least one timeline event, got none")
	}

	// Count by type
	reasoningCount := 0
	assistantCount := 0
	for _, item := range items {
		switch item.Type {
		case "reasoning":
			reasoningCount++
		case "assistant_message":
			assistantCount++
		}
	}

	t.Logf("collected %d timeline items: %d reasoning, %d assistant_message, %d other",
		len(items), reasoningCount, assistantCount, len(items)-reasoningCount-assistantCount)

	// Assert: got at least one reasoning or assistant_message
	if reasoningCount == 0 && assistantCount == 0 {
		t.Errorf("expected at least one reasoning or assistant_message event, got %d reasoning, %d assistant_message", reasoningCount, assistantCount)
	}

	// Delete agent (with longer timeout for opencode shutdown)
	deleteOpencodeAgentForTest(t, conn, agentID)
}

func TestOpenCodeReasoningDedupE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	_, ts, opencodeAvailable, _, _, thinkingModelID, thinkingOptionID := newTestWSServerWithOpenCode(t)
	if !opencodeAvailable {
		t.Skip("opencode binary not found, skipping")
	}
	if thinkingModelID == "" {
		t.Skip("no connected opencode model available, skipping")
	}
	t.Logf("using thinking model: %s with option: %s", thinkingModelID, thinkingOptionID)

	conn := dialAndHello(t, ts.URL, "test-opencode-dedup")
	defer conn.Close()
	readInitialMessages(t, conn)

	cwd := t.TempDir()

	// Create agent with opencode provider using a model with thinking enabled
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-opencode-dedup",
			"config": map[string]interface{}{
				"provider":         "opencode",
				"cwd":              cwd,
				"model":            thinkingModelID,
				"thinkingOptionId": thinkingOptionID,
			},
			"labels": map[string]string{},
		}),
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, conn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	agentID := createdPayload.AgentID
	if agentID == "" {
		t.Fatal("expected non-empty agent ID")
	}
	defer deleteOpencodeAgentForTest(t, conn, agentID)

	readUntilAgentUpdateStatus(t, conn, agentID, protocol.AgentIdle)

	// Send a reasoning prompt
	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-send-dedup",
			"agentId":   agentID,
			"text":      "What is 2+2? Think step by step.",
		}),
	}
	if err := conn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_agent_message: %v", err)
	}

	readUntilType(t, conn, "send_agent_message_response")

	items := collectTimelineEvents(t, conn, agentID, 120*time.Second)

	// Collect reasoning and assistant text
	var reasoningTexts []string
	var assistantTexts []string
	for _, item := range items {
		switch item.Type {
		case "reasoning":
			if item.Text != "" {
				reasoningTexts = append(reasoningTexts, item.Text)
			}
		case "assistant_message":
			if item.Text != "" {
				assistantTexts = append(assistantTexts, item.Text)
			}
		}
	}

	fullReasoning := strings.Join(reasoningTexts, "")
	fullAssistant := strings.Join(assistantTexts, "")

	t.Logf("reasoning: %d chars, assistant: %d chars", len(fullReasoning), len(fullAssistant))

	// Assert: model produced reasoning
	if len(reasoningTexts) == 0 {
		t.Fatal("expected at least one reasoning event with non-empty text")
	}

	// Assert: model produced assistant text
	if len(assistantTexts) == 0 {
		t.Fatal("expected at least one assistant_message event with non-empty text")
	}

	// Assert: reasoning text does NOT appear in assistant text (dedup validation)
	reasoningPrefix := fullReasoning
	if len(reasoningPrefix) > 50 {
		reasoningPrefix = reasoningPrefix[:50]
	}
	if len(reasoningPrefix) > 10 && strings.Contains(fullAssistant, reasoningPrefix) {
		t.Errorf("reasoning text (first 50 chars) should NOT appear in assistant text — dedup failed")
		t.Errorf("reasoning prefix: %q", reasoningPrefix)
		t.Errorf("assistant text: %q", truncateText(fullAssistant, 200))
	}

		deleteOpencodeAgentForTest(t, conn, agentID)
}

// deleteOpencodeAgentForTest is like deleteAgentForTest but with a longer read
// timeout (30s instead of 5s) because opencode agent shutdown involves abort +
// archive + server release which can be slow.
func deleteOpencodeAgentForTest(t *testing.T, conn *websocket.Conn, agentID string) {
	t.Helper()
	deleteReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "delete_agent_request",
			"requestId": "req-delete-" + agentID,
			"agentId":   agentID,
		}),
	}
	if err := conn.WriteJSON(deleteReq); err != nil {
		t.Logf("write delete_agent: %v (non-fatal in e2e)", err)
		return
	}
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read waiting for agent_deleted: %v (non-fatal in e2e)", err)
			return
		}
		var resp protocol.WSOutboundMessage
		if json.Unmarshal(msg, &resp) != nil {
			continue
		}
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				Status string `json:"status"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) == nil && peek.Payload.Status == "agent_deleted" {
			return
		}
	}
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}