package base

import (
	"fmt"
	"sync"
)

// TurnGuard provides thread-safe turn-level serialization for agent sessions.
// It replaces the duplicated activeTurnID + mu + nextTurnOrdinal pattern
// found in stdio-based providers (Claude, Kimi, Pi).
type TurnGuard struct {
	mu      sync.Mutex
	active  bool
	ordinal int
}

// NewTurnGuard creates a new TurnGuard.
func NewTurnGuard() *TurnGuard {
	return &TurnGuard{}
}

// Acquire attempts to acquire the turn lock.
// Returns a unique turnID on success, or an error if a turn is already active.
func (g *TurnGuard) Acquire() (string, error) {
	if g == nil {
		return "", fmt.Errorf("turn guard is nil")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.active {
		return "", fmt.Errorf("a turn is already active")
	}

	g.active = true
	g.ordinal++
	return fmt.Sprintf("turn-%d", g.ordinal), nil
}

// Release releases the turn lock, allowing the next Acquire to succeed.
// Safe to call multiple times (idempotent).
func (g *TurnGuard) Release() {
	if g == nil {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.active = false
}

// IsActive reports whether a turn is currently active.
func (g *TurnGuard) IsActive() bool {
	if g == nil {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active
}
