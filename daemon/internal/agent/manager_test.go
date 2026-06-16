package agent

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

type hangingAfterTerminalClient struct {
	session *hangingAfterTerminalSession
}

func (c *hangingAfterTerminalClient) Provider() string { return "hanging-terminal" }
func (c *hangingAfterTerminalClient) IsAvailable(_ context.Context) error {
	return nil
}
func (c *hangingAfterTerminalClient) CreateSession(_ context.Context, _ *protocol.AgentSessionConfig) (AgentSession, error) {
	c.session = &hangingAfterTerminalSession{
		events: make(chan AgentStreamEvent, 8),
	}
	return c.session, nil
}
func (c *hangingAfterTerminalClient) ResumeSession(ctx context.Context, _ *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: c.Provider()})
}
func (c *hangingAfterTerminalClient) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *hangingAfterTerminalClient) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *hangingAfterTerminalClient) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

type hangingAfterTerminalSession struct {
	events chan AgentStreamEvent
}

func (s *hangingAfterTerminalSession) Run(ctx context.Context, _ string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) (*AgentRunResult, error) {
	s.events <- AgentStreamEvent{
		Event:     protocol.TurnCompletedStreamEvent{Provider: "hanging-terminal"},
		Timestamp: time.Now(),
	}
	<-ctx.Done()
	return &AgentRunResult{SessionID: "session-hanging"}, nil
}
func (s *hangingAfterTerminalSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	go s.Run(ctx, text, images, attachments, "")
	return s.events, nil
}
func (s *hangingAfterTerminalSession) Subscribe() <-chan AgentStreamEvent { return s.events }
func (s *hangingAfterTerminalSession) Interrupt(_ context.Context) error  { return nil }
func (s *hangingAfterTerminalSession) Close() error {
	close(s.events)
	return nil
}
func (s *hangingAfterTerminalSession) RespondPermission(_ string, _ protocol.AgentPermissionResponse) error {
	return nil
}
func (s *hangingAfterTerminalSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{Provider: "hanging-terminal"}, nil
}
func (s *hangingAfterTerminalSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *hangingAfterTerminalSession) GetCurrentMode(_ context.Context) (*string, error) {
	return nil, nil
}
func (s *hangingAfterTerminalSession) SetMode(_ string) error           { return nil }
func (s *hangingAfterTerminalSession) SetModel(_ string) error          { return nil }
func (s *hangingAfterTerminalSession) SetThinkingOption(_ string) error { return nil }
func (s *hangingAfterTerminalSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "hanging-terminal", SessionID: "session-hanging"}
}
func (s *hangingAfterTerminalSession) GetPendingPermissions() []interface{} { return nil }
func (s *hangingAfterTerminalSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *hangingAfterTerminalSession) StreamHistory(_ context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}

func TestAgentManagerMarksIdleWhenTerminalStreamEventArrivesBeforeRunReturns(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(&hangingAfterTerminalClient{})
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	updates := make(chan protocol.AgentLifecycleStatus, 8)
	unsub := manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentState && event.Agent != nil && event.AgentID == ag.ID {
			updates <- event.Agent.ToSnapshot().Status
		}
	})
	defer unsub()

	runCtx, cancel := context.WithCancel(context.Background())
	if err := manager.SendAgentMessage(runCtx, ag.ID, "hello", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case status := <-updates:
			if status == protocol.AgentIdle {
				cancel()
				waitForLifecycleStatus(t, updates, protocol.AgentIdle)
				return
			}
		case <-deadline:
			cancel()
			t.Fatalf("agent did not return to idle after terminal stream event; status=%s", ag.ToSnapshot().Status)
		}
	}
}

// slowMockAgentClient creates sessions whose Run blocks until context is done.
type slowMockAgentClient struct {
	session *slowMockAgentSession
}

func (c *slowMockAgentClient) Provider() string                    { return "slow-mock" }
func (c *slowMockAgentClient) IsAvailable(_ context.Context) error { return nil }
func (c *slowMockAgentClient) CreateSession(_ context.Context, _ *protocol.AgentSessionConfig) (AgentSession, error) {
	c.session = &slowMockAgentSession{}
	return c.session, nil
}
func (c *slowMockAgentClient) ResumeSession(ctx context.Context, _ *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: c.Provider()})
}
func (c *slowMockAgentClient) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *slowMockAgentClient) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *slowMockAgentClient) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

type slowMockAgentSession struct{}

func (s *slowMockAgentSession) Run(ctx context.Context, _ string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) (*AgentRunResult, error) {
	<-ctx.Done()
	return &AgentRunResult{SessionID: "slow-session"}, nil
}
func (s *slowMockAgentSession) StartTurn(ctx context.Context, _ string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	ch := make(chan AgentStreamEvent)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}
func (s *slowMockAgentSession) Subscribe() <-chan AgentStreamEvent {
	return nil
}
func (s *slowMockAgentSession) Interrupt(_ context.Context) error { return nil }
func (s *slowMockAgentSession) Close() error                      { return nil }
func (s *slowMockAgentSession) RespondPermission(_ string, _ protocol.AgentPermissionResponse) error {
	return nil
}
func (s *slowMockAgentSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{Provider: "slow-mock"}, nil
}
func (s *slowMockAgentSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *slowMockAgentSession) GetCurrentMode(_ context.Context) (*string, error) { return nil, nil }
func (s *slowMockAgentSession) SetMode(_ string) error                            { return nil }
func (s *slowMockAgentSession) SetModel(_ string) error                           { return nil }
func (s *slowMockAgentSession) SetThinkingOption(_ string) error                  { return nil }
func (s *slowMockAgentSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "slow-mock", SessionID: "slow-session"}
}
func (s *slowMockAgentSession) GetPendingPermissions() []interface{} { return nil }
func (s *slowMockAgentSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *slowMockAgentSession) StreamHistory(_ context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}

func TestSendAgentMessageRejectsWhenAlreadyRunning(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storageDir := t.TempDir()
	storage := NewAgentStorage(storageDir, logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(&slowMockAgentClient{})
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}

	cwd := "/tmp/test-cwd"
	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "slow-mock",
		Cwd:      cwd,
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// First message starts running and blocks
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan AgentEvent, 8)
	unsub := manager.Subscribe(func(e AgentEvent) {
		if e.AgentID == ag.ID {
			select {
			case updates <- e:
			default:
			}
		}
	})
	defer unsub()

	if err := manager.SendAgentMessage(ctx, ag.ID, "hello", nil, nil, ""); err != nil {
		t.Fatalf("first SendAgentMessage: %v", err)
	}

	// Second message should be rejected while running
	err = manager.SendAgentMessage(context.Background(), ag.ID, "hello again", nil, nil, "")
	if err == nil {
		t.Fatal("expected second SendAgentMessage to fail while running, got nil")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected 'already running' error, got: %v", err)
	}

	cancel()
	// Wait for the background Run goroutine to finish so t.TempDir() can clean up.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-updates:
			if e.Agent != nil && e.Agent.ToSnapshot().Status != protocol.AgentRunning {
				return
			}
		case <-deadline:
			return
		}
	}
}

func waitForLifecycleStatus(t *testing.T, updates <-chan protocol.AgentLifecycleStatus, status protocol.AgentLifecycleStatus) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case got := <-updates:
			if got == status {
				return
			}
		case <-deadline:
			t.Fatalf("did not observe lifecycle status %s", status)
		}
	}
}
