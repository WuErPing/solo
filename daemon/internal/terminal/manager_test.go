package terminal

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestTerminalManager_CreateTerminal(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	proc, err := mgr.CreateTerminal(t.TempDir(), "Test", "/bin/sh", []string{"-c", "sleep 0.1"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if proc == nil {
		t.Fatal("expected terminal process")
	}
	if proc.Name != "Test" {
		t.Errorf("Name: got %q, want %q", proc.Name, "Test")
	}

	// Cleanup
	mgr.KillAll()
}

func TestTerminalManager_CreateTerminal_DefaultSize(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	proc, err := mgr.CreateTerminal(t.TempDir(), "Test", "/bin/sh", []string{"-c", "sleep 0.1"}, 0, 0)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if proc.Rows() != 24 {
		t.Errorf("default Rows: got %d, want 24", proc.Rows())
	}
	if proc.Cols() != 80 {
		t.Errorf("default Cols: got %d, want 80", proc.Cols())
	}
	mgr.KillAll()
}

func TestTerminalManager_GetTerminal(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	proc, err := mgr.CreateTerminal(t.TempDir(), "Test", "/bin/sh", []string{"-c", "sleep 0.1"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}

	got := mgr.GetTerminal(proc.ID)
	if got == nil {
		t.Fatal("expected to find terminal")
	}
	if got.ID != proc.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, proc.ID)
	}

	mgr.KillAll()
}

func TestTerminalManager_ListTerminals(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	cwd := t.TempDir()
	_, err := mgr.CreateTerminal(cwd, "T1", "/bin/sh", []string{"-c", "sleep 0.1"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	_, err = mgr.CreateTerminal(cwd, "T2", "/bin/sh", []string{"-c", "sleep 0.1"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}

	list := mgr.ListTerminals(cwd)
	if len(list) != 2 {
		t.Errorf("expected 2 terminals, got %d", len(list))
	}

	all := mgr.ListTerminals("")
	if len(all) != 2 {
		t.Errorf("expected 2 terminals when listing all, got %d", len(all))
	}

	mgr.KillAll()
}

func TestTerminalManager_KillTerminal(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	proc, err := mgr.CreateTerminal(t.TempDir(), "Test", "/bin/sh", []string{"-c", "sleep 10"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}

	if err := mgr.KillTerminal(proc.ID); err != nil {
		t.Fatalf("KillTerminal: %v", err)
	}

	select {
	case <-proc.Done():
		// good
	case <-time.After(3 * time.Second):
		t.Error("terminal did not exit after kill")
	}

	if mgr.GetTerminal(proc.ID) != nil {
		t.Error("expected terminal to be removed after kill")
	}
}

func TestTerminalManager_KillTerminal_NotFound(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	if err := mgr.KillTerminal("missing"); err == nil {
		t.Error("expected error for missing terminal")
	}
}

func TestTerminalManager_SubscribeTerminalsChanged(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	var events []TerminalsChangedEvent
	unsub := mgr.SubscribeTerminalsChanged(func(e TerminalsChangedEvent) {
		events = append(events, e)
	})
	defer unsub()

	proc, err := mgr.CreateTerminal(t.TempDir(), "Test", "/bin/sh", []string{"-c", "sleep 0.1"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if len(events) == 0 || events[0].Kind != "added" {
		t.Errorf("expected added event, got %+v", events)
	}

	mgr.KillTerminalAndWait(proc.ID)
	if len(events) < 2 || events[len(events)-1].Kind != "removed" {
		t.Errorf("expected removed event, got %+v", events)
	}
}

func TestTerminalManager_KillAll(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	_, _ = mgr.CreateTerminal(t.TempDir(), "T1", "/bin/sh", []string{"-c", "sleep 10"}, 24, 80)
	_, _ = mgr.CreateTerminal(t.TempDir(), "T2", "/bin/sh", []string{"-c", "sleep 10"}, 24, 80)

	mgr.KillAll()
	time.Sleep(500 * time.Millisecond)

	if len(mgr.ListTerminals("")) != 0 {
		t.Error("expected all terminals to be killed")
	}
}

func TestProcToInfo(t *testing.T) {
	proc := &TerminalProcess{
		ID:   "t1",
		Name: "Test",
		Cwd:  "/tmp",
	}
	info := procToInfo(proc)
	if info.ID != "t1" || info.Name != "Test" || info.Cwd != "/tmp" {
		t.Errorf("procToInfo mismatch: %+v", info)
	}
}

func TestTerminalManager_procToInfo_Conversion(t *testing.T) {
	mgr := NewTerminalManager(newTestLogger(t))
	proc, err := mgr.CreateTerminal(t.TempDir(), "Test", "/bin/sh", []string{"-c", "sleep 0.1"}, 24, 80)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}

	list := mgr.ListTerminals("")
	if len(list) != 1 {
		t.Fatalf("expected 1 terminal, got %d", len(list))
	}
	info := list[0]
	if info.ID != proc.ID {
		t.Errorf("expected ID %q, got %q", proc.ID, info.ID)
	}
	mgr.KillAll()
}
