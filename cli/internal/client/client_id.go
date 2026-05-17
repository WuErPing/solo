package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// GetOrCreateClientID returns a persistent client ID stored in ~/.solo/cli-client-id.
// If the file doesn't exist, a new ID is generated and persisted.
func GetOrCreateClientID() (string, error) {
	home := soloHome()
	path := filepath.Join(home, "cli-client-id")

	data, err := os.ReadFile(path)
	if err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id, nil
		}
	}

	id := "cid_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	if err := os.MkdirAll(home, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(id), 0600); err != nil {
		return "", err
	}
	return id, nil
}

func soloHome() string {
	if v := os.Getenv("SOLO_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".solo"
	}
	return filepath.Join(home, ".solo")
}

// SoloHome returns the path to the Solo home directory.
func SoloHome() string { return soloHome() }

// DaemonConfig holds the daemon configuration relevant for CLI commands.
type DaemonConfig struct {
	RelayEnabled        *bool
	RelayEndpoint       string
	RelayPublicEndpoint string
	AppBaseURL          string
}

// LoadDaemonConfig reads daemon config from ~/.solo/config.json.
func LoadDaemonConfig(home string) *DaemonConfig {
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		return nil
	}
	var cfg struct {
		Daemon *struct {
			Relay *struct {
				Enabled        *bool   `json:"enabled,omitempty"`
				Endpoint       *string `json:"endpoint,omitempty"`
				PublicEndpoint *string `json:"publicEndpoint,omitempty"`
			} `json:"relay,omitempty"`
		} `json:"daemon,omitempty"`
		App *struct {
			BaseURL *string `json:"baseUrl,omitempty"`
		} `json:"app,omitempty"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	result := &DaemonConfig{}
	if cfg.Daemon != nil && cfg.Daemon.Relay != nil {
		result.RelayEnabled = cfg.Daemon.Relay.Enabled
		if cfg.Daemon.Relay.Endpoint != nil {
			result.RelayEndpoint = *cfg.Daemon.Relay.Endpoint
		}
		if cfg.Daemon.Relay.PublicEndpoint != nil {
			result.RelayPublicEndpoint = *cfg.Daemon.Relay.PublicEndpoint
		}
	}
	if cfg.App != nil && cfg.App.BaseURL != nil {
		result.AppBaseURL = *cfg.App.BaseURL
	}
	return result
}
