package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/protocol"
)

type mockPusher struct {
	mu    sync.Mutex
	calls []mockPushCall
}

type mockPushCall struct {
	Tokens  []string
	Payload push.NotificationPayload
}

func (m *mockPusher) Send(tokens []string, payload push.NotificationPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockPushCall{Tokens: tokens, Payload: payload})
	return nil
}

func (m *mockPusher) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockPusher) CallAt(i int) mockPushCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[i]
}

func newCaptureSession() (*Session, *sendQueue) {
	q := newSendQueue()
	s := &Session{
		cfg:       &config.Config{ServerID: "test-server"},
		sendQueue: q,
		done:      make(chan struct{}),
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return s, q
}

func drainMessages(q *sendQueue) []map[string]interface{} {
	q.Close()
	var msgs []map[string]interface{}
	for {
		item, ok := q.Pop()
		if !ok {
			break
		}
		var m map[string]interface{}
		if json.Unmarshal(item.data, &m) == nil {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

func findAgentStreamEvent(msgs []map[string]interface{}) map[string]interface{} {
	for _, m := range msgs {
		if m["type"] == "session" {
			if msg, ok := m["message"].(map[string]interface{}); ok {
				if msg["type"] == "agent_stream" {
					return msg
				}
			}
		}
	}
	return nil
}

func TestSession_BroadcastAgentAttention_WithPush(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	tokenStore.Register("token-1")

	activityTracker := NewClientActivityTracker()

	session := &Session{
		clientID:        "test-client",
		cfg:             &config.Config{ServerID: "test-server"},
		pushTokenStore:  tokenStore,
		activityTracker: activityTracker,
		pusher:          mockPusher,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	time.Sleep(100 * time.Millisecond)

	if mockPusher.CallCount() != 1 {
		t.Fatalf("expected 1 push call, got %d", mockPusher.CallCount())
	}

	call := mockPusher.CallAt(0)
	if len(call.Tokens) != 1 || call.Tokens[0] != "token-1" {
		t.Errorf("expected token-1, got %v", call.Tokens)
	}
	if call.Payload.Title != "Agent finished" {
		t.Errorf("expected title 'Agent finished', got %q", call.Payload.Title)
	}
}

func TestSession_BroadcastAgentAttention_NoPushWhenFocused(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	tokenStore.Register("token-1")

	activityTracker := NewClientActivityTracker()
	activityTracker.UpdateActivity("s1", true, "agent1")

	session := &Session{
		clientID:        "test-client",
		cfg:             &config.Config{ServerID: "test-server"},
		pushTokenStore:  tokenStore,
		activityTracker: activityTracker,
		pusher:          mockPusher,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	if mockPusher.CallCount() != 0 {
		t.Fatalf("expected 0 push calls when focused, got %d", mockPusher.CallCount())
	}
}

func TestSession_BroadcastAgentAttention_NoPushWhenRecent(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()

	activityTracker := NewClientActivityTracker()
	activityTracker.UpdateActivity("s1", true, "agent2")

	session := &Session{
		clientID:        "test-client",
		cfg:             &config.Config{ServerID: "test-server"},
		pushTokenStore:  tokenStore,
		activityTracker: activityTracker,
		pusher:          mockPusher,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	if mockPusher.CallCount() != 0 {
		t.Fatalf("expected 0 push calls when client is recent, got %d", mockPusher.CallCount())
	}
}

func TestSession_BroadcastAgentAttention_NoTokens(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	activityTracker := NewClientActivityTracker()

	session := &Session{
		clientID:        "test-client",
		cfg:             &config.Config{ServerID: "test-server"},
		pushTokenStore:  tokenStore,
		activityTracker: activityTracker,
		pusher:          mockPusher,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	if mockPusher.CallCount() != 0 {
		t.Fatalf("expected 0 push calls with no tokens, got %d", mockPusher.CallCount())
	}
}

func TestSession_BroadcastAgentAttention_ErrorReason(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	tokenStore.Register("token-1")

	activityTracker := NewClientActivityTracker()

	session := &Session{
		clientID:        "test-client",
		cfg:             &config.Config{ServerID: "test-server"},
		pushTokenStore:  tokenStore,
		activityTracker: activityTracker,
		pusher:          mockPusher,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	event := agent.AgentStreamEvent{
		AgentID: "agent1",
		Event: map[string]interface{}{
			"type":     "attention_required",
			"reason":   "error",
			"provider": "opencode",
		},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	if mockPusher.CallCount() != 0 {
		t.Fatalf("expected 0 push calls for error reason, got %d", mockPusher.CallCount())
	}
}

func TestSession_AttentionRequired_ForwardsNotificationFields(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	activityTracker := NewClientActivityTracker()
	activityTracker.UpdateActivity("test-client", true, "other-agent")

	session, sendCh := newCaptureSession()
	session.clientID = "test-client"
	session.pushTokenStore = tokenStore
	session.activityTracker = activityTracker
	session.pusher = mockPusher

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	msgs := drainMessages(sendCh)
	streamMsg := findAgentStreamEvent(msgs)
	if streamMsg == nil {
		t.Fatal("expected agent_stream message to be sent")
	}

	payload, ok := streamMsg["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload to be map, got %T", streamMsg["payload"])
	}

	evt, ok := payload["event"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected event to be map, got %T", payload["event"])
	}

	notification, ok := evt["notification"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected notification field in attention_required event, got %T", evt["notification"])
	}
	if notification["title"] != "Agent finished" {
		t.Errorf("expected notification title 'Agent finished', got %v", notification["title"])
	}
	if notification["body"] == nil || notification["body"] == "" {
		t.Error("expected notification body to be non-empty")
	}
	if notification["data"] == nil {
		t.Error("expected notification data to be present")
	}

	if _, exists := evt["shouldNotify"]; !exists {
		t.Error("expected shouldNotify field in attention_required event")
	}
}

func TestSession_AttentionRequired_SetsShouldNotifyTrueWhenInAppRecipient(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	activityTracker := NewClientActivityTracker()
	activityTracker.UpdateActivity("test-client", true, "other-agent")

	session, sendCh := newCaptureSession()
	session.clientID = "test-client"
	session.pushTokenStore = tokenStore
	session.activityTracker = activityTracker
	session.pusher = mockPusher

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	msgs := drainMessages(sendCh)
	streamMsg := findAgentStreamEvent(msgs)
	if streamMsg == nil {
		t.Fatal("expected agent_stream message to be sent")
	}

	payload, _ := streamMsg["payload"].(map[string]interface{})
	evt, _ := payload["event"].(map[string]interface{})
	shouldNotify, _ := evt["shouldNotify"].(bool)
	if !shouldNotify {
		t.Error("expected shouldNotify=true when client is in-app recipient")
	}
}

func TestSession_AttentionRequired_SetsShouldNotifyFalseWhenFocused(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	activityTracker := NewClientActivityTracker()
	activityTracker.UpdateActivity("test-client", true, "agent1")

	session, sendCh := newCaptureSession()
	session.clientID = "test-client"
	session.pushTokenStore = tokenStore
	session.activityTracker = activityTracker
	session.pusher = mockPusher

	event := agent.AgentStreamEvent{
		AgentID:   "agent1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	msgs := drainMessages(sendCh)
	streamMsg := findAgentStreamEvent(msgs)
	if streamMsg == nil {
		t.Fatal("expected agent_stream message to be sent")
	}

	payload, _ := streamMsg["payload"].(map[string]interface{})
	evt, _ := payload["event"].(map[string]interface{})
	shouldNotify, _ := evt["shouldNotify"].(bool)
	if shouldNotify {
		t.Error("expected shouldNotify=false when client is focused on the agent")
	}
}
