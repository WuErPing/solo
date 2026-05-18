package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
)

var providerLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List available providers",
	RunE:  runProviderLs,
}

func init() {
	providerCmd.AddCommand(providerLsCmd)
}

func runProviderLs(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	ps := c.ProvidersSnapshot()
	if ps == nil || len(ps.Entries) == 0 {
		fmt.Fprintln(output.Stdout, "No providers available")
		return nil
	}

	opts := getOutputOpts()

	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(ps.Entries, nil), opts)
	}

	// Table output
	schema := &output.Schema{
		IDField: func(item interface{}) string {
			return item.(providerEntry).Provider
		},
		Columns: []output.ColumnDef{
			{Header: "PROVIDER", FieldFunc: func(i interface{}) string { return i.(providerEntry).Provider }, Width: 15},
			{Header: "STATUS", FieldFunc: func(i interface{}) string { return string(i.(providerEntry).Status) }, Width: 12,
				ColorFunc: func(val string, _ interface{}) string {
					switch val {
					case "ready":
						return "green"
					case "error", "unavailable":
						return "red"
					case "loading":
						return "yellow"
					}
					return ""
				}},
			{Header: "LABEL", FieldFunc: func(i interface{}) string { return i.(providerEntry).Label }, Width: 15},
			{Header: "MODELS", FieldFunc: func(i interface{}) string { return fmt.Sprintf("%d", i.(providerEntry).ModelCount) }, Width: 8},
		},
	}

	var items []interface{}
	for _, entry := range ps.Entries {
		items = append(items, providerEntry{
			Provider:   entry.Provider,
			Status:     string(entry.Status),
			Label:      entry.Label,
			ModelCount: len(entry.Models),
		})
	}

	return output.Render(output.ListResult(items, schema), opts)
}

type providerEntry struct {
	Provider   string
	Status     string
	Label      string
	ModelCount int
}
