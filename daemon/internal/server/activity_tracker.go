package server

import (
	"sync"
	"time"
)

// ClientPresenceState tracks the activity state of a single WebSocket client.
type ClientPresenceState struct {
	SessionID        string
	AppVisible       bool
	FocusedAgentID   string
	LastActivityAtMs int64
}

// ActivityTracker tracks the presence state of all connected WebSocket clients.
type ActivityTracker interface {
	// UpdateActivity updates the activity state for a session.
	UpdateActivity(sessionID string, appVisible bool, focusedAgentID string)
	// Remove deletes a session from tracking.
	Remove(sessionID string)
	// GetAllStates returns the current state of all tracked sessions.
	GetAllStates() []ClientPresenceState
}

// ClientActivityTracker is a thread-safe in-memory implementation of ActivityTracker.
type ClientActivityTracker struct {
	mu     sync.RWMutex
	states map[string]*ClientPresenceState
}

// NewClientActivityTracker creates a new client activity tracker.
func NewClientActivityTracker() *ClientActivityTracker {
	return &ClientActivityTracker{
		states: make(map[string]*ClientPresenceState),
	}
}

// UpdateActivity updates the activity state for a session.
func (t *ClientActivityTracker) UpdateActivity(sessionID string, appVisible bool, focusedAgentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.states[sessionID] = &ClientPresenceState{
		SessionID:        sessionID,
		AppVisible:       appVisible,
		FocusedAgentID:   focusedAgentID,
		LastActivityAtMs: time.Now().UnixMilli(),
	}
}

// Remove deletes a session from tracking.
func (t *ClientActivityTracker) Remove(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.states, sessionID)
}

// GetAllStates returns the current state of all tracked sessions.
func (t *ClientActivityTracker) GetAllStates() []ClientPresenceState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ClientPresenceState, 0, len(t.states))
	for _, state := range t.states {
		result = append(result, *state)
	}
	return result
}
