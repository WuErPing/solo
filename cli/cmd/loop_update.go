package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var (
	loopUpdateName          string
	loopUpdatePrompt        string
	loopUpdateCwd           string
	loopUpdateVerifyChecks  []string
	loopUpdateMaxIterations int
	loopUpdateArchive       bool
	loopUpdateUnarchive     bool
)

var loopUpdateCmd = &cobra.Command{
	Use:   "update <loop-id>",
	Short: "Update a loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopUpdate,
}

func init() {
	loopUpdateCmd.Flags().StringVar(&loopUpdateName, "name", "", "New loop name")
	loopUpdateCmd.Flags().StringVar(&loopUpdatePrompt, "prompt", "", "New prompt")
	loopUpdateCmd.Flags().StringVar(&loopUpdateCwd, "cwd", "", "New working directory")
	loopUpdateCmd.Flags().StringSliceVar(&loopUpdateVerifyChecks, "verify-checks", nil, "Verify checks (comma-separated)")
	loopUpdateCmd.Flags().IntVar(&loopUpdateMaxIterations, "max-iterations", 0, "Max iterations (0 = no change)")
	loopUpdateCmd.Flags().BoolVar(&loopUpdateArchive, "archive", false, "Archive the loop")
	loopUpdateCmd.Flags().BoolVar(&loopUpdateUnarchive, "unarchive", false, "Unarchive the loop")
	loopCmd.AddCommand(loopUpdateCmd)
}

func runLoopUpdate(cmd *cobra.Command, args []string) error {
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

	req := &protocol.LoopUpdateRequest{
		Type: "loop/update",
		ID:   id,
	}
	if loopUpdateName != "" {
		name := loopUpdateName
		req.Name = &name
	}
	if loopUpdatePrompt != "" {
		prompt := loopUpdatePrompt
		req.Prompt = &prompt
	}
	if loopUpdateCwd != "" {
		cwd := loopUpdateCwd
		req.Cwd = &cwd
	}
	if len(loopUpdateVerifyChecks) > 0 {
		req.VerifyChecks = &loopUpdateVerifyChecks
	}
	if loopUpdateMaxIterations > 0 {
		req.MaxIterations = &loopUpdateMaxIterations
	}
	if loopUpdateArchive && loopUpdateUnarchive {
		return &output.CommandError{Code: "INVALID_FLAGS", Message: "Cannot use both --archive and --unarchive"}
	}
	if loopUpdateArchive {
		archive := true
		req.Archive = &archive
	}
	if loopUpdateUnarchive {
		archive := false
		req.Archive = &archive
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("update loop: %w", err)
	}

	var updateResp protocol.LoopUpdateResponse
	if err := parseLoopResponse(resp, &updateResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if updateResp.Payload.Error != nil && *updateResp.Payload.Error != "" {
		return &output.CommandError{Code: "LOOP_UPDATE_FAILED", Message: *updateResp.Payload.Error}
	}
	if updateResp.Payload.Loop == nil {
		return &output.CommandError{Code: "LOOP_UPDATE_FAILED", Message: "unexpected response from daemon"}
	}

	return renderLoopRecord(updateResp.Payload.Loop, getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
}
