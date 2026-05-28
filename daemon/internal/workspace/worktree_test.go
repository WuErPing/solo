package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ---- fake GitCommander ----

type fakeGitCommander struct {
	mu      sync.Mutex
	wg      sync.WaitGroup // tracks all in-flight calls; drained by installFake cleanup
	calls   []fakeGitCall
	handler func(dir string, args []string) (string, error)
}

type fakeGitCall struct {
	Dir  string
	Args []string
}

func newFakeGit(handler func(dir string, args []string) (string, error)) *fakeGitCommander {
	return &fakeGitCommander{handler: handler}
}

func (f *fakeGitCommander) Run(dir string, args ...string) error {
	_, err := f.recordAndDispatch(dir, args)
	return err
}

func (f *fakeGitCommander) Output(dir string, args ...string) (string, error) {
	return f.recordAndDispatch(dir, args)
}

func (f *fakeGitCommander) recordAndDispatch(dir string, args []string) (string, error) {
	f.wg.Add(1)
	defer f.wg.Done()
	f.mu.Lock()
	f.calls = append(f.calls, fakeGitCall{Dir: dir, Args: append([]string{}, args...)})
	f.mu.Unlock()
	if f.handler != nil {
		return f.handler(dir, args)
	}
	return "", nil
}

func (f *fakeGitCommander) CalledWith(sub string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		for _, a := range c.Args {
			if a == sub {
				return true
			}
		}
	}
	return false
}

// installFake replaces the package-level gitCmd for the duration of the test.
// Cleanup swaps the original back first (so new goroutines get the real commander),
// then drains any in-flight calls still running against the fake.
func installFake(t *testing.T, fake *fakeGitCommander) {
	t.Helper()
	orig := getGitCmd()
	setGitCmd(fake)
	t.Cleanup(func() {
		setGitCmd(orig)  // stop routing new calls to fake
		fake.wg.Wait()   // drain any goroutines already inside recordAndDispatch
	})
}

// ---- helpers ----

func newTmpRegistries(t *testing.T) (*ProjectRegistry, *WorkspaceRegistry) {
	t.Helper()
	dir := t.TempDir()
	return NewProjectRegistry(dir), NewWorkspaceRegistry(dir)
}

// ---- Slugify tests ----

func TestSlugifyBasic(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"foo/bar baz", "foo-bar-baz"},
		{"  leading-trailing  ", "leading-trailing"},
		{"ALL_CAPS", "all-caps"},
		{"already-slug", "already-slug"},
		{"a!@#$%^&*()b", "a-b"},
	}
	for _, c := range cases {
		got := Slugify(c.in)
		if got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugifyTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := Slugify(long)
	if len(got) > maxSlugLength {
		t.Errorf("Slugify long input: len=%d > maxSlugLength=%d", len(got), maxSlugLength)
	}
}

func TestSlugifyTruncationAtWordBoundary(t *testing.T) {
	// Build a string that after slugify is slightly over maxSlugLength and has a hyphen
	// to split at. e.g. "aaa...aaa-tail" where total > 50
	base := strings.Repeat("a", 45) + "-tail"
	got := Slugify(base)
	// Should cut at the hyphen before the tail if that's past the midpoint
	if strings.HasSuffix(got, "-") {
		t.Errorf("Slugify should not end with hyphen, got %q", got)
	}
	if len(got) > maxSlugLength {
		t.Errorf("len=%d > %d", len(got), maxSlugLength)
	}
}

// ---- deriveWorktreeProjectHash ----

func TestDeriveWorktreeProjectHashDeterministic(t *testing.T) {
	a := deriveWorktreeProjectHash("/home/user/project")
	b := deriveWorktreeProjectHash("/home/user/project")
	if a != b {
		t.Errorf("hash not deterministic: %q vs %q", a, b)
	}
}

func TestDeriveWorktreeProjectHashDiffers(t *testing.T) {
	a := deriveWorktreeProjectHash("/home/user/project-a")
	b := deriveWorktreeProjectHash("/home/user/project-b")
	if a == b {
		t.Errorf("different paths produced same hash %q", a)
	}
}

// ---- generateSlug ----

func TestGenerateSlugFromSlugParam(t *testing.T) {
	s := "my-feature"
	got := generateSlug(&s, nil, nil)
	if got != "my-feature" {
		t.Errorf("got %q, want %q", got, "my-feature")
	}
}

func TestGenerateSlugFromRefName(t *testing.T) {
	ref := "feature/awesome"
	got := generateSlug(nil, &ref, nil)
	if got != "feature-awesome" {
		t.Errorf("got %q, want %q", got, "feature-awesome")
	}
}

func TestGenerateSlugFallbackToUUID(t *testing.T) {
	got := generateSlug(nil, nil, nil)
	if got == "" {
		t.Error("expected non-empty slug from UUID fallback")
	}
}

// ---- resolveIntent ----

func TestResolveIntentCheckout(t *testing.T) {
	ref := "existing-branch"
	action := "checkout"
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		return "", nil
	})
	installFake(t, fake)

	intent, err := resolveIntent("/repo", "my-slug", &ref, &action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Kind != "checkout-branch" {
		t.Errorf("Kind = %q, want checkout-branch", intent.Kind)
	}
	if intent.BranchName != "existing-branch" {
		t.Errorf("BranchName = %q, want existing-branch", intent.BranchName)
	}
}

func TestResolveIntentCheckoutRequiresRefName(t *testing.T) {
	action := "checkout"
	fake := newFakeGit(nil)
	installFake(t, fake)

	_, err := resolveIntent("/repo", "slug", nil, &action)
	if err == nil {
		t.Error("expected error when checkout action has no refName")
	}
}

func TestResolveIntentBranchOff(t *testing.T) {
	// HEAD -> "main", show-ref fails (branch doesn't exist yet)
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--abbrev-ref") {
				return "main\n", nil
			}
			// show-ref --verify -> branch does not exist
			return "", errors.New("not found")
		case "show-ref":
			return "", errors.New("no such ref")
		}
		return "", nil
	})
	installFake(t, fake)

	intent, err := resolveIntent("/repo", "my-slug", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Kind != "branch-off" {
		t.Errorf("Kind = %q, want branch-off", intent.Kind)
	}
	if intent.BranchName != "my-slug" {
		t.Errorf("BranchName = %q, want my-slug", intent.BranchName)
	}
}

func TestResolveIntentBranchOffDeduplicates(t *testing.T) {
	// "my-slug" exists, "my-slug-1" does not
	callCount := 0
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--abbrev-ref") {
				return "main\n", nil
			}
			return "", errors.New("not found")
		case "show-ref":
			callCount++
			if callCount == 1 {
				// First call: "my-slug" exists
				return "abc123 refs/heads/my-slug\n", nil
			}
			// Second call: "my-slug-1" does not exist
			return "", errors.New("no ref")
		}
		return "", nil
	})
	installFake(t, fake)

	intent, err := resolveIntent("/repo", "my-slug", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.BranchName != "my-slug-1" {
		t.Errorf("BranchName = %q, want my-slug-1", intent.BranchName)
	}
}

// ---- resolveDefaultBranch ----

func TestResolveDefaultBranchFromHEAD(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--abbrev-ref") {
			return "develop\n", nil
		}
		return "", errors.New("unused")
	})
	installFake(t, fake)

	got := resolveDefaultBranch("/repo")
	if got != "develop" {
		t.Errorf("got %q, want develop", got)
	}
}

func TestResolveDefaultBranchFromOriginHEAD(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--abbrev-ref") {
			// HEAD is detached
			return "HEAD\n", nil
		}
		if args[0] == "symbolic-ref" {
			return "origin/main\n", nil
		}
		return "", errors.New("unused")
	})
	installFake(t, fake)

	got := resolveDefaultBranch("/repo")
	if got != "main" {
		t.Errorf("got %q, want main", got)
	}
}

func TestResolveDefaultBranchFallback(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		return "", errors.New("fail")
	})
	installFake(t, fake)

	got := resolveDefaultBranch("/repo")
	if got != "main" {
		t.Errorf("got %q, want main", got)
	}
}

// ---- CreateSoloWorktree (integration path via fake) ----

func TestCreateSoloWorktreeNewBranch(t *testing.T) {
	soloHome := t.TempDir()
	projectReg, workspaceReg := newTmpRegistries(t)

	const repoRoot = "/fake/repo"

	fake := newFakeGit(func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--show-toplevel") {
				return repoRoot + "\n", nil
			}
			if contains(args, "--abbrev-ref") {
				return "main\n", nil
			}
			// origin/main verify -> ok
			return "", nil
		case "show-ref":
			// branch does not exist yet
			return "", errors.New("no ref")
		case "worktree":
			if args[1] == "add" {
				// Create the directory so os.Stat finds it
				if err := os.MkdirAll(args[2], 0o755); err != nil {
					return "", err
				}
				// Write a fake HEAD so resolveExistingWorktree works
				gitDir := filepath.Join(args[2], ".git")
				_ = os.MkdirAll(gitDir, 0o755)
				return "", nil
			}
			// branch --show-current inside resolveExistingWorktree
			if args[1] == "--show-current" {
				return "my-branch\n", nil
			}
		case "branch":
			return "my-branch\n", nil
		case "remote":
			return "", errors.New("no remote")
		case "status":
			return "", nil
		}
		return "", nil
	})
	installFake(t, fake)

	gitSvc := NewWorkspaceGitService()
	slug := "my-branch"
	result, err := CreateSoloWorktree(
		repoRoot, soloHome, &slug, nil, nil,
		gitSvc, projectReg, workspaceReg,
	)
	if err != nil {
		t.Fatalf("CreateSoloWorktree error: %v", err)
	}
	if !result.Created {
		t.Error("expected Created=true for new worktree")
	}
	if result.Worktree.BranchName == "" {
		t.Error("expected non-empty BranchName")
	}
	if !strings.Contains(result.Worktree.WorktreePath, "my-branch") {
		t.Errorf("WorktreePath %q should contain slug", result.Worktree.WorktreePath)
	}
}

func TestCreateSoloWorktreeReturnsExisting(t *testing.T) {
	soloHome := t.TempDir()
	projectReg, workspaceReg := newTmpRegistries(t)

	const repoRoot = "/fake/repo"

	// Pre-create the worktree directory under the expected path.
	slug := "existing-branch"
	hash := deriveWorktreeProjectHash(repoRoot)
	worktreesRoot := filepath.Join(soloHome, "worktrees", hash)
	existingPath := filepath.Join(worktreesRoot, slug)
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	fake := newFakeGit(func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--show-toplevel") {
				return repoRoot + "\n", nil
			}
			if contains(args, "--abbrev-ref") {
				// resolveDefaultBranch: current HEAD branch
				return "main\n", nil
			}
			// origin/main verify for resolveBaseRef — ok
			return "", nil
		case "show-ref":
			// localBranchExists: the slug branch does NOT exist yet as a
			// local branch (the worktree already exists on disk, but that is
			// checked separately via os.Stat).
			return "", errors.New("no such ref")
		case "branch":
			// resolveExistingWorktree: branch --show-current
			return "existing-branch\n", nil
		case "remote":
			return "", errors.New("no remote")
		case "status":
			return "", nil
		}
		return "", nil
	})
	installFake(t, fake)

	gitSvc := NewWorkspaceGitService()
	result, err := CreateSoloWorktree(
		repoRoot, soloHome, &slug, nil, nil,
		gitSvc, projectReg, workspaceReg,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Created {
		t.Error("expected Created=false for existing worktree")
	}
	if result.Worktree.WorktreePath != existingPath {
		t.Errorf("path = %q, want %q", result.Worktree.WorktreePath, existingPath)
	}
}

func TestCreateSoloWorktreeNotGitRepo(t *testing.T) {
	soloHome := t.TempDir()
	projectReg, workspaceReg := newTmpRegistries(t)

	fake := newFakeGit(func(dir string, args []string) (string, error) {
		if args[0] == "rev-parse" && contains(args, "--show-toplevel") {
			return "", errors.New("not a git repo")
		}
		return "", nil
	})
	installFake(t, fake)

	gitSvc := NewWorkspaceGitService()
	_, err := CreateSoloWorktree(
		"/not/a/repo", soloHome, nil, nil, nil,
		gitSvc, projectReg, workspaceReg,
	)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestCreateSoloWorktreeCheckoutAction(t *testing.T) {
	soloHome := t.TempDir()
	projectReg, workspaceReg := newTmpRegistries(t)
	const repoRoot = "/fake/repo"

	fake := newFakeGit(func(dir string, args []string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if contains(args, "--show-toplevel") {
				return repoRoot + "\n", nil
			}
		case "show-ref":
			// local branch exists
			return fmt.Sprintf("abc123 refs/heads/target-branch\n"), nil
		case "worktree":
			if args[1] == "add" {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			if args[1] == "--show-current" {
				return "target-branch\n", nil
			}
		case "branch":
			return "target-branch\n", nil
		case "remote", "status":
			return "", nil
		}
		return "", nil
	})
	installFake(t, fake)

	gitSvc := NewWorkspaceGitService()
	ref := "target-branch"
	action := "checkout"
	result, err := CreateSoloWorktree(
		repoRoot, soloHome, nil, &ref, &action,
		gitSvc, projectReg, workspaceReg,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Worktree.BranchName != "target-branch" {
		t.Errorf("BranchName = %q, want target-branch", result.Worktree.BranchName)
	}
}

// ---- DeleteWorktree ----

func TestDeleteWorktreeRejectsOutsideManagedDir(t *testing.T) {
	soloHome := t.TempDir()
	err := DeleteWorktree(soloHome, "/etc/passwd")
	if err == nil {
		t.Error("expected error for path outside managed directory")
	}
}

// ---- helpers ----

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
