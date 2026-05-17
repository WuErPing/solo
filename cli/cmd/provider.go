package cmd

import (
	"github.com/spf13/cobra"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage agent providers",
}

func init() {
	rootCmd.AddCommand(providerCmd)
}
