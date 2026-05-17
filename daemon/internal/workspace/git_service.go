package workspace

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WorkspaceGitService provides git metadata for workspaces.
type WorkspaceGitService interface {
	// ResolveRepoRoot resolves the git repo root for the given cwd.
	// Returns empty string if not in a git repo.
	ResolveRepoRoot(cwd string) (string, error)

	// GetMetadata returns full git metadata for the given cwd (blocking, with cache).
	GetMetadata(cwd string) (*WorkspaceGitMetadata, error)

	// GetMetadataCached returns cached metadata without blocking.
	// Returns nil if not in cache or expired; triggers async refresh.
	GetMetadataCached(cwd string) *WorkspaceGitMetadata

	// GetCurrentBranch returns the current git branch name.
	GetCurrentBranch(cwd string) (string, error)

	// GetRemoteUrl returns the remote URL for the given remote name (default "origin").
	GetRemoteUrl(cwd string, remote string) (string, error)

	// IsWorktree returns true if cwd is a git worktree (not the main repo).
	IsWorktree(cwd string) (bool, error)

	// IsDirty returns true if the working tree has uncommitted changes (blocking, with cache).
	IsDirty(cwd string) (bool, error)

	// IsDirtyCached returns cached dirty flag without blocking.
	// Returns nil if not in cache or expired; triggers async refresh.
	IsDirtyCached(cwd string) *bool

	// StartBackgroundRefresh starts a background goroutine that periodically
	// refreshes all cached entries.
	StartBackgroundRefresh(ctx context.Context, interval time.Duration)

	// StopBackgroundRefresh stops the background refresh goroutine.
	StopBackgroundRefresh()
}

// gitServiceImpl implements WorkspaceGitService using git CLI calls with caching.
type gitServiceImpl struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntry
	ttl   time.Duration

	// Background refresh
	refreshMu     sync.Mutex
	refreshCancel context.CancelFunc
	refreshWg     sync.WaitGroup
}

type cacheEntry struct {
	metadata  *WorkspaceGitMetadata
	dirty     *bool
	timestamp time.Time
}

// NewWorkspaceGitService creates a new WorkspaceGitService with 15s TTL cache.
func NewWorkspaceGitService() WorkspaceGitService {
	return &gitServiceImpl{
		cache: make(map[string]*cacheEntry),
		ttl:   15 * time.Second,
	}
}

func (s *gitServiceImpl) getCached(cwd string) *cacheEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[cwd]
	if !ok {
		return nil
	}
	if time.Since(entry.timestamp) > s.ttl {
		return nil
	}
	return entry
}

func (s *gitServiceImpl) setCache(cwd string, meta *WorkspaceGitMetadata, dirty *bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[cwd] = &cacheEntry{
		metadata:  meta,
		dirty:     dirty,
		timestamp: time.Now(),
	}
}

func (s *gitServiceImpl) ResolveRepoRoot(cwd string) (string, error) {
	out, err := gitExec(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil // not a git repo
	}
	return strings.TrimSpace(out), nil
}

func (s *gitServiceImpl) GetMetadata(cwd string) (*WorkspaceGitMetadata, error) {
	if entry := s.getCached(cwd); entry != nil && entry.metadata != nil {
		return entry.metadata, nil
	}

	meta := &WorkspaceGitMetadata{
		ProjectKind:          ProjectKindNonGit,
		ProjectDisplayName:   filepath.Base(cwd),
		WorkspaceDisplayName: filepath.Base(cwd),
	}

	repoRoot, err := s.ResolveRepoRoot(cwd)
	if err != nil {
		s.setCache(cwd, meta, nil)
		return meta, nil
	}
	if repoRoot == "" {
		s.setCache(cwd, meta, nil)
		return meta, nil
	}

	meta.ProjectKind = ProjectKindGit
	meta.RepoRoot = &repoRoot
	meta.ProjectDisplayName = filepath.Base(repoRoot)

	// Get current branch
	branch, err := s.GetCurrentBranch(cwd)
	if err == nil && branch != "" {
		meta.CurrentBranch = &branch
		meta.WorkspaceDisplayName = branch
	}

	// Get remote URL
	remoteUrl, err := s.GetRemoteUrl(cwd, "origin")
	if err == nil && remoteUrl != "" {
		meta.RemoteUrl = &remoteUrl
		meta.GitRemote = &remoteUrl
	}

	// Check if worktree
	isWorktree, _ := s.IsWorktree(cwd)
	meta.IsWorktree = isWorktree

	// Derive project slug from remote URL or directory name
	meta.ProjectSlug = deriveProjectSlug(meta.RemoteUrl, meta.ProjectDisplayName)

	// Compute dirty flag alongside metadata.
	// Reuse the cached dirty value if available to avoid an extra subprocess.
	var dirtyPtr *bool
	if prev := s.getCached(cwd); prev != nil && prev.dirty != nil {
		dirtyPtr = prev.dirty
	} else {
		dirty, _ := s.IsDirty(cwd)
		dirtyPtr = &dirty
	}
	s.setCache(cwd, meta, dirtyPtr)
	return meta, nil
}

func (s *gitServiceImpl) GetMetadataCached(cwd string) *WorkspaceGitMetadata {
	if entry := s.getCached(cwd); entry != nil && entry.metadata != nil {
		return entry.metadata
	}
	// Trigger async refresh without blocking caller
	go s.refreshAsync(cwd)
	return nil
}

func (s *gitServiceImpl) refreshAsync(cwd string) {
	_, _ = s.GetMetadata(cwd)
}

func (s *gitServiceImpl) GetCurrentBranch(cwd string) (string, error) {
	out, err := gitExec(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *gitServiceImpl) GetRemoteUrl(cwd string, remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	out, err := gitExec(cwd, "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("get remote url: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *gitServiceImpl) IsWorktree(cwd string) (bool, error) {
	// git rev-parse --git-dir returns ".git" for main repo, or a path for worktrees
	gitDir, err := gitExec(cwd, "rev-parse", "--git-dir")
	if err != nil {
		return false, nil
	}
	gitDir = strings.TrimSpace(gitDir)
	// If it's a relative path that's not ".git", it's likely a worktree
	if gitDir != ".git" && !filepath.IsAbs(gitDir) {
		return true, nil
	}
	// For absolute paths, check if it's inside .git/worktrees/
	if strings.Contains(gitDir, "/worktrees/") || strings.Contains(gitDir, "\\worktrees\\") {
		return true, nil
	}
	return false, nil
}

func (s *gitServiceImpl) IsDirty(cwd string) (bool, error) {
	out, err := gitExec(cwd, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func (s *gitServiceImpl) IsDirtyCached(cwd string) *bool {
	entry := s.getCached(cwd)
	if entry != nil && entry.dirty != nil {
		return entry.dirty
	}
	go s.refreshAsync(cwd)
	return nil
}

func (s *gitServiceImpl) StartBackgroundRefresh(ctx context.Context, interval time.Duration) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if s.refreshCancel != nil {
		return // already started
	}

	ctx, cancel := context.WithCancel(ctx)
	s.refreshCancel = cancel
	s.refreshWg.Add(1)

	go func() {
		defer s.refreshWg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refreshAll(ctx)
			}
		}
	}()
}

func (s *gitServiceImpl) StopBackgroundRefresh() {
	s.refreshMu.Lock()
	cancel := s.refreshCancel
	s.refreshCancel = nil
	s.refreshMu.Unlock()

	if cancel != nil {
		cancel()
		s.refreshWg.Wait()
	}
}

func (s *gitServiceImpl) refreshAll(ctx context.Context) {
	s.mu.RLock()
	dirs := make([]string, 0, len(s.cache))
	for dir := range s.cache {
		dirs = append(dirs, dir)
	}
	s.mu.RUnlock()

	for _, dir := range dirs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, _ = s.GetMetadata(dir)
	}
}

// deriveProjectSlug derives a URL-safe slug from the remote URL or directory name.
func deriveProjectSlug(remoteUrl *string, displayName string) string {
	if remoteUrl != nil {
		// Extract repo name from GitHub URL: https://github.com/owner/repo.git -> repo
		url := *remoteUrl
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			slug := parts[len(parts)-1]
			if slug != "" {
				return strings.ToLower(slug)
			}
		}
	}
	// Fallback to display name, lowercased
	return strings.ToLower(strings.ReplaceAll(displayName, " ", "-"))
}

// gitExec runs a git command in the given directory and returns its stdout.
func gitExec(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}
