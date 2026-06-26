package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/protocol"
)

type mockAgentClient struct {
	providerName string
	available    bool
	models       []protocol.AgentModelDefinition
}

func (m *mockAgentClient) Provider() string { return m.providerName }
func (m *mockAgentClient) IsAvailable(_ context.Context) error {
	if m.available {
		return nil
	}
	return errors.New("not available")
}
func (m *mockAgentClient) CreateSession(_ context.Context, _ *protocol.AgentSessionConfig) (AgentSession, error) {
	return nil, nil
}
func (m *mockAgentClient) ResumeSession(_ context.Context, _ *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return nil, nil
}
func (m *mockAgentClient) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return m.models, nil
}
func (m *mockAgentClient) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (m *mockAgentClient) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

func TestProviderRegistry_RegisterAndGet(t *testing.T) {
	r := NewProviderRegistry()
	client := &mockAgentClient{providerName: "claude", available: true}
	r.Register(client)

	got, err := r.Get("claude")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Provider() != "claude" {
		t.Errorf("expected claude, got %q", got.Provider())
	}

	_, err = r.Get("missing")
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestProviderRegistry_List(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockAgentClient{providerName: "claude"})
	r.Register(&mockAgentClient{providerName: "opencode"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(list))
	}
}

func TestProviderRegistry_ListAvailable(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockAgentClient{providerName: "claude", available: true})
	r.Register(&mockAgentClient{providerName: "opencode", available: false})

	avail := r.ListAvailable()
	if avail["claude"] != nil {
		t.Error("expected claude to be available")
	}
	if avail["opencode"] == nil {
		t.Error("expected opencode to have error")
	}
}

func TestProviderRegistry_SetCustomModels(t *testing.T) {
	r := NewProviderRegistry()
	custom := map[string][]protocol.AgentModelDefinition{
		"claude": {{ID: "custom1", Label: "Custom"}},
	}
	r.SetCustomModels(custom)

	entries := r.ToProviderSnapshotEntries()
	var claudeEntry *protocol.ProviderSnapshotEntry
	for i := range entries {
		if entries[i].Provider == "claude" {
			claudeEntry = &entries[i]
			break
		}
	}
	if claudeEntry == nil {
		t.Fatal("expected claude entry")
	}

	foundCustom := false
	for _, m := range claudeEntry.Models {
		if m.ID == "custom1" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Error("expected custom model to be merged")
	}
}

func TestProviderRegistry_ToProviderSnapshotEntries(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockAgentClient{providerName: "claude", available: true})

	entries := r.ToProviderSnapshotEntries()
	if len(entries) == 0 {
		t.Fatal("expected entries")
	}

	foundClaude := false
	for _, e := range entries {
		if e.Provider == "claude" {
			foundClaude = true
			if e.Status != "ready" {
				t.Errorf("expected claude ready, got %q", e.Status)
			}
		}
		if e.Provider == "opencode" {
			if e.Status != "unavailable" {
				t.Errorf("expected opencode unavailable, got %q", e.Status)
			}
		}
	}
	if !foundClaude {
		t.Error("expected claude in entries")
	}
}

func TestGetProviderDefinition(t *testing.T) {
	def, err := GetProviderDefinition("claude")
	if err != nil {
		t.Fatalf("GetProviderDefinition: %v", err)
	}
	if def.ID != "claude" {
		t.Errorf("expected claude, got %q", def.ID)
	}
	if len(def.Modes) == 0 {
		t.Error("expected modes for claude")
	}

	_, err = GetProviderDefinition("unknown")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestBuiltinProviderDefinitions(t *testing.T) {
	defs := BuiltinProviderDefinitions()
	if len(defs) == 0 {
		t.Fatal("expected builtin definitions")
	}

	ids := make(map[string]bool)
	for _, d := range defs {
		ids[d.ID] = true
	}
	expected := []string{"claude", "codex", "opencode", "kimi"}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("expected builtin provider %q", id)
		}
	}
}

func TestProviderRegistry_CustomProvidersInSnapshot(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockAgentClient{providerName: "claude", available: true})
	r.SetCustomModels(map[string][]protocol.AgentModelDefinition{
		"custom-ai": {{ID: "model-a", Label: "Model A"}},
	})
	r.SetProviderSettings(map[string]config.ProviderSettingsConfig{
		"custom-ai": {Enabled: boolPtr(true), Label: "Custom AI", Description: "My provider"},
	})

	entries := r.ToProviderSnapshotEntries()

	var custom *protocol.ProviderSnapshotEntry
	for i := range entries {
		if entries[i].Provider == "custom-ai" {
			custom = &entries[i]
			break
		}
	}
	if custom == nil {
		t.Fatal("expected custom-ai snapshot entry")
	}
	if custom.Label != "Custom AI" {
		t.Errorf("label: got %q, want Custom AI", custom.Label)
	}
	if custom.Description != "My provider" {
		t.Errorf("description: got %q, want My provider", custom.Description)
	}
	if custom.Status != "unavailable" {
		t.Errorf("status: got %q, want unavailable", custom.Status)
	}
	if len(custom.Models) != 1 || custom.Models[0].ID != "model-a" {
		t.Errorf("models: got %+v", custom.Models)
	}
}
