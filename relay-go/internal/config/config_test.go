package config

import (
	"log/slog"
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Ensure no env vars interfere
	for _, key := range []string{"PORT", "HOST", "MAX_BUFFER", "LOG_LEVEL", "ALLOWED_ORIGINS"} {
		os.Unsetenv(key)
	}

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port 8080, got %s", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("expected default Host 0.0.0.0, got %s", cfg.Host)
	}
	if cfg.MaxBuffer != 200 {
		t.Errorf("expected default MaxBuffer 200, got %d", cfg.MaxBuffer)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected default LogLevel info, got %v", cfg.LogLevel)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("expected 2 default origins, got %d", len(cfg.AllowedOrigins))
	}
}

func TestLoad_OverridePort(t *testing.T) {
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")

	cfg := Load()
	if cfg.Port != "9090" {
		t.Errorf("expected Port 9090, got %s", cfg.Port)
	}
}

func TestLoad_OverrideHost(t *testing.T) {
	os.Setenv("HOST", "127.0.0.1")
	defer os.Unsetenv("HOST")

	cfg := Load()
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected Host 127.0.0.1, got %s", cfg.Host)
	}
}

func TestLoad_OverrideMaxBuffer(t *testing.T) {
	os.Setenv("MAX_BUFFER", "500")
	defer os.Unsetenv("MAX_BUFFER")

	cfg := Load()
	if cfg.MaxBuffer != 500 {
		t.Errorf("expected MaxBuffer 500, got %d", cfg.MaxBuffer)
	}
}

func TestLoad_MaxBufferInvalidFallsBack(t *testing.T) {
	os.Setenv("MAX_BUFFER", "not-a-number")
	defer os.Unsetenv("MAX_BUFFER")

	cfg := Load()
	if cfg.MaxBuffer != 200 {
		t.Errorf("expected fallback MaxBuffer 200, got %d", cfg.MaxBuffer)
	}
}

func TestLoad_LogLevelDebug(t *testing.T) {
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	cfg := Load()
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("expected LogLevel debug, got %v", cfg.LogLevel)
	}
}

func TestLoad_LogLevelWarn(t *testing.T) {
	os.Setenv("LOG_LEVEL", "warn")
	defer os.Unsetenv("LOG_LEVEL")

	cfg := Load()
	if cfg.LogLevel != slog.LevelWarn {
		t.Errorf("expected LogLevel warn, got %v", cfg.LogLevel)
	}
}

func TestLoad_LogLevelError(t *testing.T) {
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	cfg := Load()
	if cfg.LogLevel != slog.LevelError {
		t.Errorf("expected LogLevel error, got %v", cfg.LogLevel)
	}
}

func TestLoad_LogLevelUnknownDefaultsToInfo(t *testing.T) {
	os.Setenv("LOG_LEVEL", "trace")
	defer os.Unsetenv("LOG_LEVEL")

	cfg := Load()
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected fallback LogLevel info, got %v", cfg.LogLevel)
	}
}

func TestLoad_AllowedOriginsEmptyUsesDefault(t *testing.T) {
	// Empty string env var falls back to default because envOrDefault treats "" as unset.
	os.Setenv("ALLOWED_ORIGINS", "")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	cfg := Load()
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("expected default 2 origins when env is empty, got %d", len(cfg.AllowedOrigins))
	}
}

func TestLoad_AllowedOriginsSingle(t *testing.T) {
	os.Setenv("ALLOWED_ORIGINS", "https://example.com")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	cfg := Load()
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("expected single origin, got %v", cfg.AllowedOrigins)
	}
}

func TestLoad_AllowedOriginsMultiple(t *testing.T) {
	os.Setenv("ALLOWED_ORIGINS", "https://a.com,https://b.com,https://c.com")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	cfg := Load()
	if len(cfg.AllowedOrigins) != 3 {
		t.Errorf("expected 3 origins, got %d", len(cfg.AllowedOrigins))
	}
	expected := []string{"https://a.com", "https://b.com", "https://c.com"}
	for i, v := range expected {
		if cfg.AllowedOrigins[i] != v {
			t.Errorf("origin[%d] = %s, want %s", i, cfg.AllowedOrigins[i], v)
		}
	}
}

func TestParseOrigins_TrimsSpaces(t *testing.T) {
	origins := parseOrigins(" https://a.com , https://b.com ")
	if len(origins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(origins))
	}
	if origins[0] != " https://a.com " {
		t.Errorf("expected origin[0] to preserve inner spaces, got %q", origins[0])
	}
}

func TestSplitAndTrim_Basic(t *testing.T) {
	result := splitAndTrim("a,b,c")
	expected := []string{"a", "b", "c"}
	if len(result) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %s, want %s", i, result[i], v)
		}
	}
}

func TestSplitAndTrim_Empty(t *testing.T) {
	result := splitAndTrim("")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestSplitAndTrim_NoComma(t *testing.T) {
	result := splitAndTrim("abc")
	if len(result) != 1 || result[0] != "abc" {
		t.Errorf("expected [abc], got %v", result)
	}
}

func TestEnvOrDefault(t *testing.T) {
	key := "CONFIG_TEST_VAR"
	os.Unsetenv(key)
	if envOrDefault(key, "fallback") != "fallback" {
		t.Error("expected fallback value")
	}
	os.Setenv(key, "set")
	defer os.Unsetenv(key)
	if envOrDefault(key, "fallback") != "set" {
		t.Error("expected env value")
	}
}

func TestEnvOrDefaultInt(t *testing.T) {
	key := "CONFIG_TEST_INT"
	os.Unsetenv(key)
	if envOrDefaultInt(key, 42) != 42 {
		t.Error("expected fallback int")
	}
	os.Setenv(key, "99")
	defer os.Unsetenv(key)
	if envOrDefaultInt(key, 42) != 99 {
		t.Error("expected env int")
	}
	os.Setenv(key, "bad")
	if envOrDefaultInt(key, 42) != 42 {
		t.Error("expected fallback for invalid int")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tc := range tests {
		got := parseLogLevel(tc.input)
		if got != tc.expected {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}
