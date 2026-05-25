package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

// TestMultiClientSync_NoDuplicateTimelineItems verifies that when multiple
// clients (different sessions) are connected and one sends a message, the
// shared timelineStore does not end up with duplicate entries.
func TestMultiClientSync_NoDuplicateTimelineItems(t *testing.T) {
	ws, ts := newTestWSServer(t)
	webConn := dialAndHello(t, ts.URL, "test-web-sync")
	defer webConn.Close()
	appConn := dialAndHello(t, ts.URL, "test-app-sync")
	defer appConn.Close()

	readInitialMessages(t, webConn)
	readInitialMessages(t, appConn)

	// Create agent from web client
	createReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "create_agent_request",
			"requestId": "req-create-sync",
			"config": map[string]interface{}{
				"provider": "mock",
				"cwd":      t.TempDir(),
			},
			"labels": map[string]string{},
		}),
	}
	if err := webConn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create_agent: %v", err)
	}

	created := readUntilType(t, webConn, "agent_created")
	createdPayload := decodeStatusPayload[protocol.AgentCreatedPayload](t, created)
	agentID := createdPayload.AgentID
	defer deleteAgentForTest(t, webConn, agentID)

	// Drain agent_update on both clients
	readUntilType(t, webConn, "agent_update")
	readUntilType(t, appConn, "agent_update")

	// Send message from web client
	sendReq := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "send_agent_message_request",
			"requestId": "req-send-sync",
			"agentId":   agentID,
			"text":      "hello from web",
			"messageId": "msg-web-hello-1",
		}),
	}
	if err := webConn.WriteJSON(sendReq); err != nil {
		t.Fatalf("write send_agent_message: %v", err)
	}

	// Wait for turn_completed on both clients
	waitForTurnCompleted(t, webConn, agentID)
	waitForTurnCompleted(t, appConn, agentID)

	// Small delay to let all async flush operations complete
	time.Sleep(200 * time.Millisecond)

	// Verify timelineStore has no duplicate items
	result := ws.timelineStore.Fetch(agentID, "tail", nil, 0)
	if result == nil {
		t.Fatal("timeline fetch returned nil")
	}

	duplicates := findDuplicateTimelineRows(result.Rows)
	if len(duplicates) > 0 {
		t.Fatalf("timeline has duplicate rows: %v", duplicates)
	}

	// Verify timeline contains the expected messages
	var hasUserMessage, hasAssistantMessage bool
	for _, row := range result.Rows {
		switch row.Item.Type {
		case "user_message":
			if row.Item.MessageID == "msg-web-hello-1" {
				hasUserMessage = true
			}
		case "assistant_message":
			if strings.Contains(row.Item.Text, "hello from web") {
				hasAssistantMessage = true
			}
		}
	}
	if !hasUserMessage {
		t.Error("timeline missing user_message with expected messageId")
	}
	if !hasAssistantMessage {
		t.Error("timeline missing assistant_message")
	}
}

func waitForTurnCompleted(t *testing.T, conn *websocket.Conn, agentID string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read turn_completed for %s: %v", agentID, err)
		}
		var resp protocol.WSOutboundMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		msgBytes, _ := json.Marshal(resp.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				AgentID string      `json:"agentId"`
				Event   interface{} `json:"event"`
			} `json:"payload"`
		}
		if json.Unmarshal(msgBytes, &peek) != nil {
			continue
		}
		if peek.Type != "agent_stream" || peek.Payload.AgentID != agentID {
			continue
		}
		evtBytes, _ := json.Marshal(peek.Payload.Event)
		var evt struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(evtBytes, &evt) != nil {
			continue
		}
		if evt.Type == "turn_completed" || evt.Type == "turn_failed" || evt.Type == "turn_canceled" {
			return
		}
	}
}

func findDuplicateTimelineRows(rows []agent.TimelineRow) []string {
	seen := make(map[string]int)
	var dups []string
	for _, row := range rows {
		key := fmt.Sprintf("%s:%s", row.Item.Type, timelineItemDedupKey(row.Item))
		seen[key]++
		if seen[key] == 2 {
			dups = append(dups, key)
		}
	}
	return dups
}

func timelineItemDedupKey(item agent.TimelineItem) string {
	switch item.Type {
	case "user_message":
		if item.MessageID != "" {
			return item.MessageID
		}
		return item.Text
	case "assistant_message", "reasoning":
		return item.Text
	case "tool_call":
		return item.CallID + ":" + item.Status
	case "error":
		return item.Message
	case "compaction":
		return item.CompactionStatus
	default:
		return fmt.Sprintf("%v", item)
	}
}
