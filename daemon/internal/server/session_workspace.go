package server

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

func (s *Session) projectPlacementForAgent(ag *agent.ManagedAgent) *protocol.ProjectPlacementPayload {
	if ag == nil {
		return nil
	}
	return s.projectPlacementForCwd(ag.Cwd)
}

func (s *Session) projectPlacementForCwd(cwd string) *protocol.ProjectPlacementPayload {
	normalizedCwd := normalizeProjectCwd(cwd)

	if s.workspaceReg != nil {
		if ws, ok := s.workspaceReg.FindByCwd(normalizedCwd); ok {
			if project := s.projectPlacementForWorkspace(ws); project != nil {
				return project
			}
		}
		if normalizedCwd != cwd {
			if ws, ok := s.workspaceReg.FindByCwd(cwd); ok {
				if project := s.projectPlacementForWorkspace(ws); project != nil {
					return project
				}
			}
		}
	}

	checkout := protocol.ProjectCheckoutLitePayload{
		Cwd:                 normalizedCwd,
		IsGit:               false,
		CurrentBranch:       nil,
		RemoteURL:           nil,
		WorktreeRoot:        nil,
		IsSoloOwnedWorktree: false,
		MainRepoRoot:        nil,
	}
	if s.gitSvc != nil {
		meta := s.gitSvc.GetMetadataCached(normalizedCwd)
		if meta == nil {
			meta, _ = s.gitSvc.GetMetadata(normalizedCwd)
		}
		if meta != nil && meta.ProjectKind == workspace.ProjectKindGit {
			checkout.IsGit = true
			checkout.CurrentBranch = meta.CurrentBranch
			checkout.RemoteURL = meta.RemoteURL
			if meta.RepoRoot != nil && *meta.RepoRoot != "" {
				repoRoot := normalizeProjectCwd(*meta.RepoRoot)
				checkout.WorktreeRoot = &repoRoot
			}
		}
	}

	projectKey := deriveProjectGroupingKey(checkout.Cwd, checkout.RemoteURL, checkout.MainRepoRoot)
	return &protocol.ProjectPlacementPayload{
		ProjectKey:  projectKey,
		ProjectName: deriveProjectGroupingName(projectKey),
		Checkout:    checkout,
	}
}

func (s *Session) projectPlacementForWorkspace(ws *workspace.PersistedWorkspaceRecord) *protocol.ProjectPlacementPayload {
	if ws == nil {
		return nil
	}
	projectName := ws.DisplayName
	projectKey := ws.ProjectID
	var projectRoot *string
	if s.projectReg != nil {
		if project, ok := s.projectReg.Get(ws.ProjectID); ok {
			projectKey = project.ProjectID
			projectName = project.DisplayName
			if project.RootPath != "" {
				root := normalizeProjectCwd(project.RootPath)
				projectRoot = &root
			}
		}
	}
	if projectKey == "" {
		projectKey = normalizeProjectCwd(ws.Cwd)
	}
	if strings.TrimSpace(projectName) == "" {
		projectName = deriveProjectGroupingName(projectKey)
	}

	checkout := protocol.ProjectCheckoutLitePayload{
		Cwd:                 normalizeProjectCwd(ws.Cwd),
		IsGit:               ws.Kind == workspace.WorkspaceKindLocalCheckout || ws.Kind == workspace.WorkspaceKindWorktree,
		CurrentBranch:       nil,
		RemoteURL:           nil,
		WorktreeRoot:        nil,
		IsSoloOwnedWorktree: ws.Kind == workspace.WorkspaceKindWorktree,
		MainRepoRoot:        nil,
	}
	if checkout.IsGit {
		worktreeRoot := checkout.Cwd
		checkout.WorktreeRoot = &worktreeRoot
		if checkout.IsSoloOwnedWorktree {
			checkout.MainRepoRoot = projectRoot
		}
		if s.gitSvc != nil {
			meta := s.gitSvc.GetMetadataCached(checkout.Cwd)
			if meta == nil {
				meta, _ = s.gitSvc.GetMetadata(checkout.Cwd)
			}
			if meta != nil {
				checkout.CurrentBranch = meta.CurrentBranch
				checkout.RemoteURL = meta.RemoteURL
			}
		}
	}

	return &protocol.ProjectPlacementPayload{
		ProjectKey:  projectKey,
		ProjectName: projectName,
		Checkout:    checkout,
	}
}

func (s *Session) collectAgentDirectoryEntries(agents []*agent.ManagedAgent, filter *protocol.FetchAgentsFilter, defaultIncludeArchived bool) []protocol.FetchAgentsEntry {
	entries := make([]protocol.FetchAgentsEntry, 0, len(agents))
	for _, ag := range agents {
		if ag == nil {
			continue
		}
		snapshot := ag.ToSnapshot()
		project := s.projectPlacementForAgent(ag)
		if project == nil {
			continue
		}
		if !matchesFetchAgentsFilter(snapshot, *project, filter, defaultIncludeArchived) {
			continue
		}
		entries = append(entries, protocol.FetchAgentsEntry{
			Agent:   snapshot,
			Project: *project,
		})
	}
	return entries
}

func (s *Session) upsertWorkspaceForCwd(cwd string) (*protocol.WorkspaceDescriptor, bool, error) {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return nil, false, fmt.Errorf("cwd is required")
	}

	s.workspacesMu.RLock()
	existing := s.workspaces[trimmed]
	s.workspacesMu.RUnlock()
	if existing != nil {
		return existing, false, nil
	}
	if s.workspaceStore != nil {
		if existing := s.workspaceStore.Get(trimmed); existing != nil {
			s.workspacesMu.Lock()
			s.workspaces[trimmed] = existing
			s.workspacesMu.Unlock()
			return existing, false, nil
		}
	}

	var gitMeta *workspace.WorkspaceGitMetadata
	if s.gitSvc != nil {
		gitMeta, _ = s.gitSvc.GetMetadata(trimmed)
	}
	if gitMeta == nil {
		gitMeta = &workspace.WorkspaceGitMetadata{
			ProjectKind:          workspace.ProjectKindNonGit,
			ProjectDisplayName:   filepath.Base(trimmed),
			WorkspaceDisplayName: filepath.Base(trimmed),
		}
	}

	projectID := trimmed
	if gitMeta.RepoRoot != nil && *gitMeta.RepoRoot != "" {
		projectID = *gitMeta.RepoRoot
	}
	workspaceID := trimmed

	projectKind := workspace.ProjectKindNonGit
	if gitMeta.ProjectKind == workspace.ProjectKindGit {
		projectKind = workspace.ProjectKindGit
	}
	if s.projectReg != nil {
		_ = s.projectReg.UpsertProject(projectID, projectID, projectKind, gitMeta.ProjectDisplayName)
	}

	wsKind := workspace.WorkspaceKindDirectory
	if gitMeta.ProjectKind == workspace.ProjectKindGit {
		if gitMeta.IsWorktree {
			wsKind = workspace.WorkspaceKindWorktree
		} else {
			wsKind = workspace.WorkspaceKindLocalCheckout
		}
	}
	if s.workspaceReg != nil {
		_ = s.workspaceReg.UpsertWorkspace(workspaceID, projectID, trimmed, wsKind, gitMeta.WorkspaceDisplayName)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ws := &protocol.WorkspaceDescriptor{
		ID:                 workspaceID,
		ProjectID:          projectID,
		ProjectDisplayName: gitMeta.ProjectDisplayName,
		ProjectRootPath:    projectID,
		WorkspaceDirectory: trimmed,
		ProjectKind:        string(gitMeta.ProjectKind),
		WorkspaceKind:      string(wsKind),
		Name:               gitMeta.WorkspaceDisplayName,
		Status:             "done",
		ActivityAt:         &now,
	}

	if gitMeta.ProjectKind == workspace.ProjectKindGit {
		isDirty := false
		if s.gitSvc != nil {
			// GetMetadata already computed dirty and cached it; reuse that value.
			if dirtyPtr := s.gitSvc.IsDirtyCached(trimmed); dirtyPtr != nil {
				isDirty = *dirtyPtr
			} else {
				isDirty, _ = s.gitSvc.IsDirty(trimmed)
			}
		}
		ws.GitRuntime = &protocol.WorkspaceGitRuntime{
			CurrentBranch:       gitMeta.CurrentBranch,
			RemoteURL:           gitMeta.RemoteURL,
			IsSoloOwnedWorktree: &gitMeta.IsWorktree,
			IsDirty:             &isDirty,
		}
	}

	s.workspacesMu.Lock()
	if existing := s.workspaces[workspaceID]; existing != nil {
		s.workspacesMu.Unlock()
		return existing, false, nil
	}
	s.workspaces[workspaceID] = ws
	s.workspacesMu.Unlock()

	if s.workspaceStore != nil {
		if err := s.workspaceStore.Upsert(ws); err != nil {
			s.logger.Warn("failed to persist workspace", "workspaceId", workspaceID, "error", err)
		}
	}

	return ws, true, nil
}

func (s *Session) emitWorkspaceUpdate(ws *protocol.WorkspaceDescriptor) {
	if ws == nil {
		return
	}
	msg := protocol.NewSessionMessage(&protocol.WorkspaceUpdateMessage{
		Type: "workspace_update",
		Payload: protocol.WorkspaceUpdatePayload{
			Kind:      "upsert",
			Workspace: ws,
		},
	})
	if s.broadcast != nil {
		s.broadcast(msg)
	} else {
		s.sendMessage(msg)
	}
}

func (s *Session) ensureWorkspaceForCwd(cwd string) (*protocol.WorkspaceDescriptor, error) {
	ws, created, err := s.upsertWorkspaceForCwd(cwd)
	if err != nil {
		return nil, err
	}
	if created {
		s.emitWorkspaceUpdate(ws)
	}
	return ws, nil
}

func (s *Session) handleOpenProject(m *protocol.OpenProjectRequest) {
	trimmed := strings.TrimSpace(m.Cwd)
	if trimmed == "" {
		errMsg := "cwd is required"
		s.sendMessage(protocol.NewSessionMessage(&protocol.OpenProjectResponse{
			Type: "open_project_response",
			Payload: protocol.OpenProjectResponsePayload{
				RequestID: m.RequestID,
				Error:     &errMsg,
			},
		}))
		return
	}

	ws, err := s.ensureWorkspaceForCwd(trimmed)
	if err != nil {
		errMsg := err.Error()
		s.sendMessage(protocol.NewSessionMessage(&protocol.OpenProjectResponse{
			Type: "open_project_response",
			Payload: protocol.OpenProjectResponsePayload{
				RequestID: m.RequestID,
				Error:     &errMsg,
			},
		}))
		return
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.OpenProjectResponse{
		Type: "open_project_response",
		Payload: protocol.OpenProjectResponsePayload{
			RequestID: m.RequestID,
			Workspace: ws,
		},
	}))
}

func (s *Session) handleFetchWorkspaces(m *protocol.FetchWorkspacesRequest) {
	if s.workspaceStore != nil {
		for _, ws := range s.workspaceStore.GetAll() {
			if ws == nil || ws.ID == "" {
				continue
			}
			s.workspacesMu.Lock()
			s.workspaces[ws.ID] = ws
			s.workspacesMu.Unlock()
		}
	}

	// Refresh ProjectKind from project registry to pick up changes since
	// the workspace was first registered (e.g. git init, .git deletion).
	s.workspacesMu.RLock()
	wsList := make([]*protocol.WorkspaceDescriptor, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		wsList = append(wsList, ws)
	}
	s.workspacesMu.RUnlock()
	refreshWorkspaceProjectKind(wsList, s.projectReg)
	redetectNonGitWorkspaces(wsList, s.projectReg, s.gitSvc)

	s.workspacesMu.Lock()
	entries := make([]interface{}, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		if ws.GitRuntime == nil && ws.ProjectKind == string(workspace.ProjectKindGit) {
			// Use blocking GetMetadata to handle cold cache (e.g. after daemon restart).
			// GetMetadataCached returns nil for cache misses, leaving branch info empty.
			gitMeta, _ := s.gitSvc.GetMetadata(ws.WorkspaceDirectory)
			if gitMeta != nil {
				isDirty := false
				if dirtyPtr := s.gitSvc.IsDirtyCached(ws.WorkspaceDirectory); dirtyPtr != nil {
					isDirty = *dirtyPtr
				}
				ws.GitRuntime = &protocol.WorkspaceGitRuntime{
					CurrentBranch:       gitMeta.CurrentBranch,
					RemoteURL:           gitMeta.RemoteURL,
					IsSoloOwnedWorktree: &gitMeta.IsWorktree,
					IsDirty:             &isDirty,
				}
				// Fix stale Name (directory name → branch name) for legacy workspaces.
				if gitMeta.WorkspaceDisplayName != "" {
					ws.Name = gitMeta.WorkspaceDisplayName
				}
				if gitMeta.ProjectDisplayName != "" {
					ws.ProjectDisplayName = gitMeta.ProjectDisplayName
				}
			}
		}
		entries = append(entries, ws)
	}
	s.workspacesMu.Unlock()

	// Fix stale Name for git workspaces that already have GitRuntime populated
	// (the cold-cache fix above only runs when GitRuntime == nil).
	fixStaleWorkspaceNames(wsList)

	// Persist after GitRuntime is populated so branch info is saved to disk.
	if s.workspaceStore != nil {
		for _, ws := range wsList {
			_ = s.workspaceStore.Upsert(ws)
		}
	}

	// Sync corrected workspace data to the WorkspaceRegistry so
	// ~/.solo/projects/workspaces.json stays up to date.
	syncWorkspacesToRegistry(wsList, s.workspaceReg)

	s.sendMessage(protocol.NewSessionMessage(&protocol.FetchWorkspacesResponse{
		Type: "fetch_workspaces_response",
		Payload: protocol.FetchWorkspacesResponsePayload{
			RequestID: m.RequestID,
			Entries:   entries,
			PageInfo: protocol.FetchPageInfo{
				NextCursor: nil,
				PrevCursor: nil,
				HasMore:    false,
			},
		},
	}))
}

func (s *Session) handleDirectorySuggestions(m *protocol.DirectorySuggestionsRequest) {
	cwd := m.Cwd
	if cwd == "" {
		cwd = "."
	}

	includeFiles := true
	if m.IncludeFiles != nil {
		includeFiles = *m.IncludeFiles
	}
	includeDirs := true
	if m.IncludeDirectories != nil {
		includeDirs = *m.IncludeDirectories
	}
	limit := 50
	if m.Limit != nil && *m.Limit > 0 {
		limit = *m.Limit
	}

	query := strings.ToLower(m.Query)
	var entries []protocol.DirectorySuggestionEntry

	root := cwd
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if name != "." && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" || name == ".git") {
				return filepath.SkipDir
			}
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		relLower := strings.ToLower(rel)

		if d.IsDir() {
			if includeDirs && (query == "" || strings.Contains(relLower, query)) {
				entries = append(entries, protocol.DirectorySuggestionEntry{
					Path: rel,
					Kind: "directory",
				})
			}
		} else {
			if includeFiles && (query == "" || strings.Contains(relLower, query)) {
				entries = append(entries, protocol.DirectorySuggestionEntry{
					Path: rel,
					Kind: "file",
				})
			}
		}

		if len(entries) >= limit {
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		s.logger.Warn("directory suggestions walk failed", "cwd", root, "error", err)
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.DirectorySuggestionsResponse{
		Type: "directory_suggestions_response",
		Payload: protocol.DirectorySuggestionsPayload{
			Directories: []string{},
			Entries:     entries,
			RequestID:   m.RequestID,
		},
	}))
}

func (s *Session) handleCreateSoloWorktree(m *protocol.CreateSoloWorktreeRequest) {
	result, err := workspace.CreateSoloWorktree(
		m.Cwd,
		s.cfg.SoloHome,
		m.WorktreeSlug,
		m.RefName,
		m.Action,
		s.gitSvc,
		s.projectReg,
		s.workspaceReg,
	)
	if err != nil {
		errMsg := err.Error()
		s.sendMessage(protocol.NewSessionMessage(&protocol.CreateSoloWorktreeResponse{
			Type: "create_solo_worktree_response",
			Payload: protocol.CreateSoloWorktreeResponsePayload{
				Workspace:       nil,
				Error:           &errMsg,
				SetupTerminalID: nil,
				RequestID:       m.RequestID,
			},
		}))
		return
	}

	wsDesc := s.buildWorkspaceDescriptor(result.Workspace, result.Project)

	worktree := result.Worktree
	workspaceID := result.Workspace.WorkspaceID
	repoRoot := result.Project.RootPath

	go func() {
		err := workspace.RunWorktreeSetup(
			worktree.WorktreePath,
			worktree,
			workspaceID,
			repoRoot,
			func(event workspace.SetupProgressEvent) {
				s.setupProgressMu.Lock()
				s.setupProgress[workspaceID] = &event
				s.setupProgressMu.Unlock()

				detail := protocol.WorktreeSetupDetailPayload{
					Type:         "worktree_setup",
					WorktreePath: event.Worktree.WorktreePath,
					BranchName:   event.Worktree.BranchName,
					Log:          buildSetupLog(event.Commands),
					Commands:     convertCommandSnapshots(event.Commands),
				}

				s.sendMessage(protocol.NewSessionMessage(&protocol.WorkspaceSetupProgressMessage{
					Type: "workspace_setup_progress",
					Payload: protocol.WorkspaceSetupProgressPayload{
						WorkspaceID: event.WorkspaceID,
						Status:      event.Status,
						Detail:      detail,
						Error:       event.Error,
					},
				}))
			},
		)
		if err != nil {
			s.logger.Error("worktree setup failed", "workspaceId", workspaceID, "error", err)
		}
	}()

	s.sendMessage(protocol.NewSessionMessage(&protocol.CreateSoloWorktreeResponse{
		Type: "create_solo_worktree_response",
		Payload: protocol.CreateSoloWorktreeResponsePayload{
			Workspace:       wsDesc,
			Error:           nil,
			SetupTerminalID: nil,
			RequestID:       m.RequestID,
		},
	}))

	s.sendMessage(protocol.NewSessionMessage(&protocol.WorkspaceUpdateMessage{
		Type: "workspace_update",
		Payload: protocol.WorkspaceUpdatePayload{
			Kind:      "upsert",
			Workspace: wsDesc,
		},
	}))
}

func (s *Session) handleWorkspaceSetupStatus(m *protocol.WorkspaceSetupStatusRequest) {
	s.setupProgressMu.RLock()
	progress, ok := s.setupProgress[m.WorkspaceID]
	s.setupProgressMu.RUnlock()

	var snapshot *protocol.WorkspaceSetupSnapshot
	if ok {
		snapshot = &protocol.WorkspaceSetupSnapshot{
			Status: progress.Status,
			Detail: protocol.WorktreeSetupDetailPayload{
				Type:         "worktree_setup",
				WorktreePath: progress.Worktree.WorktreePath,
				BranchName:   progress.Worktree.BranchName,
				Log:          buildSetupLog(progress.Commands),
				Commands:     convertCommandSnapshots(progress.Commands),
			},
			Error: progress.Error,
		}
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.WorkspaceSetupStatusResponse{
		Type: "workspace_setup_status_response",
		Payload: protocol.WorkspaceSetupStatusResponsePayload{
			RequestID:   m.RequestID,
			WorkspaceID: m.WorkspaceID,
			Snapshot:    snapshot,
		},
	}))
}

func (s *Session) handleArchiveWorkspace(m *protocol.ArchiveWorkspaceRequest) {
	now := time.Now()
	if err := s.workspaceReg.Archive(m.WorkspaceID, now); err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "archive workspace: "+err.Error(), nil)
		return
	}

	s.workspacesMu.Lock()
	delete(s.workspaces, m.WorkspaceID)
	s.workspacesMu.Unlock()

	if s.workspaceStore != nil {
		_ = s.workspaceStore.Delete(m.WorkspaceID)
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.WorkspaceUpdateMessage{
		Type: "workspace_update",
		Payload: protocol.WorkspaceUpdatePayload{
			Kind: "remove",
			ID:   m.WorkspaceID,
		},
	}))

	archivedAt := now.UTC().Format(time.RFC3339)
	s.sendMessage(protocol.NewSessionMessage(&protocol.ArchiveWorkspaceResponse{
		Type: "archive_workspace_response",
		Payload: protocol.ArchiveWorkspaceResponsePayload{
			RequestID:   m.RequestID,
			WorkspaceID: m.WorkspaceID,
			ArchivedAt:  &archivedAt,
		},
	}))
}

func (s *Session) handleSoloWorktreeArchive(m *protocol.SoloWorktreeArchiveRequest) {
	if m.WorktreePath == "" {
		s.sendRPCError(m.RequestID, m.MsgType(), "worktreePath is required", nil)
		return
	}

	now := time.Now()
	var removedAgents []string

	if s.agentMgr != nil {
		for _, ag := range s.agentMgr.ListAgents() {
			if ag == nil {
				continue
			}
			cwd := strings.TrimSpace(ag.Cwd)
			if cwd == "" || !strings.HasPrefix(cwd, m.WorktreePath) {
				continue
			}
			// Ensure cwd is inside the worktree, not a sibling path that happens to share a prefix.
			if len(cwd) > len(m.WorktreePath) && cwd[len(m.WorktreePath)] != filepath.Separator {
				continue
			}
			if err := s.agentMgr.ArchiveAgent(ag.ID); err != nil {
				s.logger.Warn("failed to archive agent while archiving worktree", "agentId", ag.ID, "error", err)
				continue
			}
			removedAgents = append(removedAgents, ag.ID)
		}
	}

	if err := workspace.DeleteWorktree(s.cfg.SoloHome, m.WorktreePath); err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "archive worktree: "+err.Error(), nil)
		return
	}

	if s.workspaceReg != nil {
		var projectID string
		if rec, ok := s.workspaceReg.Get(m.WorktreePath); ok {
			projectID = rec.ProjectID
		}
		_ = s.workspaceReg.Archive(m.WorktreePath, now)
		if projectID != "" {
			remaining := s.workspaceReg.FindByProjectID(projectID)
			if len(remaining) == 0 && s.projectReg != nil {
				_ = s.projectReg.Archive(projectID, now)
			}
		}
	}

	s.workspacesMu.Lock()
	delete(s.workspaces, m.WorktreePath)
	s.workspacesMu.Unlock()

	if s.workspaceStore != nil {
		_ = s.workspaceStore.Delete(m.WorktreePath)
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.WorkspaceUpdateMessage{
		Type: "workspace_update",
		Payload: protocol.WorkspaceUpdatePayload{
			Kind: "remove",
			ID:   m.WorktreePath,
		},
	}))

	s.sendMessage(protocol.NewSessionMessage(&protocol.SoloWorktreeArchiveResponse{
		Type: "solo_worktree_archive_response",
		Payload: protocol.SoloWorktreeArchiveResponsePayload{
			RequestID:     m.RequestID,
			Success:       true,
			RemovedAgents: removedAgents,
		},
	}))
}

func (s *Session) handleRemoveProject(m *protocol.RemoveProjectRequest) {
	now := time.Now()
	projectIDs := make(map[string]bool)
	var removedIDs []string

	for _, wsID := range m.WorkspaceIDs {
		_ = s.workspaceReg.Archive(wsID, now)

		if rec, ok := s.workspaceReg.Get(wsID); ok {
			projectIDs[rec.ProjectID] = true
		}

		s.workspacesMu.Lock()
		delete(s.workspaces, wsID)
		s.workspacesMu.Unlock()
		if s.workspaceStore != nil {
			_ = s.workspaceStore.Delete(wsID)
		}

		s.sendMessage(protocol.NewSessionMessage(&protocol.WorkspaceUpdateMessage{
			Type: "workspace_update",
			Payload: protocol.WorkspaceUpdatePayload{
				Kind: "remove",
				ID:   wsID,
			},
		}))
		removedIDs = append(removedIDs, wsID)
	}

	for pid := range projectIDs {
		remaining := s.workspaceReg.FindByProjectID(pid)
		if len(remaining) == 0 && s.projectReg != nil {
			_ = s.projectReg.Archive(pid, now)
		}
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.RemoveProjectResponse{
		Type: "remove_project_response",
		Payload: protocol.RemoveProjectResponsePayload{
			RequestID:    m.RequestID,
			WorkspaceIDs: removedIDs,
			RemovedCount: len(removedIDs),
		},
	}))
}

func (s *Session) handleCheckoutPrStatus(_ *protocol.CheckoutPrStatusRequest) {
}

func (s *Session) handleReadProjectConfig(m *protocol.ReadProjectConfigRequest) {
	repoRoot, ok := s.resolveKnownProjectRootForConfig(m.RepoRoot)
	if !ok {
		s.sendProjectConfigReadFailure(m.RequestID, m.RepoRoot, "project_not_found")
		return
	}

	result, err := workspace.ReadRawSoloConfigForEdit(repoRoot)
	if err != nil {
		s.logger.Warn("failed to read project config", "repoRoot", repoRoot, "requestId", m.RequestID, "error", err)
		s.sendProjectConfigReadFailure(m.RequestID, repoRoot, "invalid_project_config")
		return
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.ReadProjectConfigResponse{
		Type: "read_project_config_response",
		Payload: protocol.ReadProjectConfigResponsePayload{
			RequestID: m.RequestID,
			RepoRoot:  repoRoot,
			OK:        true,
			Config:    result.Config,
			Revision:  protocolRevisionFromWorkspace(result.Revision),
		},
	}))
}

func (s *Session) handleWriteProjectConfig(m *protocol.WriteProjectConfigRequest) {
	repoRoot, ok := s.resolveKnownProjectRootForConfig(m.RepoRoot)
	if !ok {
		s.sendProjectConfigWriteFailure(m.RequestID, m.RepoRoot, &protocol.ProjectConfigRPCError{Code: "project_not_found"})
		return
	}

	result, err := workspace.WriteRawSoloConfigForEdit(repoRoot, m.Config, workspaceRevisionFromProtocol(m.ExpectedRevision))
	if err != nil {
		if stale, ok := err.(*workspace.StaleProjectConfigError); ok {
			s.sendProjectConfigWriteFailure(m.RequestID, repoRoot, &protocol.ProjectConfigRPCError{
				Code:            "stale_project_config",
				CurrentRevision: protocolRevisionFromWorkspace(stale.CurrentRevision),
			})
			return
		}
		s.logger.Warn("failed to write project config", "repoRoot", repoRoot, "requestId", m.RequestID, "error", err)
		s.sendProjectConfigWriteFailure(m.RequestID, repoRoot, &protocol.ProjectConfigRPCError{Code: "write_failed"})
		return
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.WriteProjectConfigResponse{
		Type: "write_project_config_response",
		Payload: protocol.WriteProjectConfigResponsePayload{
			RequestID: m.RequestID,
			RepoRoot:  repoRoot,
			OK:        true,
			Config:    result.Config,
			Revision:  protocolRevisionFromWorkspace(result.Revision),
		},
	}))
}

func (s *Session) sendProjectConfigReadFailure(requestID, repoRoot, code string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.ReadProjectConfigResponse{
		Type: "read_project_config_response",
		Payload: protocol.ReadProjectConfigResponsePayload{
			RequestID: requestID,
			RepoRoot:  repoRoot,
			OK:        false,
			Error:     &protocol.ProjectConfigRPCError{Code: code},
		},
	}))
}

func (s *Session) sendProjectConfigWriteFailure(requestID, repoRoot string, rpcErr *protocol.ProjectConfigRPCError) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.WriteProjectConfigResponse{
		Type: "write_project_config_response",
		Payload: protocol.WriteProjectConfigResponsePayload{
			RequestID: requestID,
			RepoRoot:  repoRoot,
			OK:        false,
			Error:     rpcErr,
		},
	}))
}

func (s *Session) resolveKnownProjectRootForConfig(repoRoot string) (string, bool) {
	requested := canonicalizeConfigRoot(repoRoot)
	if requested == "" {
		return requested, false
	}

	if s.projectReg != nil {
		for _, project := range s.projectReg.List() {
			if project.ArchivedAt != nil {
				continue
			}
			projectRoot := canonicalizeConfigRoot(project.RootPath)
			if requested == projectRoot {
				return projectRoot, true
			}
		}
	}

	s.workspacesMu.RLock()
	for _, ws := range s.workspaces {
		if workspaceRootMatchesConfigRequest(ws, requested) {
			s.workspacesMu.RUnlock()
			return requested, true
		}
	}
	s.workspacesMu.RUnlock()

	if s.workspaceStore != nil {
		for _, ws := range s.workspaceStore.GetAll() {
			if workspaceRootMatchesConfigRequest(ws, requested) {
				return requested, true
			}
		}
	}

	return requested, false
}

func (s *Session) buildWorkspaceDescriptor(ws *workspace.PersistedWorkspaceRecord, proj *workspace.PersistedProjectRecord) *protocol.WorkspaceDescriptor {
	desc := &protocol.WorkspaceDescriptor{
		ID:                 ws.WorkspaceID,
		ProjectID:          ws.ProjectID,
		ProjectDisplayName: proj.DisplayName,
		ProjectRootPath:    proj.RootPath,
		WorkspaceDirectory: ws.Cwd,
		ProjectKind:        string(proj.Kind),
		WorkspaceKind:      string(ws.Kind),
		Name:               ws.DisplayName,
		Status:             "done",
		ActivityAt:         nil,
		Scripts:            []protocol.WorkspaceScript{},
	}

	// Use blocking GetMetadata to handle cold cache (e.g. after daemon restart).
	// GetMetadataCached returns nil for cache misses, leaving GitRuntime empty and
	// Name stale (directory name instead of branch).
	gitMeta, _ := s.gitSvc.GetMetadata(ws.Cwd)
	if gitMeta != nil && gitMeta.ProjectKind == workspace.ProjectKindGit {
		desc.GitRuntime = &protocol.WorkspaceGitRuntime{
			CurrentBranch:       gitMeta.CurrentBranch,
			RemoteURL:           gitMeta.RemoteURL,
			IsSoloOwnedWorktree: &gitMeta.IsWorktree,
		}
		if dirtyPtr := s.gitSvc.IsDirtyCached(ws.Cwd); dirtyPtr != nil {
			desc.GitRuntime.IsDirty = dirtyPtr
		}
		if gitMeta.WorkspaceDisplayName != "" {
			desc.Name = gitMeta.WorkspaceDisplayName
		}
		if gitMeta.ProjectDisplayName != "" {
			desc.ProjectDisplayName = gitMeta.ProjectDisplayName
		}
	}

	s.workspacesMu.Lock()
	s.workspaces[desc.ID] = desc
	s.workspacesMu.Unlock()

	return desc
}
