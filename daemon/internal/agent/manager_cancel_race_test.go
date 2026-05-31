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

// cancelAfterTerminalClient simulates a provider whose Run emits turn_canceled
// then returns context.Canceled. This reproduces the race where the event stream
// path sets idle, but the Run-return path overwrites it with error.
type cancelAfterTerminalClient struct {
	session *cancelAfterTerminalSession
}

func (c *cancelAfterTerminalClient) Provider() string { return "cancel-terminal" }
func (c *cancelAfterTerminalClient) IsAvailable(ctx context.Context) error {
	return nil
}
func (c *cancelAfterTerminalClient) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error) {
	c.session = &cancelAfterTerminalSession{
		events: make(chan AgentStreamEvent, 8),
	}
	return c.session, nil
}
func (c *cancelAfterTerminalClient) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return c.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: c.Provider()})
}
func (c *cancelAfterTerminalClient) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	return nil, nil
}
func (c *cancelAfterTerminalClient) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (c *cancelAfterTerminalClient) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

type cancelAfterTerminalSession struct {
	events chan AgentStreamEvent
}

func (s *cancelAfterTerminalSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) (*AgentRunResult, error) {
	// Emit turn_canceled before returning context.Canceled.
	// This mirrors what EventPump does when the context is cancelled.
	s.events <- AgentStreamEvent{
		Event: map[string]interface{}{
			"type":     "turn_canceled",
			"provider": "cancel-terminal",
			"reason":   "context_cancelled",
		},
		Timestamp: time.Now(),
	}
	<-ctx.Done()
	return &AgentRunResult{SessionID: "session-cancel", Canceled: true}, context.Canceled
}

func (s *cancelAfterTerminalSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	go s.Run(ctx, text, images, attachments, "")
	return s.events, nil
}
func (s *cancelAfterTerminalSession) Subscribe() <-chan AgentStreamEvent  { return s.events }
func (s *cancelAfterTerminalSession) Interrupt(ctx context.Context) error { return nil }
func (s *cancelAfterTerminalSession) Close() error {
	close(s.events)
	return nil
}
func (s *cancelAfterTerminalSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	return nil
}
func (s *cancelAfterTerminalSession) GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{Provider: "cancel-terminal"}, nil
}
func (s *cancelAfterTerminalSession) GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (s *cancelAfterTerminalSession) GetCurrentMode(ctx context.Context) (*string, error) { return nil, nil }
func (s *cancelAfterTerminalSession) SetMode(modeID string) error                         { return nil }
func (s *cancelAfterTerminalSession) SetModel(modelID string) error                       { return nil }
func (s *cancelAfterTerminalSession) SetThinkingOption(optionID string) error             { return nil }
func (s *cancelAfterTerminalSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{Provider: "cancel-terminal", SessionID: "session-cancel"}
}
func (s *cancelAfterTerminalSession) GetPendingPermissions() []interface{} { return nil }
func (s *cancelAfterTerminalSession) ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}
func (s *cancelAfterTerminalSession) StreamHistory(ctx context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}

// TestAgentManagerDoesNotOverwriteIdleWithErrorOnCancel verifies that when a
// turn is canceled (Run returns context.Canceled AND a turn_canceled event is
// emitted), the agent ends in idle, not error.
//
// This reproduces the bug: first conversation ends abnormally because
// SendAgentMessage's Run-return path calls SetError after the event stream
// path has already applied idle via applyTerminalStreamState.
func TestAgentManagerDoesNotOverwriteIdleWithErrorOnCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(&cancelAfterTerminalClient{})
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "cancel-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	// Clean up so storage files are closed before test framework deletes temp dirs.
	defer manager.DeleteAgent(ag.ID)

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

	// Allow the turn_canceled event to be processed before Run returns.
	time.Sleep(50 * time.Millisecond)
	cancel()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case status := <-updates:
			if status == protocol.AgentIdle {
				// Success: event stream path applied idle and Run-return path
				// did not overwrite it with error.
				return
			}
			if status == protocol.AgentError {
				t.Fatalf("agent ended in error state; expected idle. "+
					"This means the Run-return path overwrote the idle set by the event stream path.")
			}
		case <-deadline:
			t.Fatalf("agent did not reach a terminal state; status=%s", ag.ToSnapshot().Status)
		}
	}
}

// TestAgentManagerAppliesIdleWhenCancelEventLost verifies that even if the
// turn_canceled event is somehow lost (e.g. workCh full + bypass fails), the
// Run-return path still transitions from running to idle when Canceled=true.
func TestAgentManagerAppliesIdleWhenCancelEventLost(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(&cancelAfterTerminalClient{})
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "cancel-terminal",
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
	defer manager.DeleteAgent(ag.ID)

	// Use a context that is already cancelled so Run returns immediately
	// without giving the event stream time to process.
	runCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := manager.SendAgentMessage(runCtx, ag.ID, "hello", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case status := <-updates:
			if status == protocol.AgentIdle {
				return
			}
			if status == protocol.AgentError {
				t.Fatalf("agent ended in error state; expected idle even when cancel event lost")
			}
		case <-deadline:
			t.Fatalf("agent did not reach idle; status=%s", ag.ToSnapshot().Status)
		}
	}
}

// TestSendAgentMessageRunReturnRespectsExistingTerminalState verifies that the
// SendAgentMessage goroutine does not call SetError when the agent is already
// in a terminal state (idle). This prevents the Run-return path from racing
// with applyTerminalStreamState.
func TestSendAgentMessageRunReturnRespectsExistingTerminalState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(&cancelAfterTerminalClient{})
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}

	ag, err := manager.CreateAgent(context.Background(), &protocol.AgentSessionConfig{
		Provider: "cancel-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Manually set to idle (simulating event stream already processed)
	ag.SetLifecycle(protocol.AgentIdle)

	// Directly invoke what the goroutine does: if agent is not running,
	// SetError should not be called. We verify by checking that SetError
	// still works when called directly (it does), but the goroutine logic
	// now guards against it.
	var mu sync.Mutex
	var stateUpdates []protocol.AgentLifecycleStatus
	unsub := manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentState && event.Agent != nil && event.AgentID == ag.ID {
			mu.Lock()
			stateUpdates = append(stateUpdates, event.Agent.ToSnapshot().Status)
			mu.Unlock()
		}
	})
	defer unsub()
	defer manager.DeleteAgent(ag.ID)

	// Send a message while already idle. The goroutine should skip SetError
	// because lifecycle is not running.
	runCtx, cancel := context.WithCancel(context.Background())
	cancel() // immediate cancel
	if err := manager.SendAgentMessage(runCtx, ag.ID, "hello", nil, nil, ""); err != nil {
		t.Fatalf("SendAgentMessage: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// The agent should remain idle; no error state should have been emitted.
	mu.Lock()
	updatesCopy := make([]protocol.AgentLifecycleStatus, len(stateUpdates))
	copy(updatesCopy, stateUpdates)
	mu.Unlock()
	for _, s := range updatesCopy {
		if s == protocol.AgentError {
			t.Fatalf("goroutine emitted error state while agent was idle; state updates=%v", updatesCopy)
		}
	}
	if ag.Lifecycle != protocol.AgentIdle {
		t.Fatalf("expected lifecycle idle, got %s", ag.Lifecycle)
	}
}
