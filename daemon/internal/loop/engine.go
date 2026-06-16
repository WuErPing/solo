// Package loop provides the loop execution engine and store for the Solo daemon.
package loop

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

// Engine executes loop records.
type Engine struct {
	store    *Store
	agentMgr *agent.AgentManager
	logger   *slog.Logger
}

// NewEngine creates a new loop engine.
func NewEngine(store *Store, agentMgr *agent.AgentManager, logger *slog.Logger) *Engine {
	return &Engine{
		store:    store,
		agentMgr: agentMgr,
		logger:   logger.With("component", "loop-engine"),
	}
}

// Start begins executing a loop in a new goroutine.
func (e *Engine) Start(ctx context.Context, id string) {
	go e.run(ctx, id)
}

func (e *Engine) run(ctx context.Context, id string) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("loop panic recovered", "loopId", id, "panic", r)
			reason := fmt.Sprintf("internal panic: %v", r)
			if _, err := e.store.SetStatus(id, StatusFailed, &reason); err != nil {
				e.logger.Error("failed to set loop failed status after panic", "loopId", id, "error", err)
			}
		}
		_ = e.store.Save()
	}()

	record, ok := e.store.Get(id)
	if !ok {
		e.logger.Warn("loop not found", "loopId", id)
		return
	}
	if record.Status != string(StatusRunning) {
		e.logger.Warn("loop is not running", "loopId", id, "status", record.Status)
		return
	}

	maxIterations := 10
	if record.MaxIterations != nil {
		maxIterations = *record.MaxIterations
	}

	deadline := time.Time{}
	if record.MaxTimeMs != nil && *record.MaxTimeMs > 0 {
		started, err := time.Parse(time.RFC3339, record.StartedAt)
		if err == nil {
			deadline = started.Add(time.Duration(*record.MaxTimeMs) * time.Millisecond)
		}
	}

	for i := 0; i < maxIterations; i++ {
		record, ok = e.store.Get(id)
		if !ok {
			return
		}

		if ctx.Err() != nil {
			e.finish(id, StatusStopped, "stopped by user")
			return
		}
		if record.StopRequestedAt != nil {
			e.finish(id, StatusStopped, "stopped by user")
			return
		}
		if !deadline.IsZero() && time.Now().UTC().After(deadline) {
			e.finish(id, StatusFailed, "max time exceeded")
			return
		}

		iterIndex := len(record.Iterations) + 1
		now := nowISO()
		iter := protocol.LoopIterationRecord{
			Index:           iterIndex,
			WorkerStartedAt: now,
			Status:          string(StatusRunning),
		}

		if err := e.store.AppendIteration(id, iter); err != nil {
			e.logger.Error("failed to append iteration", "loopId", id, "iteration", iterIndex, "error", err)
		}

		e.log(id, iterIndex, "loop", "info", fmt.Sprintf("Starting iteration %d", iterIndex))

		outcome, workerErr := e.runWorker(ctx, record, &iter)
		if outcome != nil {
			iter.WorkerOutcome = outcome
		}
		completedAt := nowISO()
		iter.WorkerCompletedAt = &completedAt

		if workerErr != "" {
			e.log(id, iterIndex, "worker", "error", workerErr)
		} else {
			e.log(id, iterIndex, "worker", "info", fmt.Sprintf("Worker finished: %s", *outcome))
		}

		passed, reason := e.runVerifier(ctx, record, &iter)
		e.log(id, iterIndex, "verifier", "info", fmt.Sprintf("Verifier result: passed=%v reason=%s", passed, reason))

		if err := e.store.UpdateIteration(id, iter); err != nil {
			e.logger.Error("failed to update iteration", "loopId", id, "iteration", iterIndex, "error", err)
		}

		if passed {
			iter.Status = string(StatusSucceeded)
			_ = e.store.UpdateIteration(id, iter)
			e.finish(id, StatusSucceeded, "")
			return
		}

		if i == maxIterations-1 {
			iter.Status = string(StatusFailed)
			_ = e.store.UpdateIteration(id, iter)
			e.finish(id, StatusFailed, "max iterations reached")
			return
		}

		if record.SleepMs > 0 {
			select {
			case <-ctx.Done():
				e.finish(id, StatusStopped, "stopped by user")
				return
			case <-time.After(time.Duration(record.SleepMs) * time.Millisecond):
			}
		}
	}

	e.finish(id, StatusFailed, "max iterations reached")
}

func (e *Engine) runWorker(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord) (*string, string) {
	config := &protocol.AgentSessionConfig{
		Provider: record.Provider,
		Cwd:      record.Cwd,
		Model:    record.Model,
	}

	ag, err := e.agentMgr.CreateAgent(ctx, config, map[string]string{"source": "loop"})
	if err != nil {
		reason := fmt.Sprintf("create worker agent: %s", err.Error())
		outcome := "failed"
		return &outcome, reason
	}
	agentID := ag.ID
	iter.WorkerAgentID = &agentID

	defer func() {
		if err := e.agentMgr.DeleteAgent(agentID); err != nil {
			e.logger.Warn("failed to delete worker agent", "agentId", agentID, "error", err)
		}
	}()

	if err := e.agentMgr.SendAgentMessage(ctx, agentID, record.Prompt, nil, nil, ""); err != nil {
		reason := fmt.Sprintf("send worker message: %s", err.Error())
		outcome := "failed"
		return &outcome, reason
	}

	status, _ := e.waitForAgent(ctx, agentID)

	var outcome string
	switch status {
	case protocol.AgentIdle:
		outcome = "completed"
	case protocol.AgentError:
		outcome = "failed"
	case protocol.AgentClosed:
		outcome = "canceled"
	default:
		outcome = "failed"
	}

	if ctx.Err() != nil {
		outcome = "canceled"
	}

	return &outcome, ""
}

func (e *Engine) waitForAgent(ctx context.Context, agentID string) (protocol.AgentLifecycleStatus, error) {
	ag := e.agentMgr.GetAgent(agentID)
	if ag == nil {
		return protocol.AgentError, fmt.Errorf("agent %s disappeared", agentID)
	}

	snapshot := ag.ToSnapshot()
	if snapshot.Status != protocol.AgentRunning && snapshot.Status != protocol.AgentInitializing {
		return snapshot.Status, nil
	}

	done := make(chan struct{}, 1)
	unsubscribe := e.agentMgr.Subscribe(func(event agent.AgentEvent) {
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

	ag = e.agentMgr.GetAgent(agentID)
	if ag == nil {
		return protocol.AgentError, fmt.Errorf("agent %s disappeared", agentID)
	}
	snapshot = ag.ToSnapshot()
	if snapshot.Status != protocol.AgentRunning && snapshot.Status != protocol.AgentInitializing {
		return snapshot.Status, nil
	}

	select {
	case <-done:
		ag = e.agentMgr.GetAgent(agentID)
		if ag == nil {
			return protocol.AgentError, fmt.Errorf("agent %s disappeared", agentID)
		}
		return ag.ToSnapshot().Status, nil
	case <-ctx.Done():
		return protocol.AgentError, ctx.Err()
	}
}

func (e *Engine) runVerifier(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord) (bool, string) {
	if len(record.VerifyChecks) > 0 {
		return e.runVerifyChecks(ctx, record, iter)
	}
	if record.VerifyPrompt != nil && *record.VerifyPrompt != "" {
		return e.runVerifyPrompt(ctx, record, iter)
	}
	return true, "no verification configured"
}

func (e *Engine) runVerifyChecks(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord) (bool, string) {
	allPassed := true
	results := make([]protocol.LoopVerifyCheckResult, 0, len(record.VerifyChecks))

	for _, command := range record.VerifyChecks {
		startedAt := nowISO()
		exitCode, stdout, stderr, err := runShellCheck(ctx, record.Cwd, command)
		completedAt := nowISO()

		passed := exitCode == 0 && err == nil
		if !passed {
			allPassed = false
		}

		results = append(results, protocol.LoopVerifyCheckResult{
			Command:     command,
			ExitCode:    exitCode,
			Passed:      passed,
			Stdout:      stdout,
			Stderr:      stderr,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
		})
	}

	iter.VerifyChecks = results
	if allPassed {
		return true, "all checks passed"
	}
	return false, "one or more checks failed"
}

func (e *Engine) runVerifyPrompt(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord) (bool, string) {
	provider := record.Provider
	model := record.Model
	if record.VerifierProvider != nil && *record.VerifierProvider != "" {
		provider = *record.VerifierProvider
	}
	if record.VerifierModel != nil && *record.VerifierModel != "" {
		model = record.VerifierModel
	}

	config := &protocol.AgentSessionConfig{
		Provider: provider,
		Cwd:      record.Cwd,
		Model:    model,
	}

	ag, err := e.agentMgr.CreateAgent(ctx, config, map[string]string{"source": "loop-verifier"})
	if err != nil {
		iter.VerifyPrompt = &protocol.LoopVerifyPromptResult{
			Passed:    false,
			Reason:    fmt.Sprintf("create verifier agent: %s", err.Error()),
			StartedAt: nowISO(),
		}
		return false, iter.VerifyPrompt.Reason
	}
	agentID := ag.ID
	defer func() {
		if err := e.agentMgr.DeleteAgent(agentID); err != nil {
			e.logger.Warn("failed to delete verifier agent", "agentId", agentID, "error", err)
		}
	}()

	// Worker output is not directly exposed by the agent manager snapshot in this
	// minimal implementation, so we pass an empty output placeholder.
	prompt := fmt.Sprintf(
		"Original goal: %s\nWorker output: %s\nVerification instruction: %s\nRespond with JSON {\"passed\":bool, \"reason\":string}.",
		record.Prompt,
		"",
		*record.VerifyPrompt,
	)

	startedAt := nowISO()
	if err := e.agentMgr.SendAgentMessage(ctx, agentID, prompt, nil, nil, ""); err != nil {
		iter.VerifyPrompt = &protocol.LoopVerifyPromptResult{
			Passed:      false,
			Reason:      fmt.Sprintf("send verifier message: %s", err.Error()),
			StartedAt:   startedAt,
			CompletedAt: nowISO(),
		}
		return false, iter.VerifyPrompt.Reason
	}

	_, _ = e.waitForAgent(ctx, agentID)

	// Final text is not exposed on the agent snapshot; mark as unable to verify.
	iter.VerifyPrompt = &protocol.LoopVerifyPromptResult{
		Passed:          false,
		Reason:          "verifier response unavailable in minimal implementation",
		VerifierAgentID: &agentID,
		StartedAt:       startedAt,
		CompletedAt:     nowISO(),
	}
	return false, iter.VerifyPrompt.Reason
}

func (e *Engine) log(id string, iteration int, source, level, text string) {
	entry := protocol.LoopLogEntry{
		Timestamp: nowISO(),
		Iteration: &iteration,
		Source:    source,
		Level:     level,
		Text:      text,
	}
	if err := e.store.AppendLog(id, entry); err != nil {
		e.logger.Warn("failed to append log", "loopId", id, "error", err)
	}
}

func (e *Engine) finish(id string, status Status, reason string) {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	if _, err := e.store.SetStatus(id, status, reasonPtr); err != nil {
		e.logger.Error("failed to set loop status", "loopId", id, "status", status, "error", err)
	}
	_ = e.store.Save()
}

// runShellCheck runs a shell command with a 60-second timeout.
func runShellCheck(ctx context.Context, cwd, command string) (exitCode int, stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if cwd != "" {
		cmd.Dir = cwd
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		return exitCode, stdout, stderr, runErr
	}
	return 0, stdout, stderr, nil
}
