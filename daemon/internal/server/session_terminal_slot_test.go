package server

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/protocol"
)

func newSlotTestSession() *Session {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return &Session{
		logger:         logger,
		slotToTerminal: make(map[byte]*terminal.TerminalProcess),
		terminalToSlot: make(map[string]byte),
	}
}

func newSlotTestProcess(id string) *terminal.TerminalProcess {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return terminal.NewTerminalProcess(id, id, "/", "", nil, 24, 80, logger)
}

// TestSubscribeTerminalOutput_Idempotent verifies that subscribing the same
// terminal twice returns the same slot and does not consume a second slot.
func TestSubscribeTerminalOutput_Idempotent(t *testing.T) {
	s := newSlotTestSession()
	proc := newSlotTestProcess("term-1")

	slot1, err := s.subscribeTerminalOutput(proc)
	if err != nil {
		t.Fatalf("first subscribe: unexpected error: %v", err)
	}
	slot2, err := s.subscribeTerminalOutput(proc)
	if err != nil {
		t.Fatalf("second subscribe: unexpected error: %v", err)
	}
	if slot1 != slot2 {
		t.Fatalf("re-subscribe slot: got %d, want %d", slot2, slot1)
	}
	if len(s.terminalToSlot) != 1 || len(s.slotToTerminal) != 1 {
		t.Fatalf("expected exactly 1 slot in use, got terminalToSlot=%d slotToTerminal=%d",
			len(s.terminalToSlot), len(s.slotToTerminal))
	}
}

// TestSubscribeTerminalOutput_ReusesFreedSlot verifies that a slot freed by
// unsubscribe is reused by the next subscriber instead of being lost forever.
func TestSubscribeTerminalOutput_ReusesFreedSlot(t *testing.T) {
	s := newSlotTestSession()
	procA := newSlotTestProcess("term-a")
	procB := newSlotTestProcess("term-b")

	slotA, err := s.subscribeTerminalOutput(procA)
	if err != nil {
		t.Fatalf("subscribe A: %v", err)
	}
	if _, err := s.subscribeTerminalOutput(procB); err != nil {
		t.Fatalf("subscribe B: %v", err)
	}

	// Unsubscribe A — its slot must become available again.
	s.handleUnsubscribeTerminal(&protocol.UnsubscribeTerminalRequest{TerminalID: procA.ID})
	if _, ok := s.terminalToSlot[procA.ID]; ok {
		t.Fatal("terminalToSlot still holds unsubscribed terminal")
	}
	if _, ok := s.slotToTerminal[slotA]; ok {
		t.Fatal("slotToTerminal still holds freed slot")
	}

	procC := newSlotTestProcess("term-c")
	slotC, err := s.subscribeTerminalOutput(procC)
	if err != nil {
		t.Fatalf("subscribe C: %v", err)
	}
	if slotC != slotA {
		t.Fatalf("expected reused slot %d, got %d", slotA, slotC)
	}
}

// TestSubscribeTerminalOutput_ExhaustionReturnsError verifies that once all
// 256 slots are taken, further subscriptions fail with an explicit error
// instead of wrapping around and overwriting an active subscriber's slot.
func TestSubscribeTerminalOutput_ExhaustionReturnsError(t *testing.T) {
	s := newSlotTestSession()

	first := newSlotTestProcess("term-000")
	firstSlot, err := s.subscribeTerminalOutput(first)
	if err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	for i := 1; i < 256; i++ {
		if _, err := s.subscribeTerminalOutput(newSlotTestProcess(fmt.Sprintf("term-%03d", i))); err != nil {
			t.Fatalf("subscribe %d: unexpected error: %v", i, err)
		}
	}
	if len(s.slotToTerminal) != 256 {
		t.Fatalf("expected 256 slots in use, got %d", len(s.slotToTerminal))
	}

	// The 257th subscription must fail, not wrap around.
	if _, err := s.subscribeTerminalOutput(newSlotTestProcess("term-overflow")); err == nil {
		t.Fatal("expected error when slots are exhausted, got nil")
	}

	// The first subscriber's mapping must be intact (no wrap-around overwrite).
	if slot, ok := s.terminalToSlot[first.ID]; !ok || slot != firstSlot {
		t.Fatalf("first subscriber mapping corrupted: slot=%d ok=%v", slot, ok)
	}
	if got := s.slotToTerminal[firstSlot]; got != first {
		t.Fatalf("slot %d stolen by overflow subscriber: got terminal %q", firstSlot, got.ID)
	}
}

// TestHandleKillTerminal_ReleasesSlot verifies that killing a terminal frees
// its output subscription slot so it can be reused.
func TestHandleKillTerminal_ReleasesSlot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newSlotTestSession()
	s.terminalMgr = terminal.NewTerminalManager(logger)

	proc, err := s.terminalMgr.CreateTerminal(t.TempDir(), "kill-me", "", nil, 24, 80)
	if err != nil {
		t.Fatalf("create terminal: %v", err)
	}

	slot, err := s.subscribeTerminalOutput(proc)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	s.handleKillTerminal(&protocol.KillTerminalRequest{TerminalID: proc.ID})

	if _, ok := s.terminalToSlot[proc.ID]; ok {
		t.Fatal("terminalToSlot still holds killed terminal")
	}
	if _, ok := s.slotToTerminal[slot]; ok {
		t.Fatal("slotToTerminal still holds slot of killed terminal")
	}

	// The freed slot must be reusable.
	next, err := s.subscribeTerminalOutput(newSlotTestProcess("term-after-kill"))
	if err != nil {
		t.Fatalf("subscribe after kill: %v", err)
	}
	if next != slot {
		t.Fatalf("expected reused slot %d after kill, got %d", slot, next)
	}
}
