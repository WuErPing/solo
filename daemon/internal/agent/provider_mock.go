package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/WuErPing/solo/protocol"
)

// MockAgentClient is a test-only AgentClient that creates MockAgentSessions.
type MockAgentClient struct {
	mu       sync.Mutex
	sessions []*MockAgentSession
}

func NewMockAgentClient() *MockAgentClient {
	return &MockAgentClient{}
}

func (m *MockAgentClient) Provider() string { return "mock" }

func (m *MockAgentClient) IsAvailable(ctx context.Context) error { return nil }

func (m *MockAgentClient) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error) {
	s := &MockAgentSession{
		events:    make(chan AgentStreamEvent, 64),
		mode:      "default",
		model:     "mock-model",
		sessionID: uuid.NewString(),
	}
	m.mu.Lock()
	m.sessions = append(m.sessions, s)
	m.mu.Unlock()
	return s, nil
}

func (m *MockAgentClient) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return m.CreateSession(ctx, &protocol.AgentSessionConfig{Provider: "mock"})
}

func (m *MockAgentClient) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	return []protocol.AgentModelDefinition{
		{Provider: "mock", ID: "mock-model", Label: "Mock Model", IsDefault: true},
	}, nil
}

func (m *MockAgentClient) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	return []protocol.AgentMode{
		{ID: "default", Label: "Default", Description: "Mock default mode"},
	}, nil
}

func (m *MockAgentClient) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

// MockAgentSession is a test-only AgentSession.
type MockAgentSession struct {
	mu        sync.Mutex
	events    chan AgentStreamEvent
	mode      string
	model     string
	closed    bool
	sessionID string
}

func (s *MockAgentSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) (*AgentRunResult, error) {
	if !s.emit(AgentStreamEvent{
		Event: protocol.ThreadStartedStreamEvent{
			Provider:  "mock",
			SessionID: s.sessionID,
		},
		Timestamp: time.Now(),
	}) {
		return &AgentRunResult{SessionID: s.sessionID, Canceled: true}, nil
	}

	if !s.emit(AgentStreamEvent{
		Event: protocol.TimelineStreamEvent{
			Provider: "mock",
			Item:     TimelineItem{Type: "user_message", Text: text, MessageID: messageID},
		},
		Timestamp: time.Now(),
	}) {
		return &AgentRunResult{SessionID: s.sessionID, Canceled: true}, nil
	}

	if !s.emit(AgentStreamEvent{
		Event: protocol.TimelineStreamEvent{
			Provider: "mock",
			Item:     TimelineItem{Type: "assistant_message", Text: fmt.Sprintf("Mock response to: %s", text)},
		},
		Timestamp: time.Now(),
	}) {
		return &AgentRunResult{SessionID: s.sessionID, Canceled: true}, nil
	}

	if !s.emit(AgentStreamEvent{
		Event: protocol.TurnCompletedStreamEvent{
			Provider: "mock",
		},
		Timestamp: time.Now(),
	}) {
		return &AgentRunResult{SessionID: s.sessionID, Canceled: true}, nil
	}

	return &AgentRunResult{
		SessionID: s.sessionID,
		FinalText: fmt.Sprintf("Mock response to: %s", text),
		Canceled:  false,
	}, nil
}

func (s *MockAgentSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	s.mu.Lock()
	if s.closed {
		s.closed = false
		s.events = make(chan AgentStreamEvent, 64)
	}
	s.mu.Unlock()
	go s.Run(ctx, text, images, attachments, "")
	return s.events, nil
}

func (s *MockAgentSession) Subscribe() <-chan AgentStreamEvent {
	return s.events
}

func (s *MockAgentSession) Interrupt(ctx context.Context) error { return nil }

func (s *MockAgentSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
	}
	return nil
}

func (s *MockAgentSession) emit(event AgentStreamEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.events <- event
	return true
}

func (s *MockAgentSession) closeEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
	}
}

func (s *MockAgentSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	return nil
}

func (s *MockAgentSession) GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error) {
	return &protocol.AgentRuntimeInfo{
		Provider:  "mock",
		SessionID: &s.sessionID,
		Model:     &s.model,
		ModeID:    &s.mode,
	}, nil
}

func (s *MockAgentSession) GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error) {
	return []protocol.AgentMode{
		{ID: "default", Label: "Default", Description: "Mock default mode"},
	}, nil
}

func (s *MockAgentSession) GetCurrentMode(ctx context.Context) (*string, error) {
	return &s.mode, nil
}

func (s *MockAgentSession) SetMode(modeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = modeID
	return nil
}

func (s *MockAgentSession) SetModel(modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = modelID
	return nil
}

func (s *MockAgentSession) SetThinkingOption(optionID string) error { return nil }

func (s *MockAgentSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return &protocol.AgentPersistenceHandle{
		Provider:     "mock",
		SessionID:    s.sessionID,
		NativeHandle: s.sessionID,
	}
}

func (s *MockAgentSession) GetPendingPermissions() []interface{} {
	return nil
}

func (s *MockAgentSession) ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

func (s *MockAgentSession) StreamHistory(ctx context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}
