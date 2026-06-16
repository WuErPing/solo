// Command relay runs the Solo WebSocket relay server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WuErPing/solo/relay/internal/config"
	relaymetrics "github.com/WuErPing/solo/relay/internal/metrics"
	"github.com/WuErPing/solo/relay/internal/relay"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	_ = relaymetrics.Sessions

	store := relay.NewSessionStore(cfg.MaxBuffer, logger)
	srv := relay.NewServer(store, cfg.MaxBuffer, logger, cfg.AllowedOrigins)

	httpServer := &http.Server{
		Addr:    srv.Addr(cfg.Host, cfg.Port),
		Handler: srv.Handler(),
	}

	go func() {
		logger.Info("solo-relay starting", "host", cfg.Host, "port", cfg.Port, "maxBuffer", cfg.MaxBuffer, "allowedOrigins", cfg.AllowedOrigins)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}
