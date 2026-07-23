// Package config loads relay server configuration from environment variables.
package config

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Port           string
	Host           string
	MaxBuffer      int
	MaxConns       int
	LogLevel       slog.Level
	AllowedOrigins []string // CORS whitelist; empty = reject all non-empty Origin headers
}

func Load() Config {
	return Config{
		Port:           envOrDefault("PORT", "8080"),
		Host:           envOrDefault("HOST", "0.0.0.0"),
		MaxBuffer:      envOrDefaultInt("MAX_BUFFER", 200),
		MaxConns:       envOrDefaultInt("MAX_CONNS", 10000),
		LogLevel:       parseLogLevel(envOrDefault("LOG_LEVEL", "info")),
		AllowedOrigins: parseOrigins(envOrDefault("ALLOWED_ORIGINS", "https://solo.up2ai.top,http://localhost:19000")),
	}
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	var origins []string
	for _, o := range splitAndTrim(s) {
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func splitAndTrim(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
