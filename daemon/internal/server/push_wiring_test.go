package server

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/daemon/internal/schedule"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// These tests prove that the ActivityTracker wiring bugs exist.
// After the fix, they validate the corrected behavior.

// TestProveIt_SessionWithActivityTrackerPushSends proves that when
// activityTracker is set (as the fix does), broadcastAgentAttention
// actually sends a push notification.
func TestProveIt_SessionWithActivityTrackerPushSends(t *testing.T) {
	mockPusher := &mockPusher{}
	tokenStore := push.NewInMemoryTokenStore()
	tokenStore.Register("token-ios-1")

	activityTracker := NewClientActivityTracker()

	session := &Session{
		clientID:        "ios-client-1",
		cfg:             &config.Config{ServerID: "test-server"},
		pushTokenStore:  tokenStore,
		activityTracker: activityTracker,
		pusher:          mockPusher,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	event := agent.AgentStreamEvent{
		AgentID:   "agent-1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)
	time.Sleep(50 * time.Millisecond)

	if mockPusher.CallCount() == 0 {
		t.Error("push was NOT sent even though activityTracker is set")
	}
}

// TestProveIt_HeartbeatUpdatesActivityTracker proves that the
// client_heartbeat handler now calls UpdateActivity on the tracker.
func TestProveIt_HeartbeatUpdatesActivityTracker(t *testing.T) {
	tracker := NewClientActivityTracker()

	session := &Session{
		clientID:        "ios-client-1",
		activityTracker: tracker,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	session.handlerRegistry = newMessageHandlerRegistry()
	session.registerHandlers()

	msg := &protocol.ClientHeartbeatMessage{
		DeviceType:     "ios",
		AppVisible:     true,
		FocusedAgentID: strPtr("agent-1"),
	}

	session.handlerRegistry.Handle(session, msg)

	states := tracker.GetAllStates()
	if len(states) == 0 {
		t.Error("client_heartbeat handler did not call UpdateActivity; tracker is empty")
	}
}

// TestProveIt_WSServerHasActivityTracker proves that WSServer
// now stores and injects an ActivityTracker into sessions.
func TestProveIt_WSServerHasActivityTracker(t *testing.T) {
	cfg := &config.Config{
		SoloHome:   t.TempDir(),
		ServerID:   "test-server",
		Version:    "0.1.0",
		AppBaseURL: "https://solo.up2ai.top",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	agentStorage := agent.NewAgentStorage(cfg.SoloHome+"/agents", logger)
	agentStorage.Initialize()
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewMockAgentClient())
	agentMgr := agent.NewAgentManager(agentStorage, registry, logger)
	agentMgr.Initialize(context.TODO())
	timelineStore := agent.NewInMemoryTimelineStore()
	workspaceStore := NewWorkspaceStore(cfg.SoloHome, logger)
	terminalMgr := terminal.NewTerminalManager(logger)
	projectReg := workspace.NewProjectRegistry(cfg.SoloHome)
	workspaceReg := workspace.NewWorkspaceRegistry(cfg.SoloHome)
	gitSvc := workspace.NewWorkspaceGitService()
	scriptMgr := workspace.NewScriptManager()
	scriptProxy := workspace.NewScriptProxy(logger, scriptMgr)
	tokenStore := push.NewInMemoryTokenStore()
	pusher := push.NewExpoPushService("", tokenStore, logger)
	activityTracker := NewClientActivityTracker()

	ws := NewWSServerWithConfig(DaemonConfig{
		Config:          cfg,
		Logger:          logger,
		AgentMgr:        agentMgr,
		TimelineStore:   timelineStore,
		Registry:        registry,
		WorkspaceStore:  workspaceStore,
		TerminalMgr:     terminalMgr,
		ProjectReg:      projectReg,
		WorkspaceReg:    workspaceReg,
		GitSvc:          gitSvc,
		ScriptMgr:       scriptMgr,
		ScriptProxy:     scriptProxy,
		PushTokenStore:  tokenStore,
		Pusher:          pusher,
		ActivityTracker: activityTracker,
		ScheduleStore:   schedule.NewStore(),
	})

	if ws.activityTracker == nil {
		t.Error("WSServer.activityTracker is nil after fix")
	}
}

// TestProveIt_NotificationDataIncludesServerId proves that notification data
// sent in attention_required events includes the serverId field.
func TestProveIt_NotificationDataIncludesServerId(t *testing.T) {
	tracker := NewClientActivityTracker()
	tracker.UpdateActivity("client-1", false, "")

	session, sendCh := newCaptureSession()
	session.clientID = "client-1"
	session.cfg = &config.Config{ServerID: "test-server"}
	session.activityTracker = tracker
	session.pushTokenStore = push.NewInMemoryTokenStore()
	session.pusher = &mockPusher{}

	event := agent.AgentStreamEvent{
		AgentID:   "agent-1",
		Event:     protocol.AttentionRequiredStreamEvent{Provider: "opencode", Reason: "finished"},
		Timestamp: time.Now(),
	}

	session.handleStreamEvent(event)

	msgs := drainMessages(sendCh)
	streamMsg := findAgentStreamEvent(msgs)
	if streamMsg == nil {
		t.Fatal("expected agent_stream message")
	}

	payload, _ := streamMsg["payload"].(map[string]interface{})
	evt, _ := payload["event"].(map[string]interface{})
	notification, _ := evt["notification"].(map[string]interface{})
	data, _ := notification["data"].(map[string]interface{})

	if _, hasServerID := data["serverId"]; !hasServerID {
		t.Error("notification.data missing serverId field")
	}
	if data["serverId"] != "test-server" {
		t.Errorf("expected serverId 'test-server', got %v", data["serverId"])
	}
}

// TestProveIt_SessionCloseRemovesActivityTracker proves that when
// a session disconnects, the activity tracker state is cleaned up.
func TestProveIt_SessionCloseRemovesActivityTracker(t *testing.T) {
	tracker := NewClientActivityTracker()
	tracker.UpdateActivity("client-1", true, "agent-1")

	session := &Session{
		clientID:        "client-1",
		activityTracker: tracker,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	session.cleanupActivityTracker()

	states := tracker.GetAllStates()
	if len(states) != 0 {
		t.Errorf("after session close, activity tracker still has %d entries (expected 0)", len(states))
	}
}
