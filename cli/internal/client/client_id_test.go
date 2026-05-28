package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSoloHome_EnvVar(t *testing.T) {
	os.Setenv("SOLO_HOME", "/custom/solo/home")
	defer os.Unsetenv("SOLO_HOME")
	if got := SoloHome(); got != "/custom/solo/home" {
		t.Errorf("expected /custom/solo/home, got %q", got)
	}
}

func TestSoloHome_Default(t *testing.T) {
	os.Unsetenv("SOLO_HOME")
	home := SoloHome()
	if home == "" {
		t.Error("expected non-empty home")
	}
	if !strings.HasSuffix(home, ".solo") {
		t.Errorf("expected path ending in .solo, got %q", home)
	}
}

func TestGetOrCreateClientID_CreatesNew(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	id, err := GetOrCreateClientID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty client ID")
	}
	if !strings.HasPrefix(id, "cid_") {
		t.Errorf("expected id to start with cid_, got %q", id)
	}

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(home, "cli-client-id"))
	if err != nil {
		t.Fatalf("client ID file not written: %v", err)
	}
	if strings.TrimSpace(string(data)) != id {
		t.Errorf("file content mismatch: expected %q, got %q", id, string(data))
	}
}

func TestGetOrCreateClientID_ReusesExisting(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	// Pre-create an ID
	existingID := "cid_existing123"
	_ = os.WriteFile(filepath.Join(home, "cli-client-id"), []byte(existingID+"\n"), 0600)

	id, err := GetOrCreateClientID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != existingID {
		t.Errorf("expected %q, got %q", existingID, id)
	}
}

func TestLoadDaemonConfig_Missing(t *testing.T) {
	home := t.TempDir()
	cfg := LoadDaemonConfig(home)
	if cfg != nil {
		t.Error("expected nil for missing config")
	}
}

func TestLoadDaemonConfig_InvalidJSON(t *testing.T) {
	home := t.TempDir()
	_ = os.WriteFile(filepath.Join(home, "config.json"), []byte("bad json"), 0644)
	cfg := LoadDaemonConfig(home)
	if cfg != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestLoadDaemonConfig_RelayAndApp(t *testing.T) {
	home := t.TempDir()
	enabled := true
	endpoint := "wss://relay.example.com"
	publicEndpoint := "https://relay.example.com"
	baseURL := "https://app.example.com"

	cfgData := map[string]interface{}{
		"daemon": map[string]interface{}{
			"relay": map[string]interface{}{
				"enabled":        enabled,
				"endpoint":       endpoint,
				"publicEndpoint": publicEndpoint,
			},
		},
		"app": map[string]interface{}{
			"baseUrl": baseURL,
		},
	}
	data, _ := json.Marshal(cfgData)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	cfg := LoadDaemonConfig(home)
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.RelayEnabled == nil || !*cfg.RelayEnabled {
		t.Error("expected RelayEnabled true")
	}
	if cfg.RelayEndpoint != endpoint {
		t.Errorf("expected RelayEndpoint %q, got %q", endpoint, cfg.RelayEndpoint)
	}
	if cfg.RelayPublicEndpoint != publicEndpoint {
		t.Errorf("expected RelayPublicEndpoint %q, got %q", publicEndpoint, cfg.RelayPublicEndpoint)
	}
	if cfg.AppBaseURL != baseURL {
		t.Errorf("expected AppBaseURL %q, got %q", baseURL, cfg.AppBaseURL)
	}
}

func TestLoadDaemonConfig_PartialRelay(t *testing.T) {
	home := t.TempDir()
	cfgData := map[string]interface{}{
		"daemon": map[string]interface{}{
			"relay": map[string]interface{}{
				"enabled": false,
			},
		},
	}
	data, _ := json.Marshal(cfgData)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	cfg := LoadDaemonConfig(home)
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.RelayEnabled == nil || *cfg.RelayEnabled {
		t.Error("expected RelayEnabled false")
	}
	if cfg.RelayEndpoint != "" {
		t.Errorf("expected empty RelayEndpoint, got %q", cfg.RelayEndpoint)
	}
}

func TestLoadDaemonConfig_NoRelay(t *testing.T) {
	home := t.TempDir()
	cfgData := map[string]interface{}{
		"daemon": map[string]interface{}{},
	}
	data, _ := json.Marshal(cfgData)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	cfg := LoadDaemonConfig(home)
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.RelayEnabled != nil {
		t.Error("expected RelayEnabled nil when relay section missing")
	}
}
