package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/usage/config"
)

//go:embed usage.json.example
var configTemplate string

var (
	initForce bool
	initPrint bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter usage.json config file",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing config file")
	initCmd.Flags().BoolVar(&initPrint, "print", false, "print the template to stdout instead of writing")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	if initPrint {
		fmt.Fprint(cmd.OutOrStdout(), configTemplate)
		return nil
	}

	path := cfgPath
	if path == "" {
		path = config.DefaultPath()
	}
	if path == "" {
		return fmt.Errorf("cannot resolve home directory; pass --config explicitly")
	}

	if _, err := os.Stat(path); err == nil {
		if !initForce {
			return fmt.Errorf("config file %s already exists (use --force to overwrite, or --print to view the template)", path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(configTemplate), 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	// Chmod explicitly: WriteFile's mode only applies when creating the file,
	// and the config will hold API keys and cookie paths.
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), `Wrote %s

Next steps:
  1. Edit the file and set "enabled": false for providers you don't use.
  2. Provide the referenced credentials:
     - export the env vars (e.g. export KIMI_API_KEY=sk-kimi-xxx), or
     - write cookie files, e.g. pbpaste > ~/.solo/xiaomimimo.cookie
  3. Run `+"`solo-usage fetch`"+` to verify.
`, path)
	return nil
}
