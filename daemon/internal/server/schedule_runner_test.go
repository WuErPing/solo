package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

var runnerTestLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type fakeAgentManager struct {
	agents        map[string]*agent.ManagedAgent
	created       []*protocol.AgentSessionConfig
	createdLabels []map[string]string
	messagesSent  []string
	deleted       []string
}

func newFakeAgentManager() *fakeAgentManager {
	return &fakeAgentManager{
		agents:  make(map[string]*agent.ManagedAgent),
		created: nil,
	}
}

func (m *fakeAgentManager) GetAgent(agentID string) *agent.ManagedAgent {
	return m.agents[agentID]
}

func (m *fakeAgentManager) SendAgentMessage(_ context.Context, _, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) error {
	m.messagesSent = append(m.messagesSent, text)
	return nil
}

func (m *fakeAgentManager) CreateAgent(_ context.Context, config *protocol.AgentSessionConfig, labels map[string]string) (*agent.ManagedAgent, error) {
	m.created = append(m.created, config)
	m.createdLabels = append(m.createdLabels, labels)
	ag := agent.NewManagedAgent("agent-"+config.Provider, config.Provider, config.Cwd, config, labels)
	ag.SetError("fake agent finished")
	m.agents[ag.ID] = ag
	return ag, nil
}

func (m *fakeAgentManager) Subscribe(_ agent.AgentEventFunc) func() {
	return func() {}
}

func (m *fakeAgentManager) DeleteAgent(agentID string) error {
	m.deleted = append(m.deleted, agentID)
	delete(m.agents, agentID)
	return nil
}

func TestDaemonRunner_NewAgentUsesScheduleCwd(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	cwd := "/schedule-level-cwd"
	cfgCwd := "/config-cwd"
	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "test",
		Target: protocol.ScheduleTarget{
			Type: "new-agent",
			Config: &protocol.ScheduleAgentConfig{
				Provider: "claude",
				Cwd:      cfgCwd,
			},
		},
		Cwd: &cwd,
	}

	runner.Run(sched)

	if len(mgr.created) != 1 {
		t.Fatalf("expected 1 agent created, got %d", len(mgr.created))
	}
	createdConfig := mgr.created[0]
	if createdConfig.Cwd != cwd {
		t.Errorf("created agent cwd: got %q, want %q", createdConfig.Cwd, cwd)
	}
}

func TestDaemonRunner_NewAgentFallsBackToConfigCwd(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	cfgCwd := "/config-cwd"
	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "test",
		Target: protocol.ScheduleTarget{
			Type: "new-agent",
			Config: &protocol.ScheduleAgentConfig{
				Provider: "claude",
				Cwd:      cfgCwd,
			},
		},
	}

	runner.Run(sched)

	if len(mgr.created) != 1 {
		t.Fatalf("expected 1 agent created, got %d", len(mgr.created))
	}
	createdConfig := mgr.created[0]
	if createdConfig.Cwd != cfgCwd {
		t.Errorf("created agent cwd: got %q, want %q", createdConfig.Cwd, cfgCwd)
	}
}

func TestDaemonRunner_ProviderAgentUsesScheduleCwd(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	scheduleCwd := "/schedule-cwd"
	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "do work",
		Target: protocol.ScheduleTarget{
			Type:       "provider",
			ProviderID: "claude",
		},
		Cwd: &scheduleCwd,
	}

	result := runner.Run(sched)

	if len(mgr.created) != 1 {
		t.Fatalf("expected 1 agent created, got %d", len(mgr.created))
	}
	createdConfig := mgr.created[0]
	if createdConfig.Cwd != scheduleCwd {
		t.Errorf("created agent cwd: got %q, want %q", createdConfig.Cwd, scheduleCwd)
	}
	if createdConfig.Provider != "claude" {
		t.Errorf("created agent provider: got %q, want %q", createdConfig.Provider, "claude")
	}
	if len(mgr.messagesSent) != 1 || mgr.messagesSent[0] != "do work" {
		t.Errorf("messages sent: got %v, want %q", mgr.messagesSent, "do work")
	}
	if result.AgentID == nil || *result.AgentID != "agent-claude" {
		t.Errorf("result agent id: got %v, want %q", result.AgentID, "agent-claude")
	}
}

func TestDaemonRunner_ProviderAgentErrorsWhenProviderIdMissing(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "do work",
		Target: protocol.ScheduleTarget{
			Type: "provider",
		},
	}

	result := runner.Run(sched)

	if len(mgr.created) != 0 {
		t.Fatalf("expected 0 agents created, got %d", len(mgr.created))
	}
	if result.Error == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "provider target requires providerId"
	if *result.Error != want {
		t.Errorf("error: got %q, want %q", *result.Error, want)
	}
}

func TestDaemonRunner_ProviderAgentReturnsNewAgentResult(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "do work",
		Target: protocol.ScheduleTarget{
			Type:       "provider",
			ProviderID: "claude",
		},
	}

	result := runner.Run(sched)

	if result.Error == nil {
		t.Fatalf("expected error from created agent, got nil")
	}
	want := "fake agent finished"
	if *result.Error != want {
		t.Errorf("error: got %q, want %q", *result.Error, want)
	}
	if result.AgentID == nil || *result.AgentID != "agent-claude" {
		t.Errorf("result agent id: got %v, want %q", result.AgentID, "agent-claude")
	}
}

func TestDaemonRunner_ProviderAgentIsClosedAfterRun(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "do work",
		Target: protocol.ScheduleTarget{
			Type:       "provider",
			ProviderID: "claude",
		},
	}

	result := runner.Run(sched)

	if result.AgentID == nil {
		t.Fatalf("expected result agent id, got nil")
	}
	if len(mgr.deleted) != 1 {
		t.Fatalf("expected 1 agent deleted, got %d", len(mgr.deleted))
	}
	if mgr.deleted[0] != *result.AgentID {
		t.Errorf("deleted agent id: got %q, want %q", mgr.deleted[0], *result.AgentID)
	}
	if _, ok := mgr.agents[*result.AgentID]; ok {
		t.Errorf("expected agent %q to be removed from manager", *result.AgentID)
	}
}

func TestDaemonRunner_NewAgentIsClosedAfterRun(t *testing.T) {
	mgr := newFakeAgentManager()
	runner := newDaemonRunner(mgr, runnerTestLogger)

	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "do work",
		Target: protocol.ScheduleTarget{
			Type: "new-agent",
			Config: &protocol.ScheduleAgentConfig{
				Provider: "claude",
				Cwd:      "/tmp",
			},
		},
	}

	result := runner.Run(sched)

	if result.AgentID == nil {
		t.Fatalf("expected result agent id, got nil")
	}
	if len(mgr.deleted) != 1 {
		t.Fatalf("expected 1 agent deleted, got %d", len(mgr.deleted))
	}
	if mgr.deleted[0] != *result.AgentID {
		t.Errorf("deleted agent id: got %q, want %q", mgr.deleted[0], *result.AgentID)
	}
}

func TestDaemonRunner_ProviderAgentIsClosedEvenOnSendError(t *testing.T) {
	mgr := &failingSendAgentManager{
		fakeAgentManager: newFakeAgentManager(),
	}
	runner := newDaemonRunner(mgr, runnerTestLogger)

	sched := protocol.StoredSchedule{
		ID:     "sched-1",
		Prompt: "do work",
		Target: protocol.ScheduleTarget{
			Type:       "provider",
			ProviderID: "claude",
		},
	}

	runner.Run(sched)

	if len(mgr.deleted) != 1 {
		t.Fatalf("expected 1 agent deleted after send error, got %d", len(mgr.deleted))
	}
}

type failingSendAgentManager struct {
	*fakeAgentManager
}

func (m *failingSendAgentManager) SendAgentMessage(_ context.Context, _ string, _ string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) error {
	return fmt.Errorf("send failed")
}
