package server

import (
	"context"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// mockGitService is a minimal WorkspaceGitService for testing redetectNonGitWorkspaces.
type mockGitService struct {
	metas map[string]*workspace.WorkspaceGitMetadata
}

func (m *mockGitService) ResolveRepoRoot(_ string) (string, error) { return "", nil }
func (m *mockGitService) GetMetadata(cwd string) (*workspace.WorkspaceGitMetadata, error) {
	if m.metas != nil {
		if meta, ok := m.metas[cwd]; ok {
			return meta, nil
		}
	}
	return &workspace.WorkspaceGitMetadata{ProjectKind: workspace.ProjectKindNonGit}, nil
}
func (m *mockGitService) GetMetadataCached(_ string) *workspace.WorkspaceGitMetadata { return nil }
func (m *mockGitService) GetCurrentBranch(_ string) (string, error)                  { return "", nil }
func (m *mockGitService) GetRemoteURL(_ string, _ string) (string, error)            { return "", nil }
func (m *mockGitService) IsWorktree(_ string) (bool, error)                          { return false, nil }
func (m *mockGitService) IsDirty(_ string) (bool, error)                             { return false, nil }
func (m *mockGitService) IsDirtyCached(_ string) *bool                               { return nil }
func (m *mockGitService) StartBackgroundRefresh(_ context.Context, _ time.Duration)  {}
func (m *mockGitService) StopBackgroundRefresh()                                     {}

func TestRefreshWorkspaceProjectKind_UpdatesStaleNonGitToGit(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	// Project was re-detected as git in the registry.
	_ = projectReg.UpsertProject("/repo", "/repo", workspace.ProjectKindGit, "MyRepo")

	// Workspace descriptor still has the old non_git kind.
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/repo",
	}

	refreshWorkspaceProjectKind([]*protocol.WorkspaceDescriptor{ws}, projectReg)

	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
}

func TestRefreshWorkspaceProjectKind_UpdatesStaleGitToNonGit(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	// Project was re-detected as non_git (e.g. .git deleted).
	_ = projectReg.UpsertProject("/repo", "/repo", workspace.ProjectKindNonGit, "MyRepo")

	// Workspace descriptor still has the old git kind.
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindGit),
		WorkspaceDirectory: "/repo",
	}

	refreshWorkspaceProjectKind([]*protocol.WorkspaceDescriptor{ws}, projectReg)

	if ws.ProjectKind != string(workspace.ProjectKindNonGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindNonGit)
	}
}

func TestRefreshWorkspaceProjectKind_NoChangeWhenAlreadyCorrect(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	_ = projectReg.UpsertProject("/repo", "/repo", workspace.ProjectKindGit, "MyRepo")

	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindGit),
		WorkspaceDirectory: "/repo",
	}

	refreshWorkspaceProjectKind([]*protocol.WorkspaceDescriptor{ws}, projectReg)

	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
}

func TestRefreshWorkspaceProjectKind_SkipsWhenProjectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	// Project not in registry.
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/missing",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/missing",
	}

	refreshWorkspaceProjectKind([]*protocol.WorkspaceDescriptor{ws}, projectReg)

	// Should remain unchanged.
	if ws.ProjectKind != string(workspace.ProjectKindNonGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindNonGit)
	}
}

func TestRefreshWorkspaceProjectKind_NilRegistry(t *testing.T) {
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/repo",
	}

	refreshWorkspaceProjectKind([]*protocol.WorkspaceDescriptor{ws}, nil)

	if ws.ProjectKind != string(workspace.ProjectKindNonGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindNonGit)
	}
}

// ---- redetectNonGitWorkspaces ----

func TestRedetectNonGitWorkspaces_UpdatesNonGitToGit(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/repo": {ProjectKind: workspace.ProjectKindGit, ProjectDisplayName: "MyRepo"},
		},
	}

	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/repo",
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, projectReg, gitSvc)

	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
	// Project should also be registered in the registry.
	proj, ok := projectReg.Get("/repo")
	if !ok {
		t.Fatal("expected project to be registered")
	}
	if proj.Kind != workspace.ProjectKindGit {
		t.Errorf("registry Kind: got %q, want %q", proj.Kind, workspace.ProjectKindGit)
	}
}

func TestRedetectNonGitWorkspaces_NoChangeWhenAlreadyGit(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/repo": {ProjectKind: workspace.ProjectKindGit, ProjectDisplayName: "MyRepo"},
		},
	}

	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindGit),
		WorkspaceDirectory: "/repo",
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, projectReg, gitSvc)

	// Should remain git (no unnecessary detection).
	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
}

func TestRedetectNonGitWorkspaces_NoChangeWhenStillNonGit(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/plain": {ProjectKind: workspace.ProjectKindNonGit, ProjectDisplayName: "plain"},
		},
	}

	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/plain",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/plain",
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, projectReg, gitSvc)

	if ws.ProjectKind != string(workspace.ProjectKindNonGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindNonGit)
	}
}

func TestRedetectNonGitWorkspaces_NilGitService(t *testing.T) {
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/repo",
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, nil, nil)

	if ws.ProjectKind != string(workspace.ProjectKindNonGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindNonGit)
	}
}

func TestRedetectNonGitWorkspaces_NilRegistry(t *testing.T) {
	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/repo": {ProjectKind: workspace.ProjectKindGit, ProjectDisplayName: "MyRepo"},
		},
	}

	ws := &protocol.WorkspaceDescriptor{
		ID:                 "ws-1",
		ProjectID:          "/repo",
		ProjectKind:        string(workspace.ProjectKindNonGit),
		WorkspaceDirectory: "/repo",
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, nil, gitSvc)

	// Workspace should still be updated even without registry.
	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
}

func TestRedetectNonGitWorkspaces_LegacyDirectoryValue(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	branch := "main"
	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/repo": {
				ProjectKind:          workspace.ProjectKindGit,
				ProjectDisplayName:   "MyRepo",
				WorkspaceDisplayName: branch,
				CurrentBranch:        &branch,
			},
		},
	}

	// Legacy workspaces use "directory" instead of "non_git".
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "/repo",
		ProjectID:          "/repo",
		ProjectKind:        "directory",
		WorkspaceDirectory: "/repo",
		Name:               "repo", // legacy: directory name, not branch
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, projectReg, gitSvc)

	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
	if ws.WorkspaceKind != string(workspace.WorkspaceKindLocalCheckout) {
		t.Errorf("WorkspaceKind: got %q, want %q", ws.WorkspaceKind, workspace.WorkspaceKindLocalCheckout)
	}
	if ws.Name != branch {
		t.Errorf("Name: got %q, want %q (branch name)", ws.Name, branch)
	}
	if ws.ProjectDisplayName != "MyRepo" {
		t.Errorf("ProjectDisplayName: got %q, want %q", ws.ProjectDisplayName, "MyRepo")
	}
}

func TestRedetectNonGitWorkspaces_MissingWorkspaceDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/repo": {ProjectKind: workspace.ProjectKindGit, ProjectDisplayName: "MyRepo"},
		},
	}

	// Legacy workspaces may lack WorkspaceDirectory; falls back to ID.
	ws := &protocol.WorkspaceDescriptor{
		ID:          "/repo",
		ProjectID:   "/repo",
		ProjectKind: "directory",
		// WorkspaceDirectory intentionally omitted.
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, projectReg, gitSvc)

	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
	if ws.WorkspaceDirectory != "/repo" {
		t.Errorf("WorkspaceDirectory: got %q, want %q", ws.WorkspaceDirectory, "/repo")
	}
}

func TestRedetectNonGitWorkspaces_EmptyProjectKind(t *testing.T) {
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/repo": {ProjectKind: workspace.ProjectKindGit, ProjectDisplayName: "MyRepo"},
		},
	}

	ws := &protocol.WorkspaceDescriptor{
		ID:                 "/repo",
		ProjectID:          "/repo",
		ProjectKind:        "",
		WorkspaceDirectory: "/repo",
	}

	redetectNonGitWorkspaces([]*protocol.WorkspaceDescriptor{ws}, projectReg, gitSvc)

	if ws.ProjectKind != string(workspace.ProjectKindGit) {
		t.Errorf("ProjectKind: got %q, want %q", ws.ProjectKind, workspace.ProjectKindGit)
	}
}

// TestLegacyWorkspaceFullPipeline is an end-to-end test that simulates the full
// handleFetchWorkspaces pipeline for a legacy workspace (created by an older
// code path with projectKind:"directory", no workspaceDirectory, no gitRuntime).
// It verifies that after the pipeline, the workspace has all fields correct:
// projectKind, workspaceKind, workspaceDirectory, name (branch), projectDisplayName,
// and gitRuntime (branch, remote).
func TestLegacyWorkspaceFullPipeline(t *testing.T) {
	// --- Arrange: set up store with legacy workspace data ---
	store := newTestWorkspaceStore(t)
	_ = store.Initialize()

	// Simulate legacy data as it would appear in workspaces.json.
	legacyWs := &protocol.WorkspaceDescriptor{
		ID:                 "/Users/u/code/solo",
		ProjectID:          "/Users/u/code/solo",
		ProjectDisplayName: "solo",
		ProjectRootPath:    "/Users/u/code/solo",
		ProjectKind:        "directory",
		WorkspaceKind:      "directory",
		Name:               "solo", // legacy: directory name, not branch
		Status:             "done",
	}
	if err := store.Upsert(legacyWs); err != nil {
		t.Fatalf("Upsert legacy: %v", err)
	}

	// Project registry is empty (solo was never registered).
	tmpDir := t.TempDir()
	projectReg := workspace.NewProjectRegistry(tmpDir)
	_ = projectReg.Initialize()

	// Git service detects the directory as a git repo on branch "main".
	branch := "main"
	remote := "https://github.com/WuErPing/solo.git"
	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/Users/u/code/solo": {
				ProjectKind:          workspace.ProjectKindGit,
				ProjectDisplayName:   "solo",
				WorkspaceDisplayName: branch,
				CurrentBranch:        &branch,
				RemoteURL:            &remote,
			},
		},
	}

	// --- Act: simulate handleFetchWorkspaces pipeline ---

	// Step 1: Load from store.
	workspaces := make(map[string]*protocol.WorkspaceDescriptor)
	for _, ws := range store.GetAll() {
		workspaces[ws.ID] = ws
	}
	wsList := make([]*protocol.WorkspaceDescriptor, 0, len(workspaces))
	for _, ws := range workspaces {
		wsList = append(wsList, ws)
	}

	// Step 2: Refresh from project registry (no-op — not registered).
	refreshWorkspaceProjectKind(wsList, projectReg)

	// Step 3: Re-detect non-git workspaces.
	redetectNonGitWorkspaces(wsList, projectReg, gitSvc)

	// Step 4: Populate GitRuntime for git workspaces missing it.
	for _, ws := range workspaces {
		if ws.GitRuntime == nil && ws.ProjectKind == string(workspace.ProjectKindGit) {
			meta, _ := gitSvc.GetMetadata(ws.WorkspaceDirectory)
			if meta != nil {
				ws.GitRuntime = &protocol.WorkspaceGitRuntime{
					CurrentBranch: meta.CurrentBranch,
					RemoteURL:     meta.RemoteURL,
				}
			}
		}
	}

	// Step 5: Persist back to store.
	for _, ws := range wsList {
		_ = store.Upsert(ws)
	}

	// --- Assert: reload and verify all fields ---
	reloaded := store.Get("/Users/u/code/solo")
	if reloaded == nil {
		t.Fatal("workspace not found after persist")
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ProjectKind", reloaded.ProjectKind, "git"},
		{"WorkspaceKind", reloaded.WorkspaceKind, "local_checkout"},
		{"WorkspaceDirectory", reloaded.WorkspaceDirectory, "/Users/u/code/solo"},
		{"ProjectDisplayName", reloaded.ProjectDisplayName, "solo"},
		{"Name", reloaded.Name, branch}, // must be branch, not directory name
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}

	if reloaded.GitRuntime == nil {
		t.Fatal("GitRuntime: expected non-nil")
	}
	if reloaded.GitRuntime.CurrentBranch == nil || *reloaded.GitRuntime.CurrentBranch != branch {
		t.Errorf("GitRuntime.CurrentBranch: got %v, want %q", reloaded.GitRuntime.CurrentBranch, branch)
	}
	if reloaded.GitRuntime.RemoteURL == nil || *reloaded.GitRuntime.RemoteURL != remote {
		t.Errorf("GitRuntime.RemoteURL: got %v, want %q", reloaded.GitRuntime.RemoteURL, remote)
	}

	// Verify project was registered.
	proj, ok := projectReg.Get("/Users/u/code/solo")
	if !ok {
		t.Fatal("expected project to be registered in registry")
	}
	if proj.Kind != workspace.ProjectKindGit {
		t.Errorf("registry Kind: got %q, want %q", proj.Kind, workspace.ProjectKindGit)
	}
}

// TestAlreadyGitWorkspaceFixesStaleName verifies that a workspace already marked
// as git but with a stale Name (directory name instead of branch) gets its Name
// fixed when GitRuntime is populated. This simulates the state where a previous
// fix upgraded projectKind from "directory" to "git" but didn't fix the Name.
func TestAlreadyGitWorkspaceFixesStaleName(t *testing.T) {
	store := newTestWorkspaceStore(t)
	_ = store.Initialize()

	// Workspace already upgraded to git by previous fix, but Name is still stale.
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "/Users/u/code/solo",
		ProjectID:          "/Users/u/code/solo",
		ProjectDisplayName: "solo",
		ProjectRootPath:    "/Users/u/code/solo",
		WorkspaceDirectory: "/Users/u/code/solo",
		ProjectKind:        "git",
		WorkspaceKind:      "local_checkout",
		Name:               "solo", // stale: directory name, not branch
		Status:             "done",
	}
	if err := store.Upsert(ws); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	branch := "main"
	remote := "https://github.com/WuErPing/solo.git"
	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			"/Users/u/code/solo": {
				ProjectKind:          workspace.ProjectKindGit,
				ProjectDisplayName:   "solo",
				WorkspaceDisplayName: branch,
				CurrentBranch:        &branch,
				RemoteURL:            &remote,
			},
		},
	}

	// Simulate the GitRuntime population from handleFetchWorkspaces.
	// This is the code path that fixes stale Name for already-git workspaces.
	if ws.GitRuntime == nil && ws.ProjectKind == string(workspace.ProjectKindGit) {
		meta, _ := gitSvc.GetMetadata(ws.WorkspaceDirectory)
		if meta != nil {
			ws.GitRuntime = &protocol.WorkspaceGitRuntime{
				CurrentBranch: meta.CurrentBranch,
				RemoteURL:     meta.RemoteURL,
			}
			if meta.WorkspaceDisplayName != "" {
				ws.Name = meta.WorkspaceDisplayName
			}
			if meta.ProjectDisplayName != "" {
				ws.ProjectDisplayName = meta.ProjectDisplayName
			}
		}
	}
	_ = store.Upsert(ws)

	reloaded := store.Get("/Users/u/code/solo")
	if reloaded == nil {
		t.Fatal("workspace not found")
	}
	if reloaded.Name != branch {
		t.Errorf("Name: got %q, want %q (branch name, not directory name)", reloaded.Name, branch)
	}
	if reloaded.GitRuntime == nil || reloaded.GitRuntime.CurrentBranch == nil || *reloaded.GitRuntime.CurrentBranch != branch {
		t.Errorf("GitRuntime.CurrentBranch: got %v, want %q", reloaded.GitRuntime.CurrentBranch, branch)
	}
}

// ---- fixStaleWorkspaceNames ----

func TestFixStaleWorkspaceNames_FixesNameFromGitRuntimeBranch(t *testing.T) {
	branch := "main"
	ws := &protocol.WorkspaceDescriptor{
		ID:          "/Users/u/code/solo",
		ProjectKind: string(workspace.ProjectKindGit),
		Name:        "solo", // stale: directory name
		GitRuntime: &protocol.WorkspaceGitRuntime{
			CurrentBranch: &branch,
		},
	}

	fixStaleWorkspaceNames([]*protocol.WorkspaceDescriptor{ws})

	if ws.Name != branch {
		t.Errorf("Name: got %q, want %q (branch from GitRuntime)", ws.Name, branch)
	}
}

func TestFixStaleWorkspaceNames_NoChangeWhenNameMatchesBranch(t *testing.T) {
	branch := "main"
	ws := &protocol.WorkspaceDescriptor{
		ID:          "/Users/u/code/solo",
		ProjectKind: string(workspace.ProjectKindGit),
		Name:        "main", // already correct
		GitRuntime: &protocol.WorkspaceGitRuntime{
			CurrentBranch: &branch,
		},
	}

	fixStaleWorkspaceNames([]*protocol.WorkspaceDescriptor{ws})

	if ws.Name != "main" {
		t.Errorf("Name: got %q, want %q (should be unchanged)", ws.Name, "main")
	}
}

func TestFixStaleWorkspaceNames_SkipsNonGitWorkspaces(t *testing.T) {
	branch := "main"
	ws := &protocol.WorkspaceDescriptor{
		ID:          "/Users/u/code/plain",
		ProjectKind: string(workspace.ProjectKindNonGit),
		Name:        "plain",
		GitRuntime: &protocol.WorkspaceGitRuntime{
			CurrentBranch: &branch,
		},
	}

	fixStaleWorkspaceNames([]*protocol.WorkspaceDescriptor{ws})

	if ws.Name != "plain" {
		t.Errorf("Name: got %q, want %q (non-git should be unchanged)", ws.Name, "plain")
	}
}

func TestFixStaleWorkspaceNames_SkipsWhenGitRuntimeNil(t *testing.T) {
	ws := &protocol.WorkspaceDescriptor{
		ID:          "/Users/u/code/solo",
		ProjectKind: string(workspace.ProjectKindGit),
		Name:        "solo",
		// GitRuntime is nil
	}

	fixStaleWorkspaceNames([]*protocol.WorkspaceDescriptor{ws})

	if ws.Name != "solo" {
		t.Errorf("Name: got %q, want %q (no GitRuntime to fix from)", ws.Name, "solo")
	}
}

// ---- syncWorkspacesToRegistry ----

func TestSyncWorkspacesToRegistry_RegistersMissingWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceReg := workspace.NewWorkspaceRegistry(tmpDir)
	_ = workspaceReg.Initialize()

	branch := "main"
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "/Users/u/code/solo",
		ProjectID:          "/Users/u/code/solo",
		ProjectDisplayName: "solo",
		WorkspaceDirectory: "/Users/u/code/solo",
		ProjectKind:        string(workspace.ProjectKindGit),
		WorkspaceKind:      string(workspace.WorkspaceKindLocalCheckout),
		Name:               branch,
	}

	syncWorkspacesToRegistry([]*protocol.WorkspaceDescriptor{ws}, workspaceReg)

	rec, ok := workspaceReg.Get("/Users/u/code/solo")
	if !ok {
		t.Fatal("expected workspace to be registered")
	}
	if rec.DisplayName != branch {
		t.Errorf("DisplayName: got %q, want %q (branch name)", rec.DisplayName, branch)
	}
	if rec.Kind != workspace.WorkspaceKindLocalCheckout {
		t.Errorf("Kind: got %q, want %q", rec.Kind, workspace.WorkspaceKindLocalCheckout)
	}
}

func TestSyncWorkspacesToRegistry_UpdatesStaleDisplayName(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceReg := workspace.NewWorkspaceRegistry(tmpDir)
	_ = workspaceReg.Initialize()

	// Pre-existing record with stale DisplayName (directory name).
	_ = workspaceReg.UpsertWorkspace("/repo", "/repo", "/repo",
		workspace.WorkspaceKindLocalCheckout, "repo")

	// Sync with updated Name (branch name).
	ws := &protocol.WorkspaceDescriptor{
		ID:                 "/repo",
		ProjectID:          "/repo",
		WorkspaceDirectory: "/repo",
		ProjectKind:        string(workspace.ProjectKindGit),
		WorkspaceKind:      string(workspace.WorkspaceKindLocalCheckout),
		Name:               "main",
	}

	syncWorkspacesToRegistry([]*protocol.WorkspaceDescriptor{ws}, workspaceReg)

	rec, _ := workspaceReg.Get("/repo")
	if rec.DisplayName != "main" {
		t.Errorf("DisplayName: got %q, want %q (updated to branch name)", rec.DisplayName, "main")
	}
}

func TestSyncWorkspacesToRegistry_NilRegistry(t *testing.T) {
	ws := &protocol.WorkspaceDescriptor{
		ID:   "/repo",
		Name: "main",
	}

	// Should not panic.
	syncWorkspacesToRegistry([]*protocol.WorkspaceDescriptor{ws}, nil)

	if ws.Name != "main" {
		t.Errorf("Name: got %q, want %q", ws.Name, "main")
	}
}
