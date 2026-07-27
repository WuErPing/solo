package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/usage/internal/config"
	"github.com/WuErPing/solo/usage/internal/output"
	"github.com/WuErPing/solo/usage/internal/provider"
)

var providerFilter string

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch usage from configured providers",
	RunE:  runFetch,
}

func init() {
	fetchCmd.Flags().StringVar(&providerFilter, "provider", "", "comma-separated provider names to fetch")
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(cmd *cobra.Command, _ []string) error {
	path := cfgPath
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	names := targetProviders(cfg)
	if len(names) == 0 {
		return fmt.Errorf("no enabled providers found in %s", path)
	}

	snapshots, errs := fetchAll(cmd.Context(), cfg, names)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(snapshots) == 0 {
		return fmt.Errorf("all providers failed")
	}

	if jsonOutput {
		return output.JSON(os.Stdout, snapshots)
	}
	output.Table(os.Stdout, snapshots)
	return nil
}

func targetProviders(cfg *config.File) []string {
	if providerFilter == "" {
		return cfg.EnabledProviders()
	}
	var names []string
	for _, n := range strings.Split(providerFilter, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

func fetchAll(ctx context.Context, cfg *config.File, names []string) ([]*provider.Snapshot, []error) {
	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		snapshots []*provider.Snapshot
		errs      []error
	)

	for _, name := range names {
		pcfg, ok := cfg.ToProviderConfig(name)
		if !ok {
			mu.Lock()
			errs = append(errs, fmt.Errorf("%s: not enabled or not configured", name))
			mu.Unlock()
			continue
		}

		p, err := provider.Create(name, pcfg)
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			snap, err := p.Fetch(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			snapshots = append(snapshots, snap)
		}()
	}

	wg.Wait()
	return snapshots, errs
}
