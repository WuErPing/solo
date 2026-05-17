package workspace

import (
	"testing"
	"time"
)

func TestWorkspaceRegistry_FindByCwd(t *testing.T) {
	dir := t.TempDir()
	reg := NewWorkspaceRegistry(dir)
	_ = reg.Initialize()

	_ = reg.UpsertWorkspace("ws1", "proj1", "/workspace/one", WorkspaceKindLocalCheckout, "One")
	_ = reg.UpsertWorkspace("ws2", "proj1", "/workspace/two", WorkspaceKindWorktree, "Two")

	rec, ok := reg.FindByCwd("/workspace/one")
	if !ok || rec.WorkspaceID != "ws1" {
		t.Errorf("expected ws1, got %+v", rec)
	}

	if _, ok := reg.FindByCwd("/workspace/missing"); ok {
		t.Error("expected not found for missing cwd")
	}
}

func TestWorkspaceRegistry_FindByProjectID(t *testing.T) {
	dir := t.TempDir()
	reg := NewWorkspaceRegistry(dir)
	_ = reg.Initialize()

	_ = reg.UpsertWorkspace("ws1", "proj1", "/workspace/one", WorkspaceKindLocalCheckout, "One")
	_ = reg.UpsertWorkspace("ws2", "proj1", "/workspace/two", WorkspaceKindWorktree, "Two")
	_ = reg.UpsertWorkspace("ws3", "proj2", "/workspace/three", WorkspaceKindDirectory, "Three")

	list := reg.FindByProjectID("proj1")
	if len(list) != 2 {
		t.Errorf("expected 2 workspaces for proj1, got %d", len(list))
	}

	list = reg.FindByProjectID("proj-missing")
	if len(list) != 0 {
		t.Errorf("expected 0 workspaces for missing project, got %d", len(list))
	}
}

func TestWorkspaceRegistry_UpsertWorkspace_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	reg := NewWorkspaceRegistry(dir)
	_ = reg.Initialize()

	if err := reg.UpsertWorkspace("ws1", "proj1", "/workspace/one", WorkspaceKindLocalCheckout, "One"); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	rec, ok := reg.Get("ws1")
	if !ok {
		t.Fatal("expected workspace to exist")
	}
	if rec.Cwd != "/workspace/one" {
		t.Errorf("Cwd: got %q, want %q", rec.Cwd, "/workspace/one")
	}
	if rec.Kind != WorkspaceKindLocalCheckout {
		t.Errorf("Kind: got %q, want %q", rec.Kind, WorkspaceKindLocalCheckout)
	}
}

func TestWorkspaceRegistry_UpsertWorkspace_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	reg := NewWorkspaceRegistry(dir)
	_ = reg.Initialize()

	_ = reg.UpsertWorkspace("ws1", "proj1", "/workspace/one", WorkspaceKindLocalCheckout, "One")
	_ = reg.UpsertWorkspace("ws1", "proj1", "/workspace/one", WorkspaceKindLocalCheckout, "Renamed")

	rec, _ := reg.Get("ws1")
	if rec.DisplayName != "Renamed" {
		t.Errorf("DisplayName: got %q, want %q", rec.DisplayName, "Renamed")
	}
}

func TestWorkspaceRegistry_SkipsArchived(t *testing.T) {
	dir := t.TempDir()
	reg := NewWorkspaceRegistry(dir)
	_ = reg.Initialize()

	_ = reg.UpsertWorkspace("ws1", "proj1", "/workspace/one", WorkspaceKindLocalCheckout, "One")
	now := time.Now()
	_ = reg.Archive("ws1", now)

	if _, ok := reg.FindByCwd("/workspace/one"); ok {
		t.Error("expected archived workspace to be skipped by FindByCwd")
	}

	if len(reg.FindByProjectID("proj1")) != 0 {
		t.Error("expected archived workspace to be skipped by FindByProjectID")
	}
}
