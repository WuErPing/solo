package server

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// TestBuildWorkspaceDescriptor_PopulatesScriptsFromProjectConfig verifies that
// buildWorkspaceDescriptor fills Scripts from the project's solo.json scripts
// config instead of always sending an empty list. The app only renders the
// workspace scripts menu when descriptor.scripts is non-empty, so a hardcoded
// empty array hides the menu even when the project defines scripts.
func TestBuildWorkspaceDescriptor_PopulatesScriptsFromProjectConfig(t *testing.T) {
	repoRoot := t.TempDir()
	soloJSON := `{
  "scripts": {
    "dev":   {"type": "service", "command": "npm run dev", "port": 3000},
    "build": {"command": "make build"}
  }
}`
	if err := os.WriteFile(filepath.Join(repoRoot, "solo.json"), []byte(soloJSON), 0o644); err != nil {
		t.Fatalf("write solo.json: %v", err)
	}

	s := &Session{
		gitSvc:       &mockGitService{},
		workspaces:   make(map[string]*protocol.WorkspaceDescriptor),
		workspacesMu: sync.RWMutex{},
		logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	persistedWs := &workspace.PersistedWorkspaceRecord{
		WorkspaceID: repoRoot,
		ProjectID:   repoRoot,
		Cwd:         repoRoot,
		Kind:        workspace.WorkspaceKindLocalCheckout,
		DisplayName: "proj",
	}
	proj := &workspace.PersistedProjectRecord{
		ProjectID:   repoRoot,
		RootPath:    repoRoot,
		Kind:        workspace.ProjectKindNonGit,
		DisplayName: "proj",
	}

	desc := s.buildWorkspaceDescriptor(persistedWs, proj)

	if len(desc.Scripts) != 2 {
		t.Fatalf("Scripts: got %d entries, want 2 (%+v)", len(desc.Scripts), desc.Scripts)
	}

	// Scripts are sorted by name for a stable wire payload.
	byName := make(map[string]protocol.WorkspaceScript, len(desc.Scripts))
	for _, script := range desc.Scripts {
		byName[script.ScriptName] = script
	}

	dev, ok := byName["dev"]
	if !ok {
		t.Fatal("Scripts: missing 'dev' entry")
	}
	if dev.Type != "service" {
		t.Errorf("dev.Type: got %q, want %q", dev.Type, "service")
	}
	if dev.Lifecycle != "stopped" {
		t.Errorf("dev.Lifecycle: got %q, want %q (not started yet)", dev.Lifecycle, "stopped")
	}

	build, ok := byName["build"]
	if !ok {
		t.Fatal("Scripts: missing 'build' entry")
	}
	// A script without an explicit type defaults to "script".
	if build.Type != "script" {
		t.Errorf("build.Type: got %q, want %q", build.Type, "script")
	}
	if build.Lifecycle != "stopped" {
		t.Errorf("build.Lifecycle: got %q, want %q", build.Lifecycle, "stopped")
	}

	if desc.Scripts[0].ScriptName != "build" || desc.Scripts[1].ScriptName != "dev" {
		t.Errorf("Scripts not sorted by name: got [%s %s]",
			desc.Scripts[0].ScriptName, desc.Scripts[1].ScriptName)
	}
}

// TestBuildWorkspaceDescriptor_NoProjectConfigKeepsScriptsEmpty verifies that a
// project without solo.json still yields an empty (non-nil) Scripts list.
func TestBuildWorkspaceDescriptor_NoProjectConfigKeepsScriptsEmpty(t *testing.T) {
	repoRoot := t.TempDir()

	s := &Session{
		gitSvc:       &mockGitService{},
		workspaces:   make(map[string]*protocol.WorkspaceDescriptor),
		workspacesMu: sync.RWMutex{},
		logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	persistedWs := &workspace.PersistedWorkspaceRecord{
		WorkspaceID: repoRoot,
		ProjectID:   repoRoot,
		Cwd:         repoRoot,
		Kind:        workspace.WorkspaceKindLocalCheckout,
		DisplayName: "proj",
	}
	proj := &workspace.PersistedProjectRecord{
		ProjectID:   repoRoot,
		RootPath:    repoRoot,
		Kind:        workspace.ProjectKindNonGit,
		DisplayName: "proj",
	}

	desc := s.buildWorkspaceDescriptor(persistedWs, proj)

	if desc.Scripts == nil {
		t.Fatal("Scripts: expected empty non-nil slice, got nil")
	}
	if len(desc.Scripts) != 0 {
		t.Fatalf("Scripts: got %d entries, want 0", len(desc.Scripts))
	}
}

// TestBuildWorkspaceDescriptor_ColdCachePopulatesGitRuntimeAndName verifies that
// buildWorkspaceDescriptor populates GitRuntime and uses the branch name (not the
// stale directory name) even when the git metadata cache is cold (e.g. after a
// daemon restart). This is the same class of bug as the handleFetchWorkspaces
// fix: GetMetadataCached returns nil on cold cache, leaving GitRuntime empty and
// Name stale.
func TestBuildWorkspaceDescriptor_ColdCachePopulatesGitRuntimeAndName(t *testing.T) {
	branch := "main"
	remote := "https://github.com/WuErPing/solo.git"
	repoRoot := "/Users/u/code/solo"

	// mockGitService simulates a cold cache: GetMetadataCached returns nil,
	// but GetMetadata returns real metadata (blocking call).
	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			repoRoot: {
				ProjectKind:          workspace.ProjectKindGit,
				ProjectDisplayName:   "solo",
				WorkspaceDisplayName: branch,
				CurrentBranch:        &branch,
				RemoteURL:            &remote,
				RepoRoot:             &repoRoot,
			},
		},
	}

	s := &Session{
		gitSvc:       gitSvc,
		workspaces:   make(map[string]*protocol.WorkspaceDescriptor),
		workspacesMu: sync.RWMutex{},
		logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	// PersistedWorkspaceRecord with a STALE DisplayName (directory name, not branch).
	// This simulates a legacy workspace or one created before the branch fix.
	persistedWs := &workspace.PersistedWorkspaceRecord{
		WorkspaceID: repoRoot,
		ProjectID:   repoRoot,
		Cwd:         repoRoot,
		Kind:        workspace.WorkspaceKindLocalCheckout,
		DisplayName: "solo", // stale: directory name, not branch
	}
	proj := &workspace.PersistedProjectRecord{
		ProjectID:   repoRoot,
		RootPath:    repoRoot,
		Kind:        workspace.ProjectKindGit,
		DisplayName: "solo",
	}

	desc := s.buildWorkspaceDescriptor(persistedWs, proj)

	// GitRuntime must be populated even on cold cache.
	if desc.GitRuntime == nil {
		t.Fatal("GitRuntime: expected non-nil, got nil (cold cache not handled)")
	}
	if desc.GitRuntime.CurrentBranch == nil || *desc.GitRuntime.CurrentBranch != branch {
		t.Errorf("GitRuntime.CurrentBranch: got %v, want %q", desc.GitRuntime.CurrentBranch, branch)
	}

	// Name must be the branch name, not the stale directory name.
	if desc.Name != branch {
		t.Errorf("Name: got %q, want %q (branch name, not directory name)", desc.Name, branch)
	}
}

// TestUpsertWorkspaceForCwd_PopulatesScriptsFromProjectConfig verifies that
// the open-project path (upsertWorkspaceForCwd) also fills Scripts from
// solo.json — otherwise the scripts menu stays hidden for workspaces that
// never go through buildWorkspaceDescriptor.
func TestUpsertWorkspaceForCwd_PopulatesScriptsFromProjectConfig(t *testing.T) {
	repoRoot := t.TempDir()
	soloJSON := `{"scripts": {"dev": {"type": "service", "command": "npm run dev"}}}`
	if err := os.WriteFile(filepath.Join(repoRoot, "solo.json"), []byte(soloJSON), 0o644); err != nil {
		t.Fatalf("write solo.json: %v", err)
	}

	s := &Session{
		gitSvc:       &mockGitService{},
		workspaces:   make(map[string]*protocol.WorkspaceDescriptor),
		workspacesMu: sync.RWMutex{},
		logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	desc, created, err := s.upsertWorkspaceForCwd(repoRoot)
	if err != nil {
		t.Fatalf("upsertWorkspaceForCwd: %v", err)
	}
	if !created {
		t.Fatal("expected workspace to be created")
	}
	if len(desc.Scripts) != 1 {
		t.Fatalf("Scripts: got %d entries, want 1 (%+v)", len(desc.Scripts), desc.Scripts)
	}
	if desc.Scripts[0].ScriptName != "dev" || desc.Scripts[0].Type != "service" {
		t.Errorf("unexpected script entry: %+v", desc.Scripts[0])
	}
	if desc.Scripts[0].Lifecycle != "stopped" {
		t.Errorf("Lifecycle: got %q, want %q", desc.Scripts[0].Lifecycle, "stopped")
	}
}

// TestProjectPlacementForWorkspace_ColdCachePopulatesBranch verifies that
// projectPlacementForWorkspace populates CurrentBranch and RemoteURL even when
// the git metadata cache is cold. GetMetadataCached returns nil on cold cache,
// leaving branch info empty in the agents list.
func TestProjectPlacementForWorkspace_ColdCachePopulatesBranch(t *testing.T) {
	branch := "feature-x"
	remote := "https://github.com/WuErPing/solo.git"
	cwd := "/Users/u/code/solo"

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			cwd: {
				ProjectKind:   workspace.ProjectKindGit,
				CurrentBranch: &branch,
				RemoteURL:     &remote,
			},
		},
	}

	s := &Session{
		gitSvc: gitSvc,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	ws := &workspace.PersistedWorkspaceRecord{
		WorkspaceID: cwd,
		ProjectID:   cwd,
		Cwd:         cwd,
		Kind:        workspace.WorkspaceKindLocalCheckout,
		DisplayName: "solo",
	}

	placement := s.projectPlacementForWorkspace(ws)

	if placement == nil {
		t.Fatal("expected non-nil placement")
	}
	if !placement.Checkout.IsGit {
		t.Error("IsGit: expected true")
	}
	if placement.Checkout.CurrentBranch == nil || *placement.Checkout.CurrentBranch != branch {
		t.Errorf("CurrentBranch: got %v, want %q", placement.Checkout.CurrentBranch, branch)
	}
	if placement.Checkout.RemoteURL == nil || *placement.Checkout.RemoteURL != remote {
		t.Errorf("RemoteURL: got %v, want %q", placement.Checkout.RemoteURL, remote)
	}
}

// TestProjectPlacementForCwd_ColdCachePopulatesBranch verifies that
// projectPlacementForCwd (the fallback path when workspace is not in the
// registry) populates IsGit, CurrentBranch, and RemoteURL even when the git
// metadata cache is cold.
func TestProjectPlacementForCwd_ColdCachePopulatesBranch(t *testing.T) {
	branch := "dev"
	remote := "https://github.com/WuErPing/solo.git"
	cwd := "/Users/u/code/solo"
	repoRoot := cwd

	gitSvc := &mockGitService{
		metas: map[string]*workspace.WorkspaceGitMetadata{
			cwd: {
				ProjectKind:   workspace.ProjectKindGit,
				CurrentBranch: &branch,
				RemoteURL:     &remote,
				RepoRoot:      &repoRoot,
			},
		},
	}

	// No workspaceReg so the fallback path (line 40-59) is exercised.
	s := &Session{
		gitSvc: gitSvc,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	placement := s.projectPlacementForCwd(cwd)

	if placement == nil {
		t.Fatal("expected non-nil placement")
	}
	if !placement.Checkout.IsGit {
		t.Error("IsGit: expected true (cold cache should not hide git status)")
	}
	if placement.Checkout.CurrentBranch == nil || *placement.Checkout.CurrentBranch != branch {
		t.Errorf("CurrentBranch: got %v, want %q", placement.Checkout.CurrentBranch, branch)
	}
	if placement.Checkout.RemoteURL == nil || *placement.Checkout.RemoteURL != remote {
		t.Errorf("RemoteURL: got %v, want %q", placement.Checkout.RemoteURL, remote)
	}
}
