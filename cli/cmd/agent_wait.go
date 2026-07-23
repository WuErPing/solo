package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/cliutil"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var agentWaitTimeout string

var agentWaitCmd = &cobra.Command{
	Use:   "wait <id>",
	Short: "Wait for an agent to finish",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentWait,
}

func init() {
	agentWaitCmd.Flags().StringVar(&agentWaitTimeout, "timeout", "", "Max wait time (e.g. 30s, 5m)")
	agentCmd.AddCommand(agentWaitCmd)
}

func runAgentWait(cmd *cobra.Command, args []string) error { //nolint:gocyclo // grandfathered CC=25
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

	// Apply timeout if specified
	if agentWaitTimeout != "" {
		timeout, err := cliutil.ParseDuration(agentWaitTimeout)
		if err != nil {
			return &output.CommandError{Code: "INVALID_TIMEOUT", Message: "Invalid wait timeout value", Details: err.Error()}
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Subscribe to agent updates
	updates := c.Subscribe("agent_update")
	defer c.Unsubscribe(updates)
	defer warnIfDropped(updates)

	// Check current status first
	checkReq := &protocol.FetchAgentRequest{
		Type:      "fetch_agent_request",
		AgentID:   agentID,
		RequestID: c.GenerateRequestID(),
	}
	checkResp, err := c.Request(ctx, checkReq)
	if err == nil {
		payload, _ := json.Marshal(checkResp.Message)
		var fetchResp struct {
			Payload struct {
				Agent *protocol.AgentSnapshotPayload `json:"agent"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(payload, &fetchResp)
		if fetchResp.Payload.Agent != nil {
			status := string(fetchResp.Payload.Agent.Status)
			if status == "idle" || status == "error" || status == "closed" {
				return printWaitResult(agentID, status)
			}
		}
	}

	// Wait for agent to reach a terminal state
	for {
		select {
		case msg, ok := <-updates.Messages():
			if !ok || msg == nil {
				return &output.CommandError{Code: "DISCONNECTED", Message: "Lost connection to daemon while waiting"}
			}
			payload, _ := json.Marshal(msg.Message)
			var updateWrapper struct {
				Payload struct {
					Kind  string `json:"kind"`
					Agent struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"agent"`
				} `json:"payload"`
			}
			_ = json.Unmarshal(payload, &updateWrapper)
			update := updateWrapper.Payload
			if update.Agent.ID == agentID {
				status := update.Agent.Status
				if status == "idle" || status == "error" || status == "closed" {
					return printWaitResult(agentID, status)
				}
			}

		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return &output.CommandError{Code: "TIMEOUT", Message: "Timed out waiting for agent to finish"}
			}
			return ctx.Err()

		case <-time.After(30 * time.Second):
			// Periodically re-check status
			checkReq := &protocol.FetchAgentRequest{
				Type:      "fetch_agent_request",
				AgentID:   agentID,
				RequestID: c.GenerateRequestID(),
			}
			checkResp, err := c.Request(ctx, checkReq)
			if err == nil {
				payload, _ := json.Marshal(checkResp.Message)
				var fetchResp struct {
					Payload struct {
						Agent *protocol.AgentSnapshotPayload `json:"agent"`
					} `json:"payload"`
				}
				_ = json.Unmarshal(payload, &fetchResp)
				if fetchResp.Payload.Agent != nil {
					status := string(fetchResp.Payload.Agent.Status)
					if status == "idle" || status == "error" || status == "closed" {
						return printWaitResult(agentID, status)
					}
				}
			}
		}
	}
}

func printWaitResult(agentID, status string) error {
	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(map[string]string{
			"agentId": agentID,
			"status":  status,
		}, nil), opts)
	}
	if err := errFprintf(cmdStdout, "Agent %s %s\n", shortenID(agentID), status); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
