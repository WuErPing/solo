package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// crashingAgentClient creates sessions whose Run crashes (wrapped
// ErrProviderCrashed) for the first crashCount calls, then succeeds.
type crashingAgentClient struct {
	crashCount int32
	session    *crashingAgentSession
}

func (c *crashingAgentClient) Provider() string                    { return "crashing" }
func (c *crashingAgentClient) IsAvailable(_ context.Context) error { return nil }
func (c *crashingAgentClient) CreateSession(_ context.Context, _ *protocol.AgentSessionConfig) (AgentSession, error) {
	c.session = &crashingAgentSession{crashCount: c.crashCount}
	return c.session, nil
}
func (c *crashingAgentClient) ResumeSession(ctx context.Context, _ *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: c.Provider()})
}
func (c *crashingAgentClient) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *crashingAgentClient) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *crashingAgentClient) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

type crashingAgentSession struct {
	crashCount int32
	runCalls   atomic.Int32
}

func (s *crashingAgentSession) Run(_ context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) (*AgentRunResult, error) {
	call := s.runCalls.Add(1)
	if call <= s.crashCount {
		return nil, fmt.Errorf("%w: exit status 139 (simulated)", ErrProviderCrashed)
	}
	return &AgentRunResult{SessionID: "crash-session", FinalText: "recovered: " + text}, nil
}
func (s *crashingAgentSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	ch := make(chan AgentStreamEvent)
	close(ch)
	return ch, nil
}
func (s *crashingAgentSession) Subscribe() <-chan AgentStreamEvent { return nil }
func (s *crashingAgentSession) Interrupt(_ context.Context) error  { return nil }
func (s *crashingAgentSession) Close() error                       { return nil }
func (s *crashingAgentSession) RespondPermission(_ string, _ protocol.AgentPermissionResponse) error {
	return nil
}
func (s *crashingAgentSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{Provider: "crashing"}, nil
}
func (s *crashingAgentSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *crashingAgentSession) GetCurrentMode(_ context.Context) (*string, error) { return nil, nil }
func (s *crashingAgentSession) SetMode(_ string) error                            { return nil }
func (s *crashingAgentSession) SetModel(_ string) error                           { return nil }
func (s *crashingAgentSession) SetThinkingOption(_ string) error                  { return nil }
func (s *crashingAgentSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "crashing", SessionID: "crash-session"}
}
func (s *crashingAgentSession) GetPendingPermissions() []interface{} { return nil }
func (s *crashingAgentSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *crashingAgentSession) StreamHistory(_ context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}

func newCrashTestManager(t *testing.T, client AgentClient) *AgentManager {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(client)
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}
	return manager
}

func subscribeStatuses(t *testing.T, manager *AgentManager, agentID string) chan protocol.AgentLifecycleStatus {
	t.Helper()
	updates := make(chan protocol.AgentLifecycleStatus, 32)
	unsub := manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentState && event.Agent != nil && event.AgentID == agentID {
			updates <- event.Agent.ToSnapshot().Status
		}
	})
	t.Cleanup(unsub)
	return updates
}

// awaitStatus drains the updates channel until want appears or the deadline passes.
func awaitStatus(t *testing.T, updates <-chan protocol.AgentLifecycleStatus, want protocol.AgentLifecycleStatus, wait time.Duration) {
	t.Helper()
	deadline := time.After(wait)
	for {
		select {
		case status := <-updates:
			if status == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for status %q", want)
		}
	}
}

func TestCrashRecovery_RetriesOnceThenSucceeds(t *testing.T) {
	client := &crashingAgentClient{crashCount: 1}
	manager := newCrashTestManager(t, client)
	manager.crashRecoveryDelay = 50 * time.Millisecond

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "crashing",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	updates := subscribeStatuses(t, manager, ag.ID)

	if err := manager.SendAgentMessage(context.Background(), ag.ID, "do the thing", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}

	// running -> error (crash surfaced to app) -> running (retry) -> idle
	awaitStatus(t, updates, protocol.AgentRunning, 2*time.Second)
	awaitStatus(t, updates, protocol.AgentError, 2*time.Second)
	awaitStatus(t, updates, protocol.AgentRunning, 2*time.Second)
	awaitStatus(t, updates, protocol.AgentIdle, 2*time.Second)

	if got := client.session.runCalls.Load(); got != 2 {
		t.Fatalf("Run called %d times, want 2 (original + one recovery retry)", got)
	}
	if got := ag.GetFinalText(); got != "recovered: do the thing" {
		t.Fatalf("FinalText = %q, want recovered turn output", got)
	}
}

func TestCrashRecovery_SingleRetryBudget(t *testing.T) {
	client := &crashingAgentClient{crashCount: 100} // always crashes
	manager := newCrashTestManager(t, client)
	manager.crashRecoveryDelay = 50 * time.Millisecond

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "crashing",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	updates := subscribeStatuses(t, manager, ag.ID)

	if err := manager.SendAgentMessage(context.Background(), ag.ID, "doomed", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}

	// Original turn crashes, retry crashes too, then the agent stays in error.
	awaitStatus(t, updates, protocol.AgentError, 2*time.Second)
	awaitStatus(t, updates, protocol.AgentRunning, 2*time.Second) // the single retry
	awaitStatus(t, updates, protocol.AgentError, 2*time.Second)

	// Give any (incorrect) further retries time to fire, then verify the cap.
	time.Sleep(300 * time.Millisecond)
	if got := client.session.runCalls.Load(); got != 2 {
		t.Fatalf("Run called %d times, want exactly 2 (one retry max)", got)
	}
	if status := ag.ToSnapshot().Status; status != protocol.AgentError {
		t.Fatalf("final status = %q, want error", status)
	}
	lastError := ag.ToSnapshot().LastError
	if lastError == nil || *lastError == "" {
		t.Fatal("expected LastError to be set after unrecoverable crash")
	}
}

func TestCrashRecovery_SkippedWhenUserTurnStarted(t *testing.T) {
	client := &crashingAgentClient{crashCount: 1}
	manager := newCrashTestManager(t, client)
	manager.crashRecoveryDelay = 500 * time.Millisecond

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "crashing",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	updates := subscribeStatuses(t, manager, ag.ID)

	if err := manager.SendAgentMessage(context.Background(), ag.ID, "first", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}
	awaitStatus(t, updates, protocol.AgentError, 2*time.Second)

	// While the crash backoff is pending, the user starts a new turn. The new
	// turn succeeds; the stale recovery retry must not fire afterwards.
	if err := manager.SendAgentMessage(context.Background(), ag.ID, "second", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage (user retry): %v", err)
	}
	awaitStatus(t, updates, protocol.AgentIdle, 2*time.Second)

	// Outlast the stale recovery timer, then verify it never fired.
	time.Sleep(800 * time.Millisecond)
	if got := client.session.runCalls.Load(); got != 2 {
		t.Fatalf("Run called %d times, want 2 (crashed turn + user turn; stale recovery must be skipped)", got)
	}
	if status := ag.ToSnapshot().Status; status != protocol.AgentIdle {
		t.Fatalf("final status = %q, want idle", status)
	}
}
