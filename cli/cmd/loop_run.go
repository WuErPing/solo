package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/cliutil"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var (
	loopRunCwd          string
	loopRunName         string
	loopRunProvider     string
	loopRunModel        string
	loopRunVerifyCheck  string
	loopRunVerifyPrompt string
	loopRunMaxIter      int
	loopRunMaxTime      time.Duration
	loopRunDetach       bool
)

var loopRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Run a loop",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLoopRun,
}

func init() {
	loopRunCmd.Flags().StringVar(&loopRunCwd, "cwd", "", "Working directory (default: current)")
	loopRunCmd.Flags().StringVar(&loopRunName, "name", "", "Loop name")
	loopRunCmd.Flags().StringVar(&loopRunProvider, "provider", "", "Provider to use (default: first available)")
	loopRunCmd.Flags().StringVar(&loopRunModel, "model", "", "Model to use")
	loopRunCmd.Flags().StringVar(&loopRunVerifyCheck, "verify-check", "", "Shell command that verifies each iteration")
	loopRunCmd.Flags().StringVar(&loopRunVerifyPrompt, "verify-prompt", "", "Prompt for an LLM verifier")
	loopRunCmd.Flags().IntVar(&loopRunMaxIter, "max-iterations", 10, "Maximum iterations")
	loopRunCmd.Flags().DurationVar(&loopRunMaxTime, "max-time", 0, "Maximum total duration (e.g. 5m)")
	loopRunCmd.Flags().BoolVarP(&loopRunDetach, "detach", "d", false, "Start the loop and print its ID without waiting")
	loopCmd.AddCommand(loopRunCmd)
}

func runLoopRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer closeDaemonClient(c)

	prompt := args[0]
	if prompt == "" {
		return &output.CommandError{Code: "MISSING_PROMPT", Message: "A prompt is required"}
	}

	cwd := loopRunCwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	req := &protocol.LoopRunRequest{
		Type:          "loop/run",
		Prompt:        prompt,
		Cwd:           cwd,
		Name:          strPtr(loopRunName),
		MaxIterations: &loopRunMaxIter,
	}

	if loopRunProvider != "" || loopRunModel != "" {
		resolved, err := cliutil.ResolveProviderModel(loopRunProvider, loopRunModel, c.ProvidersSnapshot())
		if err != nil {
			return err
		}
		// Prefer the shared AgentTemplate so the daemon can use all template
		// fields (system prompt, MCP servers, etc.) while still receiving legacy
		// provider/model for old daemons.
		req.AgentTemplate = &protocol.AgentTemplate{
			Provider: resolved.Provider,
			Cwd:      cwd,
			Model:    strPtr(resolved.Model),
		}
		req.Provider = strPtr(resolved.Provider)
		req.Model = strPtr(resolved.Model)
	}
	if loopRunVerifyCheck != "" {
		req.VerifyChecks = []string{loopRunVerifyCheck}
	}
	if loopRunVerifyPrompt != "" {
		req.VerifyPrompt = strPtr(loopRunVerifyPrompt)
	}
	if loopRunMaxTime > 0 {
		ms := int(loopRunMaxTime.Milliseconds())
		req.MaxTimeMs = &ms
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("run loop: %w", err)
	}

	var runResp protocol.LoopRunResponse
	if err := parseLoopResponse(resp, &runResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if runResp.Payload.Error != nil && *runResp.Payload.Error != "" {
		return &output.CommandError{Code: "LOOP_RUN_FAILED", Message: *runResp.Payload.Error}
	}
	if runResp.Payload.Loop == nil {
		return &output.CommandError{Code: "LOOP_RUN_FAILED", Message: "unexpected response from daemon"}
	}

	loop := runResp.Payload.Loop
	if loopRunDetach {
		return renderLoopRecord(loop, getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
	}

	return waitLoopFinish(ctx, c, loop.ID)
}

func waitLoopFinish(ctx context.Context, c *client.DaemonClient, loopID string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			loop, err := fetchLoop(ctx, c, loopID)
			if err != nil {
				return err
			}
			switch loop.Status {
			case "succeeded", "failed", "stopped":
				return renderLoopRecord(loop, getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
			}
		}
	}
}

func fetchLoop(ctx context.Context, c *client.DaemonClient, id string) (*protocol.LoopRecord, error) {
	resp, err := c.Request(ctx, &protocol.LoopInspectRequest{
		Type: "loop/inspect",
		ID:   id,
	})
	if err != nil {
		return nil, fmt.Errorf("inspect loop: %w", err)
	}
	var inspectResp protocol.LoopInspectResponse
	if err := parseLoopResponse(resp, &inspectResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if inspectResp.Payload.Error != nil && *inspectResp.Payload.Error != "" {
		return nil, &output.CommandError{Code: "LOOP_INSPECT_FAILED", Message: *inspectResp.Payload.Error}
	}
	if inspectResp.Payload.Loop == nil {
		return nil, &output.CommandError{Code: "LOOP_NOT_FOUND", Message: "loop not found"}
	}
	return inspectResp.Payload.Loop, nil
}
