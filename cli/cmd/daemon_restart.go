package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var (
	daemonRestartTimeout string
	daemonRestartForce   bool
	daemonRestartPort    string
	daemonRestartNoRelay bool
	daemonRestartNoMCP   bool
)

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Solo daemon",
	RunE:  runDaemonRestart,
}

func init() {
	daemonRestartCmd.Flags().StringVar(&daemonRestartTimeout, "timeout", "15", "Wait timeout in seconds")
	daemonRestartCmd.Flags().BoolVar(&daemonRestartForce, "force", false, "Force kill if graceful stop times out")
	daemonRestartCmd.Flags().StringVar(&daemonRestartPort, "port", "", "Port for restarted daemon")
	daemonRestartCmd.Flags().BoolVar(&daemonRestartNoRelay, "no-relay", false, "Disable relay on restarted daemon")
	daemonRestartCmd.Flags().BoolVar(&daemonRestartNoMCP, "no-mcp", false, "Disable MCP on restarted daemon")
	daemonCmd.AddCommand(daemonRestartCmd)
}

func runDaemonRestart(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Try graceful restart via WebSocket
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return &output.CommandError{
			Code:    "DAEMON_NOT_RUNNING",
			Message: "Cannot connect to daemon",
		}
	}
	defer closeDaemonClient(c)

	reason := "cli restart"
	req := &protocol.RestartServerRequest{
		Type:   "restart_server_request",
		Reason: &reason,
	}

	_, err = c.Request(ctx, req)
	if err != nil {
		return &output.CommandError{Code: "RESTART_FAILED", Message: fmt.Sprintf("Failed to send restart: %v", err)}
	}

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(map[string]string{"status": "restarting"}, nil), opts)
	}

	_, _ = fmt.Fprintln(cmdStdout, "Daemon restarting...")
	return nil
}
