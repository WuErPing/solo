package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

// TestSession_UserMessageTimelineEventIsStoredAndSent verifies that user_message
// timeline events are persisted to the timeline store and forwarded to the client.
//
// This is a regression test for the bug where coalescer.Handle returns false for
// user_message (not a coalescable type) and the return value was ignored, causing
// the event to be silently dropped. On iOS reconnect, fetch_agent_timeline would
// not include the user's prompt, leading to the optimistic user_message being
// overwritten and lost.
func TestSession_UserMessageTimelineEventIsStoredAndSent(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	// Let session start up
	time.Sleep(100 * time.Millisecond)

	// Simulate a user_message timeline event arriving from the agent provider
	sess.handleStreamEvent(agent.AgentStreamEvent{
		AgentID: "agent-1",
		Event: protocol.TimelineStreamEvent{Provider: "claude", Item: protocol.TimelineItem{Type: "user_message", Text: "hello from user"}},
		Timestamp: time.Now(),
	})

	// Give time for async send
	time.Sleep(50 * time.Millisecond)

	// --- Assert 1: timeline store should contain the user_message ---
	result := sess.timelineStore.Fetch("agent-1", "tail", nil, 0)
	if result == nil || len(result.Rows) == 0 {
		t.Fatal("expected user_message to be stored in timeline, but timeline is empty")
	}

	found := false
	for _, row := range result.Rows {
		if row.Item.Type == "user_message" && row.Item.Text == "hello from user" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected user_message in timeline store, got rows: %+v", result.Rows)
	}

	// --- Assert 2: client should receive the agent_stream message ---
	conn.mu.Lock()
	messages := make([][]byte, len(conn.messages))
	copy(messages, conn.messages)
	conn.mu.Unlock()

	foundAgentStream := false
	for _, raw := range messages {
		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg["type"] != "session" {
			continue
		}
		innerMsg, ok := msg["message"].(map[string]interface{})
		if !ok {
			continue
		}
		if innerMsg["type"] != "agent_stream" {
			continue
		}
		payload, ok := innerMsg["payload"].(map[string]interface{})
		if !ok {
			continue
		}
		if payload["agentId"] != "agent-1" {
			continue
		}
		evt, ok := payload["event"].(map[string]interface{})
		if !ok {
			continue
		}
		if evt["type"] != "timeline" {
			continue
		}
		item, ok := evt["item"].(map[string]interface{})
		if !ok {
			continue
		}
		if item["type"] == "user_message" && item["text"] == "hello from user" {
			foundAgentStream = true
			break
		}
	}

	if !foundAgentStream {
		t.Error("expected agent_stream with user_message to be sent to client, but it was not found")
	}

	// Cleanup
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}
}
