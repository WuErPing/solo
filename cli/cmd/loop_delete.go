package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var loopDeleteCmd = &cobra.Command{
	Use:   "delete <loop-id>",
	Short: "Delete a loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopDelete,
}

func init() {
	loopCmd.AddCommand(loopDeleteCmd)
}

func runLoopDelete(cmd *cobra.Command, args []string) error {
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

	resp, err := c.Request(ctx, &protocol.LoopDeleteRequest{
		Type: "loop/delete",
		ID:   id,
	})
	if err != nil {
		return fmt.Errorf("delete loop: %w", err)
	}

	var deleteResp protocol.LoopDeleteResponse
	if err := parseLoopResponse(resp, &deleteResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if deleteResp.Payload.Error != nil && *deleteResp.Payload.Error != "" {
		return &output.CommandError{Code: "LOOP_DELETE_FAILED", Message: *deleteResp.Payload.Error}
	}

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
	if opts.Format == output.FormatQuiet {
		return nil
	}
	_, err = fmt.Fprintf(cmdStdout, "Deleted loop %s\n", shortenID(id))
	return err
}
