package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var agentArchiveForce bool

var agentArchiveCmd = &cobra.Command{
	Use:   "archive <id>",
	Short: "Archive an agent (soft delete)",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentArchive,
}

func init() {
	agentArchiveCmd.Flags().BoolVar(&agentArchiveForce, "force", false, "Archive even if running")
	agentCmd.AddCommand(agentArchiveCmd)
}

func runAgentArchive(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}

	req := &protocol.ArchiveAgentRequest{
		Type:    "archive_agent_request",
		AgentID: agentID,
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("archive agent: %w", err)
	}

	if isRPCError(resp) {
		return &output.CommandError{Code: "ARCHIVE_FAILED", Message: extractRPCError(resp)}
	}

	opts := getOutputOpts()
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(map[string]string{
			"agentId": agentID,
			"status":  "archived",
		}, nil), opts)
	}

	fmt.Fprintf(output.Stdout, "Agent %s archived\n", shortenID(agentID))
	return nil
}
