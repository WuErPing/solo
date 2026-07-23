// Package loop provides the loop execution engine and store for the Solo daemon.
package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

// defaultVerifyCheckTimeout bounds a single verify-check command. It is set
// generously because a real verification command (e.g. `make ci`) routinely
// runs builds and full test suites that take several minutes. A short timeout
// would kill legitimate checks and make the loop unable to ever pass.
const defaultVerifyCheckTimeout = 30 * time.Minute

// maxFeedbackOutputRunes caps how much of a failed check's output is fed back
// into the next worker prompt, so a huge CI log cannot blow up the prompt.
// The tail is kept because failures are usually reported at the end.
const maxFeedbackOutputRunes = 4000

// agentManager is the subset of agent.AgentManager used by the loop engine.
// It is defined as an interface so the engine can be unit-tested with a fake.
type agentManager interface {
	CreateAgent(ctx context.Context, config *protocol.AgentSessionConfig, labels map[string]string) (*agent.ManagedAgent, error)
	DeleteAgent(agentID string) error
	SendAgentMessage(ctx context.Context, agentID, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) error
	GetAgent(agentID string) *agent.ManagedAgent
	Subscribe(handler agent.AgentEventFunc) func()
}

// Engine executes loop records.
type Engine struct {
	store         *Store
	agentMgr      agentManager
	logger        *slog.Logger
	verifyTimeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewEngine creates a new loop engine.
func NewEngine(store *Store, agentMgr *agent.AgentManager, logger *slog.Logger) *Engine {
	return &Engine{
		store:         store,
		agentMgr:      agentMgr,
		logger:        logger.With("component", "loop-engine"),
		verifyTimeout: defaultVerifyCheckTimeout,
	}
}

// NewEngineWithManager creates a new loop engine with an arbitrary agentManager
// implementation. Intended for tests.
func NewEngineWithManager(store *Store, agentMgr agentManager, logger *slog.Logger) *Engine {
	return &Engine{
		store:         store,
		agentMgr:      agentMgr,
		logger:        logger.With("component", "loop-engine"),
		verifyTimeout: defaultVerifyCheckTimeout,
	}
}

// Start activates the engine and resumes any loops that were running when the
// daemon last shut down.
func (e *Engine) Start(ctx context.Context) {
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.resumeAll()
}

// Stop cancels the engine context and waits for all in-flight loops to finish.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

// Run starts executing a loop in a tracked goroutine.
func (e *Engine) Run(id string) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.run(e.ctx, id)
	}()
}

func (e *Engine) resumeAll() {
	running := e.store.Running()
	if len(running) > 0 {
		e.logger.Info("resuming loops after restart", "count", len(running))
	}
	for _, rec := range running {
		e.Run(rec.ID)
	}
}

func (e *Engine) run(ctx context.Context, id string) { //nolint:gocyclo // grandfathered CC=25
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

	// prevIter carries the previous iteration's verification result so its
	// failure output can be fed back to the next worker. nil on the first pass.
	var prevIter *protocol.LoopIterationRecord

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

		workerPrompt := buildWorkerPrompt(record.Prompt, prevIter)
		outcome, workerErr, workerOutput := e.runWorker(ctx, record, &iter, workerPrompt)
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

		if ctx.Err() != nil {
			_ = e.store.UpdateIteration(id, iter)
			e.finish(id, StatusStopped, "stopped by user")
			return
		}

		passed, reason := e.runVerifier(ctx, record, &iter, workerOutput)
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

		// Carry this iteration's verification failure into the next worker
		// prompt so the agent can act on what actually failed.
		iterCopy := iter
		prevIter = &iterCopy

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

func (e *Engine) runWorker(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord, prompt string) (*string, string, string) {
	config := agent.LoopRecordToWorkerConfig(*record)

	ag, err := e.agentMgr.CreateAgent(ctx, &config, map[string]string{"source": "loop"})
	if err != nil {
		reason := fmt.Sprintf("create worker agent: %s", err.Error())
		outcome := "failed"
		return &outcome, reason, ""
	}
	agentID := ag.ID
	iter.WorkerAgentID = &agentID

	defer func() {
		if err := e.agentMgr.DeleteAgent(agentID); err != nil {
			e.logger.Warn("failed to delete worker agent", "agentId", agentID, "error", err)
		}
	}()

	if err := e.agentMgr.SendAgentMessage(ctx, agentID, prompt, nil, nil, ""); err != nil {
		reason := fmt.Sprintf("send worker message: %s", err.Error())
		outcome := "failed"
		return &outcome, reason, ""
	}

	status, _ := e.waitForAgent(ctx, agentID)

	var finalText string
	if ag := e.agentMgr.GetAgent(agentID); ag != nil {
		finalText = ag.GetFinalText()
	}

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

	return &outcome, "", finalText
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

func (e *Engine) runVerifier(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord, workerOutput string) (bool, string) {
	if len(record.VerifyChecks) > 0 {
		return e.runVerifyChecks(ctx, record, iter)
	}
	if record.VerifyPrompt != nil && *record.VerifyPrompt != "" {
		return e.runVerifyPrompt(ctx, record, iter, workerOutput)
	}
	return true, "no verification configured"
}

func (e *Engine) runVerifyChecks(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord) (bool, string) {
	allPassed := true
	results := make([]protocol.LoopVerifyCheckResult, 0, len(record.VerifyChecks))

	for _, command := range record.VerifyChecks {
		startedAt := nowISO()
		exitCode, stdout, stderr, err := runShellCheck(ctx, record.Cwd, command, e.verifyTimeout)
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

func (e *Engine) runVerifyPrompt(ctx context.Context, record *protocol.LoopRecord, iter *protocol.LoopIterationRecord, workerOutput string) (bool, string) {
	config := agent.LoopRecordToVerifierConfig(*record)

	ag, err := e.agentMgr.CreateAgent(ctx, &config, map[string]string{"source": "loop-verifier"})
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

	prompt := fmt.Sprintf(
		"Original goal: %s\nWorker output: %s\nVerification instruction: %s\nRespond with JSON {\"passed\":bool, \"reason\":string}.",
		record.Prompt,
		workerOutput,
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

	var verifierText string
	if ag := e.agentMgr.GetAgent(agentID); ag != nil {
		verifierText = ag.GetFinalText()
	}

	passed, reason := parseVerifyResult(verifierText)
	iter.VerifyPrompt = &protocol.LoopVerifyPromptResult{
		Passed:          passed,
		Reason:          reason,
		VerifierAgentID: &agentID,
		StartedAt:       startedAt,
		CompletedAt:     nowISO(),
	}
	return passed, reason
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

// runShellCheck runs a shell command, bounded by the given timeout.
func runShellCheck(ctx context.Context, cwd, command string, timeout time.Duration) (exitCode int, stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
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

// buildWorkerPrompt augments the loop's base prompt with feedback from the
// previous iteration's failed verification, so a fresh worker can act on what
// actually went wrong instead of blindly repeating the same instruction.
func buildWorkerPrompt(basePrompt string, prev *protocol.LoopIterationRecord) string {
	if prev == nil {
		return basePrompt
	}
	feedback := buildVerificationFeedback(prev)
	if feedback == "" {
		return basePrompt
	}
	return basePrompt + "\n\n" + feedback
}

// buildVerificationFeedback renders the previous iteration's verification
// failures (failed checks and/or verifier reason) into a worker-facing block.
// It returns "" when there is nothing actionable to report.
func buildVerificationFeedback(prev *protocol.LoopIterationRecord) string {
	var b strings.Builder
	wrote := false

	for _, c := range prev.VerifyChecks {
		if c.Passed {
			continue
		}
		wrote = true
		fmt.Fprintf(&b, "\n$ %s\n(exit code %d)\n", c.Command, c.ExitCode)
		if out := combineOutput(c.Stdout, c.Stderr); out != "" {
			b.WriteString(truncateTail(out, maxFeedbackOutputRunes))
			b.WriteString("\n")
		}
	}

	if prev.VerifyPrompt != nil && !prev.VerifyPrompt.Passed && prev.VerifyPrompt.Reason != "" {
		wrote = true
		fmt.Fprintf(&b, "\nVerifier feedback: %s\n", prev.VerifyPrompt.Reason)
	}

	if !wrote {
		return ""
	}

	return "The previous attempt did not pass verification. " +
		"Fix the problems below, then make sure every check passes.\n" + b.String()
}

// combineOutput joins non-empty trimmed stdout and stderr with a newline.
func combineOutput(stdout, stderr string) string {
	parts := make([]string, 0, 2)
	if s := strings.TrimSpace(stdout); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(stderr); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n")
}

// truncateTail keeps the last maxRunes runes of s, prefixed with a marker when
// truncation occurs. The tail is kept because command failures are typically
// reported at the end of the output.
func truncateTail(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return "...(truncated)\n" + string(r[len(r)-maxRunes:])
}

// parseVerifyResult extracts the pass/fail outcome from a verifier agent's
// text response. It tolerates markdown code fences and surrounding prose.
func parseVerifyResult(text string) (bool, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return false, "verifier produced no output"
	}
	if idx := strings.Index(text, "{"); idx >= 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "}"); idx >= 0 {
		text = text[:idx+1]
	}
	var vr VerifyResult
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		return false, fmt.Sprintf("verifier output not valid JSON: %s", truncateTail(text, 200))
	}
	return vr.Passed, vr.Reason
}
