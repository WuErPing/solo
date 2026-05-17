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
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := daemon.Stop(ctx); err != nil {
		logger.Error("error during shutdown", "error", err)
	}

	if releasePID != nil {
		releasePID()
	}
	logger.Info("daemon stopped")
}
