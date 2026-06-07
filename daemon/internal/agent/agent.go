package agent

import (
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// AttentionState tracks whether an agent requires user attention.
type AttentionState struct {
	Requires  bool
	Reason    string // "finished" | "error" | "permission"
	Timestamp time.Time
}

// ManagedAgent is the in-memory representation of a live agent.
type ManagedAgent struct {
	mu sync.RWMutex

	ID           string
	Provider     string
	Cwd          string
	Capabilities protocol.AgentCapabilityFlags
	Config       *protocol.AgentSessionConfig
	RuntimeInfo  *protocol.AgentRuntimeInfo
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Lifecycle         protocol.AgentStatus
	CurrentModeID     *string
	AvailableModes    []protocol.AgentMode
	Features          []protocol.AgentFeature
	Persistence       *protocol.AgentPersistenceHandle
	LastUserMessageAt *time.Time
	LastUsage         *protocol.AgentUsage
	LastError         *string
	Attention         AttentionState
	Internal          bool
	Labels            map[string]string
	ArchivedAt        *string

	// Session is the active agent session (nil when closed)
	Session AgentSession

	// historyPrimed is true once StreamHistory has been called for this agent.
	// Prevents re-fetching history on every timeline request.
	historyPrimed bool

	// PendingPermissions maps requestID -> permission request payload
	PendingPermissions map[string]interface{}

	// Subscribers for this agent's events
	subscribers map[uint64]AgentEventFunc
	nextSubID   uint64
}

// NewManagedAgent creates a new ManagedAgent in initializing state.
func NewManagedAgent(id string, provider string, cwd string, config *protocol.AgentSessionConfig, labels map[string]string) *ManagedAgent {
	now := time.Now()
	if labels == nil {
		labels = make(map[string]string)
	}
	return &ManagedAgent{
		ID:                 id,
		Provider:           provider,
		Cwd:                cwd,
		Config:             config,
		CreatedAt:          now,
		UpdatedAt:          now,
		Lifecycle:          protocol.AgentInitializing,
		Labels:             labels,
		PendingPermissions: make(map[string]interface{}),
		subscribers:        make(map[uint64]AgentEventFunc),
	}
}

// SetLifecycle transitions the agent to a new lifecycle state.
func (a *ManagedAgent) SetLifecycle(lifecycle protocol.AgentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Lifecycle = lifecycle
	a.UpdatedAt = time.Now()
}

// SetAttention sets the attention state.
func (a *ManagedAgent) SetAttention(requires bool, reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Attention = AttentionState{
		Requires:  requires,
		Reason:    reason,
		Timestamp: time.Now(),
	}
}

// ClearAttention clears the unread attention marker.
func (a *ManagedAgent) ClearAttention() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.Attention.Requires {
		return false
	}
	a.Attention = AttentionState{}
	a.UpdatedAt = time.Now()
	return true
}

// ClearAttentionUnlessPermission clears stale attention while preserving active permission prompts.
func (a *ManagedAgent) ClearAttentionUnlessPermission() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.Attention.Requires || a.Attention.Reason == "permission" {
		return false
	}
	a.Attention = AttentionState{}
	a.UpdatedAt = time.Now()
	return true
}

// TouchUpdatedAt updates the UpdatedAt timestamp.
func (a *ManagedAgent) TouchUpdatedAt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.UpdatedAt = time.Now()
}

// SetError sets the last error and transitions to error state.
func (a *ManagedAgent) SetError(err string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.LastError = &err
	a.Lifecycle = protocol.AgentError
	a.UpdatedAt = time.Now()
}

// ClearError clears the last error.
func (a *ManagedAgent) ClearError() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.LastError = nil
}

// ToSnapshot projects the ManagedAgent into an AgentSnapshotPayload for the wire protocol.
func (a *ManagedAgent) ToSnapshot() protocol.AgentSnapshotPayload {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var lastUserMessageAt *string
	if a.LastUserMessageAt != nil {
		s := a.LastUserMessageAt.UTC().Format(time.RFC3339)
		lastUserMessageAt = &s
	}

	var attentionReason *string
	var attentionTimestamp *string
	if a.Attention.Requires {
		reason := a.Attention.Reason
		attentionReason = &reason
		ts := a.Attention.Timestamp.UTC().Format(time.RFC3339)
		attentionTimestamp = &ts
	}

	// Pending permissions as generic slice
	pendingPerms := make([]interface{}, 0)
	availableModes := a.AvailableModes
	if availableModes == nil {
		availableModes = []protocol.AgentMode{}
	}
	features := a.Features
	if features == nil {
		features = []protocol.AgentFeature{}
	}
	labels := a.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	var thinkingOptionID *string
	var effectiveThinkingOptionID *string
	if a.RuntimeInfo != nil {
		thinkingOptionID = a.RuntimeInfo.ThinkingOptionID
	}
	if a.Config != nil {
		effectiveThinkingOptionID = a.Config.ThinkingOptionID
	}

	return protocol.AgentSnapshotPayload{
		ID:                        a.ID,
		Provider:                  a.Provider,
		Cwd:                       a.Cwd,
		Model:                     a.model(),
		Features:                  features,
		ThinkingOptionID:          thinkingOptionID,
		EffectiveThinkingOptionID: effectiveThinkingOptionID,
		CreatedAt:                 a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                 a.UpdatedAt.UTC().Format(time.RFC3339),
		LastUserMessageAt:         lastUserMessageAt,
		Status:                    protocol.AgentLifecycleStatus(a.Lifecycle),
		Capabilities:              a.Capabilities,
		CurrentModeID:             a.CurrentModeID,
		AvailableModes:            availableModes,
		PendingPermissions:        pendingPerms,
		Persistence:               a.Persistence,
		RuntimeInfo:               a.RuntimeInfo,
		LastUsage:                 a.LastUsage,
		LastError:                 a.LastError,
		Title:                     a.title(),
		Labels:                    labels,
		RequiresAttention:         a.Attention.Requires,
		AttentionReason:           attentionReason,
		AttentionTimestamp:        attentionTimestamp,
		ArchivedAt:                a.ArchivedAt,
		ProviderUnavailable:       false,
	}
}

func (a *ManagedAgent) model() *string {
	if a.RuntimeInfo != nil && a.RuntimeInfo.Model != nil {
		return a.RuntimeInfo.Model
	}
	if a.Config != nil {
		return a.Config.Model
	}
	return nil
}

func (a *ManagedAgent) title() *string {
	if a.Config != nil && a.Config.Title != nil {
		return a.Config.Title
	}
	return nil
}

// --- Agent Event Types ---

// AgentEventType distinguishes between state changes and stream events.
type AgentEventType int

const (
	EventAgentState  AgentEventType = iota // agent state changed
	EventAgentStream                       // agent stream event
)

// AgentEvent is an event emitted by the AgentManager.
type AgentEvent struct {
	Type    AgentEventType
	AgentID string
	Agent   *ManagedAgent     // set for EventAgentState
	Stream  *AgentStreamEvent // set for EventAgentStream
	Seq     *int
	Epoch   *string
}

// AgentStreamEvent wraps a protocol stream event with metadata.
type AgentStreamEvent struct {
	AgentID   string
	Event     interface{} // one of the AgentStreamEventPayload variants
	Timestamp time.Time
}

// IsCriticalEvent returns true for terminal stream events that must never be dropped.
func (e AgentStreamEvent) IsCriticalEvent() bool {
	switch e.Event.(type) {
	case protocol.TurnCompletedStreamEvent,
		protocol.TurnFailedStreamEvent,
		protocol.TurnCanceledStreamEvent:
		return true
	}
	return false
}

// IsSemiCriticalEvent returns true for reasoning/thinking timeline events that
// should survive transient backpressure with a short blocking timeout rather
// than being silently dropped.
func (e AgentStreamEvent) IsSemiCriticalEvent() bool {
	switch evt := e.Event.(type) {
	case protocol.TimelineStreamEvent:
		return evt.Item.Type == "reasoning"
	}
	return false
}

// AgentEventFunc is a callback for agent events.
type AgentEventFunc func(AgentEvent)

// Subscribe registers a callback for agent events.
// Returns an unsubscribe function.
func (a *ManagedAgent) Subscribe(fn AgentEventFunc) func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := a.nextSubID
	a.nextSubID++
	a.subscribers[id] = fn
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		delete(a.subscribers, id)
	}
}

// Emit sends an event to all per-agent subscribers synchronously.
func (a *ManagedAgent) Emit(event AgentEvent) {
	a.mu.RLock()
	subs := make(map[uint64]AgentEventFunc, len(a.subscribers))
	for k, v := range a.subscribers {
		subs[k] = v
	}
	a.mu.RUnlock()

	for _, fn := range subs {
		fn(event)
	}
}

// IsBusy returns true if the agent is in initializing or running state.
func (a *ManagedAgent) IsBusy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Lifecycle == protocol.AgentInitializing || a.Lifecycle == protocol.AgentRunning
}

// GetSession returns the current session under the read lock.
func (a *ManagedAgent) GetSession() AgentSession {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Session
}

// SetSession replaces the session under the write lock.
func (a *ManagedAgent) SetSession(s AgentSession) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Session = s
}

// IsActive returns true if the agent has an active session.
func (a *ManagedAgent) IsActive() bool {
	return a.GetSession() != nil
}

// ShortID returns the first 8 chars of the agent ID for display.
func (a *ManagedAgent) ShortID() string {
	if len(a.ID) > 8 {
		return a.ID[:8]
	}
	return a.ID
}

// DisplayTitle returns a human-readable title for the agent.
func (a *ManagedAgent) DisplayTitle() string {
	if t := a.title(); t != nil && *t != "" {
		return *t
	}
	// Fallback: use cwd basename + provider
	parts := strings.Split(a.Cwd, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return a.Provider
}
