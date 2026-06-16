package base

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTurnGuard_Acquire_Success(t *testing.T) {
	g := NewTurnGuard()
	turnID, err := g.Acquire()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if turnID == "" {
		t.Error("expected non-empty turnID")
	}
	if !strings.HasPrefix(turnID, "turn-") {
		t.Errorf("expected turnID to start with 'turn-', got %q", turnID)
	}
}

func TestTurnGuard_Acquire_SecondFails(t *testing.T) {
	g := NewTurnGuard()
	_, err := g.Acquire()
	if err != nil {
		t.Fatalf("first acquire should succeed: %v", err)
	}

	_, err = g.Acquire()
	if err == nil {
		t.Fatal("second acquire should fail when a turn is already active")
	}
}

func TestTurnGuard_Release_AllowsReacquire(t *testing.T) {
	g := NewTurnGuard()
	turnID1, err := g.Acquire()
	if err != nil {
		t.Fatalf("first acquire should succeed: %v", err)
	}

	g.Release()

	turnID2, err := g.Acquire()
	if err != nil {
		t.Fatalf("re-acquire after release should succeed: %v", err)
	}
	if turnID2 == "" {
		t.Error("expected non-empty turnID after re-acquire")
	}
	if turnID2 == turnID1 {
		t.Error("expected different turnID after re-acquire")
	}
}

func TestTurnGuard_Release_Idempotent(t *testing.T) {
	g := NewTurnGuard()
	_, _ = g.Acquire()
	g.Release()
	g.Release() // must not panic

	// Should still allow re-acquire
	_, err := g.Acquire()
	if err != nil {
		t.Fatalf("acquire after double release should succeed: %v", err)
	}
}

func TestTurnGuard_Concurrent(t *testing.T) {
	g := NewTurnGuard()
	var winners int
	var losers int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Acquire()
			if err == nil {
				mu.Lock()
				winners++
				mu.Unlock()
				// Hold the turn briefly
				time.Sleep(10 * time.Millisecond)
				g.Release()
			} else {
				mu.Lock()
				losers++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Only one goroutine should win the initial race; the rest should fail.
	if winners != 1 {
		t.Errorf("expected exactly 1 winner in the initial race, got %d", winners)
	}
	if losers != 9 {
		t.Errorf("expected exactly 9 losers in the initial race, got %d", losers)
	}
}

func TestTurnGuard_Acquire_NilGuard(t *testing.T) {
	var g *TurnGuard
	_, err := g.Acquire()
	if err == nil {
		t.Fatal("expected error for nil TurnGuard")
	}
}

func TestTurnGuard_Release_NilGuard(_ *testing.T) {
	var g *TurnGuard
	g.Release() // must not panic
}

func TestTurnGuard_IsActive(t *testing.T) {
	g := NewTurnGuard()
	if g.IsActive() {
		t.Error("expected IsActive=false before Acquire")
	}

	_, _ = g.Acquire()
	if !g.IsActive() {
		t.Error("expected IsActive=true after Acquire")
	}

	g.Release()
	if g.IsActive() {
		t.Error("expected IsActive=false after Release")
	}
}
