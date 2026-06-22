package server

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

var runnerTestLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type fakeAgentManager struct {
	agents       map[string]*agent.ManagedAgent
	created      []*protocol.AgentSessionConfig
	messagesSent []string
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
	ag := agent.NewManagedAgent("agent-1", config.Provider, config.Cwd, config, labels)
	ag.SetLifecycle(protocol.AgentError)
	m.agents[ag.ID] = ag
	return ag, nil
}

func (m *fakeAgentManager) Subscribe(_ agent.AgentEventFunc) func() {
	return func() {}
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
