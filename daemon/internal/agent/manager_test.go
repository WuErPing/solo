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
func (c *hangingAfterTerminalClient) IsAvailable(ctx context.Context) error {
	return nil
}
func (c *hangingAfterTerminalClient) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error) {
	c.session = &hangingAfterTerminalSession{
		events: make(chan AgentStreamEvent, 8),
	}
	return c.session, nil
}
func (c *hangingAfterTerminalClient) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: c.Provider()})
}
func (c *hangingAfterTerminalClient) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *hangingAfterTerminalClient) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *hangingAfterTerminalClient) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

type hangingAfterTerminalSession struct {
	events chan AgentStreamEvent
}

func (s *hangingAfterTerminalSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (*AgentRunResult, error) {
	s.events <- AgentStreamEvent{
		Event: map[string]interface{}{
			"type":     "turn_completed",
			"provider": "hanging-terminal",
		},
		Timestamp: time.Now(),
	}
	<-ctx.Done()
	return &AgentRunResult{SessionID: "session-hanging"}, nil
}
func (s *hangingAfterTerminalSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	go s.Run(ctx, text, images, attachments)
	return s.events, nil
}
func (s *hangingAfterTerminalSession) Subscribe() <-chan AgentStreamEvent  { return s.events }
func (s *hangingAfterTerminalSession) Interrupt(ctx context.Context) error { return nil }
func (s *hangingAfterTerminalSession) Close() error {
	close(s.events)
	return nil
}
func (s *hangingAfterTerminalSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	return nil
}
func (s *hangingAfterTerminalSession) GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{Provider: "hanging-terminal"}, nil
}
func (s *hangingAfterTerminalSession) GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *hangingAfterTerminalSession) GetCurrentMode(ctx context.Context) (*string, error) {
	return nil, nil
}
func (s *hangingAfterTerminalSession) SetMode(modeID string) error             { return nil }
func (s *hangingAfterTerminalSession) SetModel(modelID string) error           { return nil }
func (s *hangingAfterTerminalSession) SetThinkingOption(optionID string) error { return nil }
func (s *hangingAfterTerminalSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "hanging-terminal", SessionID: "session-hanging"}
}
func (s *hangingAfterTerminalSession) GetPendingPermissions() []interface{} { return nil }
func (s *hangingAfterTerminalSession) ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *hangingAfterTerminalSession) StreamHistory(ctx context.Context) ([]AgentStreamEvent, error) {
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
	if err := manager.SendAgentMessage(runCtx, ag.ID, "hello", nil, nil); err != nil {
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

func (c *slowMockAgentClient) Provider() string                      { return "slow-mock" }
func (c *slowMockAgentClient) IsAvailable(ctx context.Context) error { return nil }
func (c *slowMockAgentClient) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error) {
	c.session = &slowMockAgentSession{}
	return c.session, nil
}
func (c *slowMockAgentClient) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: c.Provider()})
}
func (c *slowMockAgentClient) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *slowMockAgentClient) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *slowMockAgentClient) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

type slowMockAgentSession struct{}

func (s *slowMockAgentSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (*AgentRunResult, error) {
	<-ctx.Done()
	return &AgentRunResult{SessionID: "slow-session"}, nil
}
func (s *slowMockAgentSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
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
func (s *slowMockAgentSession) Interrupt(ctx context.Context) error { return nil }
func (s *slowMockAgentSession) Close() error                        { return nil }
func (s *slowMockAgentSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	return nil
}
func (s *slowMockAgentSession) GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{Provider: "slow-mock"}, nil
}
func (s *slowMockAgentSession) GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *slowMockAgentSession) GetCurrentMode(ctx context.Context) (*string, error) { return nil, nil }
func (s *slowMockAgentSession) SetMode(modeID string) error                         { return nil }
func (s *slowMockAgentSession) SetModel(modelID string) error                       { return nil }
func (s *slowMockAgentSession) SetThinkingOption(optionID string) error             { return nil }
func (s *slowMockAgentSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "slow-mock", SessionID: "slow-session"}
}
func (s *slowMockAgentSession) GetPendingPermissions() []interface{} { return nil }
func (s *slowMockAgentSession) ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *slowMockAgentSession) StreamHistory(ctx context.Context) ([]AgentStreamEvent, error) {
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
	if err := manager.SendAgentMessage(ctx, ag.ID, "hello", nil, nil); err != nil {
		t.Fatalf("first SendAgentMessage: %v", err)
	}

	// Second message should be rejected while running
	err = manager.SendAgentMessage(context.Background(), ag.ID, "hello again", nil, nil)
	if err == nil {
		t.Fatal("expected second SendAgentMessage to fail while running, got nil")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected 'already running' error, got: %v", err)
	}

	cancel()
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
