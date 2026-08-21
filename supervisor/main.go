// Command solo-supervisor spawns the Solo daemon as a child process and
// watches it: requested restarts (exit code 42) are respawned immediately,
// crashes are respawned with backoff, and a clean exit (code 0) stops the
// supervisor too. It owns ~/.solo/solo.pid and the daemon log file while
// supervising.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/WuErPing/solo/supervisor/internal/supervisor"
)

func main() {
	var (
		port    = flag.String("port", "", "Daemon listen port (passed to the daemon as PORT)")
		home    = flag.String("home", "", "Solo home directory (passed to the daemon as SOLO_HOME)")
		noRelay = flag.Bool("no-relay", false, "Disable relay (SOLO_RELAY_ENABLED=false)")
		noMCP   = flag.Bool("no-mcp", false, "Disable MCP (SOLO_MCP_ENABLED=false)")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := supervisor.Config{Logger: logger}
	if *port != "" {
		cfg.ExtraEnv = append(cfg.ExtraEnv, "PORT="+*port)
	}
	if *home != "" {
		cfg.ExtraEnv = append(cfg.ExtraEnv, "SOLO_HOME="+*home)
		cfg.SoloHome = *home
	}
	if *noRelay {
		cfg.ExtraEnv = append(cfg.ExtraEnv, "SOLO_RELAY_ENABLED=false")
	}
	if *noMCP {
		cfg.ExtraEnv = append(cfg.ExtraEnv, "SOLO_MCP_ENABLED=false")
	}

	sup, err := supervisor.New(cfg)
	if err != nil {
		logger.Error("cannot start supervisor", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(sup.Run(ctx))
}
