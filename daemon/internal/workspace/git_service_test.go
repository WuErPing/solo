package workspace

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ---- WorkspaceGitService (gitServiceImpl) ----

func newGitSvcWithFake(t *testing.T, handler func(dir string, args []string) (string, error)) WorkspaceGitService {
	t.Helper()
	fake := newFakeGit(handler)
	installFake(t, fake)
	return NewWorkspaceGitService()
}

func TestResolveRepoRootSuccess(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--show-toplevel") {
			return "/home/user/project\n", nil
		}
		return "", nil
	})
	got, err := svc.ResolveRepoRoot("/home/user/project/subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/user/project" {
		t.Errorf("got %q, want /home/user/project", got)
	}
}

func TestResolveRepoRootNotGit(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		return "", errors.New("not a git repo")
	})
	got, err := svc.ResolveRepoRoot("/not/a/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for non-git dir, got %q", got)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--abbrev-ref") {
			return "feature-xyz\n", nil
		}
		return "", nil
	})
	got, err := svc.GetCurrentBranch("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "feature-xyz" {
		t.Errorf("got %q, want feature-xyz", got)
	}
}

func TestGetCurrentBranchError(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		return "", errors.New("git error")
	})
	_, err := svc.GetCurrentBranch("/repo")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetRemoteUrl(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "remote" && contains(args, "get-url") {
			return "https://github.com/owner/repo.git\n", nil
		}
		return "", nil
	})
	got, err := svc.GetRemoteUrl("/repo", "origin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://github.com/owner/repo.git" {
		t.Errorf("got %q", got)
	}
}

func TestGetRemoteUrlDefaultsToOrigin(t *testing.T) {
	var capturedArgs []string
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		capturedArgs = args
		return "https://github.com/o/r.git\n", nil
	})
	_, _ = svc.GetRemoteUrl("/repo", "")
	if !contains(capturedArgs, "origin") {
		t.Errorf("expected origin in args, got %v", capturedArgs)
	}
}

func TestIsWorktreeMainRepo(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--git-dir") {
			return ".git\n", nil
		}
		return "", nil
	})
	got, err := svc.IsWorktree("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("main repo should not be identified as worktree")
	}
}

func TestIsWorktreeLinkedWorktree(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--git-dir") {
			return ".git/worktrees/my-branch\n", nil
		}
		return "", nil
	})
	got, err := svc.IsWorktree("/repo/worktrees/my-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("linked worktree should be identified as worktree")
	}
}

func TestIsDirtyClean(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "status" {
			return "", nil // empty output = clean
		}
		return "", nil
	})
	got, err := svc.IsDirty("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected clean repo")
	}
}

func TestIsDirtyDirty(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		if args[0] == "status" {
			return " M README.md\n", nil
		}
		return "", nil
	})
	got, err := svc.IsDirty("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected dirty repo")
	}
}

func TestGetMetadataGitRepo(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--show-toplevel") {
				return "/home/user/myproject\n", nil
			}
			if contains(args, "--abbrev-ref") {
				return "main\n", nil
			}
			if contains(args, "--git-dir") {
				return ".git\n", nil
			}
		case "remote":
			return "https://github.com/owner/myproject.git\n", nil
		case "status":
			return "", nil
		}
		return "", nil
	})
	meta, err := svc.GetMetadata("/home/user/myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.ProjectKind != ProjectKindGit {
		t.Errorf("ProjectKind = %q, want git", meta.ProjectKind)
	}
	if meta.CurrentBranch == nil || *meta.CurrentBranch != "main" {
		t.Errorf("CurrentBranch = %v, want main", meta.CurrentBranch)
	}
	if meta.ProjectSlug != "myproject" {
		t.Errorf("ProjectSlug = %q, want myproject", meta.ProjectSlug)
	}
}

func TestGetMetadataNonGitDir(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		return "", errors.New("not a git repo")
	})
	meta, err := svc.GetMetadata("/some/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.ProjectKind != ProjectKindNonGit {
		t.Errorf("ProjectKind = %q, want non_git", meta.ProjectKind)
	}
}

func TestGetMetadataCacheHit(t *testing.T) {
	callCount := 0
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		callCount++
		switch args[0] {
		case "rev-parse":
			if contains(args, "--show-toplevel") {
				return "/repo\n", nil
			}
			if contains(args, "--abbrev-ref") {
				return "main\n", nil
			}
			if contains(args, "--git-dir") {
				return ".git\n", nil
			}
		case "remote":
			return "", errors.New("no remote")
		case "status":
			return "", nil
		}
		return "", nil
	})

	_, _ = svc.GetMetadata("/repo")
	firstCount := callCount

	// Second call should hit cache
	_, _ = svc.GetMetadata("/repo")
	if callCount != firstCount {
		t.Errorf("expected cache hit on second call, but git was called again (count %d→%d)", firstCount, callCount)
	}
}

func TestGetMetadataCachedReturnsNilBeforePopulated(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		// slow git — return empty to avoid blocking
		return "", errors.New("noop")
	})
	got := svc.GetMetadataCached("/brand-new-dir")
	if got != nil {
		t.Error("expected nil for unpopulated cache")
	}
}

func TestIsDirtyCachedReturnsNilBeforePopulated(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		return "", errors.New("noop")
	})
	got := svc.IsDirtyCached("/brand-new-dir")
	if got != nil {
		t.Error("expected nil for unpopulated dirty cache")
	}
}

func TestIsDirtyCachedReturnsValueAfterGetMetadata(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--show-toplevel") {
				return "/repo\n", nil
			}
			if contains(args, "--abbrev-ref") {
				return "main\n", nil
			}
			if contains(args, "--git-dir") {
				return ".git\n", nil
			}
		case "remote":
			return "", errors.New("no remote")
		case "status":
			return " M file.go\n", nil
		}
		return "", nil
	})
	_, _ = svc.GetMetadata("/repo")
	got := svc.IsDirtyCached("/repo")
	if got == nil {
		t.Fatal("expected non-nil dirty flag after GetMetadata")
	}
	if !*got {
		t.Error("expected dirty=true")
	}
}

func TestBackgroundRefreshStartStop(t *testing.T) {
	svc := newGitSvcWithFake(t, func(dir string, args []string) (string, error) {
		return "", errors.New("noop")
	})
	ctx := context.Background()
	svc.StartBackgroundRefresh(ctx, 10*time.Millisecond)
	// Starting again is a no-op
	svc.StartBackgroundRefresh(ctx, 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	svc.StopBackgroundRefresh()
	// Should not panic or deadlock
}

// ---- deriveProjectSlug ----

func TestDeriveProjectSlugFromGitHubURL(t *testing.T) {
	url := "https://github.com/owner/myrepo.git"
	got := deriveProjectSlug(&url, "fallback")
	if got != "myrepo" {
		t.Errorf("got %q, want myrepo", got)
	}
}

func TestDeriveProjectSlugFallbackToDisplayName(t *testing.T) {
	got := deriveProjectSlug(nil, "My Project")
	if got != "my-project" {
		t.Errorf("got %q, want my-project", got)
	}
}
