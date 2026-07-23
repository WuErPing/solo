package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/protocol"
)

// ProviderRegistry manages available agent providers.
type ProviderRegistry struct {
	mu               sync.RWMutex
	clients          map[string]AgentClient
	customModels     map[string][]protocol.AgentModelDefinition
	providerSettings map[string]config.ProviderSettingsConfig
}

// NewProviderRegistry creates a new empty registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		clients:          make(map[string]AgentClient),
		customModels:     make(map[string][]protocol.AgentModelDefinition),
		providerSettings: make(map[string]config.ProviderSettingsConfig),
	}
}

// SetCustomModels sets user-defined custom models per provider.
func (r *ProviderRegistry) SetCustomModels(models map[string][]protocol.AgentModelDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customModels = models
}

// SetProviderSettings sets user-defined metadata and enabled flag per provider.
func (r *ProviderRegistry) SetProviderSettings(settings map[string]config.ProviderSettingsConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerSettings = settings
}

// Register adds a provider client to the registry.
func (r *ProviderRegistry) Register(client AgentClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[client.Provider()] = client
}

// Get returns a provider client by name.
func (r *ProviderRegistry) Get(provider string) (AgentClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[provider]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", provider)
	}
	return c, nil
}

// List returns all registered provider names.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	return names
}

// ListAvailable returns providers that are currently available.
func (r *ProviderRegistry) ListAvailable() map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]error, len(r.clients))
	for name, client := range r.clients {
		result[name] = client.IsAvailable(nil) //nolint:staticcheck // TODO: pass context
	}
	return result
}

// ProviderManifest contains the static definitions for all known providers.
type ProviderDefinition struct {
	ID            string
	Label         string
	Description   string
	DefaultModeID *string
	Modes         []ProviderModeDefinition
	Models        []ProviderModelDefinition
	Voice         *ProviderVoiceConfig
}

type ProviderModeDefinition struct {
	ID          string
	Label       string
	Description string
	Icon        string
	ColorTier   string
}

type ProviderModelDefinition struct {
	ID          string
	Label       string
	Description string
	IsDefault   bool
}

type ProviderVoiceConfig struct {
	Enabled       bool
	DefaultModeID string
	DefaultModel  string
}

// BuiltinProviderDefinitions returns the static definitions for built-in providers.
func BuiltinProviderDefinitions() []ProviderDefinition {
	return []ProviderDefinition{
		{
			ID: "claude", Label: "Claude", Description: "Anthropic Claude Code",
			DefaultModeID: strPtr("default"),
			Modes: []ProviderModeDefinition{
				{ID: "default", Label: "Default", Description: "Standard mode", Icon: "ShieldCheck", ColorTier: "safe"},
				{ID: "acceptEdits", Label: "Accept Edits", Description: "Auto-accept file edits", Icon: "ShieldAlert", ColorTier: "moderate"},
				{ID: "plan", Label: "Plan", Description: "Planning mode", Icon: "ShieldCheck", ColorTier: "planning"},
				{ID: "bypassPermissions", Label: "Bypass Permissions", Description: "Skip all permission prompts", Icon: "ShieldOff", ColorTier: "dangerous"},
			},
			Models: []ProviderModelDefinition{
				{ID: "auto", Label: "Auto", Description: "Use Claude's default model", IsDefault: true},
				{ID: "claude-sonnet-4-6", Label: "Sonnet 4.6", Description: "Fast and capable"},
				{ID: "claude-opus-4-7", Label: "Opus 4.7", Description: "Most capable"},
				{ID: "claude-haiku-4-5", Label: "Haiku 4.5", Description: "Fastest"},
			},
			Voice: &ProviderVoiceConfig{Enabled: true, DefaultModeID: "default", DefaultModel: "haiku"},
		},
		{
			ID: "codex", Label: "Codex", Description: "OpenAI Codex",
			DefaultModeID: strPtr("auto"),
			Modes: []ProviderModeDefinition{
				{ID: "auto", Label: "Auto", Description: "Suggested mode", Icon: "ShieldAlert", ColorTier: "moderate"},
				{ID: "full-access", Label: "Full Access", Description: "Full system access", Icon: "ShieldOff", ColorTier: "dangerous"},
			},
			Voice: &ProviderVoiceConfig{Enabled: true, DefaultModeID: "auto", DefaultModel: "gpt-5.4-mini"},
		},
		{
			ID: "opencode", Label: "OpenCode", Description: "OpenCode Agent",
			DefaultModeID: strPtr("build"),
			Modes: []ProviderModeDefinition{
				{ID: "build", Label: "Build", Description: "Build mode", Icon: "ShieldAlert", ColorTier: "moderate"},
				{ID: "plan", Label: "Plan", Description: "Planning mode", Icon: "ShieldCheck", ColorTier: "planning"},
			},
			Voice: &ProviderVoiceConfig{Enabled: true, DefaultModeID: "build"},
		},
		{
			ID: "kimi", Label: "Kimi", Description: "Moonshot AI Kimi",
			DefaultModeID: strPtr("default"),
			Modes: []ProviderModeDefinition{
				{ID: "default", Label: "Default", Description: "Standard mode", Icon: "ShieldCheck", ColorTier: "safe"},
				{ID: "bypassPermissions", Label: "Bypass Permissions", Description: "Skip all permission prompts", Icon: "ShieldOff", ColorTier: "dangerous"},
				{ID: "plan", Label: "Plan", Description: "Planning mode", Icon: "ShieldCheck", ColorTier: "planning"},
			},
		},
		{
			ID: "pi", Label: "Pi", Description: "Minimal terminal coding harness",
			DefaultModeID: strPtr("default"),
			Modes: []ProviderModeDefinition{
				{ID: "default", Label: "Default", Description: "Standard mode", Icon: "ShieldCheck", ColorTier: "safe"},
				{ID: "readOnly", Label: "Read Only", Description: "Read-only mode with no file modifications", Icon: "ShieldAlert", ColorTier: "moderate"},
			},
		},
	}
}

// GetProviderDefinition returns the definition for a given provider.
func GetProviderDefinition(provider string) (*ProviderDefinition, error) {
	for _, def := range BuiltinProviderDefinitions() {
		if def.ID == provider {
			return &def, nil
		}
	}
	return nil, fmt.Errorf("unknown provider: %s", provider)
}

// ToProviderSnapshotEntries converts provider definitions + registry to snapshot entries.
func (r *ProviderRegistry) ToProviderSnapshotEntries() []protocol.ProviderSnapshotEntry { //nolint:gocyclo // grandfathered CC=29
	defs := BuiltinProviderDefinitions()
	avail := r.ListAvailable()
	entries := make([]protocol.ProviderSnapshotEntry, 0, len(defs))

	r.mu.RLock()
	settings := r.providerSettings
	customModels := r.customModels
	r.mu.RUnlock()

	seen := make(map[string]bool, len(defs))
	for _, def := range defs {
		seen[def.ID] = true
		entry := protocol.ProviderSnapshotEntry{
			Provider:      def.ID,
			Status:        "ready",
			Enabled:       true,
			Label:         def.Label,
			Description:   def.Description,
			DefaultModeID: def.DefaultModeID,
		}

		if err, ok := avail[def.ID]; ok {
			if err != nil {
				entry.Status = "unavailable"
				entry.Error = err.Error()
			}
		} else {
			entry.Status = "unavailable"
			entry.Error = "not registered"
		}

		if s, ok := settings[def.ID]; ok {
			if s.Enabled != nil {
				entry.Enabled = *s.Enabled
			}
			if s.Label != "" {
				entry.Label = s.Label
			}
			if s.Description != "" {
				entry.Description = s.Description
			}
		}

		// Convert modes
		for _, m := range def.Modes {
			entry.Modes = append(entry.Modes, protocol.AgentMode{
				ID:          m.ID,
				Label:       m.Label,
				Description: m.Description,
				Icon:        m.Icon,
				ColorTier:   m.ColorTier,
			})
		}

		// Build model list: start with static definitions
		models := make([]protocol.AgentModelDefinition, 0, len(def.Models))
		for _, m := range def.Models {
			models = append(models, protocol.AgentModelDefinition{
				Provider:    def.ID,
				ID:          m.ID,
				Label:       m.Label,
				Description: m.Description,
				IsDefault:   m.IsDefault,
			})
		}

		// For providers with no static models (e.g. OpenCode), fetch live models
		if len(models) == 0 && entry.Status == "ready" {
			r.mu.RLock()
			client, clientOK := r.clients[def.ID]
			r.mu.RUnlock()
			if clientOK {
				if liveModels, err := client.ListModels(context.TODO(), ""); err == nil && len(liveModels) > 0 {
					models = liveModels
				}
			}
		}

		// Merge custom models: override-by-ID or append
		if custom := customModels[def.ID]; len(custom) > 0 {
			existing := make(map[string]int, len(models))
			for i, m := range models {
				existing[m.ID] = i
			}
			for _, cm := range custom {
				if idx, ok := existing[cm.ID]; ok {
					models[idx] = cm
				} else {
					models = append(models, cm)
				}
			}
		}

		entry.Models = models

		entries = append(entries, entry)
	}

	// Append custom providers that are not built-in.
	for providerID := range customModels {
		if seen[providerID] {
			continue
		}
		seen[providerID] = true
		s := settings[providerID]
		entry := protocol.ProviderSnapshotEntry{
			Provider: providerID,
			Status:   "unavailable",
			Enabled:  s.Enabled == nil || *s.Enabled,
			Label:    s.Label,
			Models:   customModels[providerID],
		}
		if entry.Label == "" {
			entry.Label = providerID
		}
		if s.Description != "" {
			entry.Description = s.Description
		}
		entries = append(entries, entry)
	}
	for providerID := range settings {
		if seen[providerID] {
			continue
		}
		seen[providerID] = true
		s := settings[providerID]
		entry := protocol.ProviderSnapshotEntry{
			Provider: providerID,
			Status:   "unavailable",
			Enabled:  s.Enabled == nil || *s.Enabled,
			Label:    s.Label,
		}
		if entry.Label == "" {
			entry.Label = providerID
		}
		if s.Description != "" {
			entry.Description = s.Description
		}
		entries = append(entries, entry)
	}

	return entries
}

func strPtr(s string) *string { return &s }
