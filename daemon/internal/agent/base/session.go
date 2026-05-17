package base

import (
	"context"
	"log/slog"
	"sync"

	"github.com/WuErPing/solo/protocol"
)

// BaseSession provides common session state and lifecycle management
// shared across all agent providers.
type BaseSession struct {
	mu sync.RWMutex

	config  *protocol.AgentSessionConfig
	logger  *slog.Logger
	provider string

	// Identity
	sessionID string

	// State
	currentMode     string
	currentModel    string
	currentThinking string
	closed          bool

	// Lifecycle
	cancelFn context.CancelFunc
}

// NewBaseSession creates a new base session with the given configuration.
func NewBaseSession(provider string, config *protocol.AgentSessionConfig, logger *slog.Logger) *BaseSession {
	mode := ""
	if config.ModeID != nil {
		mode = *config.ModeID
	}
	model := ""
	if config.Model != nil {
		model = *config.Model
	}
	thinking := ""
	if config.ThinkingOptionID != nil {
		thinking = *config.ThinkingOptionID
	}

	return &BaseSession{
		provider:        provider,
		config:          config,
		logger:          logger,
		currentMode:     mode,
		currentModel:    model,
		currentThinking: thinking,
	}
}

// --- Identity ---

func (s *BaseSession) Provider() string { return s.provider }

func (s *BaseSession) SessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

func (s *BaseSession) SetSessionID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionID = id
}

// --- State Accessors ---

func (s *BaseSession) Logger() *slog.Logger {
	return s.logger
}

func (s *BaseSession) Config() *protocol.AgentSessionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *BaseSession) CurrentMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMode
}

func (s *BaseSession) CurrentModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentModel
}

func (s *BaseSession) CurrentThinking() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentThinking
}

func (s *BaseSession) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// --- State Mutators ---

func (s *BaseSession) SetMode(modeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentMode = modeID
	return nil
}

func (s *BaseSession) SetModel(modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentModel = modelID
	return nil
}

func (s *BaseSession) SetThinkingOption(optionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentThinking = optionID
	return nil
}

// SetCurrentModel updates the current model from within the session (e.g., from system init message).
func (s *BaseSession) SetCurrentModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if model != "" {
		s.currentModel = model
	}
}

// SetCurrentMode updates the current mode from within the session.
func (s *BaseSession) SetCurrentMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode != "" {
		s.currentMode = mode
	}
}

// --- Lifecycle ---

func (s *BaseSession) SetCancelFn(fn context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelFn = fn
}

func (s *BaseSession) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
}

// Close marks the session as closed and cancels any running operation.
// The caller is responsible for cleanup (killing processes, releasing resources).
func (s *BaseSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}

	return nil
}

// --- AgentSession Interface Helpers ---

func (s *BaseSession) GetRuntimeInfo() *protocol.AgentRuntimeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := &protocol.AgentRuntimeInfo{
		Provider:  s.provider,
		SessionID: strPtr(s.sessionID),
	}
	if s.currentModel != "" {
		info.Model = &s.currentModel
	}
	if s.currentMode != "" {
		info.ModeID = &s.currentMode
	}
	if s.currentThinking != "" {
		info.ThinkingOptionID = &s.currentThinking
	}
	return info
}

func (s *BaseSession) GetCurrentModePtr() *string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &s.currentMode
}

func (s *BaseSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sessionID == "" {
		return nil
	}

	cwd := ""
	if s.config != nil {
		cwd = s.config.Cwd
	}

	return &protocol.AgentPersistenceHandle{
		Provider:     s.provider,
		SessionID:    s.sessionID,
		NativeHandle: s.sessionID,
		Metadata: map[string]interface{}{
			"cwd":   cwd,
			"model": s.currentModel,
		},
	}
}

// --- Lock Access for Subclasses ---

func (s *BaseSession) Lock() {
	s.mu.Lock()
}

func (s *BaseSession) Unlock() {
	s.mu.Unlock()
}

func (s *BaseSession) RLock() {
	s.mu.RLock()
}

func (s *BaseSession) RUnlock() {
	s.mu.RUnlock()
}

// --- Helpers ---

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
