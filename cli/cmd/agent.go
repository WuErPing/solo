// Package cmd implements the Solo CLI commands.
package cmd

import (
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
