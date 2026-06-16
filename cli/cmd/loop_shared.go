package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

type loopListItem struct {
	ID      string
	ShortID string
	Name    string
	Status  string
	CWD     string
	Created string
}

type loopRecordItem struct {
	ID            string
	Name          string
	Status        string
	Prompt        string
	CWD           string
	Provider      string
	MaxIterations *int
	Iterations    int
	Created       string
}

func renderLoopRecord(loop *protocol.LoopRecord, opts output.OutputOptions) error {
	name := "-"
	if loop.Name != nil && *loop.Name != "" {
		name = *loop.Name
	}
	maxIter := "unlimited"
	if loop.MaxIterations != nil {
		maxIter = fmt.Sprintf("%d", *loop.MaxIterations)
	}
	item := &loopRecordItem{
		ID:            loop.ID,
		Name:          name,
		Status:        loop.Status,
		Prompt:        loop.Prompt,
		CWD:           shortenPath(loop.Cwd),
		Provider:      loop.Provider,
		MaxIterations: loop.MaxIterations,
		Iterations:    len(loop.Iterations),
		Created:       relativeTime(loop.CreatedAt),
	}
	_ = maxIter

	schema := &output.Schema{
		IDField: func(item interface{}) string { return item.(*loopRecordItem).ID },
		Columns: []output.ColumnDef{
			{Header: "ID", FieldFunc: func(i interface{}) string { return shortenID(i.(*loopRecordItem).ID) }, Width: 8},
			{Header: "NAME", FieldFunc: func(i interface{}) string { return i.(*loopRecordItem).Name }, Width: 20},
			{Header: "STATUS", FieldFunc: func(i interface{}) string { return i.(*loopRecordItem).Status }, Width: 10},
			{Header: "PROVIDER", FieldFunc: func(i interface{}) string { return i.(*loopRecordItem).Provider }, Width: 12},
			{Header: "ITERATIONS", FieldFunc: func(i interface{}) string { return fmt.Sprintf("%d", i.(*loopRecordItem).Iterations) }, Width: 10},
			{Header: "CWD", FieldFunc: func(i interface{}) string { return i.(*loopRecordItem).CWD }, Width: 30},
			{Header: "CREATED", FieldFunc: func(i interface{}) string { return i.(*loopRecordItem).Created }, Width: 15},
		},
	}

	return output.Render(cmdStdout, output.SingleResult(item, schema), opts)
}

func renderLoopList(loops []protocol.LoopListItem, opts output.OutputOptions) error {
	var items []interface{}
	for _, loop := range loops {
		name := "-"
		if loop.Name != nil && *loop.Name != "" {
			name = *loop.Name
		}
		items = append(items, &loopListItem{
			ID:      loop.ID,
			ShortID: shortenID(loop.ID),
			Name:    name,
			Status:  loop.Status,
			CWD:     shortenPath(loop.Cwd),
			Created: relativeTime(loop.CreatedAt),
		})
	}

	schema := &output.Schema{
		IDField: func(item interface{}) string { return item.(*loopListItem).ShortID },
		Columns: []output.ColumnDef{
			{Header: "ID", FieldFunc: func(i interface{}) string { return i.(*loopListItem).ShortID }, Width: 8},
			{Header: "NAME", FieldFunc: func(i interface{}) string { return i.(*loopListItem).Name }, Width: 20},
			{Header: "STATUS", FieldFunc: func(i interface{}) string { return i.(*loopListItem).Status }, Width: 10},
			{Header: "CWD", FieldFunc: func(i interface{}) string { return i.(*loopListItem).CWD }, Width: 30},
			{Header: "CREATED", FieldFunc: func(i interface{}) string { return i.(*loopListItem).Created }, Width: 15},
		},
	}

	return output.Render(cmdStdout, output.ListResult(items, schema), opts)
}

func resolveLoopID(ctx context.Context, c *client.DaemonClient, idOrPrefix string) (string, error) {
	if idOrPrefix == "" {
		return "", &output.CommandError{Code: "MISSING_LOOP_ID", Message: "Loop ID is required"}
	}

	resp, err := c.Request(ctx, &protocol.LoopListRequest{Type: "loop/list"})
	if err != nil {
		return "", fmt.Errorf("list loops: %w", err)
	}
	var listResp protocol.LoopListResponse
	if err := parseLoopResponse(resp, &listResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if listResp.Payload.Error != nil && *listResp.Payload.Error != "" {
		return "", &output.CommandError{Code: "LOOP_LIST_FAILED", Message: *listResp.Payload.Error}
	}

	// Exact match first.
	for _, loop := range listResp.Payload.Loops {
		if loop.ID == idOrPrefix {
			return loop.ID, nil
		}
	}
	// Prefix match.
	var prefixMatches []string
	for _, loop := range listResp.Payload.Loops {
		if strings.HasPrefix(loop.ID, idOrPrefix) {
			prefixMatches = append(prefixMatches, loop.ID)
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0], nil
	}
	if len(prefixMatches) > 1 {
		return "", &output.CommandError{Code: "AMBIGUOUS_LOOP_ID", Message: fmt.Sprintf("%q matches multiple loops", idOrPrefix)}
	}

	return "", &output.CommandError{Code: "LOOP_NOT_FOUND", Message: fmt.Sprintf("no loop matching %q", idOrPrefix)}
}

func parseLoopResponse(resp *protocol.WSOutboundMessage, target interface{}) error {
	payload, _ := json.Marshal(resp.Message)
	return json.Unmarshal(payload, target)
}
