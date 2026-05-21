package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

type mockAgentClient struct {
	providerName string
	available    bool
	models       []protocol.AgentModelDefinition
}

func (m *mockAgentClient) Provider() string { return m.providerName }
func (m *mockAgentClient) IsAvailable(ctx context.Context) error {
	if m.available {
		return nil
	}
	return errors.New("not available")
}
func (m *mockAgentClient) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error) {
	return nil, nil
}
func (m *mockAgentClient) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error) {
	return nil, nil
}
func (m *mockAgentClient) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	return m.models, nil
}
func (m *mockAgentClient) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	return nil, nil
}
func (m *mockAgentClient) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
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
			if e.Status != protocol.ProviderReady {
				t.Errorf("expected claude ready, got %q", e.Status)
			}
		}
		if e.Provider == "opencode" {
			if e.Status != protocol.ProviderUnavailable {
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
