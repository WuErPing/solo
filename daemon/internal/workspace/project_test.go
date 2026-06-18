package workspace

import (
	"path/filepath"
	"testing"
)

func TestProjectRegistry_FindByRootPath(t *testing.T) {
	dir := t.TempDir()
	reg := NewProjectRegistry(dir)
	_ = reg.Initialize()

	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindGit, "One")
	_ = reg.UpsertProject("proj2", "/repo/two", ProjectKindNonGit, "Two")

	rec, ok := reg.FindByRootPath("/repo/one")
	if !ok || rec.ProjectID != "proj1" {
		t.Errorf("expected proj1, got %+v", rec)
	}

	if _, ok := reg.FindByRootPath("/repo/missing"); ok {
		t.Error("expected not found for missing path")
	}
}

func TestProjectRegistry_UpsertProject_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	reg := NewProjectRegistry(dir)
	_ = reg.Initialize()

	if err := reg.UpsertProject("proj1", "/repo/one", ProjectKindGit, "One"); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	rec, ok := reg.Get("proj1")
	if !ok {
		t.Fatal("expected project to exist")
	}
	if rec.RootPath != "/repo/one" {
		t.Errorf("RootPath: got %q, want %q", rec.RootPath, "/repo/one")
	}
	if rec.Kind != ProjectKindGit {
		t.Errorf("Kind: got %q, want %q", rec.Kind, ProjectKindGit)
	}
	if rec.DisplayName != "One" {
		t.Errorf("DisplayName: got %q, want %q", rec.DisplayName, "One")
	}
}

func TestProjectRegistry_UpsertProject_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	reg := NewProjectRegistry(dir)
	_ = reg.Initialize()

	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindGit, "One")
	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindGit, "Renamed")

	rec, _ := reg.Get("proj1")
	if rec.DisplayName != "Renamed" {
		t.Errorf("DisplayName: got %q, want %q", rec.DisplayName, "Renamed")
	}
}

func TestProjectRegistry_UpsertProject_UpdatesKind(t *testing.T) {
	dir := t.TempDir()
	reg := NewProjectRegistry(dir)
	_ = reg.Initialize()

	// Register as non_git (e.g. directory opened before git init).
	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindNonGit, "One")

	rec, _ := reg.Get("proj1")
	if rec.Kind != ProjectKindNonGit {
		t.Fatalf("precondition: Kind: got %q, want %q", rec.Kind, ProjectKindNonGit)
	}

	// Re-upsert as git (e.g. user ran git init, or daemon re-detected).
	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindGit, "One")

	rec, _ = reg.Get("proj1")
	if rec.Kind != ProjectKindGit {
		t.Errorf("Kind: got %q, want %q", rec.Kind, ProjectKindGit)
	}
}

func TestProjectRegistry_UpsertProject_UpdatesKind_GitToNonGit(t *testing.T) {
	dir := t.TempDir()
	reg := NewProjectRegistry(dir)
	_ = reg.Initialize()

	// Register as git.
	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindGit, "One")

	// Re-upsert as non_git (e.g. user deleted .git directory).
	_ = reg.UpsertProject("proj1", "/repo/one", ProjectKindNonGit, "One")

	rec, _ := reg.Get("proj1")
	if rec.Kind != ProjectKindNonGit {
		t.Errorf("Kind: got %q, want %q", rec.Kind, ProjectKindNonGit)
	}
}

func TestProjectRegistry_FilePath(t *testing.T) {
	dir := t.TempDir()
	reg := NewProjectRegistry(dir)
	expected := filepath.Join(dir, "projects", "projects.json")
	if reg.filePath != expected {
		t.Errorf("filePath: got %q, want %q", reg.filePath, expected)
	}
}
