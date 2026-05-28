package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var agentInspectCmd = &cobra.Command{
	Use:   "inspect <id>",
	Short: "Show detailed agent information",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentInspect,
}

func init() {
	agentCmd.AddCommand(agentInspectCmd)
}

func runAgentInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer c.Close()

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}

	req := &protocol.FetchAgentRequest{
		Type:      "fetch_agent_request",
		AgentID:   agentID,
		RequestID: c.GenerateRequestID(),
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("fetch agent: %w", err)
	}

	payload, _ := json.Marshal(resp.Message)
	var fetchResp struct {
		Payload struct {
			Agent *protocol.AgentSnapshotPayload `json:"agent"`
			Error *string                        `json:"error,omitempty"`
		} `json:"payload"`
	}
	json.Unmarshal(payload, &fetchResp)

	if fetchResp.Payload.Error != nil {
		return &output.CommandError{Code: "AGENT_NOT_FOUND", Message: *fetchResp.Payload.Error}
	}
	if fetchResp.Payload.Agent == nil {
		return &output.CommandError{Code: "AGENT_NOT_FOUND", Message: fmt.Sprintf("No agent found matching: %s", args[0])}
	}

	agent := fetchResp.Payload.Agent
	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)

	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(agent, nil), opts)
	}

	// Table/key-value output
	rows := []struct{ Key, Value string }{
		{"ID", agent.ID},
		{"Provider", agent.Provider},
		{"Status", string(agent.Status)},
		{"CWD", agent.Cwd},
	}

	if agent.Title != nil {
		rows = append(rows, struct{ Key, Value string }{"Title", *agent.Title})
	}
	if agent.Model != nil {
		rows = append(rows, struct{ Key, Value string }{"Model", *agent.Model})
	}
	if agent.CurrentModeID != nil {
		rows = append(rows, struct{ Key, Value string }{"Mode", *agent.CurrentModeID})
	}
	rows = append(rows, struct{ Key, Value string }{"Created", agent.CreatedAt})

	if agent.LastError != nil {
		rows = append(rows, struct{ Key, Value string }{"Error", *agent.LastError})
	}
	if agent.ArchivedAt != nil && *agent.ArchivedAt != "" {
		rows = append(rows, struct{ Key, Value string }{"Archived", *agent.ArchivedAt})
	}

	maxKeyLen := 0
	for _, r := range rows {
		if len(r.Key) > maxKeyLen {
			maxKeyLen = len(r.Key)
		}
	}

	for _, r := range rows {
		if err := errFprintf(cmdStdout, "%-*s  %s\n", maxKeyLen, r.Key, r.Value); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}

	return nil
}
