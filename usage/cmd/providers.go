package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/usage/internal/provider"
)

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List registered providers",
	Run: func(_ *cobra.Command, _ []string) {
		for _, name := range provider.Names() {
			fmt.Println(name)
		}
	},
}

func init() {
	rootCmd.AddCommand(providersCmd)
}
