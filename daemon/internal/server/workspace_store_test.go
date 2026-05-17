package server

import (
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func newTestWorkspaceStore(t *testing.T) *WorkspaceStore {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewWorkspaceStore(t.TempDir(), logger)
}

func TestWorkspaceStore_InitializeEmpty(t *testing.T) {
	s := newTestWorkspaceStore(t)
	if err := s.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if len(s.GetAll()) != 0 {
		t.Error("expected empty store")
	}
}

func TestWorkspaceStore_UpsertAndGet(t *testing.T) {
	s := newTestWorkspaceStore(t)
	_ = s.Initialize()

	ws := &protocol.WorkspaceDescriptor{
		ID:   "ws1",
		Name: "Test Workspace",
	}
	if err := s.Upsert(ws); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got := s.Get("ws1")
	if got == nil {
		t.Fatal("expected workspace")
	}
	if got.Name != "Test Workspace" {
		t.Errorf("Name: got %q, want %q", got.Name, "Test Workspace")
	}
}

func TestWorkspaceStore_GetAll(t *testing.T) {
	s := newTestWorkspaceStore(t)
	_ = s.Initialize()

	_ = s.Upsert(&protocol.WorkspaceDescriptor{ID: "ws1", Name: "One"})
	_ = s.Upsert(&protocol.WorkspaceDescriptor{ID: "ws2", Name: "Two"})

	all := s.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(all))
	}
}

func TestWorkspaceStore_Delete(t *testing.T) {
	s := newTestWorkspaceStore(t)
	_ = s.Initialize()

	_ = s.Upsert(&protocol.WorkspaceDescriptor{ID: "ws1", Name: "One"})
	if err := s.Delete("ws1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if s.Get("ws1") != nil {
		t.Error("expected workspace to be deleted")
	}
}

func TestWorkspaceStore_Persistence(t *testing.T) {
	home := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s1 := NewWorkspaceStore(home, logger)
	_ = s1.Initialize()
	_ = s1.Upsert(&protocol.WorkspaceDescriptor{ID: "ws1", Name: "Persisted"})

	s2 := NewWorkspaceStore(home, logger)
	if err := s2.Initialize(); err != nil {
		t.Fatalf("Initialize second: %v", err)
	}

	got := s2.Get("ws1")
	if got == nil || got.Name != "Persisted" {
		t.Errorf("expected persisted workspace, got %+v", got)
	}
}
