package server

import (
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

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
