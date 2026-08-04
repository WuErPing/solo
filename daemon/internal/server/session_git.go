package server

import (
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// handleBranchSuggestions answers branch_suggestions_request with the merged
// local/remote branches of the repository at the requested cwd (app
// new-workspace ref picker, branch switcher).
func (s *Session) handleBranchSuggestions(m *protocol.BranchSuggestionsRequest) {
	query := ""
	if m.Query != nil {
		query = *m.Query
	}
	limit := 0
	if m.Limit != nil {
		limit = *m.Limit
	}

	payload := protocol.BranchSuggestionsResponsePayload{
		RequestID: m.RequestID,
		Branches:  []string{},
	}

	suggestions, err := workspace.ListBranchSuggestions(m.Cwd, query, limit)
	if err != nil {
		errMsg := err.Error()
		payload.Error = &errMsg
	} else {
		details := make([]protocol.BranchSuggestionDetail, 0, len(suggestions))
		for _, b := range suggestions {
			hasLocal := b.HasLocal
			hasRemote := b.HasRemote
			payload.Branches = append(payload.Branches, b.Name)
			details = append(details, protocol.BranchSuggestionDetail{
				Name:          b.Name,
				CommitterDate: b.CommitterDate,
				HasLocal:      &hasLocal,
				HasRemote:     &hasRemote,
			})
		}
		payload.BranchDetails = details
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.BranchSuggestionsResponse{
		Type:    "branch_suggestions_response",
		Payload: payload,
	}))
}

// handleGitHubSearch answers github_search_request by searching issues/PRs of
// the repository at the requested cwd via the gh CLI. When gh is unavailable
// or not authenticated the response carries githubFeaturesEnabled=false so
// clients can hide GitHub picker entries gracefully.
func (s *Session) handleGitHubSearch(m *protocol.GitHubSearchRequest) {
	limit := 0
	if m.Limit != nil {
		limit = *m.Limit
	}

	items, enabled, err := workspace.SearchGitHub(m.Cwd, m.Query, m.Kinds, limit)

	payload := protocol.GitHubSearchResponsePayload{
		RequestID:             m.RequestID,
		Items:                 []protocol.GitHubSearchItem{},
		GitHubFeaturesEnabled: enabled,
	}
	if err != nil {
		errMsg := err.Error()
		payload.Error = &errMsg
	}
	for _, it := range items {
		payload.Items = append(payload.Items, protocol.GitHubSearchItem{
			Kind:        it.Kind,
			Number:      it.Number,
			Title:       it.Title,
			URL:         it.URL,
			State:       it.State,
			Body:        it.Body,
			Labels:      it.Labels,
			BaseRefName: it.BaseRefName,
			HeadRefName: it.HeadRefName,
			UpdatedAt:   it.UpdatedAt,
		})
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.GitHubSearchResponse{
		Type:    "github_search_response",
		Payload: payload,
	}))
}
