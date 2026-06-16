package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/schedule"
	"github.com/WuErPing/solo/protocol"
)

type daemonRunner struct {
	agentMgr *agent.AgentManager
	logger   *slog.Logger
}

func newDaemonRunner(agentMgr *agent.AgentManager, logger *slog.Logger) *daemonRunner {
	return &daemonRunner{
		agentMgr: agentMgr,
		logger:   logger.With("component", "schedule-runner"),
	}
}

func (r *daemonRunner) Run(sched protocol.StoredSchedule) schedule.RunResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var agentID string

	switch sched.Target.Type {
	case "agent":
		agentID = sched.Target.AgentID
		ag := r.agentMgr.GetAgent(agentID)
		if ag == nil {
			errMsg := fmt.Sprintf("agent %s not found", agentID)
			return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
		}
		if err := r.agentMgr.SendAgentMessage(ctx, agentID, sched.Prompt, nil, nil, ""); err != nil {
			errMsg := err.Error()
			return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
		}

	case "new-agent":
		if sched.Target.Config == nil {
			errMsg := "new-agent target requires config"
			return schedule.RunResult{Error: &errMsg}
		}
		config := &protocol.AgentSessionConfig{
			Provider: sched.Target.Config.Provider,
			Cwd:      sched.Target.Config.Cwd,
		}
		ag, err := r.agentMgr.CreateAgent(ctx, config, map[string]string{
			"source":     "schedule",
			"scheduleId": sched.ID,
		})
		if err != nil {
			errMsg := fmt.Sprintf("create agent: %s", err.Error())
			return schedule.RunResult{Error: &errMsg}
		}
		agentID = ag.ID
		if err := r.agentMgr.SendAgentMessage(ctx, agentID, sched.Prompt, nil, nil, ""); err != nil {
			errMsg := err.Error()
			return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
		}

	default:
		errMsg := fmt.Sprintf("unsupported target type: %s", sched.Target.Type)
		return schedule.RunResult{Error: &errMsg}
	}

	return r.waitForAgent(ctx, agentID)
}

func (r *daemonRunner) waitForAgent(ctx context.Context, agentID string) schedule.RunResult {
	ag := r.agentMgr.GetAgent(agentID)
	if ag == nil {
		errMsg := fmt.Sprintf("agent %s disappeared", agentID)
		return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
	}

	snapshot := ag.ToSnapshot()
	if snapshot.Status != protocol.AgentRunning && snapshot.Status != protocol.AgentInitializing {
		return agentResult(agentID, snapshot)
	}

	done := make(chan struct{}, 1)
	unsubscribe := r.agentMgr.Subscribe(func(event agent.AgentEvent) {
		if event.Type != agent.EventAgentState || event.AgentID != agentID || event.Agent == nil {
			return
		}
		snap := event.Agent.ToSnapshot()
		if snap.Status == protocol.AgentRunning || snap.Status == protocol.AgentInitializing {
			return
		}
		select {
		case done <- struct{}{}:
		default:
		}
	})
	defer unsubscribe()

	ag = r.agentMgr.GetAgent(agentID)
	if ag == nil {
		errMsg := fmt.Sprintf("agent %s disappeared", agentID)
		return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
	}
	snapshot = ag.ToSnapshot()
	if snapshot.Status != protocol.AgentRunning && snapshot.Status != protocol.AgentInitializing {
		return agentResult(agentID, snapshot)
	}

	select {
	case <-done:
		ag = r.agentMgr.GetAgent(agentID)
		if ag == nil {
			errMsg := fmt.Sprintf("agent %s disappeared", agentID)
			return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
		}
		return agentResult(agentID, ag.ToSnapshot())
	case <-ctx.Done():
		errMsg := "schedule run timed out"
		return schedule.RunResult{AgentID: &agentID, Error: &errMsg}
	}
}

func agentResult(agentID string, snapshot protocol.AgentSnapshotPayload) schedule.RunResult {
	result := schedule.RunResult{AgentID: &agentID}

	if snapshot.Status == protocol.AgentError {
		errText := "agent ended with error"
		if snapshot.LastError != nil && *snapshot.LastError != "" {
			errText = *snapshot.LastError
		}
		result.Error = &errText
	}

	return result
}
