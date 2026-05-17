package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/spf13/cobra"
)

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatus,
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx)
	if err != nil {
		return &output.CommandError{
			Code:    "DAEMON_NOT_RUNNING",
			Message: "Cannot connect to daemon",
			Details: "Start the daemon with: solo daemon start",
		}
	}
	defer c.Close()

	si := c.ServerInfo()
	ps := c.ProvidersSnapshot()

	opts := getOutputOpts()

	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		result := map[string]interface{}{
			"status":    "running",
			"serverId":  "",
			"providers": 0,
		}
		if si != nil {
			result["serverId"] = si.ServerID
			if si.Version != nil {
				result["version"] = *si.Version
			}
		}
		if ps != nil {
			result["providers"] = len(ps.Entries)
		}
		return output.Render(output.SingleResult(result, nil), opts)
	}

	// Human-readable output
	fmt.Fprintf(output.Stdout, "Daemon is running\n")
	if si != nil {
		fmt.Fprintf(output.Stdout, "  Server ID: %s\n", si.ServerID)
		if si.Version != nil {
			fmt.Fprintf(output.Stdout, "  Version:   %s\n", *si.Version)
		}
	}
	if ps != nil {
		ready := 0
		for _, e := range ps.Entries {
			if e.Status == "ready" {
				ready++
			}
		}
		fmt.Fprintf(output.Stdout, "  Providers: %d (%d ready)\n", len(ps.Entries), ready)
	}

	// Fetch agent count
	fetchReq := map[string]interface{}{
		"type": "fetch_agents_request",
	}
	reqData, _ := json.Marshal(fetchReq)
	var req struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
	}
	json.Unmarshal(reqData, &req)

	// Use client to count agents
	fetchResp, err := c.Request(ctx, &agentCountRequest{})
	if err == nil {
		payload, _ := json.Marshal(fetchResp.Message)
		var fr struct {
			Payload struct {
				Entries []interface{} `json:"entries"`
			} `json:"payload"`
		}
		json.Unmarshal(payload, &fr)
		fmt.Fprintf(output.Stdout, "  Agents:    %d\n", len(fr.Payload.Entries))
	}

	return nil
}

// agentCountRequest is a minimal request just to get agent count
type agentCountRequest struct{}

func (a *agentCountRequest) MsgType() string { return "fetch_agents_request" }
