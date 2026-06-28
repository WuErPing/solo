package workspace

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const maxSlugLength = 50

// WorktreeConfig is the result of a worktree creation.
type WorktreeConfig struct {
	WorktreePath string `json:"worktreePath"`
	BranchName   string `json:"branchName"`
}

// WorktreeCreationIntent describes the kind of worktree operation.
type WorktreeCreationIntent struct {
	Kind       string // "branch-off" | "checkout-branch"
	BaseBranch string // for branch-off: the base to branch from
	BranchName string // for branch-off: the new branch; for checkout: the existing branch
}

// CreateWorktreeResult is the full result of CreateSoloWorktree.
type CreateWorktreeResult struct {
	Worktree  WorktreeConfig
	Workspace *PersistedWorkspaceRecord
	Project   *PersistedProjectRecord
	Created   bool
}

// CreateSoloWorktree creates a git worktree and upserts the workspace/project registries.
func CreateSoloWorktree(
	cwd string,
	soloHome string,
	slug *string,
	refName *string,
	action *string,
	gitSvc WorkspaceGitService,
	projectReg *ProjectRegistry,
	workspaceReg *WorkspaceRegistry,
) (*CreateWorktreeResult, error) {
	// Resolve repo root
	repoRoot, err := gitSvc.ResolveRepoRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("not a git repository: %s", cwd)
	}

	// Generate slug
	effectiveSlug := generateSlug(slug, refName, action)

	// Resolve intent
	intent, err := resolveIntent(repoRoot, effectiveSlug, refName, action)
	if err != nil {
		return nil, fmt.Errorf("resolve intent: %w", err)
	}

	// Compute worktree path
	projectHash := deriveWorktreeProjectHash(repoRoot)
	worktreesRoot := filepath.Join(soloHome, "worktrees", projectHash)
	worktreePath := filepath.Join(worktreesRoot, effectiveSlug)

	// Check for existing worktree with this slug
	if existing, branch, ok := resolveExistingWorktree(worktreesRoot, effectiveSlug); ok {
		result := &CreateWorktreeResult{
			Worktree: WorktreeConfig{
				WorktreePath: existing,
				BranchName:   branch,
			},
			Created: false,
		}
		result.Workspace, result.Project, err = upsertRegistries(
			existing, branch, repoRoot, cwd, gitSvc, projectReg, workspaceReg,
		)
		if err != nil {
			return nil, fmt.Errorf("register existing worktree: %w", err)
		}
		return result, nil
	}

	// Handle path collisions
	finalPath := worktreePath
	suffix := 1
	for {
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			break
		}
		finalPath = worktreePath + "-" + strconv.Itoa(suffix)
		suffix++
	}

	// Create the worktree
	if err := executeCreateWorktree(repoRoot, finalPath, intent); err != nil {
		return nil, fmt.Errorf("git worktree add: %w", err)
	}

	result := &CreateWorktreeResult{
		Worktree: WorktreeConfig{
			WorktreePath: finalPath,
			BranchName:   intent.BranchName,
		},
		Created: true,
	}
	result.Workspace, result.Project, err = upsertRegistries(
		finalPath, intent.BranchName, repoRoot, cwd, gitSvc, projectReg, workspaceReg,
	)
	if err != nil {
		return nil, fmt.Errorf("register new worktree: %w", err)
	}
	return result, nil
}

// resolveIntent determines the worktree creation intent.
func resolveIntent(repoRoot, slug string, refName *string, action *string) (*WorktreeCreationIntent, error) {
	if action != nil && *action == "checkout" {
		// Checkout existing branch
		if refName == nil || *refName == "" {
			return nil, fmt.Errorf("checkout action requires refName")
		}
		return &WorktreeCreationIntent{
			Kind:       "checkout-branch",
			BranchName: *refName,
		}, nil
	}

	// Default: branch-off
	baseBranch := resolveDefaultBranch(repoRoot)
	if refName != nil && *refName != "" {
		baseBranch = *refName
	}

	// Resolve unique branch name
	newBranch, err := resolveUniqueBranchName(repoRoot, slug)
	if err != nil {
		return nil, err
	}

	return &WorktreeCreationIntent{
		Kind:       "branch-off",
		BaseBranch: baseBranch,
		BranchName: newBranch,
	}, nil
}

// executeCreateWorktree runs the appropriate git worktree add command.
func executeCreateWorktree(repoRoot, worktreePath string, intent *WorktreeCreationIntent) error {
	switch intent.Kind {
	case "branch-off":
		// Resolve base: try origin/<base> first, then bare <base>
		base := resolveBaseRef(repoRoot, intent.BaseBranch)
		return gitRun(repoRoot, "worktree", "add", worktreePath, "-b", intent.BranchName, base)

	case "checkout-branch":
		// Check if branch exists locally
		if !localBranchExists(repoRoot, intent.BranchName) {
			// Try to fetch from origin
			if err := gitRun(repoRoot, "fetch", "origin", intent.BranchName+":"+intent.BranchName); err != nil {
				return fmt.Errorf("unknown branch: %s", intent.BranchName)
			}
		}
		return gitRun(repoRoot, "worktree", "add", worktreePath, intent.BranchName)

	default:
		return fmt.Errorf("unsupported intent kind: %s", intent.Kind)
	}
}

// resolveBaseRef tries origin/<branch> first, then bare <branch>.
func resolveBaseRef(repoRoot, branch string) string {
	originRef := "origin/" + branch
	if err := gitRun(repoRoot, "rev-parse", "--verify", originRef); err == nil {
		return originRef
	}
	return branch
}

// resolveDefaultBranch returns the default branch name (main or master).
func resolveDefaultBranch(repoRoot string) string {
	out, err := gitOutput(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(out)
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}
	// Try to detect default branch from remote
	out, err = gitOutput(repoRoot, "symbolic-ref", "refs/remotes/origin/HEAD", "--short")
	if err == nil {
		branch := strings.TrimSpace(out)
		branch = strings.TrimPrefix(branch, "origin/")
		if branch != "" {
			return branch
		}
	}
	return "main"
}

// resolveUniqueBranchName appends -1, -2, etc. until a unique branch name is found.
func resolveUniqueBranchName(repoRoot, candidate string) (string, error) {
	name := candidate
	suffix := 1
	for localBranchExists(repoRoot, name) {
		name = candidate + "-" + strconv.Itoa(suffix)
		suffix++
		if suffix > 1000 {
			return "", fmt.Errorf("could not find unique branch name after 1000 attempts")
		}
	}
	return name, nil
}

// localBranchExists checks if a local branch exists.
func localBranchExists(repoRoot, branch string) bool {
	err := gitRun(repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// resolveExistingWorktree checks if a worktree already exists for the given slug.
func resolveExistingWorktree(worktreesRoot, slug string) (path string, branch string, found bool) {
	candidatePath := filepath.Join(worktreesRoot, slug)
	if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
		return "", "", false
	}
	// Check if it's a valid git worktree
	branchOut, err := gitOutput(candidatePath, "branch", "--show-current")
	if err != nil {
		return "", "", false
	}
	return candidatePath, strings.TrimSpace(branchOut), true
}

// generateSlug produces a URL-safe slug from the input.
func generateSlug(slug *string, refName *string, _ *string) string {
	var seed string
	if slug != nil && *slug != "" {
		seed = *slug
	} else if refName != nil && *refName != "" {
		seed = *refName
	} else {
		seed = uuid.New().String()
	}
	return Slugify(seed)
}

// Slugify normalizes a string into a URL-safe slug.
func Slugify(input string) string {
	slug := strings.ToLower(input)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug = re.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) <= maxSlugLength {
		return slug
	}
	truncated := slug[:maxSlugLength]
	lastHyphen := strings.LastIndex(truncated, "-")
	if lastHyphen > maxSlugLength/2 {
		return truncated[:lastHyphen]
	}
	return strings.TrimRight(truncated, "-")
}

// deriveWorktreeProjectHash computes a short hash from the repo root path.
func deriveWorktreeProjectHash(repoRoot string) string {
	h := sha256.Sum256([]byte(repoRoot))
	// Use first 8 bytes, encode as base-36
	val := uint64(0)
	for i := 0; i < 8; i++ {
		val = val<<8 | uint64(h[i])
	}
	return strconv.FormatUint(val, 36)
}

// upsertRegistries creates or updates project and workspace records for a worktree.
func upsertRegistries(
	worktreePath string,
	branchName string,
	repoRoot string,
	inputCwd string,
	gitSvc WorkspaceGitService,
	projectReg *ProjectRegistry,
	workspaceReg *WorkspaceRegistry,
) (*PersistedWorkspaceRecord, *PersistedProjectRecord, error) {
	// Resolve source project
	meta, _ := gitSvc.GetMetadata(inputCwd)
	displayName := filepath.Base(repoRoot)
	if meta != nil && meta.ProjectDisplayName != "" {
		displayName = meta.ProjectDisplayName
	}

	// Find or create project
	projectID := worktreePath // use path as ID (same as Solo)
	if existing, ok := projectReg.FindByRootPath(repoRoot); ok {
		projectID = existing.ProjectID
	}

	if err := projectReg.UpsertProject(projectID, repoRoot, ProjectKindGit, displayName); err != nil {
		return nil, nil, fmt.Errorf("upsert project: %w", err)
	}

	// Create workspace record
	workspaceID := worktreePath
	wsDisplayName := branchName
	if wsDisplayName == "" {
		wsDisplayName = filepath.Base(worktreePath)
	}

	if err := workspaceReg.UpsertWorkspace(workspaceID, projectID, worktreePath, WorkspaceKindWorktree, wsDisplayName); err != nil {
		return nil, nil, fmt.Errorf("upsert workspace: %w", err)
	}

	// Fetch the persisted records to return
	ws, _ := workspaceReg.Get(workspaceID)
	proj, _ := projectReg.Get(projectID)
	return ws, proj, nil
}

// gitRun executes a git command in the given directory via gitCmd.
func gitRun(dir string, args ...string) error {
	return getGitCmd().Run(dir, args...)
}

// gitOutput executes a git command and returns its stdout via gitCmd.
func gitOutput(dir string, args ...string) (string, error) {
	return getGitCmd().Output(dir, args...)
}

// DeleteWorktree removes a git worktree.
func DeleteWorktree(soloHome, worktreePath string) error {
	if soloHome == "" || worktreePath == "" {
		return fmt.Errorf("soloHome and worktreePath are required")
	}

	// Safety: only delete paths that are direct children of soloHome/worktrees/<projectHash>.
	worktreesRoot := filepath.Join(soloHome, "worktrees")
	rel, err := filepath.Rel(worktreesRoot, worktreePath)
	if err != nil || rel == worktreePath || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return fmt.Errorf("refusing to delete worktree outside managed directory: %s", worktreePath)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 || parts[0] == "" || parts[0] == "." || parts[1] == "" || parts[1] == "." {
		return fmt.Errorf("refusing to delete worktree outside managed directory: %s", worktreePath)
	}

	// Resolve the main repo root from the worktree so git commands run in the right place.
	repoRoot := strings.TrimSpace(gitOutputOrEmpty(worktreePath, "rev-parse", "--show-toplevel"))

	if repoRoot == "" {
		// Worktree is not usable as a git checkout; just remove the directory.
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("remove worktree directory: %w", err)
		}
		return nil
	}

	// Try git worktree remove first, then fall back to a plain directory removal.
	if err := gitRun(repoRoot, "worktree", "remove", worktreePath, "--force"); err != nil {
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("remove worktree directory: %w", err)
		}
	}

	// Prune stale worktree references; failures here are non-fatal.
	_ = gitRun(repoRoot, "worktree", "prune")
	return nil
}

// gitOutputOrEmpty runs a git command and returns stdout, or an empty string on error.
func gitOutputOrEmpty(dir string, args ...string) string {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return ""
	}
	return out
}
