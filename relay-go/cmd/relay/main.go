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

	"github.com/WuErPing/solo/relay/internal/accesslog"
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
	srv := relay.NewServer(store, cfg.MaxBuffer, int64(cfg.MaxConns), logger, cfg.AllowedOrigins)

	// JSON-lines access log for offline analysis; empty path keeps it off.
	var accessLogger *accesslog.Logger
	if cfg.AccessLogPath != "" {
		al, err := accesslog.Open(cfg.AccessLogPath)
		if err != nil {
			logger.Error("failed to open access log", "path", cfg.AccessLogPath, "error", err)
			os.Exit(1)
		}
		accessLogger = al
		srv.SetAccessLogger(al)
	}

	httpServer := &http.Server{
		Addr:    srv.Addr(cfg.Host, cfg.Port),
		Handler: srv.Handler(),
		// ReadHeaderTimeout bounds the upgrade-request header phase (slowloris
		// defense). Read/WriteTimeout are intentionally unset: they apply to
		// hijacked WebSocket connections and would kill long-lived sessions.
		// Per-connection liveness is enforced by read deadlines in readPump.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		logger.Info("solo-relay starting", "host", cfg.Host, "port", cfg.Port, "maxBuffer", cfg.MaxBuffer, "maxConns", cfg.MaxConns, "allowedOrigins", cfg.AllowedOrigins)
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

	if accessLogger != nil {
		// Note: ws_close events for connections still alive at shutdown are
		// dropped — Shutdown does not wait for hijacked WebSocket handlers.
		if err := accessLogger.Close(); err != nil {
			logger.Warn("access log close failed", "error", err)
		}
	}

	logger.Info("server stopped gracefully")
}
