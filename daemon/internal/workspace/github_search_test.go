package workspace

import (
	"errors"
	"strings"
	"testing"
)

// ---- fake GhCommander ----

type fakeGhCommander struct {
	available bool
	handler   func(dir string, args []string) (string, error)
}

func (f *fakeGhCommander) Available() bool { return f.available }

func (f *fakeGhCommander) Output(dir string, args ...string) (string, error) {
	if f.handler != nil {
		return f.handler(dir, args)
	}
	return "", nil
}

// installFakeGh replaces the package-level ghCmd for the duration of the test.
func installFakeGh(t *testing.T, fake GhCommander) {
	t.Helper()
	orig := getGhCmd()
	setGhCmd(fake)
	t.Cleanup(func() { setGhCmd(orig) })
}

const fakeGhPRListOutput = `[
  {
    "number": 515,
    "title": "Review selected start ref",
    "url": "https://github.com/getsolo/solo/pull/515",
    "state": "OPEN",
    "body": "Fixture pull request for app e2e.",
    "labels": [{"name": "bug"}, {"name": "e2e"}],
    "baseRefName": "main",
    "headRefName": "feature/start-from-pr"
  }
]`

func authedGh(handler func(dir string, args []string) (string, error)) *fakeGhCommander {
	return &fakeGhCommander{available: true, handler: handler}
}

func TestSearchGitHub_ParsesPullRequests(t *testing.T) {
	gh := authedGh(func(dir string, args []string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
			return "", nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "list":
			return fakeGhPRListOutput, nil
		case len(args) >= 2 && args[0] == "issue" && args[1] == "list":
			return "[]", nil
		}
		return "", errors.New("unsupported gh invocation: " + strings.Join(args, " "))
	})
	installFakeGh(t, gh)

	items, enabled, err := SearchGitHub("/repo", "start ref", []string{"github-pr"}, 20)
	if err != nil {
		t.Fatalf("SearchGitHub: %v", err)
	}
	if !enabled {
		t.Fatal("expected featuresEnabled=true")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	pr := items[0]
	if pr.Kind != "pr" {
		t.Errorf("kind: got %q, want pr", pr.Kind)
	}
	if pr.Number != 515 || pr.Title != "Review selected start ref" {
		t.Errorf("unexpected item: %+v", pr)
	}
	if len(pr.Labels) != 2 || pr.Labels[0] != "bug" || pr.Labels[1] != "e2e" {
		t.Errorf("labels: got %v, want [bug e2e]", pr.Labels)
	}
	if pr.BaseRefName == nil || *pr.BaseRefName != "main" {
		t.Errorf("baseRefName: got %v", pr.BaseRefName)
	}
	if pr.HeadRefName == nil || *pr.HeadRefName != "feature/start-from-pr" {
		t.Errorf("headRefName: got %v", pr.HeadRefName)
	}
}

func TestSearchGitHub_GhNotInstalled(t *testing.T) {
	installFakeGh(t, &fakeGhCommander{available: false})

	items, enabled, err := SearchGitHub("/repo", "q", nil, 20)
	if err != nil {
		t.Fatalf("SearchGitHub: %v", err)
	}
	if enabled {
		t.Error("expected featuresEnabled=false when gh is missing")
	}
	if len(items) != 0 {
		t.Errorf("expected no items, got %v", items)
	}
}

func TestSearchGitHub_NotAuthenticated(t *testing.T) {
	gh := authedGh(func(dir string, args []string) (string, error) {
		return "", errors.New("gh: not logged in")
	})
	installFakeGh(t, gh)

	items, enabled, err := SearchGitHub("/repo", "q", nil, 20)
	if err != nil {
		t.Fatalf("SearchGitHub: %v", err)
	}
	if enabled {
		t.Error("expected featuresEnabled=false when gh auth fails")
	}
	if len(items) != 0 {
		t.Errorf("expected no items, got %v", items)
	}
}

func TestSearchGitHub_SearchFailureReturnsError(t *testing.T) {
	gh := authedGh(func(dir string, args []string) (string, error) {
		if len(args) >= 2 && args[0] == "auth" {
			return "", nil
		}
		return "", errors.New("could not resolve to a Repository")
	})
	installFakeGh(t, gh)

	_, enabled, err := SearchGitHub("/not-a-repo", "q", []string{"github-pr"}, 20)
	if err == nil {
		t.Fatal("expected error when gh pr list fails")
	}
	if !enabled {
		t.Error("featuresEnabled should stay true when gh works but the repo has no GitHub remote")
	}
}

func TestSearchGitHub_DefaultKindsSearchBoth(t *testing.T) {
	gh := authedGh(func(dir string, args []string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "auth":
			return "", nil
		case len(args) >= 2 && args[0] == "pr":
			return fakeGhPRListOutput, nil
		case len(args) >= 2 && args[0] == "issue":
			return `[{"number": 7, "title": "An issue", "url": "https://x/7", "state": "OPEN", "body": null, "labels": []}]`, nil
		}
		return "", errors.New("unsupported")
	})
	installFakeGh(t, gh)

	items, enabled, err := SearchGitHub("/repo", "", nil, 20)
	if err != nil {
		t.Fatalf("SearchGitHub: %v", err)
	}
	if !enabled {
		t.Fatal("expected featuresEnabled=true")
	}
	if len(items) != 2 {
		t.Fatalf("expected pr+issue, got %v", items)
	}
	kinds := map[string]bool{items[0].Kind: true, items[1].Kind: true}
	if !kinds["pr"] || !kinds["issue"] {
		t.Errorf("expected both pr and issue kinds, got %v", kinds)
	}
}
