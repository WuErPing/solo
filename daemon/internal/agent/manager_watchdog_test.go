package agent

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// watchdogMockSession blocks Run() indefinitely until Interrupt() is called,
// simulating a Claude process that hangs (e.g. waiting for an MCP tool that
// never responds).
type watchdogMockSession struct {
	mu          sync.Mutex
	interruptCh chan struct{}
	interrupted bool
	events      chan AgentStreamEvent
}

func newWatchdogMockSession() *watchdogMockSession {
	return &watchdogMockSession{
		interruptCh: make(chan struct{}),
		events:      make(chan AgentStreamEvent),
	}
}

func (s *watchdogMockSession) Run(ctx context.Context, _ string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) (*AgentRunResult, error) {
	select {
	case <-s.interruptCh:
		return &AgentRunResult{Canceled: true}, nil
	case <-ctx.Done():
		return &AgentRunResult{Canceled: true}, ctx.Err()
	}
}

func (s *watchdogMockSession) Interrupt(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.interrupted {
		s.interrupted = true
		close(s.interruptCh)
	}
	return nil
}

func (s *watchdogMockSession) Subscribe() <-chan AgentStreamEvent { return s.events }
func (s *watchdogMockSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	go s.Run(ctx, text, images, attachments, "")
	return s.events, nil
}
func (s *watchdogMockSession) Close() error { return nil }
func (s *watchdogMockSession) RespondPermission(_ string, _ protocol.AgentPermissionResponse) error {
	return nil
}
func (s *watchdogMockSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	p := "watchdog-mock"
	return &protocol.AgentRuntimeInfo{Provider: p}, nil
}
func (s *watchdogMockSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *watchdogMockSession) GetCurrentMode(_ context.Context) (*string, error) { return nil, nil }
func (s *watchdogMockSession) SetMode(_ string) error                            { return nil }
func (s *watchdogMockSession) SetModel(_ string) error                           { return nil }
func (s *watchdogMockSession) SetThinkingOption(_ string) error                  { return nil }
func (s *watchdogMockSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "watchdog-mock", SessionID: "watchdog-session"}
}
func (s *watchdogMockSession) GetPendingPermissions() []interface{} { return nil }
func (s *watchdogMockSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *watchdogMockSession) StreamHistory(_ context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}

type watchdogMockClient struct {
	mu      sync.Mutex
	session *watchdogMockSession
}

func (c *watchdogMockClient) Provider() string                    { return "watchdog-mock" }
func (c *watchdogMockClient) IsAvailable(_ context.Context) error { return nil }
func (c *watchdogMockClient) CreateSession(_ context.Context, _ *protocol.AgentSessionConfig) (AgentSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session = newWatchdogMockSession()
	return c.session, nil
}
func (c *watchdogMockClient) ResumeSession(ctx context.Context, _ *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, nil)
}
func (c *watchdogMockClient) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *watchdogMockClient) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *watchdogMockClient) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

// TestSendAgentMessage_WatchdogInterruptsHangingRun verifies that a watchdog
// timer fires after maxAgentRunDuration and calls session.Interrupt(), which
// unblocks a Run() that would otherwise hang forever (simulating a Claude
// process stuck waiting for an MCP tool that never responds).
//
// Before the fix: maxAgentRunDuration does not exist and there is no watchdog
// in SendAgentMessage; the agent stays in LifecycleRunning indefinitely.
func TestSendAgentMessage_WatchdogInterruptsHangingRun(t *testing.T) {
	origDuration := maxAgentRunDuration.Load()
	maxAgentRunDuration.Store(int64(150 * time.Millisecond))
	defer func() { maxAgentRunDuration.Store(origDuration) }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("storage.Initialize: %v", err)
	}
	client := &watchdogMockClient{}
	registry := NewProviderRegistry()
	registry.Register(client)
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("manager.Initialize: %v", err)
	}

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "watchdog-mock",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	updates := make(chan protocol.AgentLifecycleStatus, 16)
	unsub := manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentState && event.AgentID == ag.ID && event.Agent != nil {
			updates <- event.Agent.ToSnapshot().Status
		}
	})
	defer unsub()

	// Send a message; watchdogMockSession.Run() blocks until Interrupt() is called.
	if err := manager.SendAgentMessage(context.Background(), ag.ID, "hang forever", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}

	// Agent must leave LifecycleRunning within watchdog window + 500ms buffer.
	deadline := time.After(time.Duration(maxAgentRunDuration.Load()) + 500*time.Millisecond)
	for {
		select {
		case status := <-updates:
			if status != protocol.AgentRunning {
				return // watchdog fired, agent transitioned away from running
			}
		case <-deadline:
			t.Fatalf("agent is still running after watchdog should have fired (maxAgentRunDuration=%v)", maxAgentRunDuration)
		}
	}
}
