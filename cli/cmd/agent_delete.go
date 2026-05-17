package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
	"github.com/spf13/cobra"
)

var (
	agentDeleteAll bool
	agentDeleteCwd string
)

var agentDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an agent",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAgentDelete,
}

func init() {
	agentDeleteCmd.Flags().BoolVar(&agentDeleteAll, "all", false, "Delete all agents")
	agentDeleteCmd.Flags().StringVar(&agentDeleteCwd, "cwd", "", "Delete agents in this directory")
	agentCmd.AddCommand(agentDeleteCmd)
}

func runAgentDelete(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if agentDeleteAll || agentDeleteCwd != "" {
		return deleteMultipleAgents(ctx, c)
	}

	if len(args) == 0 {
		return &output.CommandError{Code: "MISSING_ID", Message: "Agent ID required (or use --all)"}
	}

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}

	req := &protocol.DeleteAgentRequest{
		Type:    "delete_agent_request",
		AgentID: agentID,
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	if isRPCError(resp) {
		return &output.CommandError{Code: "DELETE_FAILED", Message: extractRPCError(resp)}
	}

	opts := getOutputOpts()
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(map[string]string{
			"agentId": agentID,
			"status":  "deleted",
		}, nil), opts)
	}

	fmt.Fprintf(output.Stdout, "Agent %s deleted\n", shortenID(agentID))
	return nil
}

func deleteMultipleAgents(ctx context.Context, c *client.DaemonClient) error {
	req := &protocol.FetchAgentsRequest{
		Type:      "fetch_agents_request",
		RequestID: c.GenerateRequestID(),
	}
	scope := "active"
	req.Scope = &scope

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("fetch agents: %w", err)
	}

	payload, _ := json.Marshal(resp.Message)
	var fetchResp struct {
		Payload struct {
			Entries []struct {
				Agent protocol.AgentSnapshotPayload `json:"agent"`
			} `json:"entries"`
		} `json:"payload"`
	}
	json.Unmarshal(payload, &fetchResp)

	deleted := 0
	for _, entry := range fetchResp.Payload.Entries {
		if agentDeleteCwd != "" && !isCwdMatch(agentDeleteCwd, entry.Agent.Cwd) {
			continue
		}

		delReq := &protocol.DeleteAgentRequest{
			Type:    "delete_agent_request",
			AgentID: entry.Agent.ID,
		}
		if _, err := c.Request(ctx, delReq); err == nil {
			deleted++
		}
	}

	opts := getOutputOpts()
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(map[string]int{"deleted": deleted}, nil), opts)
	}

	fmt.Fprintf(output.Stdout, "Deleted %d agent(s)\n", deleted)
	return nil
}
