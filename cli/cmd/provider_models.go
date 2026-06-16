package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var providerModelsThinking bool

var providerModelsCmd = &cobra.Command{
	Use:   "models <provider>",
	Short: "List models for a provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runProviderModels,
}

func init() {
	providerModelsCmd.Flags().BoolVar(&providerModelsThinking, "thinking", false, "Show thinking options")
	providerCmd.AddCommand(providerModelsCmd)
}

func runProviderModels(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer closeDaemonClient(c)

	ps := c.ProvidersSnapshot()
	if ps == nil {
		_, _ = fmt.Fprintln(cmdStdout, "No providers available")
		return nil
	}

	providerName := args[0]
	var found *protocol.ProviderSnapshotEntry
	for i := range ps.Entries {
		if ps.Entries[i].Provider == providerName {
			found = &ps.Entries[i]
			break
		}
	}

	if found == nil {
		return &output.CommandError{
			Code:    "PROVIDER_NOT_FOUND",
			Message: fmt.Sprintf("Provider %q not found", providerName),
			Details: "Use `solo provider ls` to list available providers",
		}
	}

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)

	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(found.Models, nil), opts)
	}

	// Table output
	schema := &output.Schema{
		IDField: func(item interface{}) string { return item.(modelEntry).ID },
		Columns: []output.ColumnDef{
			{Header: "MODEL ID", FieldFunc: func(i interface{}) string { return i.(modelEntry).ID }, Width: 40},
			{Header: "LABEL", FieldFunc: func(i interface{}) string { return i.(modelEntry).Label }, Width: 25},
			{Header: "DEFAULT", FieldFunc: func(i interface{}) string {
				if i.(modelEntry).IsDefault {
					return "*"
				}
				return ""
			}, Width: 8},
		},
	}

	var items []interface{}
	for _, m := range found.Models {
		items = append(items, modelEntry{
			ID:        m.ID,
			Label:     m.Label,
			IsDefault: m.IsDefault,
		})
	}

	if len(items) == 0 {
		_, _ = fmt.Fprintln(cmdStdout, "No models available")
		return nil
	}

	return output.Render(cmdStdout, output.ListResult(items, schema), opts)
}

type modelEntry struct {
	ID        string
	Label     string
	IsDefault bool
}
