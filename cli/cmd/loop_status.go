package cmd

import (
	"github.com/spf13/cobra"
)

var loopStatusCmd = &cobra.Command{
	Use:   "status <loop-id>",
	Short: "Show loop status",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopStatus,
}

func init() {
	loopCmd.AddCommand(loopStatusCmd)
}

func runLoopStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer closeDaemonClient(c)

	id, err := resolveLoopID(ctx, c, args[0])
	if err != nil {
		return err
	}

	loop, err := fetchLoop(ctx, c, id)
	if err != nil {
		return err
	}

	return renderLoopRecord(loop, getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
}
