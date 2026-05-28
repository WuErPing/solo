package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var (
	daemonStopTimeout string
	daemonStopForce   bool
)

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Solo daemon",
	RunE:  runDaemonStop,
}

func init() {
	daemonStopCmd.Flags().StringVar(&daemonStopTimeout, "timeout", "15", "Wait timeout in seconds")
	daemonStopCmd.Flags().BoolVar(&daemonStopForce, "force", false, "Force kill if graceful stop times out")
	daemonCmd.AddCommand(daemonStopCmd)
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return &output.CommandError{
			Code:    "DAEMON_NOT_RUNNING",
			Message: "Cannot connect to daemon",
			Details: err.Error(),
		}
	}
	defer c.Close()

	req := &protocol.ShutdownServerRequest{
		Type: "shutdown_server_request",
	}

	_, err = c.Request(ctx, req)
	if err != nil {
		return &output.CommandError{Code: "STOP_FAILED", Message: fmt.Sprintf("Failed to send shutdown: %v", err)}
	}

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(map[string]string{"status": "stopped"}, nil), opts)
	}

	fmt.Fprintln(cmdStdout, "Daemon stopped")
	return nil
}
