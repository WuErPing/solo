package bridge

import (
	"log/slog"
	"sync"
	"time"
)

// Default values for the SafeBridge circuit breaker.
const (
	defaultFailureThreshold = 3
	defaultFailureCooldown  = 30 * time.Second
)

// safeBridgeInner is the contract SafeBridge wraps. It is identical to
// the public MemoryBridge (in the server package) but lives here so
// SafeBridge can be tested without pulling the server module in.
//
// *Bridge (defined in bridge.go) already satisfies this contract.
type safeBridgeInner interface {
	OnUserTurn(sessionID, agentID, content string)
	OnAssistantTurn(sessionID, agentID, content string)
	OnAssistantChunk(agentID, sessionID, fragment string)
	OnAssistantTurnEnd(agentID, sessionID string)
	OnSystemTurn(sessionID, agentID, content string)
	Close() error
}

// SafeBridge wraps a MemoryBridge with panic recovery and a circuit
// breaker so that a buggy or slow recorder can never take down the
// daemon's main session loop.
//
//   - Panics from inner hooks are recovered and counted as failures.
//   - After FailureThreshold consecutive failures the breaker opens and
//     short-circuits further calls for FailureCooldown.
//   - Close is always delegated (so pending chunks can flush on shutdown)
//     and is idempotent at the wrapper level.
type SafeBridge struct {
	inner safeBridgeInner
	log   *slog.Logger

	threshold int
	cooldown  time.Duration

	mu          sync.Mutex
	failures    int
	openUntil   time.Time
	closeCalled bool
}

// SafeOption configures a SafeBridge.
type SafeOption func(*SafeBridge)

// WithFailureThreshold overrides the default consecutive-failure
// threshold (default 3) that opens the circuit breaker.
func WithFailureThreshold(n int) SafeOption {
	return func(s *SafeBridge) {
		if n > 0 {
			s.threshold = n
		}
	}
}

// WithFailureCooldown overrides the default cooldown (default 30s) after
// which the breaker allows a probe call through.
func WithFailureCooldown(d time.Duration) SafeOption {
	return func(s *SafeBridge) {
		if d > 0 {
			s.cooldown = d
		}
	}
}

// WithLogger overrides the default slog.Default() logger.
func WithLoggerForSafe(log *slog.Logger) SafeOption {
	return func(s *SafeBridge) {
		if log != nil {
			s.log = log
		}
	}
}

// NewSafeBridge constructs a SafeBridge. inner may be nil, in which case
// all hooks are silent no-ops.
func NewSafeBridge(inner safeBridgeInner, opts ...SafeOption) *SafeBridge {
	s := &SafeBridge{
		inner:     inner,
		log:       slog.Default(),
		threshold: defaultFailureThreshold,
		cooldown:  defaultFailureCooldown,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ---------- hook delegation ----------

func (s *SafeBridge) OnUserTurn(sessionID, agentID, content string) {
	s.safeCall("OnUserTurn", func() {
		s.inner.OnUserTurn(sessionID, agentID, content)
	})
}

func (s *SafeBridge) OnAssistantTurn(sessionID, agentID, content string) {
	s.safeCall("OnAssistantTurn", func() {
		s.inner.OnAssistantTurn(sessionID, agentID, content)
	})
}

func (s *SafeBridge) OnAssistantChunk(agentID, sessionID, fragment string) {
	s.safeCall("OnAssistantChunk", func() {
		s.inner.OnAssistantChunk(agentID, sessionID, fragment)
	})
}

func (s *SafeBridge) OnAssistantTurnEnd(agentID, sessionID string) {
	s.safeCall("OnAssistantTurnEnd", func() {
		s.inner.OnAssistantTurnEnd(agentID, sessionID)
	})
}

func (s *SafeBridge) OnSystemTurn(sessionID, agentID, content string) {
	s.safeCall("OnSystemTurn", func() {
		s.inner.OnSystemTurn(sessionID, agentID, content)
	})
}

// Close delegates to the inner bridge. SafeBridge guarantees this is
// idempotent: the inner Close is called exactly once, even if the
// daemon invokes Close multiple times.
func (s *SafeBridge) Close() error {
	if s == nil || s.inner == nil {
		return nil
	}
	s.mu.Lock()
	if s.closeCalled {
		s.mu.Unlock()
		return nil
	}
	s.closeCalled = true
	s.mu.Unlock()
	return s.inner.Close()
}

// ---------- internals ----------

// safeCall runs fn inside a deferred panic recovery. A panic counts as
// a failure toward the circuit breaker; successful completion resets
// the consecutive-failure counter.
//
// Returns without doing anything when the breaker is open or the inner
// bridge is nil.
func (s *SafeBridge) safeCall(name string, fn func()) {
	if s == nil || s.inner == nil {
		return
	}
	if s.isCircuitOpen() {
		return
	}

	var didPanic bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
				s.recordFailure(name, r)
			}
		}()
		fn()
	}()

	if !didPanic {
		s.resetFailures()
	}
}

// isCircuitOpen reports whether the breaker is currently tripped.
func (s *SafeBridge) isCircuitOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failures < s.threshold {
		return false
	}
	if time.Now().Before(s.openUntil) {
		return true
	}
	// Cooldown elapsed: allow a probe call through, reset counter so the
	// next run of failures must again reach the threshold.
	s.failures = 0
	return false
}

// recordFailure bumps the consecutive-failure counter and, when the
// threshold is reached, records the open-until deadline.
func (s *SafeBridge) recordFailure(name string, r interface{}) {
	s.mu.Lock()
	s.failures++
	justOpened := s.failures == s.threshold
	if justOpened {
		s.openUntil = time.Now().Add(s.cooldown)
	}
	s.mu.Unlock()

	if justOpened {
		s.log.Warn("memory: circuit breaker opened",
			"hook", name,
			"consecutiveFailures", s.threshold,
			"cooldown", s.cooldown.String(),
			"panic", r,
		)
	} else {
		s.log.Warn("memory: bridge hook panicked",
			"hook", name,
			"consecutiveFailures", s.failures,
			"panic", r,
		)
	}
}

// resetFailures clears the consecutive-failure counter on success.
func (s *SafeBridge) resetFailures() {
	s.mu.Lock()
	if s.failures > 0 {
		s.failures = 0
	}
	s.mu.Unlock()
}
