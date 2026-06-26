// Package config loads and validates daemon configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the daemon configuration.
type Config struct {
	SoloHome                     string
	Listen                       string // "127.0.0.1:17612" or "unix:///path" or "pipe:///path"
	ServerID                     string
	RelayEnabled                 bool
	RelayEndpoint                string
	RelayPublicEndpoint          string
	RelayDisableControlKeepalive bool
	MCPEnabled                   bool
	MCPInjectIntoAgents          bool
	CORSOrigins                  []string
	Hostnames                    []string
	AppBaseURL                   string
	Supervised                   bool
	Version                      string
	CustomModels                 map[string][]CustomModelConfig    // key = provider ID
	ProviderSettings             map[string]ProviderSettingsConfig // key = provider ID
	LLMProviders                 []LLMProviderConfig               // user-configured LLM API providers
	TmuxAgentNames               []string                          // additional tmux agent names (merged with built-in defaults)
	TimelineMaxRowsPerAgent      int                               // hard upper bound for in-memory timeline rows per agent
	Memory                       MemoryConfig
}

// builtInTmuxAgentNames are always included in agent scanning.
var builtInTmuxAgentNames = []string{
	"claude", "opencode", "qodercli", "pi", "cursor", "kimi", "kimi-cli", "codex",
}

// GetTmuxAgentNames returns the merged set of built-in and user-configured tmux agent names.
func (c *Config) GetTmuxAgentNames() map[string]bool {
	names := make(map[string]bool, len(builtInTmuxAgentNames)+len(c.TmuxAgentNames))
	for _, n := range builtInTmuxAgentNames {
		names[n] = true
	}
	for _, n := range c.TmuxAgentNames {
		names[strings.ToLower(n)] = true
	}
	return names
}

// PersistedConfig mirrors the on-disk config.json structure.
type PersistedConfig struct {
	Daemon *DaemonConfig `json:"daemon,omitempty"`
	App    *AppConfig    `json:"app,omitempty"`
}

type DaemonConfig struct {
	Listen                  *string             `json:"listen,omitempty"`
	Hostnames               []string            `json:"hostnames,omitempty"`
	MCP                     *MCPConfig          `json:"mcp,omitempty"`
	CORS                    *CORSConfig         `json:"cors,omitempty"`
	Relay                   *RelayConfig        `json:"relay,omitempty"`
	Providers               *ProvidersConfig    `json:"providers,omitempty"`
	LLMProviders            []LLMProviderConfig `json:"llmProviders,omitempty"`
	TmuxAgentNames          []string            `json:"tmuxAgentNames,omitempty"`
	TimelineMaxRowsPerAgent *int                `json:"timelineMaxRowsPerAgent,omitempty"`
}

type ProvidersConfig struct {
	CustomModels     map[string][]CustomModelConfig    `json:"customModels,omitempty"`
	ProviderSettings map[string]ProviderSettingsConfig `json:"providerSettings,omitempty"`
}

type ProviderSettingsConfig struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// LLMProviderConfig holds a user-configured LLM API provider.
type LLMProviderConfig struct {
	ID          string           `json:"id"`
	Label       string           `json:"label,omitempty"`
	Description string           `json:"description,omitempty"`
	Enabled     *bool            `json:"enabled,omitempty"`
	BaseURL     string           `json:"baseURL,omitempty"`
	APIKey      string           `json:"apiKey,omitempty"`
	Models      []LLMModelConfig `json:"models,omitempty"`
}

// LLMModelConfig holds a model exposed by an LLMProviderConfig.
type LLMModelConfig struct {
	ID          string `json:"id"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	IsDefault   *bool  `json:"isDefault,omitempty"`
}

type MCPConfig struct {
	InjectIntoAgents *bool `json:"injectIntoAgents,omitempty"`
}

type CORSConfig struct {
	Origins []string `json:"origins,omitempty"`
}

type RelayConfig struct {
	Enabled                 *bool   `json:"enabled,omitempty"`
	Endpoint                *string `json:"endpoint,omitempty"`
	PublicEndpoint          *string `json:"publicEndpoint,omitempty"`
	DisableControlKeepalive *bool   `json:"disableControlKeepalive,omitempty"`
}

type AppConfig struct {
	BaseURL *string `json:"baseUrl,omitempty"`
}

type CustomModelConfig struct {
	ID                      string               `json:"id"`
	Label                   string               `json:"label,omitempty"`
	Description             string               `json:"description,omitempty"`
	IsDefault               *bool                `json:"isDefault,omitempty"`
	ThinkingOptions         []CustomSelectOption `json:"thinkingOptions,omitempty"`
	DefaultThinkingOptionID *string              `json:"defaultThinkingOptionId,omitempty"`
}

type CustomSelectOption struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	IsDefault *bool  `json:"isDefault,omitempty"`
}

// Version is the daemon version, injected at build time via -ldflags.
// Default "dev" is overridden by Makefile builds and CI.
var Version = "0.4.0"

// DefaultConfig returns a Config with sensible defaults.
// DefaultTimelineMaxRowsPerAgent is the default hard upper bound for
// in-memory timeline rows kept per agent. Older rows are dropped once the
// limit is exceeded.
const DefaultTimelineMaxRowsPerAgent = 10000

func DefaultConfig() *Config {
	return &Config{
		SoloHome:                "",
		Listen:                  "127.0.0.1:17612",
		ServerID:                "",
		RelayEnabled:            true,
		RelayEndpoint:           "relay.solo.sh:443",
		RelayPublicEndpoint:     "relay.solo.sh:443",
		MCPEnabled:              true,
		MCPInjectIntoAgents:     false,
		CORSOrigins:             []string{"https://solo.up2ai.top", "http://localhost:19000"},
		Hostnames:               nil,
		AppBaseURL:              "https://solo.up2ai.top",
		Supervised:              false,
		TimelineMaxRowsPerAgent: DefaultTimelineMaxRowsPerAgent,
		Version:                 Version,
	}
}

// Load resolves configuration from defaults, config file, and environment.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Resolve SOLO_HOME
	home := os.Getenv("SOLO_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot resolve home directory: %w", err)
		}
		home = filepath.Join(userHome, ".solo")
	}
	cfg.SoloHome = home

	// Load persisted config
	configPath := filepath.Join(home, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var pc PersistedConfig
		if err := json.Unmarshal(data, &pc); err == nil {
			applyPersistedConfig(cfg, &pc)
		}
	}

	// Overlay environment variables
	if v := os.Getenv("SOLO_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.Listen = "127.0.0.1:" + v
	}
	if v := os.Getenv("SOLO_RELAY_ENABLED"); v != "" {
		cfg.RelayEnabled = v != "false" && v != "0"
	}
	if v := os.Getenv("SOLO_RELAY_ENDPOINT"); v != "" {
		cfg.RelayEndpoint = v
	}
	if v := os.Getenv("SOLO_RELAY_PUBLIC_ENDPOINT"); v != "" {
		cfg.RelayPublicEndpoint = v
	}
	if v := os.Getenv("SOLO_RELAY_DISABLE_CONTROL_KEEPALIVE"); v != "" {
		cfg.RelayDisableControlKeepalive = v != "false" && v != "0"
	}
	if v := os.Getenv("SOLO_CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = splitAndTrim(v)
	}
	if v := os.Getenv("SOLO_HOSTNAMES"); v != "" {
		cfg.Hostnames = strings.Split(v, ",")
	}
	if v := os.Getenv("SOLO_APP_BASE_URL"); v != "" {
		cfg.AppBaseURL = v
	}
	if v := os.Getenv("SOLO_SUPERVISED"); v == "1" {
		cfg.Supervised = true
	}

	// Generate or load server ID
	if err := ensureServerID(cfg); err != nil {
		return nil, fmt.Errorf("cannot ensure server ID: %w", err)
	}

	return cfg, nil
}

func applyPersistedConfig(cfg *Config, pc *PersistedConfig) {
	if pc.Daemon != nil {
		if pc.Daemon.Listen != nil {
			cfg.Listen = *pc.Daemon.Listen
		}
		if len(pc.Daemon.Hostnames) > 0 {
			cfg.Hostnames = pc.Daemon.Hostnames
		}
		if pc.Daemon.MCP != nil {
			if pc.Daemon.MCP.InjectIntoAgents != nil {
				cfg.MCPInjectIntoAgents = *pc.Daemon.MCP.InjectIntoAgents
			}
		}
		if pc.Daemon.CORS != nil {
			if len(pc.Daemon.CORS.Origins) > 0 {
				cfg.CORSOrigins = pc.Daemon.CORS.Origins
			}
		}
		if pc.Daemon.Relay != nil {
			if pc.Daemon.Relay.Enabled != nil {
				cfg.RelayEnabled = *pc.Daemon.Relay.Enabled
			}
			if pc.Daemon.Relay.Endpoint != nil {
				cfg.RelayEndpoint = *pc.Daemon.Relay.Endpoint
			}
			if pc.Daemon.Relay.PublicEndpoint != nil {
				cfg.RelayPublicEndpoint = *pc.Daemon.Relay.PublicEndpoint
			}
			if pc.Daemon.Relay.DisableControlKeepalive != nil {
				cfg.RelayDisableControlKeepalive = *pc.Daemon.Relay.DisableControlKeepalive
			}
		}
		if pc.Daemon.Providers != nil {
			if len(pc.Daemon.Providers.CustomModels) > 0 {
				cfg.CustomModels = pc.Daemon.Providers.CustomModels
			}
			if len(pc.Daemon.Providers.ProviderSettings) > 0 {
				cfg.ProviderSettings = pc.Daemon.Providers.ProviderSettings
			}
		}
		if len(pc.Daemon.LLMProviders) > 0 {
			cfg.LLMProviders = pc.Daemon.LLMProviders
		}
		if len(pc.Daemon.TmuxAgentNames) > 0 {
			cfg.TmuxAgentNames = pc.Daemon.TmuxAgentNames
		}
		if pc.Daemon.TimelineMaxRowsPerAgent != nil && *pc.Daemon.TimelineMaxRowsPerAgent > 0 {
			cfg.TimelineMaxRowsPerAgent = *pc.Daemon.TimelineMaxRowsPerAgent
		}
	}
	if pc.App != nil {
		if pc.App.BaseURL != nil {
			cfg.AppBaseURL = *pc.App.BaseURL
		}
	}
}

func ensureServerID(cfg *Config) error {
	idPath := filepath.Join(cfg.SoloHome, "server-id")
	data, err := os.ReadFile(idPath)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		cfg.ServerID = strings.TrimSpace(string(data))
		return nil
	}

	// Generate new server ID
	if err := os.MkdirAll(cfg.SoloHome, 0755); err != nil {
		return err
	}
	id := generateID()
	cfg.ServerID = id
	return os.WriteFile(idPath, []byte(id), 0600)
}

// Save persists the current config to $SOLO_HOME/config.json.
// It reads the existing file first to preserve fields not managed by this method.
func (c *Config) Save() error {
	configPath := filepath.Join(c.SoloHome, "config.json")

	var pc PersistedConfig
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &pc)
	}

	if pc.Daemon == nil {
		pc.Daemon = &DaemonConfig{}
	}
	pc.Daemon.TmuxAgentNames = c.TmuxAgentNames

	if len(c.CustomModels) > 0 || len(c.ProviderSettings) > 0 {
		if pc.Daemon.Providers == nil {
			pc.Daemon.Providers = &ProvidersConfig{}
		}
		if len(c.CustomModels) > 0 {
			pc.Daemon.Providers.CustomModels = c.CustomModels
		}
		if len(c.ProviderSettings) > 0 {
			pc.Daemon.Providers.ProviderSettings = c.ProviderSettings
		}
	}
	if len(c.LLMProviders) > 0 {
		pc.Daemon.LLMProviders = c.LLMProviders
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(c.SoloHome, 0755); err != nil {
		return fmt.Errorf("create solo home: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

func generateID() string {
	// Simple random hex ID (8 chars), matching TS behavior
	f, _ := os.Open("/dev/urandom")
	if f == nil {
		return fmt.Sprintf("%08x", os.Getpid())
	}
	defer func() { _ = f.Close() }()
	b := make([]byte, 4)
	_, _ = f.Read(b)
	return fmt.Sprintf("%08x", b)
}

// ListenTarget returns the resolved listen address type.
type ListenTarget struct {
	Type string // "tcp", "socket", "pipe"
	Host string
	Port int
	Path string
}

// ResolveListenTarget parses the Listen string into a ListenTarget.
func (c *Config) ResolveListenTarget() (*ListenTarget, error) {
	addr := c.Listen
	if strings.HasPrefix(addr, "unix://") {
		return &ListenTarget{Type: "socket", Path: strings.TrimPrefix(addr, "unix://")}, nil
	}
	if strings.HasPrefix(addr, "pipe://") {
		return &ListenTarget{Type: "pipe", Path: strings.TrimPrefix(addr, "pipe://")}, nil
	}

	// TCP: host:port
	host, portStr, err := netSplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in listen address %q: %w", addr, err)
	}
	return &ListenTarget{Type: "tcp", Host: host, Port: port}, nil
}

func netSplitHostPort(addr string) (string, string, error) {
	// Handle [::1]:port
	if strings.HasPrefix(addr, "[") {
		closeBracket := strings.Index(addr, "]")
		if closeBracket < 0 {
			return "", "", fmt.Errorf("missing ] in address")
		}
		host := addr[:closeBracket+1]
		rest := addr[closeBracket+1:]
		if len(rest) == 0 || rest[0] != ':' {
			return "", "", fmt.Errorf("missing port in address")
		}
		return host, rest[1:], nil
	}
	lastColon := strings.LastIndex(addr, ":")
	if lastColon < 0 {
		return "", "", fmt.Errorf("missing port in address")
	}
	return addr[:lastColon], addr[lastColon+1:], nil
}

func splitAndTrim(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
