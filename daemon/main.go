// Command solo is the local daemon backend for the Solo application.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/pidlock"
	"github.com/WuErPing/solo/daemon/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	var releasePID func()
	if !cfg.Supervised {
		releasePID, err = pidlock.Acquire(cfg.SoloHome)
		if err != nil {
			logger.Error("cannot acquire PID lock", "error", err)
			os.Exit(1)
		}
	}

	daemon, err := server.NewDaemon(cfg, logger)
	if err != nil {
		logger.Error("failed to create daemon", "error", err)
		os.Exit(1)
	}

	if err := daemon.Start(); err != nil {
		logger.Error("failed to start daemon", "error", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	exitCode := 0
	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
	case req := <-daemon.ExitRequested():
		// A client asked for restart/shutdown via the WS protocol. Exit with
		// the request's code so a supervising process can tell an intentional
		// restart (respawn) from a clean shutdown (no respawn).
		logger.Info("exit requested, shutting down", "code", req.Code, "reason", req.Reason)
		exitCode = req.Code
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := daemon.Stop(ctx); err != nil {
		logger.Error("error during shutdown", "error", err)
	}

	if releasePID != nil {
		releasePID()
	}
	logger.Info("daemon stopped")
	os.Exit(exitCode)
}
