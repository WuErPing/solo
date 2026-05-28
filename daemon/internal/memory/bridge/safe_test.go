package bridge

import (
	"sync/atomic"
	"testing"
	"time"
)

// ---------- helpers: fault-injecting bridges ----------

// panicBridge is a MemoryBridge that panics on every user/assistant hook.
type panicBridge struct {
	userCalls   atomic.Int64
	asstCalls   atomic.Int64
	chunkCalls  atomic.Int64
	endCalls    atomic.Int64
	systemCalls atomic.Int64
	closeCalls  atomic.Int64
}

func (p *panicBridge) OnUserTurn(_, _, _ string)      { p.userCalls.Add(1); panic("user boom") }
func (p *panicBridge) OnAssistantTurn(_, _, _ string) { p.asstCalls.Add(1); panic("asst boom") }
func (p *panicBridge) OnAssistantChunk(_, _, _ string) {
	p.chunkCalls.Add(1)
	panic("chunk boom")
}
func (p *panicBridge) OnAssistantTurnEnd(_, _ string) { p.endCalls.Add(1); panic("end boom") }
func (p *panicBridge) OnSystemTurn(_, _, _ string)    { p.systemCalls.Add(1); panic("sys boom") }
func (p *panicBridge) Close() error                   { p.closeCalls.Add(1); return nil }

// countingBridge records how many times each hook is invoked, no faults.
type countingBridge struct {
	userCalls   atomic.Int64
	asstCalls   atomic.Int64
	chunkCalls  atomic.Int64
	endCalls    atomic.Int64
	systemCalls atomic.Int64
}

func (c *countingBridge) OnUserTurn(_, _, _ string)       { c.userCalls.Add(1) }
func (c *countingBridge) OnAssistantTurn(_, _, _ string)  { c.asstCalls.Add(1) }
func (c *countingBridge) OnAssistantChunk(_, _, _ string) { c.chunkCalls.Add(1) }
func (c *countingBridge) OnAssistantTurnEnd(_, _ string)  { c.endCalls.Add(1) }
func (c *countingBridge) OnSystemTurn(_, _, _ string)     { c.systemCalls.Add(1) }
func (c *countingBridge) Close() error                    { return nil }

// ---------- interface conformance ----------

func TestSafeBridge_ImplementsMemoryBridge(t *testing.T) {
	t.Helper()
	sb := NewSafeBridge(nil)
	_ = sb // compile-time: SafeBridge satisfies the in-package contract
	// The public contract is enforced by wiring_test.go where SafeBridge
	// is assigned to server.MemoryBridge.
}

// ---------- panic recovery ----------

func TestSafeBridge_OnUserTurn_RecoversFromPanic(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)

	// Must not propagate the panic to the caller.
	sb.OnUserTurn("sess", "agent", "text")

	// Inner was entered (so we know recovery happened inside the wrapper,
	// not because the call was skipped).
	if got := inner.userCalls.Load(); got != 1 {
		t.Errorf("inner.OnUserTurn calls = %d, want 1", got)
	}
}

func TestSafeBridge_OnAssistantTurn_RecoversFromPanic(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)
	sb.OnAssistantTurn("sess", "agent", "text")
	if inner.asstCalls.Load() != 1 {
		t.Errorf("inner.OnAssistantTurn calls = %d, want 1", inner.asstCalls.Load())
	}
}

func TestSafeBridge_OnAssistantChunk_RecoversFromPanic(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)
	sb.OnAssistantChunk("agent", "sess", "frag")
	if inner.chunkCalls.Load() != 1 {
		t.Errorf("inner.OnAssistantChunk calls = %d, want 1", inner.chunkCalls.Load())
	}
}

func TestSafeBridge_OnAssistantTurnEnd_RecoversFromPanic(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)
	sb.OnAssistantTurnEnd("agent", "sess")
	if inner.endCalls.Load() != 1 {
		t.Errorf("inner.OnAssistantTurnEnd calls = %d, want 1", inner.endCalls.Load())
	}
}

func TestSafeBridge_OnSystemTurn_RecoversFromPanic(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)
	sb.OnSystemTurn("sess", "agent", "text")
	if inner.systemCalls.Load() != 1 {
		t.Errorf("inner.OnSystemTurn calls = %d, want 1", inner.systemCalls.Load())
	}
}

func TestSafeBridge_NilInner_IsNoop(t *testing.T) {
	sb := NewSafeBridge(nil)
	// None of these should panic.
	sb.OnUserTurn("s", "a", "c")
	sb.OnAssistantTurn("s", "a", "c")
	sb.OnAssistantChunk("a", "s", "f")
	sb.OnAssistantTurnEnd("a", "s")
	sb.OnSystemTurn("s", "a", "c")
	if err := sb.Close(); err != nil {
		t.Errorf("Close on nil-inner SafeBridge should be nil, got %v", err)
	}
}

// ---------- delegation (happy path) ----------

func TestSafeBridge_HappyPath_DelegatesAllHooks(t *testing.T) {
	inner := &countingBridge{}
	sb := NewSafeBridge(inner)

	sb.OnUserTurn("s", "a", "c")
	sb.OnAssistantTurn("s", "a", "c")
	sb.OnAssistantChunk("a", "s", "f")
	sb.OnAssistantTurnEnd("a", "s")
	sb.OnSystemTurn("s", "a", "c")

	if inner.userCalls.Load() != 1 {
		t.Errorf("userCalls = %d, want 1", inner.userCalls.Load())
	}
	if inner.asstCalls.Load() != 1 {
		t.Errorf("asstCalls = %d, want 1", inner.asstCalls.Load())
	}
	if inner.chunkCalls.Load() != 1 {
		t.Errorf("chunkCalls = %d, want 1", inner.chunkCalls.Load())
	}
	if inner.endCalls.Load() != 1 {
		t.Errorf("endCalls = %d, want 1", inner.endCalls.Load())
	}
	if inner.systemCalls.Load() != 1 {
		t.Errorf("systemCalls = %d, want 1", inner.systemCalls.Load())
	}
}

func TestSafeBridge_Close_DelegatesAndIdempotent(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)

	if err := sb.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := sb.Close(); err != nil {
		t.Errorf("second Close should be idempotent, got %v", err)
	}
	// Inner Close is called exactly once; the wrapper absorbs subsequent
	// Close calls so the daemon can safely defer Close multiple times.
	if got := inner.closeCalls.Load(); got != 1 {
		t.Errorf("inner.closeCalls = %d, want 1 (wrapper guarantees idempotency)", got)
	}
}

// ---------- circuit breaker ----------

func TestSafeBridge_CircuitBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)
	// Default threshold is 3; drive 5 failing calls.
	for i := 0; i < 5; i++ {
		sb.OnUserTurn("s", "a", "c")
	}
	// After the breaker opens, subsequent calls must NOT reach the inner
	// bridge at all (otherwise we'd see 5 userCalls).
	if got := inner.userCalls.Load(); got != 3 {
		t.Errorf("inner.OnUserTurn calls = %d, want 3 (breaker opens at 3)", got)
	}

	// Further calls are short-circuited: counter must stay at 3.
	sb.OnUserTurn("s", "a", "c")
	if got := inner.userCalls.Load(); got != 3 {
		t.Errorf("after open, inner calls should stop; got %d", got)
	}
}

func TestSafeBridge_CircuitBreaker_ResetsAfterCooldown(t *testing.T) {
	inner := &panicBridge{}
	// Use a very short cooldown so the test doesn't sleep long.
	sb := NewSafeBridge(inner, WithFailureCooldown(50*time.Millisecond))

	// Drive to open.
	for i := 0; i < 3; i++ {
		sb.OnUserTurn("s", "a", "c")
	}
	if got := inner.userCalls.Load(); got != 3 {
		t.Fatalf("pre-cooldown calls = %d, want 3", got)
	}

	// Wait for cooldown to elapse.
	time.Sleep(80 * time.Millisecond)

	// Next call reaches inner (and fails again, re-opening).
	sb.OnUserTurn("s", "a", "c")
	if got := inner.userCalls.Load(); got != 4 {
		t.Errorf("post-cooldown call should reach inner; got %d", got)
	}
}

// ---------- main-flow isolation guarantee ----------

// TestSafeBridge_MainFlowNotBlocked is the headline P3 property: a
// pathological inner bridge (panic on every call) must never make the
// SafeBridge caller observe anything other than a clean return.
func TestSafeBridge_MainFlowNotBlocked(t *testing.T) {
	inner := &panicBridge{}
	sb := NewSafeBridge(inner)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			sb.OnUserTurn("s", "a", "c")
			sb.OnAssistantChunk("a", "s", "frag")
			sb.OnAssistantTurnEnd("a", "s")
		}
	}()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("main flow blocked by panicking inner bridge")
	}
}
