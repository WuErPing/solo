package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/config"
	"github.com/WuErPing/solo/cli/internal/output"
)

var (
	flagHost      string
	flagFormat    string
	flagJSON      bool
	flagQuiet     bool
	flagNoHeaders bool
	flagNoColor   bool

	// cmdStdout and cmdStderr are the destinations for command output.
	// They are package-level variables so tests can swap them for capture.
	cmdStdout io.Writer = os.Stdout
	cmdStderr io.Writer = os.Stderr
)

// closeDaemonClient closes c, swallowing close errors; safe for defer cleanup.
func closeDaemonClient(c *client.DaemonClient) { _ = c.Close() }

// warnIfDropped prints a warning to stderr when the subscription dropped
// messages because the consumer could not keep up.
func warnIfDropped(sub *client.Subscription) {
	if n := sub.DroppedCount(); n > 0 {
		_, _ = fmt.Fprintf(cmdStderr, "warning: %d message(s) dropped due to slow consumption\n", n)
	}
}

// errFprintf wraps fmt.Fprintf and returns its error, so callers cannot
// accidentally discard the write result (satisfies errcheck).
func errFprintf(w io.Writer, format string, a ...any) error {
	_, err := fmt.Fprintf(w, format, a...)
	return err
}

// errFprintln wraps fmt.Fprintln and returns its error.
func errFprintln(w io.Writer, a ...any) error {
	_, err := fmt.Fprintln(w, a...)
	return err
}

// errFprint wraps fmt.Fprint and returns its error.
func errFprint(w io.Writer, a ...any) error {
	_, err := fmt.Fprint(w, a...)
	return err
}

var rootCmd = &cobra.Command{
	Use:           "solo",
	Short:         "Solo CLI - manage AI coding agents from the command line",
	Version:       config.Version,
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
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getOutputOpts resolves the output options from the provided flags.
func getOutputOpts(format string, json bool, quiet bool, noHeaders bool, noColor bool) output.OutputOptions {
	if noColor {
		output.DisableColor()
	}
	if quiet {
		return output.OutputOptions{Format: output.FormatQuiet, NoHeaders: noHeaders, NoColor: noColor}
	}
	if json {
		return output.OutputOptions{Format: output.FormatJSON, NoHeaders: noHeaders, NoColor: noColor}
	}
	f, err := output.ParseOutputFormat(format)
	if err != nil {
		_, _ = fmt.Fprintln(cmdStderr, err.Error())
		os.Exit(1)
	}
	return output.OutputOptions{Format: f, NoHeaders: noHeaders, NoColor: noColor}
}

// newClient creates a connected DaemonClient using the provided host.
func newClient(ctx context.Context, host string) (*client.DaemonClient, error) {
	clientID, err := client.GetOrCreateClientID()
	if err != nil {
		return nil, fmt.Errorf("get client ID: %w", err)
	}

	wsURL, err := client.ResolveHost(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	c, err := client.NewDaemonClient(ctx, host, clientID)
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
