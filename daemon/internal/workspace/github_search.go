package workspace

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// GitHub search kinds accepted by SearchGitHub.
const (
	GitHubSearchKindPR    = "github-pr"
	GitHubSearchKindIssue = "github-issue"
)

// GitHubSearchItem is a single issue or pull request returned by GitHub search.
type GitHubSearchItem struct {
	Kind        string // "issue" | "pr"
	Number      int
	Title       string
	URL         string
	State       string
	Body        *string
	Labels      []string
	BaseRefName *string
	HeadRefName *string
	UpdatedAt   *string
}

// SearchGitHub searches issues and/or pull requests of the repository at cwd
// via the gh CLI. kinds accepts GitHubSearchKindPR and GitHubSearchKindIssue;
// an empty slice searches both. featuresEnabled is false when gh is not
// installed or not authenticated, in which case the result is empty and err
// is nil. A failing search (e.g. no GitHub remote) keeps featuresEnabled true
// and returns the error.
func SearchGitHub(cwd, query string, kinds []string, limit int) (items []GitHubSearchItem, featuresEnabled bool, err error) {
	items = []GitHubSearchItem{}
	gh := getGhCmd()
	if !gh.Available() {
		return items, false, nil
	}
	if _, authErr := gh.Output(cwd, "auth", "status"); authErr != nil {
		return items, false, nil
	}
	if limit <= 0 {
		limit = 20
	}

	wantPR, wantIssue := resolveGitHubSearchKinds(kinds)
	if wantPR {
		prs, prErr := searchGitHubPRs(gh, cwd, query, limit)
		if prErr != nil {
			return items, true, prErr
		}
		items = append(items, prs...)
	}
	if wantIssue {
		issues, issueErr := searchGitHubIssues(gh, cwd, query, limit)
		if issueErr != nil {
			return items, true, issueErr
		}
		items = append(items, issues...)
	}
	return items, true, nil
}

func resolveGitHubSearchKinds(kinds []string) (wantPR, wantIssue bool) {
	if len(kinds) == 0 {
		return true, true
	}
	for _, k := range kinds {
		switch k {
		case GitHubSearchKindPR:
			wantPR = true
		case GitHubSearchKindIssue:
			wantIssue = true
		}
	}
	return wantPR, wantIssue
}

type ghLabel struct {
	Name string `json:"name"`
}

func flattenGhLabels(labels []ghLabel) []string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
}

type ghPR struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	State       string    `json:"state"`
	Body        *string   `json:"body"`
	Labels      []ghLabel `json:"labels"`
	BaseRefName *string   `json:"baseRefName"`
	HeadRefName *string   `json:"headRefName"`
	UpdatedAt   *string   `json:"updatedAt"`
}

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	State     string    `json:"state"`
	Body      *string   `json:"body"`
	Labels    []ghLabel `json:"labels"`
	UpdatedAt *string   `json:"updatedAt"`
}

func searchGitHubPRs(gh GhCommander, cwd, query string, limit int) ([]GitHubSearchItem, error) {
	out, err := gh.Output(cwd, "pr", "list",
		"--search", query,
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,url,state,body,labels,baseRefName,headRefName,updatedAt")
	if err != nil {
		return nil, err
	}
	var prs []ghPR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parse gh pr list output: %w", err)
	}
	items := make([]GitHubSearchItem, 0, len(prs))
	for _, pr := range prs {
		items = append(items, GitHubSearchItem{
			Kind:        "pr",
			Number:      pr.Number,
			Title:       pr.Title,
			URL:         pr.URL,
			State:       pr.State,
			Body:        pr.Body,
			Labels:      flattenGhLabels(pr.Labels),
			BaseRefName: pr.BaseRefName,
			HeadRefName: pr.HeadRefName,
			UpdatedAt:   pr.UpdatedAt,
		})
	}
	return items, nil
}

func searchGitHubIssues(gh GhCommander, cwd, query string, limit int) ([]GitHubSearchItem, error) {
	out, err := gh.Output(cwd, "issue", "list",
		"--search", query,
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,url,state,body,labels,updatedAt")
	if err != nil {
		return nil, err
	}
	var issues []ghIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return nil, fmt.Errorf("parse gh issue list output: %w", err)
	}
	items := make([]GitHubSearchItem, 0, len(issues))
	for _, issue := range issues {
		items = append(items, GitHubSearchItem{
			Kind:      "issue",
			Number:    issue.Number,
			Title:     issue.Title,
			URL:       issue.URL,
			State:     issue.State,
			Body:      issue.Body,
			Labels:    flattenGhLabels(issue.Labels),
			UpdatedAt: issue.UpdatedAt,
		})
	}
	return items, nil
}
