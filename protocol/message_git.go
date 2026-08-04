package protocol

// --- Branch suggestions ---

// BranchSuggestionsRequest asks the daemon for branch suggestions of the git
// repository at Cwd (app new-workspace ref picker, branch switcher).
type BranchSuggestionsRequest struct {
	Type      string  `json:"type"`
	Cwd       string  `json:"cwd"`
	Query     *string `json:"query,omitempty"`
	Limit     *int    `json:"limit,omitempty"`
	RequestID string  `json:"requestId"`
}

func (m *BranchSuggestionsRequest) MsgType() string { return "branch_suggestions_request" }

// BranchSuggestionDetail carries per-branch metadata for the picker UI.
type BranchSuggestionDetail struct {
	Name          string `json:"name"`
	CommitterDate int64  `json:"committerDate"`
	HasLocal      *bool  `json:"hasLocal,omitempty"`
	HasRemote     *bool  `json:"hasRemote,omitempty"`
}

type BranchSuggestionsResponse struct {
	Type    string                           `json:"type"`
	Payload BranchSuggestionsResponsePayload `json:"payload"`
}

type BranchSuggestionsResponsePayload struct {
	RequestID     string                   `json:"requestId"`
	Branches      []string                 `json:"branches"`
	BranchDetails []BranchSuggestionDetail `json:"branchDetails,omitempty"`
	Error         *string                  `json:"error"`
}

func (m *BranchSuggestionsResponse) MsgType() string { return "branch_suggestions_response" }

// --- GitHub search ---

// GitHubSearchRequest asks the daemon to search GitHub issues/PRs of the
// repository at Cwd via the gh CLI. Kinds accepts "github-pr" and
// "github-issue"; empty searches both.
type GitHubSearchRequest struct {
	Type      string   `json:"type"`
	Cwd       string   `json:"cwd"`
	Query     string   `json:"query"`
	Limit     *int     `json:"limit,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
	RequestID string   `json:"requestId"`
}

func (m *GitHubSearchRequest) MsgType() string { return "github_search_request" }

// GitHubSearchItem is a single issue or pull request search result.
type GitHubSearchItem struct {
	Kind        string   `json:"kind"` // "issue" | "pr"
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	State       string   `json:"state"`
	Body        *string  `json:"body"`
	Labels      []string `json:"labels"`
	BaseRefName *string  `json:"baseRefName,omitempty"`
	HeadRefName *string  `json:"headRefName,omitempty"`
	UpdatedAt   *string  `json:"updatedAt,omitempty"`
}

type GitHubSearchResponse struct {
	Type    string                      `json:"type"`
	Payload GitHubSearchResponsePayload `json:"payload"`
}

type GitHubSearchResponsePayload struct {
	RequestID             string             `json:"requestId"`
	Items                 []GitHubSearchItem `json:"items"`
	GitHubFeaturesEnabled bool               `json:"githubFeaturesEnabled"`
	Error                 *string            `json:"error"`
}

func (m *GitHubSearchResponse) MsgType() string { return "github_search_response" }
