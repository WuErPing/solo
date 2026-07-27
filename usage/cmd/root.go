package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgPath    string
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "solo-usage",
	Short: "Fetch coding plan usage and quota from multiple platforms",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default ~/.solo/usage.json)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
}
