package workspace

import (
	"errors"
	"strings"
	"testing"
)

func forEachRefFakeOutput() string {
	return strings.Join([]string{
		"refs/heads/main\t1700000100",
		"refs/heads/dev\t1700000200",
		"refs/heads/feature/login\t1700000300",
		"refs/remotes/origin/main\t1700000150",
		"refs/remotes/origin/HEAD\t1700000150",
		"refs/remotes/origin/dev\t1700000250",
		"",
	}, "\n")
}

func TestListBranchSuggestions_MergesLocalAndRemote(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		return forEachRefFakeOutput(), nil
	})
	installFake(t, fake)

	got, err := ListBranchSuggestions("/repo", "", 50)
	if err != nil {
		t.Fatalf("ListBranchSuggestions: %v", err)
	}

	byName := make(map[string]BranchSuggestion)
	for _, b := range got {
		byName[b.Name] = b
	}

	// HEAD symref must be filtered out.
	if _, ok := byName["HEAD"]; ok {
		t.Error("HEAD symref should be filtered out")
	}

	main := byName["main"]
	if !main.HasLocal || !main.HasRemote {
		t.Errorf("main: HasLocal=%v HasRemote=%v, want both true", main.HasLocal, main.HasRemote)
	}
	// Deduped entry keeps the newest committer date.
	if main.CommitterDate != 1700000150 {
		t.Errorf("main.CommitterDate: got %d, want 1700000150", main.CommitterDate)
	}

	login := byName["feature/login"]
	if !login.HasLocal || login.HasRemote {
		t.Errorf("feature/login: HasLocal=%v HasRemote=%v, want true/false", login.HasLocal, login.HasRemote)
	}

	// Sorted by committer date desc: feature/login (300) > dev (250) > main (150).
	if len(got) != 3 {
		t.Fatalf("expected 3 branches, got %d: %v", len(got), got)
	}
	if got[0].Name != "feature/login" || got[1].Name != "dev" || got[2].Name != "main" {
		t.Errorf("unexpected order: %v, %v, %v", got[0].Name, got[1].Name, got[2].Name)
	}
}

func TestListBranchSuggestions_QueryFilter(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		return forEachRefFakeOutput(), nil
	})
	installFake(t, fake)

	got, err := ListBranchSuggestions("/repo", "DEV", 50)
	if err != nil {
		t.Fatalf("ListBranchSuggestions: %v", err)
	}
	if len(got) != 1 || got[0].Name != "dev" {
		t.Fatalf("query filter: got %v, want only dev", got)
	}
}

func TestListBranchSuggestions_Limit(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		return forEachRefFakeOutput(), nil
	})
	installFake(t, fake)

	got, err := ListBranchSuggestions("/repo", "", 2)
	if err != nil {
		t.Fatalf("ListBranchSuggestions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("limit: got %d branches, want 2", len(got))
	}
	if got[0].Name != "feature/login" || got[1].Name != "dev" {
		t.Errorf("limit should keep the newest branches, got %v, %v", got[0].Name, got[1].Name)
	}
}

func TestListBranchSuggestions_NotAGitRepo(t *testing.T) {
	fake := newFakeGit(func(dir string, args []string) (string, error) {
		return "", errors.New("fatal: not a git repository")
	})
	installFake(t, fake)

	if _, err := ListBranchSuggestions("/nowhere", "", 50); err == nil {
		t.Fatal("expected error for non-git directory")
	}
}
