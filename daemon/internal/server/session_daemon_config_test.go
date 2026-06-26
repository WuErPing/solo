package server

import (
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/protocol"
)

func TestBuildDaemonConfigResponse_WithMcpAndTmuxAgentNames(t *testing.T) {
	cfg := &config.Config{
		MCPInjectIntoAgents: true,
		TmuxAgentNames:      []string{"aider"},
	}
	result := buildDaemonConfigResponse(cfg)

	mcp, ok := result["mcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'mcp' in config, got: %v", result)
	}
	if mcp["injectIntoAgents"] != true {
		t.Errorf("mcp.injectIntoAgents: got %v, want true", mcp["injectIntoAgents"])
	}

	names, ok := result["tmuxAgentNames"].([]string)
	if !ok {
		t.Fatalf("expected 'tmuxAgentNames' ([]string), got %T: %v", result["tmuxAgentNames"], result["tmuxAgentNames"])
	}
	if len(names) != 1 || names[0] != "aider" {
		t.Errorf("tmuxAgentNames: got %v, want [aider]", names)
	}
}

func TestBuildDaemonConfigResponse_DefaultMcp(t *testing.T) {
	cfg := &config.Config{
		// MCPInjectIntoAgents defaults to false
	}
	result := buildDaemonConfigResponse(cfg)

	mcp, ok := result["mcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'mcp' in config even when not explicitly set, got: %v", result)
	}
	if mcp["injectIntoAgents"] != false {
		t.Errorf("mcp.injectIntoAgents: got %v, want false", mcp["injectIntoAgents"])
	}
}

func TestBuildDaemonConfigResponse_EmptyTmuxAgentNames(t *testing.T) {
	cfg := &config.Config{}
	result := buildDaemonConfigResponse(cfg)

	names, ok := result["tmuxAgentNames"].([]string)
	if !ok {
		t.Fatalf("expected 'tmuxAgentNames' ([]string), got %T", result["tmuxAgentNames"])
	}
	if names != nil {
		t.Errorf("tmuxAgentNames: got %v, want nil", names)
	}
}

func TestBuildDaemonConfigResponse_LLMProviders(t *testing.T) {
	cfg := &config.Config{
		LLMProviders: []config.LLMProviderConfig{
			{
				ID:      "openai",
				Label:   "OpenAI",
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "sk-test",
				Models:  []config.LLMModelConfig{{ID: "gpt-5", Label: "GPT-5"}},
			},
		},
	}

	result := buildDaemonConfigResponse(cfg)

	providers, ok := result["llmProviders"].([]map[string]interface{})
	if !ok || len(providers) != 1 {
		t.Fatalf("expected 1 llm provider, got %v", result["llmProviders"])
	}
	if providers[0]["id"] != "openai" {
		t.Errorf("id: got %v, want openai", providers[0]["id"])
	}
	if providers[0]["apiKey"] != "sk-test" {
		t.Errorf("apiKey: got %v, want sk-test", providers[0]["apiKey"])
	}
	models, ok := providers[0]["models"].([]map[string]interface{})
	if !ok || len(models) != 1 || models[0]["id"] != "gpt-5" {
		t.Errorf("models: got %v", providers[0]["models"])
	}
}

func TestHandleSetDaemonConfig_LLMProviders(t *testing.T) {
	ws, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-set-llm")
	defer conn.Close()
	readInitialMessages(t, conn)

	patch := map[string]interface{}{
		"llmProviders": []map[string]interface{}{
			{
				"id":      "anthropic",
				"label":   "Anthropic",
				"baseURL": "https://api.anthropic.com/v1",
				"apiKey":  "sk-ant",
				"models": []map[string]interface{}{
					{"id": "claude-opus", "label": "Claude Opus"},
				},
			},
		},
	}

	conn.WriteJSON(protocol.WSInboundMessage{
		Type:    "session",
		Message: mustMarshal(map[string]interface{}{"type": "set_daemon_config_request", "requestId": "req-sllm-1", "config": patch}),
	})

	resp := readUntilType(t, conn, "set_daemon_config_response")
	payload := decodeSessionPayload[protocol.SetDaemonConfigResponsePayload](t, resp)
	if payload.RequestID != "req-sllm-1" {
		t.Fatalf("requestId: got %q, want req-sllm-1", payload.RequestID)
	}

	if len(ws.cfg.LLMProviders) != 1 {
		t.Fatalf("expected 1 LLM provider persisted, got %d", len(ws.cfg.LLMProviders))
	}
	p := ws.cfg.LLMProviders[0]
	if p.ID != "anthropic" || p.APIKey != "sk-ant" || len(p.Models) != 1 {
		t.Errorf("persisted provider mismatch: %+v", p)
	}

	providers, ok := payload.Config["llmProviders"].([]interface{})
	if !ok || len(providers) != 1 {
		t.Errorf("expected 1 provider in response, got %v", payload.Config["llmProviders"])
	}

	// Verify config file round-trip.
	t.Setenv("SOLO_HOME", ws.cfg.SoloHome)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(loaded.LLMProviders) != 1 || loaded.LLMProviders[0].ID != "anthropic" {
		t.Errorf("reloaded providers mismatch: %+v", loaded.LLMProviders)
	}
}

func TestBuildDaemonConfigResponse_Providers(t *testing.T) {
	cfg := &config.Config{
		CustomModels: map[string][]config.CustomModelConfig{
			"claude": {{ID: "custom-claude", Label: "Custom Claude"}},
		},
		ProviderSettings: map[string]config.ProviderSettingsConfig{
			"claude": {Enabled: boolPtr(true), Label: "Claude", Description: "Anthropic"},
		},
	}

	result := buildDaemonConfigResponse(cfg)

	providers, ok := result["providers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'providers' map, got %T", result["providers"])
	}
	claude, ok := providers["claude"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected providers.claude map, got %T", providers["claude"])
	}
	if claude["enabled"] != true {
		t.Errorf("claude.enabled: got %v, want true", claude["enabled"])
	}
	if claude["label"] != "Claude" {
		t.Errorf("claude.label: got %v, want Claude", claude["label"])
	}
	if claude["description"] != "Anthropic" {
		t.Errorf("claude.description: got %v, want Anthropic", claude["description"])
	}

	models, ok := claude["additionalModels"].([]map[string]interface{})
	if !ok || len(models) != 1 {
		t.Fatalf("expected 1 additionalModel, got %v", claude["additionalModels"])
	}
	if models[0]["id"] != "custom-claude" {
		t.Errorf("model id: got %v, want custom-claude", models[0]["id"])
	}
	if models[0]["label"] != "Custom Claude" {
		t.Errorf("model label: got %v, want Custom Claude", models[0]["label"])
	}
}

func TestBuildDaemonConfigResponse_CustomProviderWithoutModels(t *testing.T) {
	cfg := &config.Config{
		ProviderSettings: map[string]config.ProviderSettingsConfig{
			"custom-ai": {Enabled: boolPtr(true), Label: "Custom AI", Description: "My provider"},
		},
	}

	result := buildDaemonConfigResponse(cfg)

	providers, ok := result["providers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'providers' map, got %T", result["providers"])
	}
	custom, ok := providers["custom-ai"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected providers.custom-ai map, got %T", providers["custom-ai"])
	}
	if custom["enabled"] != true {
		t.Errorf("custom-ai.enabled: got %v, want true", custom["enabled"])
	}
	if custom["label"] != "Custom AI" {
		t.Errorf("custom-ai.label: got %v, want Custom AI", custom["label"])
	}
}

func TestHandleSetDaemonConfig_ProvidersAdditionalModels(t *testing.T) {
	ws, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-set-providers")
	defer conn.Close()
	readInitialMessages(t, conn)

	patch := map[string]interface{}{
		"providers": map[string]interface{}{
			"claude": map[string]interface{}{
				"additionalModels": []map[string]interface{}{
					{"id": "custom-1", "label": "Custom One"},
				},
			},
		},
	}

	conn.WriteJSON(protocol.WSInboundMessage{
		Type:    "session",
		Message: mustMarshal(map[string]interface{}{"type": "set_daemon_config_request", "requestId": "req-sp-1", "config": patch}),
	})

	resp := readUntilType(t, conn, "set_daemon_config_response")
	payload := decodeSessionPayload[protocol.SetDaemonConfigResponsePayload](t, resp)
	if payload.RequestID != "req-sp-1" {
		t.Fatalf("requestId: got %q, want req-sp-1", payload.RequestID)
	}

	providers, ok := payload.Config["providers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected providers in response, got %T", payload.Config["providers"])
	}
	claude, ok := providers["claude"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected claude config, got %T", providers["claude"])
	}
	models, ok := claude["additionalModels"].([]interface{})
	if !ok || len(models) != 1 {
		t.Fatalf("expected 1 additional model, got %v", claude["additionalModels"])
	}

	if ws.cfg.CustomModels["claude"][0].ID != "custom-1" {
		t.Errorf("persisted custom model id: got %v, want custom-1", ws.cfg.CustomModels["claude"][0].ID)
	}

	// A subsequent snapshot should include the custom model.
	entries := ws.registry.ToProviderSnapshotEntries()
	var claudeEntry *protocol.ProviderSnapshotEntry
	for i := range entries {
		if entries[i].Provider == "claude" {
			claudeEntry = &entries[i]
			break
		}
	}
	if claudeEntry == nil {
		t.Fatal("expected claude snapshot entry")
	}
	found := false
	for _, m := range claudeEntry.Models {
		if m.ID == "custom-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("custom model not in snapshot: %+v", claudeEntry.Models)
	}
}

func TestHandleSetDaemonConfig_ProvidersEnabledAndMetadata(t *testing.T) {
	ws, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-set-provider-meta")
	defer conn.Close()
	readInitialMessages(t, conn)

	patch := map[string]interface{}{
		"providers": map[string]interface{}{
			"custom-ai": map[string]interface{}{
				"enabled":     true,
				"label":       "Custom AI",
				"description": "Custom provider",
				"additionalModels": []map[string]interface{}{
					{"id": "model-a", "label": "Model A"},
				},
			},
		},
	}

	conn.WriteJSON(protocol.WSInboundMessage{
		Type:    "session",
		Message: mustMarshal(map[string]interface{}{"type": "set_daemon_config_request", "requestId": "req-sp-2", "config": patch}),
	})

	resp := readUntilType(t, conn, "set_daemon_config_response")
	payload := decodeSessionPayload[protocol.SetDaemonConfigResponsePayload](t, resp)
	if payload.RequestID != "req-sp-2" {
		t.Fatalf("requestId: got %q, want req-sp-2", payload.RequestID)
	}

	settings, ok := ws.cfg.ProviderSettings["custom-ai"]
	if !ok {
		t.Fatal("expected custom-ai provider settings to be persisted")
	}
	if settings.Enabled == nil || !*settings.Enabled {
		t.Errorf("expected custom-ai enabled")
	}
	if settings.Label != "Custom AI" {
		t.Errorf("label: got %q, want Custom AI", settings.Label)
	}
	if settings.Description != "Custom provider" {
		t.Errorf("description: got %q, want Custom provider", settings.Description)
	}

	_, exists := ws.cfg.CustomModels["custom-ai"]
	if !exists {
		t.Fatal("expected custom-ai custom models to be persisted")
	}

	// Verify config file round-trip.
	t.Setenv("SOLO_HOME", ws.cfg.SoloHome)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if loaded.ProviderSettings["custom-ai"].Label != "Custom AI" {
		t.Errorf("reloaded label mismatch: %+v", loaded.ProviderSettings["custom-ai"])
	}
	if len(loaded.CustomModels["custom-ai"]) != 1 {
		t.Errorf("reloaded custom models mismatch: %+v", loaded.CustomModels["custom-ai"])
	}
}
