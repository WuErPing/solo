package server

import (
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// refreshWorkspaceProjectKind updates each workspace descriptor's ProjectKind
// to match the current value in the project registry. This handles the case
// where a project's git status changed after the workspace was first registered
// (e.g. git init, or .git deletion).
func refreshWorkspaceProjectKind(workspaces []*protocol.WorkspaceDescriptor, projectReg *workspace.ProjectRegistry) {
	if projectReg == nil {
		return
	}
	for _, ws := range workspaces {
		if ws == nil || ws.ProjectID == "" {
			continue
		}
		proj, ok := projectReg.Get(ws.ProjectID)
		if !ok {
			continue
		}
		kind := string(proj.Kind)
		if kind != "" && ws.ProjectKind != kind {
			ws.ProjectKind = kind
		}
	}
}

// redetectNonGitWorkspaces re-detects git status for workspaces still marked
// as non_git (or the legacy "directory" value). This handles the case where a
// workspace was created before the project was registered in the project
// registry (e.g. first agent run before git init, then git init happened later).
func redetectNonGitWorkspaces(workspaces []*protocol.WorkspaceDescriptor, projectReg *workspace.ProjectRegistry, gitSvc workspace.WorkspaceGitService) {
	if gitSvc == nil {
		return
	}
	for _, ws := range workspaces {
		if ws == nil || !isNonGitKind(ws.ProjectKind) {
			continue
		}
		dir := ws.WorkspaceDirectory
		if dir == "" {
			dir = ws.ID
		}
		if dir == "" {
			continue
		}
		meta, err := gitSvc.GetMetadata(dir)
		if err != nil || meta == nil || meta.ProjectKind != workspace.ProjectKindGit {
			continue
		}
		ws.ProjectKind = string(workspace.ProjectKindGit)
		ws.WorkspaceKind = string(workspace.WorkspaceKindLocalCheckout)
		ws.WorkspaceDirectory = dir
		if ws.ProjectID == "" {
			ws.ProjectID = dir
		}
		if ws.ProjectRootPath == "" {
			ws.ProjectRootPath = dir
		}
		if meta.ProjectDisplayName != "" {
			ws.ProjectDisplayName = meta.ProjectDisplayName
		}
		// Update workspace name to branch name for git workspaces.
		if meta.WorkspaceDisplayName != "" {
			ws.Name = meta.WorkspaceDisplayName
		}
		if projectReg != nil && ws.ProjectID != "" {
			_ = projectReg.UpsertProject(ws.ProjectID, ws.ProjectID, workspace.ProjectKindGit, meta.ProjectDisplayName)
		}
	}
}

// isNonGitKind returns true if the kind represents a non-git workspace,
// including the legacy "directory" value used by older versions.
func isNonGitKind(kind string) bool {
	return kind == string(workspace.ProjectKindNonGit) || kind == "directory" || kind == ""
}

// fixStaleWorkspaceNames corrects the Name field of git workspaces whose Name
// (directory name) doesn't match the branch in GitRuntime. This handles
// workspaces that already have GitRuntime populated (so the cold-cache fix in
// handleFetchWorkspaces is skipped) but still carry a stale Name from before
// the branch-name fix was introduced.
func fixStaleWorkspaceNames(workspaces []*protocol.WorkspaceDescriptor) {
	for _, ws := range workspaces {
		if ws == nil || ws.ProjectKind != string(workspace.ProjectKindGit) {
			continue
		}
		if ws.GitRuntime == nil || ws.GitRuntime.CurrentBranch == nil {
			continue
		}
		branch := *ws.GitRuntime.CurrentBranch
		if branch != "" && ws.Name != branch {
			ws.Name = branch
		}
	}
}

// syncWorkspacesToRegistry upserts each workspace descriptor into the
// WorkspaceRegistry so that ~/.solo/projects/workspaces.json stays in sync
// with the corrected ProjectKind, WorkspaceKind, and Name (branch).
func syncWorkspacesToRegistry(workspaces []*protocol.WorkspaceDescriptor, workspaceReg *workspace.WorkspaceRegistry) {
	if workspaceReg == nil {
		return
	}
	for _, ws := range workspaces {
		if ws == nil || ws.ID == "" {
			continue
		}
		wsKind := workspace.WorkspaceKindDirectory
		if ws.WorkspaceKind != "" {
			wsKind = workspace.WorkspaceKind(ws.WorkspaceKind)
		}
		_ = workspaceReg.UpsertWorkspace(ws.ID, ws.ProjectID, ws.WorkspaceDirectory, wsKind, ws.Name)
	}
}
