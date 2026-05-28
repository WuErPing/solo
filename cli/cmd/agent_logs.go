package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/protocol"
)

var (
	agentLogsFollow bool
	agentLogsTail   int
	agentLogsFilter string
)

var agentLogsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "View agent activity/timeline",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentLogs,
}

func init() {
	agentLogsCmd.Flags().BoolVarP(&agentLogsFollow, "follow", "f", false, "Follow log output (streaming)")
	agentLogsCmd.Flags().IntVar(&agentLogsTail, "tail", 0, "Show last N entries")
	agentLogsCmd.Flags().StringVar(&agentLogsFilter, "filter", "", "Filter by event type (tools, text, errors)")
	agentCmd.AddCommand(agentLogsCmd)
}

func runAgentLogs(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer c.Close()

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}

	// Fetch timeline
	req := &protocol.FetchAgentTimelineRequest{
		Type:      "fetch_agent_timeline_request",
		AgentID:   agentID,
		RequestID: c.GenerateRequestID(),
		Direction: strPtr("tail"),
	}

	if agentLogsTail > 0 {
		req.Limit = intPtr(agentLogsTail)
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("fetch timeline: %w", err)
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

	if len(timeline.Payload.Entries) == 0 {
		if err := errFprintln(cmdStdout, "No activity to display."); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}

	for _, entry := range timeline.Payload.Entries {
		if matchesLogFilter(entry.Item.Type, agentLogsFilter) {
			if err := printLogEntry(entry.Item.Type, entry.Item.Text, entry.Item.Name); err != nil {
				return fmt.Errorf("write log entry: %w", err)
			}
		}
	}

	// If --follow, subscribe to streaming events
	if !agentLogsFollow {
		return nil
	}

	streams := c.Subscribe("agent_stream")
	defer c.Unsubscribe("agent_stream", streams)

	if err := errFprintln(cmdStdout, "\n--- streaming ---"); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	for {
		select {
		case msg := <-streams:
			if msg == nil {
				return nil
			}
			sp, _ := json.Marshal(msg.Message)
			var streamMsg struct {
				Payload protocol.AgentStreamPayload `json:"payload"`
			}
			json.Unmarshal(sp, &streamMsg)
			stream := streamMsg.Payload
			if stream.AgentID != agentID {
				continue
			}
			evtData, _ := json.Marshal(stream.Event)
			var evt struct {
				Type string `json:"type"`
				Item struct {
					Type string `json:"type"`
					Text string `json:"text,omitempty"`
					Name string `json:"name,omitempty"`
				} `json:"item,omitempty"`
			}
			json.Unmarshal(evtData, &evt)
			if evt.Type == "timeline" && matchesLogFilter(evt.Item.Type, agentLogsFilter) {
				if err := printLogEntry(evt.Item.Type, evt.Item.Text, evt.Item.Name); err != nil {
					return fmt.Errorf("write log entry: %w", err)
				}
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func matchesLogFilter(itemType, filter string) bool {
	if filter == "" {
		return true
	}
	f := toLower(filter)
	t := toLower(itemType)
	switch f {
	case "tools":
		return t == "tool_call"
	case "text":
		return t == "user_message" || t == "assistant_message" || t == "reasoning"
	case "errors":
		return t == "error"
	default:
		return t == f
	}
}

func printLogEntry(itemType, text, name string) error {
	switch itemType {
	case "assistant_message":
		return errFprintf(cmdStdout, "  %s\n", text)
	case "reasoning":
		return errFprintf(cmdStdout, "  [Reasoning] %s\n", text)
	case "tool_call":
		return errFprintf(cmdStdout, "  [Tool: %s]\n", name)
	case "error":
		return errFprintf(cmdStdout, "  [Error] %s\n", text)
	case "user_message":
		return errFprintf(cmdStdout, "  [User] %s\n", text)
	default:
		return errFprintf(cmdStdout, "  [%s] %s\n", itemType, text)
	}
}
