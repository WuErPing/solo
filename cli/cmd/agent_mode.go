package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var agentModeList bool

var agentModeCmd = &cobra.Command{
	Use:   "mode <id> [mode]",
	Short: "Change an agent's operational mode",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runAgentMode,
}

func init() {
	agentModeCmd.Flags().BoolVar(&agentModeList, "list", false, "List available modes for this agent")
	agentCmd.AddCommand(agentModeCmd)
}

func runAgentMode(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer closeDaemonClient(c)

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}

	// If --list, fetch agent and show modes
	if agentModeList {
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
			} `json:"payload"`
		}
		if err := json.Unmarshal(payload, &fetchResp); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if fetchResp.Payload.Agent == nil {
			return &output.CommandError{Code: "AGENT_NOT_FOUND", Message: "Agent not found"}
		}

		agent := fetchResp.Payload.Agent
		opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)

		if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
			return output.Render(cmdStdout, output.SingleResult(agent.AvailableModes, nil), opts)
		}

		if len(agent.AvailableModes) == 0 {
			if err := errFprintln(cmdStdout, "No modes available"); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		}

		for _, mode := range agent.AvailableModes {
			current := ""
			if agent.CurrentModeID != nil && *agent.CurrentModeID == mode.ID {
				current = " (current)"
			}
			if err := errFprintf(cmdStdout, "  %s\t%s%s\n", mode.ID, mode.Label, current); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
		}
		return nil
	}

	// Set mode
	if len(args) < 2 {
		return &output.CommandError{Code: "MISSING_MODE", Message: "Mode ID required (or use --list)"}
	}

	req := &protocol.SetAgentModeRequest{
		Type:    "set_agent_mode_request",
		AgentID: agentID,
		ModeID:  args[1],
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("set mode: %w", err)
	}

	if isRPCError(resp) {
		return &output.CommandError{Code: "MODE_FAILED", Message: extractRPCError(resp)}
	}

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(map[string]string{
			"agentId": agentID,
			"mode":    args[1],
		}, nil), opts)
	}

	if err := errFprintf(cmdStdout, "Agent %s mode set to %s\n", shortenID(agentID), args[1]); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
