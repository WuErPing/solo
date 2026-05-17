package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagHost      string
	flagFormat    string
	flagJSON      bool
	flagQuiet     bool
	flagNoHeaders bool
	flagNoColor   bool
)

var rootCmd = &cobra.Command{
	Use:           "solo",
	Short:         "Solo CLI - manage AI coding agents from the command line",
	Version:       "0.1.0",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "Daemon host (default: 127.0.0.1:17612)")
	rootCmd.PersistentFlags().StringVarP(&flagFormat, "format", "o", "table", "Output format (table, json, yaml, quiet)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output in JSON format (alias for --format json)")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Minimal output (IDs only)")
	rootCmd.PersistentFlags().BoolVar(&flagNoHeaders, "no-headers", false, "Omit table headers")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")

	rootCmd.Flags().BoolP("version", "v", false, "output the version number")

	// Hide auto-generated completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getOutputOpts resolves the output options from global flags.
func getOutputOpts() output.OutputOptions {
	noColor := flagNoColor
	if noColor {
		output.DisableColor()
	}
	if flagQuiet {
		return output.OutputOptions{Format: output.FormatQuiet, NoHeaders: flagNoHeaders, NoColor: noColor}
	}
	if flagJSON {
		return output.OutputOptions{Format: output.FormatJSON, NoHeaders: flagNoHeaders, NoColor: noColor}
	}
	format, err := output.ParseOutputFormat(flagFormat)
	if err != nil {
		fmt.Fprintln(output.Stderr, err.Error())
		os.Exit(1)
	}
	return output.OutputOptions{Format: format, NoHeaders: flagNoHeaders, NoColor: noColor}
}

// newClient creates a connected DaemonClient using the global --host flag.
func newClient(ctx context.Context) (*client.DaemonClient, error) {
	clientID, err := client.GetOrCreateClientID()
	if err != nil {
		return nil, fmt.Errorf("get client ID: %w", err)
	}

	wsURL, err := client.ResolveHost(flagHost)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	c, err := client.NewDaemonClient(ctx, flagHost, clientID)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon at %s: %w\nStart the daemon with: solo daemon start", wsURL, err)
	}
	return c, nil
}

// resolveAgentID resolves an agent ID from a partial ID or name.
func resolveAgentID(idOrName string, agents []agentEntry) string {
	if idOrName == "" || len(agents) == 0 {
		return ""
	}
	query := toLower(idOrName)

	// Exact ID match
	for _, a := range agents {
		if a.ID == idOrName {
			return a.ID
		}
	}

	// ID prefix match
	var prefixMatches []string
	for _, a := range agents {
		if toLower(a.ID[:min(len(a.ID), len(query))]) == query {
			prefixMatches = append(prefixMatches, a.ID)
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0]
	}

	// Title match
	var titleMatches []string
	for _, a := range agents {
		if a.Title != "" && toLower(a.Title) == query {
			titleMatches = append(titleMatches, a.ID)
		}
	}
	if len(titleMatches) == 1 {
		return titleMatches[0]
	}

	// Partial title match
	var partialMatches []string
	for _, a := range agents {
		if a.Title != "" && contains(toLower(a.Title), query) {
			partialMatches = append(partialMatches, a.ID)
		}
	}
	if len(partialMatches) == 1 {
		return partialMatches[0]
	}

	// Return first prefix match if ambiguous
	if len(prefixMatches) > 0 {
		return prefixMatches[0]
	}

	return ""
}

type agentEntry struct {
	ID    string
	Title string
}

func toLower(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result = append(result, c)
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
