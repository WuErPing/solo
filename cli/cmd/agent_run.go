package cmd

import (
	"context"
	"encoding/json"
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
	agentRunDetach   bool
	agentRunTitle    string
	agentRunProvider string
	agentRunModel    string
	agentRunMode     string
	agentRunCwd      string
	agentRunLabel    []string
	agentRunTimeout  string
)

var agentRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Create and run an agent with a task",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAgentRun,
}

func init() {
	agentRunCmd.Flags().BoolVarP(&agentRunDetach, "detach", "d", false, "Run in background (detached)")
	agentRunCmd.Flags().StringVar(&agentRunTitle, "title", "", "Assign a title to the agent")
	agentRunCmd.Flags().StringVar(&agentRunProvider, "provider", "", "Agent provider (e.g. claude, mock)")
	agentRunCmd.Flags().StringVar(&agentRunModel, "model", "", "Model to use")
	agentRunCmd.Flags().StringVar(&agentRunMode, "mode", "", "Provider-specific mode")
	agentRunCmd.Flags().StringVar(&agentRunCwd, "cwd", "", "Working directory (default: current)")
	agentRunCmd.Flags().StringArrayVar(&agentRunLabel, "label", nil, "Add label(s) (key=value)")
	agentRunCmd.Flags().StringVar(&agentRunTimeout, "wait-timeout", "", "Max wait time (e.g. 30s, 5m)")
	agentCmd.AddCommand(agentRunCmd)
}

func runAgentRun(cmd *cobra.Command, args []string) error {
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

	// Resolve provider/model
	resolved, err := cliutil.ResolveProviderModel(agentRunProvider, agentRunModel, c.ProvidersSnapshot())
	if err != nil {
		return err
	}

	// Resolve cwd
	cwd := agentRunCwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Build labels
	labels := parseLabels(agentRunLabel)

	// Build config
	cfg := protocol.AgentSessionConfig{
		Provider: resolved.Provider,
		Cwd:      cwd,
	}
	if resolved.Model != "" {
		cfg.Model = strPtr(resolved.Model)
	}
	if agentRunMode != "" {
		cfg.ModeID = strPtr(agentRunMode)
	}
	if agentRunTitle != "" {
		cfg.Title = strPtr(agentRunTitle)
	}

	req := &protocol.CreateAgentRequest{
		Type:          "create_agent_request",
		Config:        cfg,
		InitialPrompt: &prompt,
		Labels:        labels,
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Parse response to get agent ID
	agentID, err := parseAgentCreatedResponse(resp)
	if err != nil {
		return err
	}

	if agentRunDetach {
		// Just print the agent ID and return
		opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
		if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
			result := output.SingleResult(map[string]string{
				"agentId": agentID,
				"status":  "created",
			}, nil)
			return output.Render(cmdStdout, result, opts)
		}
		if err := errFprintf(cmdStdout, "Agent %s created (detached)\n", shortenID(agentID)); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}

	// Foreground mode: stream output and wait for completion
	return runAgentForeground(ctx, c, agentID)
}

func runAgentForeground(ctx context.Context, c *client.DaemonClient, agentID string) error {
	result := waitForAgentFinish(ctx, c, agentID)
	if result.err != nil {
		return result.err
	}
	return renderRunResult(result.agent, mapRunStatus(result.status))
}

type waitForFinishResult struct {
	status string
	agent  *protocol.AgentSnapshotPayload
	err    error
}

func waitForAgentFinish(ctx context.Context, c *client.DaemonClient, agentID string) waitForFinishResult {
	if agentRunTimeout != "" {
		timeout, err := cliutil.ParseDuration(agentRunTimeout)
		if err != nil {
			return waitForFinishResult{err: &output.CommandError{Code: "INVALID_TIMEOUT", Message: "Invalid wait timeout value", Details: err.Error()}}
		}
		ms := int(timeout / time.Millisecond)
		req := &protocol.WaitForFinishRequest{
			Type:      "wait_for_finish_request",
			AgentID:   agentID,
			TimeoutMs: &ms,
		}
		return requestWaitForFinish(ctx, c, req)
	}

	req := &protocol.WaitForFinishRequest{
		Type:    "wait_for_finish_request",
		AgentID: agentID,
	}
	return requestWaitForFinish(ctx, c, req)
}

func requestWaitForFinish(ctx context.Context, c *client.DaemonClient, req *protocol.WaitForFinishRequest) waitForFinishResult {
	resp, err := c.Request(ctx, req)
	if err != nil {
		return waitForFinishResult{err: fmt.Errorf("wait for agent: %w", err)}
	}
	payload, _ := json.Marshal(resp.Message)
	var waitResp struct {
		Type    string `json:"type"`
		Payload struct {
			Status string                         `json:"status"`
			Final  *protocol.AgentSnapshotPayload `json:"final"`
			Error  *string                        `json:"error"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(payload, &waitResp); err != nil {
		return waitForFinishResult{err: fmt.Errorf("parse wait response: %w", err)}
	}
	if waitResp.Type == "rpc_error" {
		var rpcErr struct {
			Payload struct {
				Error string `json:"error"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(payload, &rpcErr); err != nil {
			return waitForFinishResult{err: fmt.Errorf("parse rpc error: %w", err)}
		}
		return waitForFinishResult{err: &output.CommandError{Code: "WAIT_FOR_FINISH_FAILED", Message: rpcErr.Payload.Error}}
	}
	if waitResp.Payload.Status == "timeout" {
		return waitForFinishResult{err: &output.CommandError{Code: "TIMEOUT", Message: "Timed out waiting for agent to finish"}}
	}
	if waitResp.Payload.Status == "error" {
		msg := "Agent failed"
		if waitResp.Payload.Error != nil && *waitResp.Payload.Error != "" {
			msg = *waitResp.Payload.Error
		}
		return waitForFinishResult{status: "error", agent: waitResp.Payload.Final, err: &output.CommandError{Code: "AGENT_ERROR", Message: msg}}
	}
	if waitResp.Payload.Status == "permission" {
		return waitForFinishResult{status: "permission", agent: waitResp.Payload.Final, err: &output.CommandError{Code: "PERMISSION_REQUIRED", Message: "Agent is waiting for permission"}}
	}
	return waitForFinishResult{status: waitResp.Payload.Status, agent: waitResp.Payload.Final}
}

func mapRunStatus(status string) string {
	if status == "idle" {
		return "completed"
	}
	return status
}

type agentRunResultItem struct {
	AgentID  string  `json:"agentId"`
	ShortID  string  `json:"-"`
	Status   string  `json:"status"`
	Provider string  `json:"provider"`
	CWD      string  `json:"cwd"`
	Title    *string `json:"title"`
}

func renderRunResult(agent *protocol.AgentSnapshotPayload, status string) error {
	if agent == nil {
		return &output.CommandError{Code: "AGENT_FETCH_FAILED", Message: "Agent finished without a final snapshot"}
	}
	item := &agentRunResultItem{
		AgentID:  agent.ID,
		ShortID:  shortenID(agent.ID),
		Status:   status,
		Provider: formatProvider(*agent),
		CWD:      agent.Cwd,
		Title:    agent.Title,
	}
	schema := &output.Schema{
		IDField: func(item interface{}) string {
			return item.(*agentRunResultItem).AgentID
		},
		Columns: []output.ColumnDef{
			{Header: "AGENT ID", FieldFunc: func(i interface{}) string { return i.(*agentRunResultItem).AgentID }, Width: 12},
			{Header: "STATUS", FieldFunc: func(i interface{}) string { return i.(*agentRunResultItem).Status }, Width: 10},
			{Header: "PROVIDER", FieldFunc: func(i interface{}) string { return i.(*agentRunResultItem).Provider }, Width: 10},
			{Header: "CWD", FieldFunc: func(i interface{}) string { return i.(*agentRunResultItem).CWD }, Width: 30},
			{Header: "TITLE", FieldFunc: func(i interface{}) string {
				if i.(*agentRunResultItem).Title == nil {
					return ""
				}
				return *i.(*agentRunResultItem).Title
			}, Width: 20},
		},
	}
	return output.Render(cmdStdout, output.SingleResult(item, schema), getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor))
}

func printStreamEvent(event interface{}) error {
	data, _ := json.Marshal(event)
	var evt struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
		Item struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"item,omitempty"`
	}
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("parse stream event: %w", err)
	}

	switch evt.Type {
	case "timeline":
		return printTimelineItem(evt.Item.Type, evt.Item.Text, evt.Item.Name)
	case "permission_requested":
		return errFprintln(cmdStdout, "\n[Permission Required]")
	case "turn_failed":
		return errFprintln(cmdStdout, "\n[Turn Failed]")
	case "attention_required":
		return errFprintln(cmdStdout, "\n[Attention Required]")
	}
	return nil
}

func printTimelineItem(itemType, text, name string) error {
	switch itemType {
	case "assistant_message":
		return errFprint(cmdStdout, text)
	case "reasoning":
		return errFprintf(cmdStdout, "\n[Reasoning] %s", text)
	case "tool_call":
		return errFprintf(cmdStdout, "\n[Tool: %s]", name)
	case "error":
		return errFprintf(cmdStdout, "\n[Error] %s", text)
	case "user_message":
		return errFprintf(cmdStdout, "\n[User] %s", text)
	}
	return nil
}

func parseAgentCreatedResponse(resp *protocol.WSOutboundMessage) (string, error) {
	payload, _ := json.Marshal(resp.Message)
	var created struct {
		Payload struct {
			AgentID string `json:"agentId"`
			Status  string `json:"status"`
			Error   string `json:"error,omitempty"`
		} `json:"payload"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &created); err != nil {
		return "", fmt.Errorf("parse create response: %w", err)
	}

	// Check for RPC error
	var rpcErr struct {
		Payload struct {
			Error string `json:"error"`
		} `json:"payload"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &rpcErr); err != nil {
		return "", fmt.Errorf("parse rpc error: %w", err)
	}
	if rpcErr.Type == "rpc_error" && rpcErr.Payload.Error != "" {
		return "", &output.CommandError{Code: "AGENT_CREATE_FAILED", Message: rpcErr.Payload.Error}
	}

	if created.Payload.AgentID != "" {
		return created.Payload.AgentID, nil
	}

	return "", &output.CommandError{Code: "AGENT_CREATE_FAILED", Message: "unexpected response from daemon"}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
