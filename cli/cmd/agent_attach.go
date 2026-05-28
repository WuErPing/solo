package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var agentAttachCmd = &cobra.Command{
	Use:   "attach <id>",
	Short: "Attach to a running agent's output stream",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentAttach,
}

func init() {
	agentCmd.AddCommand(agentAttachCmd)
}

func runAgentAttach(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer c.Close()

	agentID := args[0]

	// First fetch the agent to resolve the ID and check it exists
	resolvedID, err := fetchAndResolveAgentID(ctx, c, agentID)
	if err != nil {
		return err
	}

	if err := errFprintf(cmdStdout, "Attaching to agent %s...\n(Press Ctrl+C to detach)\n\n", shortenID(resolvedID)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	// Fetch existing timeline
	if err := printExistingTimeline(ctx, c, resolvedID); err != nil {
		return fmt.Errorf("print timeline: %w", err)
	}

	// Subscribe to new events
	streams := c.Subscribe("agent_stream")
	defer c.Unsubscribe("agent_stream", streams)

	for {
		select {
		case msg := <-streams:
			if msg == nil {
				return nil
			}
			payload, _ := json.Marshal(msg.Message)
			var streamMsg struct {
				Payload protocol.AgentStreamPayload `json:"payload"`
			}
			json.Unmarshal(payload, &streamMsg)
			stream := streamMsg.Payload
			if stream.AgentID != resolvedID {
				continue
			}
			if err := printStreamEvent(stream.Event); err != nil {
				return fmt.Errorf("write stream event: %w", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func printExistingTimeline(ctx context.Context, c *client.DaemonClient, agentID string) error {
	req := &protocol.FetchAgentTimelineRequest{
		Type:       "fetch_agent_timeline_request",
		AgentID:    agentID,
		RequestID:  c.GenerateRequestID(),
		Direction:  strPtr("tail"),
		Limit:      intPtr(200),
		Projection: strPtr("projected"),
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return nil
	}

	payload, _ := json.Marshal(resp.Message)
	var timeline struct {
		Payload struct {
			Entries []struct {
				Item struct {
					Type string `json:"type"`
					Text string `json:"text,omitempty"`
					Name string `json:"name,omitempty"`
				} `json:"item"`
			} `json:"entries"`
		} `json:"payload"`
	}
	json.Unmarshal(payload, &timeline)

	for _, entry := range timeline.Payload.Entries {
		if err := printTimelineItem(entry.Item.Type, entry.Item.Text, entry.Item.Name); err != nil {
			return err
		}
	}
	return nil
}

func fetchAndResolveAgentID(ctx context.Context, c *client.DaemonClient, idOrPrefix string) (string, error) {
	// Try direct fetch first
	req := &protocol.FetchAgentRequest{
		Type:      "fetch_agent_request",
		AgentID:   idOrPrefix,
		RequestID: c.GenerateRequestID(),
	}

	resp, err := c.Request(ctx, req)
	if err == nil {
		payload, _ := json.Marshal(resp.Message)
		var fetchResp struct {
			Payload struct {
				Agent *protocol.AgentSnapshotPayload `json:"agent"`
				Error *string                        `json:"error,omitempty"`
			} `json:"payload"`
		}
		json.Unmarshal(payload, &fetchResp)
		if fetchResp.Payload.Agent != nil {
			return fetchResp.Payload.Agent.ID, nil
		}
	}

	// Try listing all agents and resolving by prefix/name
	listReq := &protocol.FetchAgentsRequest{
		Type:      "fetch_agents_request",
		RequestID: c.GenerateRequestID(),
	}
	scope := "active"
	listReq.Scope = &scope

	listResp, err := c.Request(ctx, listReq)
	if err != nil {
		return "", &output.CommandError{Code: "AGENT_NOT_FOUND", Message: fmt.Sprintf("No agent found matching: %s", idOrPrefix)}
	}

	payload, _ := json.Marshal(listResp.Message)
	var fetchResp struct {
		Payload struct {
			Entries []struct {
				Agent protocol.AgentSnapshotPayload `json:"agent"`
			} `json:"entries"`
		} `json:"payload"`
	}
	json.Unmarshal(payload, &fetchResp)

	var agents []agentEntry
	for _, e := range fetchResp.Payload.Entries {
		title := ""
		if e.Agent.Title != nil {
			title = *e.Agent.Title
		}
		agents = append(agents, agentEntry{ID: e.Agent.ID, Title: title})
	}

	resolved := resolveAgentID(idOrPrefix, agents)
	if resolved == "" {
		return "", &output.CommandError{Code: "AGENT_NOT_FOUND", Message: fmt.Sprintf("No agent found matching: %s", idOrPrefix)}
	}
	return resolved, nil
}

func intPtr(v int) *int { return &v }
