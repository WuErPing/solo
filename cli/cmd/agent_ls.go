package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var (
	agentLsAll      bool
	agentLsStatus   string
	agentLsCwd      string
	agentLsLabel    []string
	agentLsThinking string
)

var agentLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List agents",
	RunE:  runAgentLs,
}

func init() {
	agentLsCmd.Flags().BoolVarP(&agentLsAll, "all", "a", false, "Include archived agents")
	agentLsCmd.Flags().StringVar(&agentLsStatus, "status", "", "Filter by status")
	agentLsCmd.Flags().StringVar(&agentLsCwd, "cwd", "", "Filter by working directory")
	agentLsCmd.Flags().StringArrayVar(&agentLsLabel, "label", nil, "Filter by label (key=value)")
	agentLsCmd.Flags().StringVar(&agentLsThinking, "thinking", "", "Filter by thinking option ID")
	agentCmd.AddCommand(agentLsCmd)
}

func runAgentLs(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer closeDaemonClient(c)

	// Build fetch request
	req := &protocol.FetchAgentsRequest{
		Type:      "fetch_agents_request",
		RequestID: c.GenerateRequestID(),
	}

	var filter protocol.FetchAgentsFilter
	hasFilter := false

	if agentLsAll {
		t := true
		filter.IncludeArchived = &t
		hasFilter = true
	} else {
		scope := "active"
		req.Scope = &scope
	}

	if agentLsThinking != "" {
		filter.ThinkingOptionID = &agentLsThinking
		hasFilter = true
	}

	if len(agentLsLabel) > 0 {
		filter.Labels = parseLabels(agentLsLabel)
		hasFilter = true
	}

	if hasFilter {
		req.Filter = &filter
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("fetch agents: %w", err)
	}

	// Parse response - the message is the FetchAgentsResponse which has payload.entries
	payload, _ := json.Marshal(resp.Message)
	var fetchResp struct {
		Payload struct {
			Entries []struct {
				Agent protocol.AgentSnapshotPayload `json:"agent"`
			} `json:"entries"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(payload, &fetchResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Convert to display items
	var items []interface{}
	for _, entry := range fetchResp.Payload.Entries {
		a := entry.Agent

		// Client-side filtering
		if agentLsCwd != "" && !isCwdMatch(agentLsCwd, a.Cwd) {
			continue
		}
		if agentLsStatus != "" && string(a.Status) != agentLsStatus {
			continue
		}
		if !agentLsAll && a.ArchivedAt != nil && *a.ArchivedAt != "" {
			continue
		}

		title := "-"
		if a.Title != nil {
			title = *a.Title
		}
		thinking := "-"
		if a.EffectiveThinkingOptionID != nil && *a.EffectiveThinkingOptionID != "" {
			thinking = *a.EffectiveThinkingOptionID
		} else if a.ThinkingOptionID != nil && *a.ThinkingOptionID != "" {
			thinking = *a.ThinkingOptionID
		}

		items = append(items, &agentListItem{
			ID:       a.ID,
			ShortID:  shortenID(a.ID),
			Name:     title,
			Provider: formatProvider(a),
			Thinking: thinking,
			Status:   string(a.Status),
			CWD:      shortenPath(a.Cwd),
			Created:  relativeTime(a.CreatedAt),
		})
	}

	schema := &output.Schema{
		IDField: func(item interface{}) string {
			return item.(*agentListItem).ShortID
		},
		Columns: []output.ColumnDef{
			{Header: "ID", FieldFunc: func(i interface{}) string { return i.(*agentListItem).ShortID }, Width: 8},
			{Header: "NAME", FieldFunc: func(i interface{}) string { return i.(*agentListItem).Name }, Width: 20},
			{Header: "PROVIDER", FieldFunc: func(i interface{}) string { return i.(*agentListItem).Provider }, Width: 15},
			{Header: "THINKING", FieldFunc: func(i interface{}) string { return i.(*agentListItem).Thinking }, Width: 10},
			{Header: "STATUS", FieldFunc: func(i interface{}) string { return i.(*agentListItem).Status }, Width: 10,
				ColorFunc: func(val string, _ interface{}) string {
					switch val {
					case "running":
						return "green"
					case "idle":
						return "yellow"
					case "error":
						return "red"
					}
					return ""
				}},
			{Header: "CWD", FieldFunc: func(i interface{}) string { return i.(*agentListItem).CWD }, Width: 30},
			{Header: "CREATED", FieldFunc: func(i interface{}) string { return i.(*agentListItem).Created }, Width: 15},
		},
	}

	return output.Render(cmdStdout, output.ListResult(items, schema), getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
}

type agentListItem struct {
	ID       string
	ShortID  string
	Name     string
	Provider string
	Thinking string
	Status   string
	CWD      string
	Created  string
}

func shortenID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func formatProvider(a protocol.AgentSnapshotPayload) string {
	model := ""
	if a.Model != nil && *a.Model != "" {
		model = *a.Model
		lower := toLower(model)
		if lower == "default" {
			model = ""
		}
	}
	if model != "" {
		return a.Provider + "/" + model
	}
	return a.Provider
}

func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && len(p) > len(home) && p[:len(home)] == home {
		return "~" + p[len(home):]
	}
	return p
}

func relativeTime(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func parseLabels(labels []string) map[string]string {
	result := make(map[string]string)
	for _, l := range labels {
		if idx := indexByte(l, '='); idx >= 0 {
			result[l[:idx]] = l[idx+1:]
		}
	}
	return result
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func isCwdMatch(filter, agentCwd string) bool {
	return agentCwd == filter || (len(agentCwd) > len(filter) && agentCwd[:len(filter)] == filter && agentCwd[len(filter)] == '/')
}
