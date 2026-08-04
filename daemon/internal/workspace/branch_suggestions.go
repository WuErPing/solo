package workspace

import (
	"sort"
	"strconv"
	"strings"
)

// BranchSuggestion describes a single branch known to the repository,
// merging local and remote refs of the same name.
type BranchSuggestion struct {
	Name          string
	CommitterDate int64 // unix seconds of the newest commit among merged refs
	HasLocal      bool
	HasRemote     bool
}

// ListBranchSuggestions lists local and remote branches of the git repository
// at cwd, filtered by a case-insensitive substring query, sorted by most
// recent committer date, and capped at limit (default 50 when limit <= 0).
func ListBranchSuggestions(cwd, query string, limit int) ([]BranchSuggestion, error) {
	if limit <= 0 {
		limit = 50
	}
	out, err := getGitCmd().Output(cwd, "for-each-ref",
		"--sort=-committerdate",
		"--format=%(refname)%09%(committerdate:unix)",
		"refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}

	merged := make(map[string]*BranchSuggestion)
	order := make([]string, 0)
	add := func(name string, ts int64, local, remote bool) {
		// Skip the origin/HEAD symref (and any branch literally named HEAD).
		if name == "" || name == "HEAD" {
			return
		}
		existing, ok := merged[name]
		if !ok {
			existing = &BranchSuggestion{Name: name}
			merged[name] = existing
			order = append(order, name)
		}
		if ts > existing.CommitterDate {
			existing.CommitterDate = ts
		}
		existing.HasLocal = existing.HasLocal || local
		existing.HasRemote = existing.HasRemote || remote
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		ref := strings.TrimSpace(parts[0])
		var ts int64
		if len(parts) == 2 {
			ts, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		}
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			add(strings.TrimPrefix(ref, "refs/heads/"), ts, true, false)
		case strings.HasPrefix(ref, "refs/remotes/"):
			short := strings.TrimPrefix(ref, "refs/remotes/")
			// Strip the remote name (first path component).
			if idx := strings.Index(short, "/"); idx >= 0 {
				short = short[idx+1:]
			}
			add(short, ts, false, true)
		}
	}

	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]BranchSuggestion, 0, len(merged))
	for _, name := range order {
		b := merged[name]
		if query != "" && !strings.Contains(strings.ToLower(b.Name), query) {
			continue
		}
		result = append(result, *b)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CommitterDate != result[j].CommitterDate {
			return result[i].CommitterDate > result[j].CommitterDate
		}
		return result[i].Name < result[j].Name
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
