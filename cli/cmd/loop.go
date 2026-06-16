package cmd

import (
	"github.com/spf13/cobra"
)

var loopCmd = &cobra.Command{
	Use:   "loop",
	Short: "Manage loops",
}

func init() {
	rootCmd.AddCommand(loopCmd)
}
