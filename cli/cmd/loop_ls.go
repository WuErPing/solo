package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var loopLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List loops",
	RunE:  runLoopLs,
}

func init() {
	loopCmd.AddCommand(loopLsCmd)
}

func runLoopLs(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer closeDaemonClient(c)

	resp, err := c.Request(ctx, &protocol.LoopListRequest{Type: "loop/list"})
	if err != nil {
		return fmt.Errorf("list loops: %w", err)
	}

	var listResp protocol.LoopListResponse
	if err := parseLoopResponse(resp, &listResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if listResp.Payload.Error != nil && *listResp.Payload.Error != "" {
		return &output.CommandError{Code: "LOOP_LIST_FAILED", Message: *listResp.Payload.Error}
	}

	return renderLoopList(listResp.Payload.Loops, getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
}
