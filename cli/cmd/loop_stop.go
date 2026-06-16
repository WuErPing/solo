package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var loopStopCmd = &cobra.Command{
	Use:   "stop <loop-id>",
	Short: "Stop a running loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopStop,
}

func init() {
	loopCmd.AddCommand(loopStopCmd)
}

func runLoopStop(cmd *cobra.Command, args []string) error {
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

	resp, err := c.Request(ctx, &protocol.LoopStopRequest{
		Type: "loop/stop",
		ID:   id,
	})
	if err != nil {
		return fmt.Errorf("stop loop: %w", err)
	}

	var stopResp protocol.LoopStopResponse
	if err := parseLoopResponse(resp, &stopResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if stopResp.Payload.Error != nil && *stopResp.Payload.Error != "" {
		return &output.CommandError{Code: "LOOP_STOP_FAILED", Message: *stopResp.Payload.Error}
	}
	if stopResp.Payload.Loop == nil {
		return &output.CommandError{Code: "LOOP_STOP_FAILED", Message: "unexpected response from daemon"}
	}

	return renderLoopRecord(stopResp.Payload.Loop, getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
}
