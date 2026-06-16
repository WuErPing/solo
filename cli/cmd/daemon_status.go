package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
)

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatus,
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
}

func runDaemonStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return &output.CommandError{
			Code:    "DAEMON_NOT_RUNNING",
			Message: "Cannot connect to daemon",
			Details: "Start the daemon with: solo daemon start",
		}
	}
	defer closeDaemonClient(c)

	si := c.ServerInfo()
	ps := c.ProvidersSnapshot()

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)

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
		return output.Render(cmdStdout, output.SingleResult(result, nil), opts)
	}

	// Human-readable output
	_, _ = fmt.Fprintf(cmdStdout, "Daemon is running\n")
	if si != nil {
		_, _ = fmt.Fprintf(cmdStdout, "  Server ID: %s\n", si.ServerID)
		if si.Version != nil {
			_, _ = fmt.Fprintf(cmdStdout, "  Version:   %s\n", *si.Version)
		}
	}
	if ps != nil {
		ready := 0
		for _, e := range ps.Entries {
			if e.Status == "ready" {
				ready++
			}
		}
		_, _ = fmt.Fprintf(cmdStdout, "  Providers: %d (%d ready)\n", len(ps.Entries), ready)
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
	_ = json.Unmarshal(reqData, &req)

	// Use client to count agents
	fetchResp, err := c.Request(ctx, &agentCountRequest{})
	if err == nil {
		payload, _ := json.Marshal(fetchResp.Message)
		var fr struct {
			Payload struct {
				Entries []interface{} `json:"entries"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(payload, &fr)
		_, _ = fmt.Fprintf(cmdStdout, "  Agents:    %d\n", len(fr.Payload.Entries))
	}

	return nil
}

// agentCountRequest is a minimal request just to get agent count
type agentCountRequest struct{}

func (a *agentCountRequest) MsgType() string { return "fetch_agents_request" }
