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
	agentStopAll bool
	agentStopCwd string
)

var agentStopCmd = &cobra.Command{
	Use:   "stop [id]",
	Short: "Stop/cancel a running agent",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAgentStop,
}

func init() {
	agentStopCmd.Flags().BoolVar(&agentStopAll, "all", false, "Stop all running agents")
	agentStopCmd.Flags().StringVar(&agentStopCwd, "cwd", "", "Stop agents in this directory")
	agentCmd.AddCommand(agentStopCmd)
}

func runAgentStop(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if agentStopAll || agentStopCwd != "" {
		return stopMultipleAgents(ctx, c)
	}

	if len(args) == 0 {
		return &output.CommandError{Code: "MISSING_ID", Message: "Agent ID required (or use --all)"}
	}

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}

	req := &protocol.CancelAgentRequest{
		Type:    "cancel_agent_request",
		AgentID: agentID,
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}

	// Check for error
	if isRPCError(resp) {
		return &output.CommandError{Code: "STOP_FAILED", Message: extractRPCError(resp)}
	}

	opts := getOutputOpts()
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(map[string]string{
			"agentId": agentID,
			"status":  "stopped",
		}, nil), opts)
	}

	fmt.Fprintf(output.Stdout, "Agent %s stopped\n", shortenID(agentID))
	return nil
}

func stopMultipleAgents(ctx context.Context, c *client.DaemonClient) error {
	// Fetch all agents
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

	stopped := 0
	for _, entry := range fetchResp.Payload.Entries {
		if entry.Agent.Status != protocol.AgentRunning {
			continue
		}
		if agentStopCwd != "" && !isCwdMatch(agentStopCwd, entry.Agent.Cwd) {
			continue
		}

		stopReq := &protocol.CancelAgentRequest{
			Type:    "cancel_agent_request",
			AgentID: entry.Agent.ID,
		}
		c.Request(ctx, stopReq) // best effort
		stopped++
	}

	opts := getOutputOpts()
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(map[string]int{"stopped": stopped}, nil), opts)
	}

	fmt.Fprintf(output.Stdout, "Stopped %d agent(s)\n", stopped)
	return nil
}

func isRPCError(resp *protocol.WSOutboundMessage) bool {
	payload, _ := json.Marshal(resp.Message)
	var peek struct {
		Type string `json:"type"`
	}
	json.Unmarshal(payload, &peek)
	return peek.Type == "rpc_error"
}

func extractRPCError(resp *protocol.WSOutboundMessage) string {
	payload, _ := json.Marshal(resp.Message)
	var rpcErr struct {
		Payload struct {
			Error string `json:"error"`
		} `json:"payload"`
	}
	json.Unmarshal(payload, &rpcErr)
	return rpcErr.Payload.Error
}
