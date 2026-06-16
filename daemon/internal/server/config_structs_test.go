package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/daemon/internal/schedule"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// TestDaemonConfig verifies that DaemonConfig aggregates all dependencies
// needed by NewWSServer, reducing the parameter count from 15 to 1.
func TestDaemonConfig(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
	agentStorage.Initialize()
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewMockAgentClient())
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

	// RED: This should fail because DaemonConfig doesn't exist yet
	daemonCfg := DaemonConfig{
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
		PushTokenStore:  pushTokenStore,
		Pusher:          pusher,
		ActivityTracker: activityTracker,
		ScheduleStore:   schedule.NewStore(),
	}

	if daemonCfg.Config == nil {
		t.Error("DaemonConfig.Config should not be nil")
	}
	if daemonCfg.Logger == nil {
		t.Error("DaemonConfig.Logger should not be nil")
	}
	if daemonCfg.AgentMgr == nil {
		t.Error("DaemonConfig.AgentMgr should not be nil")
	}
	if daemonCfg.TimelineStore == nil {
		t.Error("DaemonConfig.TimelineStore should not be nil")
	}
	if daemonCfg.Registry == nil {
		t.Error("DaemonConfig.Registry should not be nil")
	}
	if daemonCfg.WorkspaceStore == nil {
		t.Error("DaemonConfig.WorkspaceStore should not be nil")
	}
	if daemonCfg.TerminalMgr == nil {
		t.Error("DaemonConfig.TerminalMgr should not be nil")
	}
	if daemonCfg.ProjectReg == nil {
		t.Error("DaemonConfig.ProjectReg should not be nil")
	}
	if daemonCfg.WorkspaceReg == nil {
		t.Error("DaemonConfig.WorkspaceReg should not be nil")
	}
	if daemonCfg.GitSvc == nil {
		t.Error("DaemonConfig.GitSvc should not be nil")
	}
	if daemonCfg.ScriptMgr == nil {
		t.Error("DaemonConfig.ScriptMgr should not be nil")
	}
	if daemonCfg.ScriptProxy == nil {
		t.Error("DaemonConfig.ScriptProxy should not be nil")
	}
	if daemonCfg.PushTokenStore == nil {
		t.Error("DaemonConfig.PushTokenStore should not be nil")
	}
	if daemonCfg.Pusher == nil {
		t.Error("DaemonConfig.Pusher should not be nil")
	}
	if daemonCfg.ActivityTracker == nil {
		t.Error("DaemonConfig.ActivityTracker should not be nil")
	}
}

// TestNewWSServerWithConfig verifies that NewWSServer can be created with
// a DaemonConfig instead of 15 individual parameters.
func TestNewWSServerWithConfig(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
	agentStorage.Initialize()
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewMockAgentClient())
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

	daemonCfg := DaemonConfig{
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
		PushTokenStore:  pushTokenStore,
		Pusher:          pusher,
		ActivityTracker: activityTracker,
		ScheduleStore:   schedule.NewStore(),
	}

	// RED: This should fail because NewWSServerWithConfig doesn't exist yet
	ws := NewWSServerWithConfig(daemonCfg)
	if ws == nil {
		t.Fatal("NewWSServerWithConfig should return a non-nil WSServer")
	}

	// Verify the server was created with the correct config
	if ws.cfg != cfg {
		t.Error("WSServer.cfg should match the provided config")
	}
	if ws.logger != logger {
		t.Error("WSServer.logger should match the provided logger")
	}
	if ws.agentMgr != agentMgr {
		t.Error("WSServer.agentMgr should match the provided agentMgr")
	}
}

// TestSessionConfig verifies that SessionConfig aggregates all dependencies
// needed by NewSession, reducing the parameter count from 16 to 3.
func TestSessionConfig(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
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

	// RED: This should fail because SessionConfig doesn't exist yet
	sessionCfg := SessionConfig{
		Config:         cfg,
		Logger:         logger,
		AgentMgr:       agentMgr,
		TimelineStore:  timelineStore,
		Registry:       registry,
		WorkspaceStore: workspaceStore,
		TerminalMgr:    terminalMgr,
		ProjectReg:     projectReg,
		WorkspaceReg:   workspaceReg,
		GitSvc:         gitSvc,
		ScriptMgr:      scriptMgr,
		ScriptProxy:    scriptProxy,
		Broadcast:      func(_ protocol.WSOutboundMessage) {},
	}

	// Verify all fields are set
	if sessionCfg.Config == nil {
		t.Error("SessionConfig.Config should not be nil")
	}
	if sessionCfg.Logger == nil {
		t.Error("SessionConfig.Logger should not be nil")
	}
	if sessionCfg.AgentMgr == nil {
		t.Error("SessionConfig.AgentMgr should not be nil")
	}
	if sessionCfg.TimelineStore == nil {
		t.Error("SessionConfig.TimelineStore should not be nil")
	}
	if sessionCfg.Registry == nil {
		t.Error("SessionConfig.Registry should not be nil")
	}
	if sessionCfg.WorkspaceStore == nil {
		t.Error("SessionConfig.WorkspaceStore should not be nil")
	}
	if sessionCfg.TerminalMgr == nil {
		t.Error("SessionConfig.TerminalMgr should not be nil")
	}
	if sessionCfg.ProjectReg == nil {
		t.Error("SessionConfig.ProjectReg should not be nil")
	}
	if sessionCfg.WorkspaceReg == nil {
		t.Error("SessionConfig.WorkspaceReg should not be nil")
	}
	if sessionCfg.GitSvc == nil {
		t.Error("SessionConfig.GitSvc should not be nil")
	}
	if sessionCfg.ScriptMgr == nil {
		t.Error("SessionConfig.ScriptMgr should not be nil")
	}
	if sessionCfg.ScriptProxy == nil {
		t.Error("SessionConfig.ScriptProxy should not be nil")
	}
	if sessionCfg.Broadcast == nil {
		t.Error("SessionConfig.Broadcast should not be nil")
	}
}

// TestNewSessionWithConfig verifies that NewSession can be created with
// a SessionConfig instead of 16 individual parameters.
func TestNewSessionWithConfig(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
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

	sessionCfg := SessionConfig{
		Config:         cfg,
		Logger:         logger,
		AgentMgr:       agentMgr,
		TimelineStore:  timelineStore,
		Registry:       registry,
		WorkspaceStore: workspaceStore,
		TerminalMgr:    terminalMgr,
		ProjectReg:     projectReg,
		WorkspaceReg:   workspaceReg,
		GitSvc:         gitSvc,
		ScriptMgr:      scriptMgr,
		ScriptProxy:    scriptProxy,
		Broadcast:      func(_ protocol.WSOutboundMessage) {},
	}

	// RED: This should fail because NewSessionWithConfig doesn't exist yet
	conn := newMockConn()
	sess := NewSessionWithConfig("test-client", string(protocol.ClientCLI), conn, sessionCfg)
	if sess == nil {
		t.Fatal("NewSessionWithConfig should return a non-nil Session")
	}

	// Verify the session was created with the correct config
	if sess.clientID != "test-client" {
		t.Errorf("Session.clientID should be 'test-client', got %q", sess.clientID)
	}
	if sess.clientType != string(protocol.ClientCLI) {
		t.Errorf("Session.clientType should be %q, got %q", string(protocol.ClientCLI), sess.clientType)
	}
	if sess.cfg != cfg {
		t.Error("Session.cfg should match the provided config")
	}
	if sess.agentMgr != agentMgr {
		t.Error("Session.agentMgr should match the provided agentMgr")
	}
}
