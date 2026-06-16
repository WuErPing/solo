package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/protocol"
)

// AgentClient is the interface that each agent provider must implement.
type AgentClient interface {
	// Provider returns the provider identifier (e.g., "claude", "codex").
	Provider() string

	// IsAvailable checks whether this provider can be used right now.
	IsAvailable(ctx context.Context) error

	// CreateSession creates a new agent session.
	CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error)

	// ResumeSession resumes a previously persisted session.
	ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error)

	// ListModels returns available models for this provider.
	ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error)

	// ListModes returns available modes for this provider.
	ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error)

	// ListClientCommands returns available slash commands for this provider (without an active session).
	ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error)
}

// AgentSession is the interface for an active agent session.
type AgentSession interface {
	// Run starts a prompt and blocks until completion.
	Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) (*AgentRunResult, error)

	// StartTurn starts a non-blocking turn, returning events via channel.
	StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error)

	// Subscribe returns a channel of stream events.
	Subscribe() <-chan AgentStreamEvent

	// Interrupt cancels the current running turn.
	Interrupt(ctx context.Context) error

	// Close terminates the session.
	Close() error

	// RespondPermission responds to a pending permission request.
	RespondPermission(requestID string, response protocol.AgentPermissionResponse) error

	// GetRuntimeInfo returns current runtime info.
	GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error)

	// GetAvailableModes returns the modes available in this session.
	GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error)

	// GetCurrentMode returns the current mode ID.
	GetCurrentMode(ctx context.Context) (*string, error)

	// SetMode changes the current mode.
	SetMode(modeID string) error

	// SetModel changes the current model.
	SetModel(modelID string) error

	// SetThinkingOption changes the thinking option.
	SetThinkingOption(optionID string) error

	// DescribePersistence returns the persistence handle for resumption.
	DescribePersistence() *protocol.AgentPersistenceHandle

	// GetPendingPermissions returns current pending permission requests.
	GetPendingPermissions() []interface{}

	// ListCommands returns available slash commands for this session.
	ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error)

	// StreamHistory fetches past messages and returns them as timeline events.
	// Called once on session resume to hydrate the in-memory timeline store.
	StreamHistory(ctx context.Context) ([]AgentStreamEvent, error)
}

// AgentRunResult is the result of a blocking agent run.
type AgentRunResult struct {
	SessionID string
	FinalText string
	Usage     *protocol.AgentUsage
	Canceled  bool
}

// workChCapacity is the buffer size for the per-agent event work channel.
// Exposed as an atomic so tests can reduce it to trigger drops without races.
var workChCapacity atomic.Int64

func init() { workChCapacity.Store(256) }

// semiCriticalWorkChTimeout is the blocking timeout for semi-critical events
// (reasoning/thinking) in the subscribeToSession work channel send path.
// Long enough to survive transient backpressure, short enough to not block
// the dispatcher drain under sustained load.
var semiCriticalWorkChTimeout = 100 * time.Millisecond

// criticalWorkChSendTimeout is the max time the drain loop will block trying
// to deliver a critical event (turn_completed/failed/canceled) into workCh.
// When the consumer is stalled and workCh is full after this timeout, the
// terminal state is applied directly — bypassing the stalled consumer —
// so that turn_completed is never silently dropped.
// Must be < dispatcher's subscriberCriticalTimeout (500ms) so the fallback
// fires before the dispatcher gives up on the subscriber channel.
// Exposed as a var so tests can reduce it.
var criticalWorkChSendTimeout = 200 * time.Millisecond

// maxAgentRunDuration is the maximum time a single agent run is allowed before
// the watchdog fires an Interrupt(). Exposed as an atomic so tests can reduce it without races.
var maxAgentRunDuration atomic.Int64

func init() { maxAgentRunDuration.Store(int64(35 * time.Minute)) }

// AgentManager orchestrates all agent lifecycle operations.
type AgentManager struct {
	mu       sync.RWMutex
	agents   map[string]*ManagedAgent
	storage  *AgentStorage
	registry *ProviderRegistry
	logger   *slog.Logger

	subscribers map[uint64]AgentEventFunc
	nextSubID   uint64

	// coalescerFlushFuncs is called when a critical event takes the
	// workCh fallback path (pipeline congested). It ensures buffered
	// timeline entries are flushed even when the normal event path is
	// stalled, preventing cross-turn message mixing.
	coalescerFlushFuncs  map[uint64]func(agentID string)
	nextCoalescerFlushID uint64

	droppedEventCount atomic.Int64

	stallMonitor *StallMonitor
}

// NewAgentManager creates a new AgentManager.
func NewAgentManager(storage *AgentStorage, registry *ProviderRegistry, logger *slog.Logger) *AgentManager {
	m := &AgentManager{
		agents:              make(map[string]*ManagedAgent),
		storage:             storage,
		registry:            registry,
		logger:              logger.With("component", "agent-manager"),
		subscribers:         make(map[uint64]AgentEventFunc),
		coalescerFlushFuncs: make(map[uint64]func(agentID string)),
	}
	m.stallMonitor = NewStallMonitor(logger, m.stallInterrupt)
	m.stallMonitor.Start()
	return m
}

// stallInterrupt is the callback used by StallMonitor to cancel a stuck turn.
func (m *AgentManager) stallInterrupt(agentID string) error {
	return m.CancelAgentRun(context.Background(), agentID)
}

// RegisterCoalescerFlusher registers a function to be called when a critical
// event takes the workCh fallback path. This ensures buffered timeline entries
// are flushed even when the normal event pipeline is stalled.
func (m *AgentManager) RegisterCoalescerFlusher(fn func(agentID string)) uint64 {
	m.mu.Lock()
	id := m.nextCoalescerFlushID
	m.nextCoalescerFlushID++
	m.coalescerFlushFuncs[id] = fn
	m.mu.Unlock()
	return id
}

// UnregisterCoalescerFlusher removes a previously registered flusher.
func (m *AgentManager) UnregisterCoalescerFlusher(id uint64) {
	m.mu.Lock()
	delete(m.coalescerFlushFuncs, id)
	m.mu.Unlock()
}

// fireCoalescerFlush calls all registered coalescer flush functions.
func (m *AgentManager) fireCoalescerFlush(agentID string) {
	m.mu.RLock()
	fns := make([]func(string), 0, len(m.coalescerFlushFuncs))
	for _, fn := range m.coalescerFlushFuncs {
		fns = append(fns, fn)
	}
	m.mu.RUnlock()
	for _, fn := range fns {
		fn(agentID)
	}
}

// DroppedEventCount returns the total number of non-critical events that were
// dropped because the per-agent work channel was full.
func (m *AgentManager) DroppedEventCount() int64 {
	return m.droppedEventCount.Load()
}

// Initialize loads persisted agents from storage.
func (m *AgentManager) Initialize(_ context.Context) error {
	records := m.storage.List()
	m.logger.Info("initializing from storage", "records", len(records))
	for _, r := range records {
		if r.ArchivedAt != nil {
			continue
		}
		agent := recordToManagedAgent(r)
		m.agents[agent.ID] = agent
	}
	return nil
}

// CreateAgent creates a new agent session.
func (m *AgentManager) CreateAgent(ctx context.Context, config *protocol.AgentSessionConfig, labels map[string]string) (*ManagedAgent, error) {
	agentID := uuid.New().String()
	provider := config.Provider

	client, err := m.registry.Get(provider)
	if err != nil {
		return nil, fmt.Errorf("provider %s not available: %w", provider, err)
	}

	if err := client.IsAvailable(ctx); err != nil {
		return nil, fmt.Errorf("provider %s not available: %w", provider, err)
	}

	session, err := client.CreateSession(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	agent := NewManagedAgent(agentID, provider, config.Cwd, config, labels)
	agent.Session = session
	agent.Persistence = session.DescribePersistence()

	// Get runtime info
	if ri, err := session.GetRuntimeInfo(ctx); err == nil && ri != nil {
		agent.RuntimeInfo = ri
	}

	// Get available modes
	if modes, err := session.GetAvailableModes(ctx); err == nil {
		agent.AvailableModes = modes
	}

	// Get current mode
	if modeID, err := session.GetCurrentMode(ctx); err == nil {
		agent.CurrentModeID = modeID
	}

	// Transition to idle
	agent.SetLifecycle(protocol.AgentIdle)

	// Store in memory
	m.mu.Lock()
	m.agents[agentID] = agent
	m.mu.Unlock()

	// Persist to disk
	if err := m.storage.ApplySnapshot(agent); err != nil {
		m.logger.Warn("failed to persist agent", "agentId", agentID, "error", err)
	}

	// Subscribe to session events
	go m.subscribeToSession(agent)

	// Emit state change
	m.emitState(agent)

	m.logger.Info("agent created", "agentId", agentID, "provider", provider)
	return agent, nil
}

// ResumeAgentFromPersistence reconstructs an agent session from a persistence
// handle. If the handle matches an existing agent, that agent ID is preserved.
func (m *AgentManager) ResumeAgentFromPersistence(ctx context.Context, handle *protocol.AgentPersistenceHandle, overrides *protocol.AgentSessionConfig) (*ManagedAgent, error) {
	if handle == nil || handle.Provider == "" || handle.SessionID == "" {
		return nil, fmt.Errorf("persistence handle is required")
	}

	if existing := m.findAgentByPersistence(handle); existing != nil {
		if overrides != nil {
			existing.Config = mergeAgentConfig(existing.Config, overrides, existing.Provider, existing.Cwd)
		}
		return m.ensureAgentSession(ctx, existing)
	}

	client, err := m.registry.Get(handle.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %s not available: %w", handle.Provider, err)
	}
	if err := client.IsAvailable(ctx); err != nil {
		return nil, fmt.Errorf("provider %s not available: %w", handle.Provider, err)
	}

	config := configFromPersistenceHandle(handle, overrides)
	effectiveHandle := attachPersistenceMetadata(handle, config.Cwd, config)
	session, err := client.ResumeSession(ctx, effectiveHandle)
	if err != nil {
		return nil, fmt.Errorf("resume session: %w", err)
	}

	agentID := uuid.New().String()
	ag := NewManagedAgent(agentID, handle.Provider, config.Cwd, config, nil)
	ag.Session = session
	ag.Persistence = attachPersistenceMetadata(session.DescribePersistence(), ag.Cwd, config)
	if ag.Persistence == nil {
		ag.Persistence = effectiveHandle
	}
	m.refreshSessionMetadata(ctx, ag, session)
	ag.SetLifecycle(protocol.AgentIdle)

	m.mu.Lock()
	m.agents[agentID] = ag
	m.mu.Unlock()

	if err := m.storage.ApplySnapshot(ag); err != nil {
		m.logger.Warn("failed to persist resumed agent", "agentId", agentID, "error", err)
	}
	go m.subscribeToSession(ag)
	m.emitState(ag)
	m.logger.Info("agent resumed", "agentId", agentID, "provider", handle.Provider)
	return ag, nil
}

// GetAgent returns a managed agent by ID.
func (m *AgentManager) GetAgent(agentID string) *ManagedAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[agentID]
}

// ListAgents returns all active (non-archived) agents.
func (m *AgentManager) ListAgents() []*ManagedAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ManagedAgent, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result
}

// ListAgentsWithPersisted returns live agents merged with non-archived persisted
// agents that are not currently in memory. This mirrors Solo's listAgentPayloads
// which combines agentManager.listAgents() + agentStorage.list().
func (m *AgentManager) ListAgentsWithPersisted() []*ManagedAgent {
	m.mu.RLock()
	liveIDs := make(map[string]bool, len(m.agents))
	result := make([]*ManagedAgent, 0, len(m.agents))
	for _, a := range m.agents {
		liveIDs[a.ID] = true
		result = append(result, a)
	}
	m.mu.RUnlock()

	for _, r := range m.storage.List() {
		if liveIDs[r.ID] || r.ArchivedAt != nil {
			continue
		}
		result = append(result, recordToManagedAgent(r))
	}
	return result
}

// ListAllAgents returns all agents from storage (including archived).
func (m *AgentManager) ListAllAgents() []*ManagedAgent {
	records := m.storage.List()
	result := make([]*ManagedAgent, 0, len(records))
	for _, r := range records {
		result = append(result, recordToManagedAgent(r))
	}
	return result
}

// HasRunningAgentsWithRecentProgress returns true if any agent is
// protocol.AgentRunning AND has produced a stream event within the stall monitor's
// inactivity threshold. Used by the session grace-period logic to avoid
// extending grace for agents that are stuck but still report "running".
func (m *AgentManager) HasRunningAgentsWithRecentProgress() bool {
	for _, ag := range m.ListAgents() {
		if ag.Lifecycle == protocol.AgentRunning && m.stallMonitor.HasRecentProgress(ag.ID) {
			return true
		}
	}
	return false
}

// DeleteAgent closes and removes an agent.
func (m *AgentManager) DeleteAgent(agentID string) error {
	agent := m.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}

	m.stallMonitor.UnregisterAgent(agentID)

	var closeErr error
	if sess := agent.GetSession(); sess != nil {
		closeErr = sess.Close()
	}

	agent.SetLifecycle(protocol.AgentClosed)
	agent.SetSession(nil)

	m.storage.BeginDelete(agentID)
	removeErr := m.storage.Remove(agentID)

	m.mu.Lock()
	delete(m.agents, agentID)
	m.mu.Unlock()

	m.emitState(agent)
	m.logger.Info("agent deleted", "agentId", agentID)
	return errors.Join(closeErr, removeErr)
}

// ArchiveAgent marks an agent as archived.
func (m *AgentManager) ArchiveAgent(agentID string) error {
	agent := m.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}

	m.stallMonitor.UnregisterAgent(agentID)

	var closeErr error
	if sess := agent.GetSession(); sess != nil {
		closeErr = sess.Close()
		agent.SetSession(nil)
	}

	agent.SetLifecycle(protocol.AgentClosed)
	now := timeNowUTC().Format(time.RFC3339)
	agent.mu.Lock()
	agent.ArchivedAt = &now
	agent.mu.Unlock()

	if err := m.storage.ApplySnapshot(agent); err != nil {
		m.logger.Warn("failed to persist archive", "agentId", agentID, "error", err)
	}

	m.mu.Lock()
	delete(m.agents, agentID)
	m.mu.Unlock()

	m.emitState(agent)
	m.logger.Info("agent archived", "agentId", agentID)
	return closeErr
}

// CancelAgentRun interrupts the currently running turn.
func (m *AgentManager) CancelAgentRun(ctx context.Context, agentID string) error {
	agent := m.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}
	sess := agent.GetSession()
	if sess == nil {
		return fmt.Errorf("agent %s has no active session", agentID)
	}
	return sess.Interrupt(ctx)
}

// SendAgentMessage sends a prompt to a running agent.
func (m *AgentManager) SendAgentMessage(ctx context.Context, agentID string, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) error {
	agent := m.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}
	if agent.GetSession() == nil {
		var err error
		agent, err = m.ensureAgentSession(ctx, agent)
		if err != nil {
			return err
		}
	}

	// Prevent concurrent turns across all providers
	if agent.Lifecycle == protocol.AgentRunning {
		return fmt.Errorf("agent %s is already running", agentID)
	}

	session := agent.GetSession()

	agent.SetLifecycle(protocol.AgentRunning)
	now := time.Now()
	agent.LastUserMessageAt = &now
	agent.ClearAttentionUnlessPermission()
	agent.TouchUpdatedAt()

	// Persist state change
	if err := m.storage.ApplySnapshot(agent); err != nil {
		m.logger.Warn("failed to persist agent running state", "agentId", agent.ID, "error", err)
	}
	m.emitState(agent)

	m.stallMonitor.RegisterAgent(agentID)

	// Run in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("agent run panic recovered, setting error state",
					"agentId", agent.ID, "panic", r)
				agent.SetError(fmt.Sprintf("internal panic: %v", r))
				if err := m.storage.ApplySnapshot(agent); err != nil {
					m.logger.Warn("failed to persist agent panic state", "agentId", agent.ID, "error", err)
				}
				m.emitState(agent)
			}
		}()
		watchdog := time.AfterFunc(time.Duration(maxAgentRunDuration.Load()), func() {
			m.logger.Warn("agent run watchdog fired, interrupting",
				"agentId", agent.ID, "maxDuration", time.Duration(maxAgentRunDuration.Load()))
			if err := session.Interrupt(context.Background()); err != nil {
				m.logger.Warn("watchdog interrupt failed", "agentId", agent.ID, "error", err)
			}
		})
		defer watchdog.Stop()
		result, err := session.Run(ctx, text, images, attachments, messageID)
		m.stallMonitor.UnregisterAgent(agentID)
		m.refreshSessionMetadata(ctx, agent, session)
		if err != nil {
			// A canceled turn is not an error state. The event stream path already
			// applied idle via turn_canceled; if that was missed, apply it here.
			if result != nil && result.Canceled {
				if agent.Lifecycle == protocol.AgentRunning {
					agent.SetLifecycle(protocol.AgentIdle)
					if err := m.storage.ApplySnapshot(agent); err != nil {
						m.logger.Warn("failed to persist agent canceled state", "agentId", agent.ID, "error", err)
					}
					m.emitState(agent)
				}
				return
			}
			// If the event stream path already applied a terminal state (idle or
			// error), don't overwrite it. This prevents the Run-return path from
			// racing with applyTerminalStreamState.
			if agent.Lifecycle == protocol.AgentRunning {
				agent.SetError(err.Error())
				if err := m.storage.ApplySnapshot(agent); err != nil {
					m.logger.Warn("failed to persist agent error state", "agentId", agent.ID, "error", err)
				}
				m.emitState(agent)
			}
			return
		}
		agent.SetLifecycle(protocol.AgentIdle)
		agent.SetAttention(true, "finished")
		if err := m.storage.ApplySnapshot(agent); err != nil {
			m.logger.Warn("failed to persist agent idle state", "agentId", agent.ID, "error", err)
		}
		m.emitState(agent)
	}()

	return nil
}

func (m *AgentManager) ensureAgentSession(ctx context.Context, agent *ManagedAgent) (*ManagedAgent, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.GetSession() != nil {
		return agent, nil
	}

	client, err := m.registry.Get(agent.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %s not available: %w", agent.Provider, err)
	}
	if err := client.IsAvailable(ctx); err != nil {
		return nil, fmt.Errorf("provider %s not available: %w", agent.Provider, err)
	}

	config := cloneAgentConfig(agent.Config, agent.Provider, agent.Cwd)
	var session AgentSession
	if agent.Persistence != nil && agent.Persistence.SessionID != "" {
		effectiveHandle := attachPersistenceMetadata(agent.Persistence, agent.Cwd, config)
		session, err = client.ResumeSession(ctx, effectiveHandle)
		if err != nil {
			return nil, fmt.Errorf("resume session: %w", err)
		}
	} else {
		session, err = client.CreateSession(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
	}

	m.mu.Lock()
	current := m.agents[agent.ID]
	if current != nil && current.GetSession() != nil {
		m.mu.Unlock()
		_ = session.Close()
		return current, nil
	}
	if current != nil {
		agent = current
	}
	agent.Config = config
	agent.SetSession(session)
	agent.Persistence = attachPersistenceMetadata(session.DescribePersistence(), agent.Cwd, config)
	agent.SetLifecycle(protocol.AgentIdle)
	m.agents[agent.ID] = agent
	m.mu.Unlock()

	m.refreshSessionMetadata(ctx, agent, session)
	if err := m.storage.ApplySnapshot(agent); err != nil {
		m.logger.Warn("failed to persist ensured agent session", "agentId", agent.ID, "error", err)
	}
	go m.subscribeToSession(agent)
	m.emitState(agent)
	m.logger.Info("agent session restored", "agentId", agent.ID, "provider", agent.Provider)
	return agent, nil
}

func (m *AgentManager) refreshSessionMetadata(ctx context.Context, agent *ManagedAgent, session AgentSession) {
	if agent == nil || session == nil {
		return
	}
	// Collect values outside the lock to avoid holding agent.mu during slow I/O.
	persistence := attachPersistenceMetadata(session.DescribePersistence(), agent.Cwd, agent.Config)
	ri, riErr := session.GetRuntimeInfo(ctx)
	modes, modesErr := session.GetAvailableModes(ctx)
	modeID, modeIDErr := session.GetCurrentMode(ctx)

	// Write under the agent's lock to prevent races with concurrent ToSnapshot() calls.
	agent.mu.Lock()
	defer agent.mu.Unlock()
	if persistence != nil {
		agent.Persistence = persistence
	}
	if riErr == nil && ri != nil {
		agent.RuntimeInfo = ri
		if agent.Persistence == nil && ri.SessionID != nil && *ri.SessionID != "" {
			agent.Persistence = attachPersistenceMetadata(&protocol.AgentPersistenceHandle{
				Provider:  agent.Provider,
				SessionID: *ri.SessionID,
			}, agent.Cwd, agent.Config)
		}
	}
	if modesErr == nil {
		agent.AvailableModes = modes
	}
	if modeIDErr == nil {
		agent.CurrentModeID = modeID
	}
}

func (m *AgentManager) findAgentByPersistence(handle *protocol.AgentPersistenceHandle) *ManagedAgent {
	if handle == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ag := range m.agents {
		if ag == nil || ag.Persistence == nil {
			continue
		}
		if ag.Persistence.Provider == handle.Provider && ag.Persistence.SessionID == handle.SessionID {
			return ag
		}
	}
	return nil
}

func cloneAgentConfig(config *protocol.AgentSessionConfig, provider, cwd string) *protocol.AgentSessionConfig {
	if config == nil {
		return &protocol.AgentSessionConfig{Provider: provider, Cwd: cwd}
	}
	cloned := *config
	if cloned.Provider == "" {
		cloned.Provider = provider
	}
	if cloned.Cwd == "" {
		cloned.Cwd = cwd
	}
	return &cloned
}

func mergeAgentConfig(base *protocol.AgentSessionConfig, overrides *protocol.AgentSessionConfig, provider, cwd string) *protocol.AgentSessionConfig {
	merged := cloneAgentConfig(base, provider, cwd)
	if overrides == nil {
		return merged
	}
	if overrides.Provider != "" {
		merged.Provider = overrides.Provider
	}
	if overrides.Cwd != "" {
		merged.Cwd = overrides.Cwd
	}
	if overrides.ModeID != nil {
		merged.ModeID = overrides.ModeID
	}
	if overrides.Model != nil {
		merged.Model = overrides.Model
	}
	if overrides.ThinkingOptionID != nil {
		merged.ThinkingOptionID = overrides.ThinkingOptionID
	}
	if overrides.FeatureValues != nil {
		merged.FeatureValues = overrides.FeatureValues
	}
	if overrides.Title != nil {
		merged.Title = overrides.Title
	}
	if overrides.Extra != nil {
		merged.Extra = overrides.Extra
	}
	if overrides.SystemPrompt != "" {
		merged.SystemPrompt = overrides.SystemPrompt
	}
	if overrides.McpServers != nil {
		merged.McpServers = overrides.McpServers
	}
	if overrides.OutputSchema != nil {
		merged.OutputSchema = overrides.OutputSchema
	}
	return merged
}

func configFromPersistenceHandle(handle *protocol.AgentPersistenceHandle, overrides *protocol.AgentSessionConfig) *protocol.AgentSessionConfig {
	config := &protocol.AgentSessionConfig{Provider: handle.Provider}
	if handle.Metadata != nil {
		if cwd, ok := handle.Metadata["cwd"].(string); ok {
			config.Cwd = cwd
		}
		if model, ok := handle.Metadata["model"].(string); ok && model != "" {
			config.Model = &model
		}
		if mode, ok := handle.Metadata["modeId"].(string); ok && mode != "" {
			config.ModeID = &mode
		}
		if thinking, ok := handle.Metadata["thinkingOptionId"].(string); ok && thinking != "" {
			config.ThinkingOptionID = &thinking
		}
	}
	return mergeAgentConfig(config, overrides, handle.Provider, config.Cwd)
}

func attachPersistenceMetadata(handle *protocol.AgentPersistenceHandle, cwd string, config *protocol.AgentSessionConfig) *protocol.AgentPersistenceHandle {
	if handle == nil {
		return nil
	}
	attached := *handle
	if attached.SessionID == "" {
		attached.SessionID = attached.NativeHandle
	}
	if attached.NativeHandle == "" {
		attached.NativeHandle = attached.SessionID
	}
	metadata := make(map[string]interface{}, len(attached.Metadata)+4)
	for k, v := range attached.Metadata {
		metadata[k] = v
	}
	if cwd == "" && config != nil {
		cwd = config.Cwd
	}
	if cwd != "" {
		metadata["cwd"] = cwd
	}
	if config != nil {
		if config.Model != nil && *config.Model != "" {
			metadata["model"] = *config.Model
		}
		if config.ModeID != nil && *config.ModeID != "" {
			metadata["modeId"] = *config.ModeID
		}
		if config.ThinkingOptionID != nil && *config.ThinkingOptionID != "" {
			metadata["thinkingOptionId"] = *config.ThinkingOptionID
		}
	}
	if len(metadata) > 0 {
		attached.Metadata = metadata
	}
	return &attached
}

// HydrateTimeline loads history from the provider into the given store for an agent.
// It is idempotent: a second call is a no-op if history was already loaded.
// The caller passes in the timeline store so that the agent package does not
// depend on the server package.
func (m *AgentManager) HydrateTimeline(ctx context.Context, agentID string, store TimelineAppender) error {
	agent := m.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.mu.Lock()
	if agent.historyPrimed {
		agent.mu.Unlock()
		return nil
	}
	agent.historyPrimed = true
	agent.mu.Unlock()

	// Ensure a live session exists before calling StreamHistory
	agent, err := m.ensureAgentSession(ctx, agent)
	if err != nil {
		// Reset flag so a retry is possible
		agent.mu.Lock()
		agent.historyPrimed = false
		agent.mu.Unlock()
		return fmt.Errorf("ensure session for history: %w", err)
	}

	events, err := agent.GetSession().StreamHistory(ctx)
	if err != nil {
		m.logger.Warn("StreamHistory failed, history unavailable", "agentId", agentID, "error", err)
		return nil // non-fatal; return empty timeline
	}

	for _, evt := range events {
		switch e := evt.Event.(type) {
		case protocol.TimelineStreamEvent:
			store.AppendFromHistory(agentID, e.Item)
		case map[string]interface{}:
			if evtType, _ := e["type"].(string); evtType == "timeline" {
				store.AppendFromHistory(agentID, e["item"])
			}
		}
	}
	return nil
}

// TimelineAppender is implemented by InMemoryTimelineStore (server layer).
// It allows the agent package to populate history without importing server.
type TimelineAppender interface {
	AppendFromHistory(agentID string, item interface{})
}

// ClearAgentAttention clears the unread attention marker for an agent.
func (m *AgentManager) ClearAgentAttention(agentID string) (*protocol.AgentSnapshotPayload, error) {
	agent := m.GetAgent(agentID)
	if agent == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	if agent.ClearAttention() {
		if err := m.storage.ApplySnapshot(agent); err != nil {
			m.logger.Warn("failed to persist agent attention clear", "agentId", agentID, "error", err)
		}
		m.emitState(agent)
	}
	snapshot := agent.ToSnapshot()
	return &snapshot, nil
}

// Subscribe registers a global agent event subscriber.
// Returns an unsubscribe function.
func (m *AgentManager) Subscribe(fn AgentEventFunc) func() {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextSubID
	m.nextSubID++
	m.subscribers[id] = fn
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subscribers, id)
	}
}

// subscribeToSession hooks into an agent session's event stream.
//
// A buffered work channel decouples the dispatcher's subscriber channel drain
// from handleStreamEvent processing. Without this, slow processing (ApplySnapshot
// disk I/O, sendMessage blocking on full sendQueue) causes the dispatcher's
// subscriber channel to fill up. When the dispatcher's 500ms critical timeout
// fires, turn_completed is silently dropped — the client never sees terminal state.
func (m *AgentManager) subscribeToSession(agent *ManagedAgent) {
	sess := agent.GetSession()
	if sess == nil {
		return
	}
	ch := sess.Subscribe()
	if ch == nil {
		return
	}

	workCh := make(chan AgentStreamEvent, workChCapacity.Load())
	go func() {
		for event := range workCh {
			m.handleStreamEvent(agent, event)
		}
	}()

	for event := range ch {
		if event.IsCriticalEvent() {
			// Deliver critical events to workCh with a bounded timeout.
			// If the consumer is stalled (e.g. handleStreamEvent blocked in
			// emit → sendMessage for 5s), workCh fills up and an unbounded
			// send here would block the dispatcher drain goroutine. The
			// dispatcher's subscriberCriticalTimeout (500ms) would then fire
			// and silently drop turn_completed, leaving the agent stuck in
			// protocol.AgentRunning forever.
			// Fallback: apply terminal state directly, bypassing workCh.
			select {
			case workCh <- event:
			case <-time.After(criticalWorkChSendTimeout):
				eventType := streamEventTypeString(event.Event)
				m.logger.Warn("workCh full: applying critical event state directly",
					"agentId", agent.ID,
					"eventType", eventType)
				m.applyTerminalStreamState(agent, event)
				// Flush buffered timeline entries so they are not
				// merged with the next turn's entries (cross-turn mixing).
				m.fireCoalescerFlush(agent.ID)
				// Forward the critical event to subscribers in a
				// goroutine so it reaches the session for client
				// delivery.  Without this, the bypass path applies the
				// terminal state but never delivers the turn_completed
				// event to the client, leaving the iOS app stuck with
				// an idle agent but no completed turn.
				m.forwardCriticalEventAsync(agent, event)
			}
		} else if event.IsSemiCriticalEvent() {
			// Semi-critical events (reasoning/thinking) use a short blocking
			// timeout to survive transient backpressure without blocking the
			// dispatcher drain indefinitely.
			select {
			case workCh <- event:
			case <-time.After(semiCriticalWorkChTimeout):
				total := m.droppedEventCount.Add(1)
				eventType := streamEventTypeString(event.Event)
				m.logger.Warn("semi-critical event dropped, workCh full after timeout",
					"agentId", agent.ID,
					"eventType", eventType,
					"droppedTotal", total)
			}
		} else {
			// Drop non-critical events if workCh is full to avoid blocking
			// the dispatcher drain
			select {
			case workCh <- event:
			default:
				total := m.droppedEventCount.Add(1)
				eventType := streamEventTypeString(event.Event)
				m.logger.Warn("non-critical event dropped, workCh full",
					"agentId", agent.ID,
					"eventType", eventType,
					"droppedTotal", total)
			}
		}
	}
	close(workCh)
}

// forwardCriticalEventAsync emits a critical stream event to per-agent and
// global subscribers in a goroutine.  Used by the workCh bypass path to
// deliver turn_completed/turn_failed/turn_canceled events to sessions when
// the normal pipeline is congested (workCh full).  The goroutine isolation
// prevents the potentially-blocking sendMessage path from stalling the
// dispatcher drain goroutine.
func (m *AgentManager) forwardCriticalEventAsync(agent *ManagedAgent, event AgentStreamEvent) {
	streamCopy := event
	go func() {
		agent.Emit(AgentEvent{
			Type:    EventAgentStream,
			AgentID: agent.ID,
			Stream:  &streamCopy,
		})
		m.emit(AgentEvent{
			Type:    EventAgentStream,
			AgentID: agent.ID,
			Stream:  &streamCopy,
		})
	}()
}

// handleStreamEvent processes a stream event from an agent session.
func (m *AgentManager) handleStreamEvent(agent *ManagedAgent, event AgentStreamEvent) {
	// Feed progress tracker so stall detection knows the agent is alive.
	m.stallMonitor.RecordEvent(agent.ID, event)

	// Copy the event so async emit goroutines get their own copy.
	// Without this, goroutines that write to stream.AgentID race with the
	// caller's local variable which is still read below (applyTerminalStreamState).
	streamCopy := event

	// Forward to agent subscribers
	agent.Emit(AgentEvent{
		Type:    EventAgentStream,
		AgentID: agent.ID,
		Stream:  &streamCopy,
	})

	// Forward to global subscribers
	m.emit(AgentEvent{
		Type:    EventAgentStream,
		AgentID: agent.ID,
		Stream:  &streamCopy,
	})

	// Handle permission requests
	switch event.Event.(type) {
	case protocol.PermissionRequestedStreamEvent:
		agent.SetAttention(true, "permission")
		m.emitAttentionRequired(agent, "permission")
	}

	// Persist state changes
	agent.TouchUpdatedAt()
	if err := m.storage.ApplySnapshot(agent); err != nil {
		m.logger.Warn("failed to persist agent stream state", "agentId", agent.ID, "error", err)
	}
	if m.applyTerminalStreamState(agent, event) {
		return
	}
}

// streamEventTypeString returns a human-readable type for logging purposes.
func streamEventTypeString(event interface{}) string {
	switch e := event.(type) {
	case protocol.StreamEvent:
		return e.StreamEventType()
	case map[string]interface{}:
		if t, ok := e["type"].(string); ok {
			return t
		}
	}
	return ""
}

func (m *AgentManager) applyTerminalStreamState(agent *ManagedAgent, event AgentStreamEvent) bool {
	switch e := event.Event.(type) {
	case protocol.TurnCompletedStreamEvent:
		m.stallMonitor.UnregisterAgent(agent.ID)
		if e.Usage != nil {
			agent.mu.Lock()
			agent.LastUsage = e.Usage
			agent.mu.Unlock()
		}
		agent.SetLifecycle(protocol.AgentIdle)
		agent.SetAttention(true, "finished")
		m.emitAttentionRequired(agent, "finished")
	case protocol.TurnFailedStreamEvent:
		m.stallMonitor.UnregisterAgent(agent.ID)
		errMsg := e.Error
		if errMsg == "" {
			errMsg = "agent turn failed"
		}
		agent.SetError(errMsg)
		m.emitAttentionRequired(agent, "error")
	case protocol.TurnCanceledStreamEvent:
		m.stallMonitor.UnregisterAgent(agent.ID)
		agent.SetLifecycle(protocol.AgentIdle)
	default:
		return false
	}
	if err := m.storage.ApplySnapshot(agent); err != nil {
		m.logger.Warn("failed to persist agent terminal state", "agentId", agent.ID, "error", err)
	}
	m.emitState(agent)
	return true
}

// emitAttentionRequired broadcasts an attention_required stream event.
func (m *AgentManager) emitAttentionRequired(agent *ManagedAgent, reason string) {
	notification := push.BuildAttentionNotification(agent.ID, reason, "")
	event := AgentEvent{
		Type:    EventAgentStream,
		AgentID: agent.ID,
		Stream: &AgentStreamEvent{
			AgentID: agent.ID,
			Event: protocol.AttentionRequiredStreamEvent{
				Provider: agent.Provider,
				Reason:   reason,
				Notification: map[string]interface{}{
					"title": notification.Title,
					"body":  notification.Body,
					"data": map[string]interface{}{
						"agentId": notification.Data.AgentID,
						"reason":  notification.Data.Reason,
					},
				},
			},
			Timestamp: time.Now(),
		},
	}
	agent.Emit(event)
	m.emit(event)
}

// emitState broadcasts an agent state change.
func (m *AgentManager) emitState(agent *ManagedAgent) {
	event := AgentEvent{
		Type:    EventAgentState,
		AgentID: agent.ID,
		Agent:   agent,
	}
	agent.Emit(event)
	m.emit(event)
}

// emit sends an event to all global subscribers synchronously.
// Subscribers must not block for significant time; use a non-blocking send
// (e.g. goroutine inside the subscriber) if the subscriber may block.
func (m *AgentManager) emit(event AgentEvent) {
	m.mu.RLock()
	subs := make([]AgentEventFunc, 0, len(m.subscribers))
	for _, fn := range m.subscribers {
		subs = append(subs, fn)
	}
	m.mu.RUnlock()

	for _, fn := range subs {
		fn(event)
	}
}

// recordToManagedAgent converts a StoredAgentRecord back to a ManagedAgent.
func recordToManagedAgent(r *StoredAgentRecord) *ManagedAgent {
	createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)

	var attention AttentionState
	if r.RequiresAttention && r.AttentionReason != nil {
		attention = AttentionState{
			Requires: true,
			Reason:   *r.AttentionReason,
		}
		if r.AttentionTimestamp != nil {
			if ts, err := time.Parse(time.RFC3339, *r.AttentionTimestamp); err == nil {
				attention.Timestamp = ts
			}
		}
	}

	var config *protocol.AgentSessionConfig
	if r.Config != nil {
		config = &protocol.AgentSessionConfig{
			Provider:         r.Provider,
			Cwd:              r.Cwd,
			ModeID:           r.Config.ModeID,
			Model:            r.Config.Model,
			ThinkingOptionID: r.Config.ThinkingOptionID,
			FeatureValues:    r.Config.FeatureValues,
			Title:            r.Config.Title,
			Extra:            r.Config.Extra,
			SystemPrompt:     r.Config.SystemPrompt,
			McpServers:       r.Config.McpServers,
		}
	}

	return &ManagedAgent{
		ID:            r.ID,
		Provider:      r.Provider,
		Cwd:           r.Cwd,
		Config:        config,
		RuntimeInfo:   storedToRuntimeInfo(r.RuntimeInfo),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		Lifecycle:     protocol.AgentStatus(r.LastStatus),
		CurrentModeID: r.LastModeID,
		Features:      r.Features,
		Persistence:   r.Persistence,
		LastError:     r.LastError,
		Attention:     attention,
		Internal:      r.Internal,
		Labels:        r.Labels,
		ArchivedAt:    r.ArchivedAt,
		// Session is nil until the agent is resumed
		PendingPermissions: make(map[string]interface{}),
		subscribers:        make(map[uint64]AgentEventFunc),
	}
}

func storedToRuntimeInfo(s *StoredRuntimeInfo) *protocol.AgentRuntimeInfo {
	if s == nil {
		return nil
	}
	return &protocol.AgentRuntimeInfo{
		Provider:         s.Provider,
		SessionID:        s.SessionID,
		Model:            s.Model,
		ThinkingOptionID: s.ThinkingOptionID,
		ModeID:           s.ModeID,
		Extra:            s.Extra,
	}
}

// timeNowUTC returns the current UTC time (extracted for testability).
var timeNowUTC = func() time.Time { return time.Now().UTC() }
